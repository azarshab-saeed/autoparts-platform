package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxImportRows = 2000

type ImportRow struct {
	RowNumber        int     `json:"row_number"`
	SKU              string  `json:"sku,omitempty"`
	Title            string  `json:"title"`
	Brand            string  `json:"brand,omitempty"`
	OEMCode          string  `json:"oem_code,omitempty"`
	Barcode          string  `json:"barcode,omitempty"`
	Unit             string  `json:"unit,omitempty"`
	OnHand           float64 `json:"on_hand"`
	AvgUnitCost      int64   `json:"avg_unit_cost"`
	SellingPrice     int64   `json:"selling_price"`
	Visible          bool    `json:"visible"`
	AllowReservation bool    `json:"allow_reservation"`
	AllowProcurement bool    `json:"allow_procurement"`
}

type ImportCommand struct {
	TenantID       uuid.UUID
	StoreID        uuid.UUID
	WarehouseID    uuid.UUID
	ActorUserID    uuid.UUID
	IdempotencyKey string
	Rows           []ImportRow
}

type ImportRowResult struct {
	RowNumber       int       `json:"row_number"`
	ProductID       uuid.UUID `json:"product_id"`
	ProductAction   string    `json:"product_action"`
	InventoryAction string    `json:"inventory_action"`
	OfferAction     string    `json:"offer_action"`
	Note            string    `json:"note,omitempty"`
}

type ImportResult struct {
	BatchID                   uuid.UUID         `json:"batch_id"`
	RowCount                  int               `json:"row_count"`
	CreatedCount              int               `json:"created_count"`
	UpdatedCount              int               `json:"updated_count"`
	InventoryInitializedCount int               `json:"inventory_initialized_count"`
	InventoryPreservedCount   int               `json:"inventory_preserved_count"`
	OffersUpsertedCount       int               `json:"offers_upserted_count"`
	OpeningInventoryValue     int64             `json:"opening_inventory_value"`
	Rows                      []ImportRowResult `json:"rows"`
}

type ImportRowError struct {
	RowNumber int    `json:"row_number"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

type ImportValidationError struct {
	Rows []ImportRowError `json:"rows"`
}

func (e *ImportValidationError) Error() string {
	if len(e.Rows) == 0 {
		return "import validation failed"
	}
	return fmt.Sprintf("row %d: %s", e.Rows[0].RowNumber, e.Rows[0].Message)
}

func (s *Service) Import(ctx context.Context, cmd ImportCommand) (ImportResult, error) {
	if cmd.TenantID == uuid.Nil || cmd.StoreID == uuid.Nil || cmd.WarehouseID == uuid.Nil || cmd.ActorUserID == uuid.Nil {
		return ImportResult{}, errors.New("authenticated store, warehouse and user are required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return ImportResult{}, errors.New("idempotency key is required")
	}
	rows, validationErr := normalizeImportRows(cmd.Rows)
	if validationErr != nil {
		return ImportResult{}, validationErr
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback(ctx)

	if existing, found, err := loadImportByKey(ctx, tx, cmd.TenantID, cmd.IdempotencyKey); err != nil {
		return ImportResult{}, err
	} else if found {
		if err := tx.Commit(ctx); err != nil {
			return ImportResult{}, err
		}
		return existing, nil
	}

	var warehouseOK bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, cmd.WarehouseID, cmd.TenantID, cmd.StoreID).Scan(&warehouseOK); err != nil {
		return ImportResult{}, err
	}
	if !warehouseOK {
		return ImportResult{}, errors.New("destination warehouse does not belong to authenticated store")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active) SELECT $1,$2,'retail','خرده / مصرف‌کننده',true,true WHERE NOT EXISTS (SELECT 1 FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active)`, cmd.TenantID, cmd.StoreID); err != nil {
		return ImportResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO store_pricing_settings(tenant_id,store_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, cmd.TenantID, cmd.StoreID); err != nil {
		return ImportResult{}, err
	}
	var defaultPriceListID uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active ORDER BY created_at LIMIT 1`, cmd.TenantID, cmd.StoreID).Scan(&defaultPriceListID); err != nil {
		return ImportResult{}, err
	}

	out := ImportResult{BatchID: uuid.New(), RowCount: len(rows), Rows: make([]ImportRowResult, 0, len(rows))}

	for _, row := range rows {
		productID, existed, err := findImportProduct(ctx, tx, cmd.TenantID, row)
		if err != nil {
			return ImportResult{}, err
		}
		productAction := "updated"
		if existed {
			_, err = tx.Exec(ctx, `
				UPDATE products
				SET sku=COALESCE($3,sku),title=$4,brand=COALESCE($5,brand),oem_code=COALESCE($6,oem_code),
				    barcode=COALESCE($7,barcode),unit=COALESCE(NULLIF($8,''),unit),
				    normalized_title=lower(trim($4)),active=true,deleted_at=NULL,updated_at=now()
				WHERE id=$1 AND tenant_id=$2`, productID, cmd.TenantID,
				nullable(row.SKU), row.Title, nullable(row.Brand), nullable(row.OEMCode), nullable(row.Barcode), row.Unit)
			if err != nil {
				return ImportResult{}, err
			}
			out.UpdatedCount++
		} else {
			productID = uuid.New()
			_, err = tx.Exec(ctx, `
				INSERT INTO products(id,tenant_id,sku,title,brand,oem_code,barcode,unit,normalized_title,active)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,lower(trim($4)),true)`,
				productID, cmd.TenantID, nullable(row.SKU), row.Title, nullable(row.Brand), nullable(row.OEMCode), nullable(row.Barcode), unitOrPCS(row.Unit))
			if err != nil {
				return ImportResult{}, err
			}
			productAction = "created"
			out.CreatedCount++
		}

		inventoryAction, note, openingValue, err := initializeImportInventory(ctx, tx, cmd, out.BatchID, row, productID)
		if err != nil {
			return ImportResult{}, err
		}
		if inventoryAction == "initialized" {
			out.InventoryInitializedCount++
			out.OpeningInventoryValue += openingValue
		} else if inventoryAction == "preserved" {
			out.InventoryPreservedCount++
		}

		offerAction := "none"
		if row.SellingPrice > 0 {
			_, err = tx.Exec(ctx, `
				INSERT INTO store_product_offers(tenant_id,store_id,warehouse_id,product_id,selling_price,visible,allow_reservation,allow_procurement,last_verified_at)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
				ON CONFLICT(tenant_id,store_id,warehouse_id,product_id)
				DO UPDATE SET selling_price=EXCLUDED.selling_price,visible=EXCLUDED.visible,
				              allow_reservation=EXCLUDED.allow_reservation,allow_procurement=EXCLUDED.allow_procurement,
				              last_verified_at=now(),updated_at=now()`,
				cmd.TenantID, cmd.StoreID, cmd.WarehouseID, productID, row.SellingPrice, row.Visible, row.AllowReservation, row.AllowProcurement)
			if err != nil {
				return ImportResult{}, err
			}
			if _, err = tx.Exec(ctx, `
				INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price)
				VALUES($1,$2,$3,$4,1,$5)
				ON CONFLICT(tenant_id,store_id,product_id,price_list_id,min_qty)
				DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now()`, cmd.TenantID, cmd.StoreID, productID, defaultPriceListID, row.SellingPrice); err != nil {
				return ImportResult{}, err
			}
			offerAction = "upserted"
			out.OffersUpsertedCount++
		}

		out.Rows = append(out.Rows, ImportRowResult{
			RowNumber: row.RowNumber, ProductID: productID, ProductAction: productAction,
			InventoryAction: inventoryAction, OfferAction: offerAction, Note: note,
		})
	}

	if out.OpeningInventoryValue > 0 {
		if err := postOpeningInventoryJournal(ctx, tx, cmd.TenantID, out.BatchID, out.OpeningInventoryValue); err != nil {
			return ImportResult{}, err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO catalog_import_batches(
		  id,tenant_id,store_id,warehouse_id,requested_by_user_id,idempotency_key,row_count,
		  created_count,updated_count,inventory_initialized_count,inventory_preserved_count,
		  offers_upserted_count,opening_inventory_value,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'completed')`,
		out.BatchID, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, cmd.ActorUserID, cmd.IdempotencyKey, out.RowCount,
		out.CreatedCount, out.UpdatedCount, out.InventoryInitializedCount, out.InventoryPreservedCount,
		out.OffersUpsertedCount, out.OpeningInventoryValue)
	if err != nil {
		return ImportResult{}, err
	}
	for _, r := range out.Rows {
		_, err = tx.Exec(ctx, `
			INSERT INTO catalog_import_row_results(batch_id,row_number,product_id,product_action,inventory_action,offer_action,note)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, out.BatchID, r.RowNumber, r.ProductID, r.ProductAction, r.InventoryAction, r.OfferAction, nullable(r.Note))
		if err != nil {
			return ImportResult{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"batch_id": out.BatchID, "row_count": out.RowCount, "created_count": out.CreatedCount,
		"updated_count": out.UpdatedCount, "inventory_initialized_count": out.InventoryInitializedCount,
		"inventory_preserved_count": out.InventoryPreservedCount, "offers_upserted_count": out.OffersUpsertedCount,
	})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'catalog_import',$2,'catalog.import.completed',$3)`, cmd.TenantID, out.BatchID, payload); err != nil {
		return ImportResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ImportResult{}, err
	}
	return out, nil
}

func normalizeImportRows(in []ImportRow) ([]ImportRow, error) {
	if len(in) == 0 {
		return nil, &ImportValidationError{Rows: []ImportRowError{{RowNumber: 1, Field: "rows", Message: "حداقل یک ردیف برای ورود لازم است"}}}
	}
	if len(in) > maxImportRows {
		return nil, &ImportValidationError{Rows: []ImportRowError{{RowNumber: 1, Field: "rows", Message: fmt.Sprintf("حداکثر %d ردیف در هر ورود مجاز است", maxImportRows)}}}
	}
	out := make([]ImportRow, len(in))
	seenSKU := map[string]int{}
	seenBarcode := map[string]int{}
	seenFallback := map[string]int{}
	errs := make([]ImportRowError, 0)
	for i, row := range in {
		row.RowNumber = row.RowNumber
		if row.RowNumber <= 0 {
			row.RowNumber = i + 2
		}
		row.Title = strings.TrimSpace(row.Title)
		row.SKU = strings.TrimSpace(row.SKU)
		row.Brand = strings.TrimSpace(row.Brand)
		row.OEMCode = strings.TrimSpace(row.OEMCode)
		row.Barcode = strings.TrimSpace(row.Barcode)
		row.Unit = strings.TrimSpace(row.Unit)
		if row.Title == "" {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "title", Message: "نام کالا الزامی است"})
		}
		if len(row.Title) > 300 {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "title", Message: "نام کالا بیش از حد طولانی است"})
		}
		if !isFinite(row.OnHand) || row.OnHand < 0 {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "on_hand", Message: "موجودی باید عددی نامنفی باشد"})
		}
		if row.AvgUnitCost < 0 {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "avg_unit_cost", Message: "قیمت خرید نمی‌تواند منفی باشد"})
		}
		if row.SellingPrice < 0 {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "selling_price", Message: "قیمت فروش نمی‌تواند منفی باشد"})
		}
		if row.OnHand > 0 && row.AvgUnitCost == 0 {
			errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "avg_unit_cost", Message: "برای موجودی اولیه، قیمت خرید میانگین را وارد کنید"})
		}
		if row.SKU != "" {
			k := strings.ToLower(row.SKU)
			if first, ok := seenSKU[k]; ok {
				errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "sku", Message: fmt.Sprintf("کد کالا در همین فایل تکراری است (ردیف %d)", first)})
			} else {
				seenSKU[k] = row.RowNumber
			}
		}
		if row.Barcode != "" {
			k := strings.ToLower(row.Barcode)
			if first, ok := seenBarcode[k]; ok {
				errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "barcode", Message: fmt.Sprintf("بارکد در همین فایل تکراری است (ردیف %d)", first)})
			} else {
				seenBarcode[k] = row.RowNumber
			}
		}
		if row.SKU == "" && row.Barcode == "" {
			k := normalizeCatalogSearch(row.Title) + "|" + strings.ToLower(row.OEMCode)
			if first, ok := seenFallback[k]; ok {
				errs = append(errs, ImportRowError{RowNumber: row.RowNumber, Field: "title", Message: fmt.Sprintf("کالای بدون SKU/بارکد در همین فایل تکراری است (ردیف %d)", first)})
			} else {
				seenFallback[k] = row.RowNumber
			}
		}
		out[i] = row
	}
	if len(errs) > 0 {
		return nil, &ImportValidationError{Rows: errs}
	}
	return out, nil
}

func findImportProduct(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, row ImportRow) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var err error
	switch {
	case row.SKU != "":
		err = tx.QueryRow(ctx, `SELECT id FROM products WHERE tenant_id=$1 AND lower(sku)=lower($2) ORDER BY (sku=$2) DESC,created_at DESC LIMIT 1`, tenantID, row.SKU).Scan(&id)
	case row.Barcode != "":
		err = tx.QueryRow(ctx, `SELECT id FROM products WHERE tenant_id=$1 AND barcode=$2 AND deleted_at IS NULL LIMIT 1`, tenantID, row.Barcode).Scan(&id)
	default:
		err = tx.QueryRow(ctx, `SELECT id FROM products WHERE tenant_id=$1 AND lower(trim(title))=lower(trim($2)) AND lower(COALESCE(oem_code,''))=lower($3) AND deleted_at IS NULL ORDER BY created_at LIMIT 1`, tenantID, row.Title, row.OEMCode).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func initializeImportInventory(ctx context.Context, tx pgx.Tx, cmd ImportCommand, batchID uuid.UUID, row ImportRow, productID uuid.UUID) (string, string, int64, error) {
	var onHand, reserved float64
	var avgCost int64
	err := tx.QueryRow(ctx, `SELECT on_hand::float8,reserved::float8,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, cmd.WarehouseID, productID).Scan(&onHand, &reserved, &avgCost)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES($1,$2,$3,$4,0,$5)`, cmd.TenantID, cmd.WarehouseID, productID, row.OnHand, row.AvgUnitCost)
		if err != nil {
			return "", "", 0, err
		}
		return recordOpeningMovement(ctx, tx, cmd, batchID, row, productID)
	}
	if err != nil {
		return "", "", 0, err
	}
	var hasMovement bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inventory_movements WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3)`, cmd.TenantID, cmd.WarehouseID, productID).Scan(&hasMovement); err != nil {
		return "", "", 0, err
	}
	if onHand == 0 && reserved == 0 && !hasMovement {
		_, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,avg_unit_cost=$5,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, cmd.WarehouseID, productID, row.OnHand, row.AvgUnitCost)
		if err != nil {
			return "", "", 0, err
		}
		return recordOpeningMovement(ctx, tx, cmd, batchID, row, productID)
	}
	return "preserved", "موجودی زنده قبلی حفظ شد؛ Import Center موجودی کالای دارای سابقه را بازنویسی نمی‌کند.", 0, nil
}

func recordOpeningMovement(ctx context.Context, tx pgx.Tx, cmd ImportCommand, batchID uuid.UUID, row ImportRow, productID uuid.UUID) (string, string, int64, error) {
	if row.OnHand <= 0 {
		return "initialized", "", 0, nil
	}
	valueFloat := row.OnHand * float64(row.AvgUnitCost)
	if valueFloat > math.MaxInt64 {
		return "", "", 0, errors.New("opening inventory value is too large")
	}
	value := int64(math.Round(valueFloat))
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id)
		VALUES($1,$2,$3,'adjustment',$4,$5,$6,'catalog_import',$7)`,
		cmd.TenantID, cmd.WarehouseID, productID, row.OnHand, row.AvgUnitCost, value, batchID)
	if err != nil {
		return "", "", 0, err
	}
	return "initialized", "", value, nil
}

func postOpeningInventoryJournal(ctx context.Context, tx pgx.Tx, tenantID, batchID uuid.UUID, value int64) error {
	accounts := map[string]uuid.UUID{}
	defs := []struct{ code, name, typ string }{
		{"INVENTORY", "Inventory", "asset"},
		{"OPENING_BALANCE", "Opening Balance Equity", "equity"},
	}
	for _, d := range defs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id); err != nil {
			return err
		}
		accounts[d.code] = id
	}
	journalID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'catalog_import',$3)`, journalID, tenantID, batchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,0)`, tenantID, journalID, accounts["INVENTORY"], value); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,0,$4)`, tenantID, journalID, accounts["OPENING_BALANCE"], value)
	return err
}

func loadImportByKey(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string) (ImportResult, bool, error) {
	var out ImportResult
	err := tx.QueryRow(ctx, `
		SELECT id,row_count,created_count,updated_count,inventory_initialized_count,inventory_preserved_count,offers_upserted_count,opening_inventory_value
		FROM catalog_import_batches WHERE tenant_id=$1 AND idempotency_key=$2`, tenantID, key).
		Scan(&out.BatchID, &out.RowCount, &out.CreatedCount, &out.UpdatedCount, &out.InventoryInitializedCount, &out.InventoryPreservedCount, &out.OffersUpsertedCount, &out.OpeningInventoryValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImportResult{}, false, nil
	}
	if err != nil {
		return ImportResult{}, false, err
	}
	rows, err := tx.Query(ctx, `SELECT row_number,product_id,product_action,inventory_action,offer_action,COALESCE(note,'') FROM catalog_import_row_results WHERE batch_id=$1 ORDER BY row_number`, out.BatchID)
	if err != nil {
		return ImportResult{}, false, err
	}
	defer rows.Close()
	out.Rows = make([]ImportRowResult, 0, out.RowCount)
	for rows.Next() {
		var r ImportRowResult
		if err := rows.Scan(&r.RowNumber, &r.ProductID, &r.ProductAction, &r.InventoryAction, &r.OfferAction, &r.Note); err != nil {
			return ImportResult{}, false, err
		}
		out.Rows = append(out.Rows, r)
	}
	return out, true, rows.Err()
}

func nullable(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func unitOrPCS(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "pcs"
	}
	return v
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
