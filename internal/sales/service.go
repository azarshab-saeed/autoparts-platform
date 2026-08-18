package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/example/autoparts-core/internal/pricing"
	"github.com/example/autoparts-core/internal/productunit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Create(ctx context.Context, cmd CreateSaleCommand) (Sale, error) {
	if strings.TrimSpace(cmd.Source) == "" {
		cmd.Source = "web"
	}
	if cmd.Source != "web" && cmd.Source != "edge" {
		return Sale{}, errors.New("sale source must be web or edge")
	}
	if cmd.Source == "edge" && (cmd.EdgeDeviceID == nil || *cmd.EdgeDeviceID == uuid.Nil || strings.TrimSpace(cmd.EdgeLocalOperationID) == "") {
		return Sale{}, errors.New("edge sale requires device and local operation id")
	}
	if len(cmd.Items) == 0 {
		return Sale{}, errors.New("sale requires at least one item")
	}
	if cmd.WarehouseID == uuid.Nil {
		return Sale{}, errors.New("warehouse_id is required")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Sale{}, err
	}
	defer tx.Rollback(ctx)

	var warehouseOK bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, cmd.WarehouseID, cmd.TenantID, cmd.StoreID).Scan(&warehouseOK); err != nil {
		return Sale{}, err
	}
	if !warehouseOK {
		return Sale{}, errors.New("warehouse does not belong to authenticated store")
	}
	if cmd.CustomerID != nil {
		var customerOK bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, *cmd.CustomerID, cmd.TenantID, cmd.StoreID).Scan(&customerOK); err != nil {
			return Sale{}, err
		}
		if !customerOK {
			return Sale{}, errors.New("customer does not belong to authenticated store")
		}
	}

	var existing Sale
	err = tx.QueryRow(ctx, `SELECT id,gross_amount,discount_amount,total_amount,paid_amount,due_amount,status FROM sales WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing.ID, &existing.GrossAmount, &existing.DiscountAmount, &existing.TotalAmount, &existing.PaidAmount, &existing.DueAmount, &existing.Status)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Sale{}, err
	}

	minMarginBPS, cashierMayOverride, defaultListID, selectedListID, err := salePricingContext(ctx, tx, cmd)
	if err != nil {
		return Sale{}, err
	}
	prepared := make([]preparedSaleItem, 0, len(cmd.Items))
	grossTotal := int64(0)
	discountTotal := int64(0)
	total := int64(0)
	cogsTotal := int64(0)
	for _, item := range cmd.Items {
		if item.ProductID == uuid.Nil || item.Qty <= 0 || item.UnitPrice < 0 {
			return Sale{}, errors.New("invalid sale item")
		}
		unit, err := productunit.Resolve(ctx, tx, cmd.TenantID, item.ProductID, item.ProductUnitID)
		if err != nil {
			return Sale{}, fmt.Errorf("product unit: %w", err)
		}
		if !unit.AllowSale {
			return Sale{}, fmt.Errorf("unit %s is not enabled for sale", unit.Name)
		}
		baseQty, err := productunit.BaseQty(item.Qty, unit)
		if err != nil {
			return Sale{}, fmt.Errorf("invalid quantity for %s: %w", unit.Name, err)
		}
		var onHand, reserved float64
		var avgUnitCost int64
		err = tx.QueryRow(ctx, `SELECT on_hand,reserved,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, cmd.WarehouseID, item.ProductID).Scan(&onHand, &reserved, &avgUnitCost)
		if err != nil {
			return Sale{}, fmt.Errorf("inventory lookup: %w", err)
		}
		if onHand-reserved+1e-9 < baseQty {
			return Sale{}, fmt.Errorf("insufficient stock for %s: requested %.3f %s = %.3f base units", item.ProductID, item.Qty, unit.Name, baseQty)
		}
		suggested, appliedListID, source, configured, err := resolveSalePrice(ctx, tx, cmd, item.ProductID, unit, item.Qty, baseQty, selectedListID, defaultListID)
		if err != nil {
			return Sale{}, err
		}
		priceOverride := !configured || item.UnitPrice != suggested
		reason := strings.TrimSpace(item.OverrideReason)
		if configured && priceOverride && reason == "" && cmd.Source != "edge" {
			return Sale{}, fmt.Errorf("price override reason is required for product %s", item.ProductID)
		}
		if !configured && reason == "" {
			reason = "manual_unconfigured_price"
		}
		if cmd.Source == "edge" && priceOverride && reason == "" {
			reason = "offline_snapshot_price"
		}
		if (cmd.ActorRole == "cashier" || cmd.ActorRole == "edge") && priceOverride && !cashierMayOverride {
			return Sale{}, errors.New("cashier price override is disabled")
		}
		commercialCost := productunit.CommercialMoney(avgUnitCost, unit)
		minAllowed := productunit.CommercialMoney(pricing.MinimumPriceForMargin(avgUnitCost, minMarginBPS), unit)
		belowMarginGuard := item.UnitPrice < minAllowed
		if belowMarginGuard && cmd.ActorRole != "owner" && cmd.ActorRole != "admin" {
			return Sale{}, fmt.Errorf("price below minimum margin for product %s (minimum %d per %s)", item.ProductID, minAllowed, unit.Name)
		}
		if belowMarginGuard && reason == "" && cmd.Source != "edge" {
			return Sale{}, fmt.Errorf("margin override reason is required for product %s", item.ProductID)
		}
		line := int64(math.Round(item.Qty * float64(item.UnitPrice)))
		if line < 0 || total > math.MaxInt64-line {
			return Sale{}, errors.New("invalid line total")
		}
		grossLine := line
		var listUnitPrice *int64
		if configured {
			v := suggested
			listUnitPrice = &v
			listLine := int64(math.Round(item.Qty * float64(suggested)))
			if listLine > grossLine {
				grossLine = listLine
			}
		}
		discount := grossLine - line
		if grossLine < 0 || grossTotal > math.MaxInt64-grossLine || discount < 0 || discountTotal > math.MaxInt64-discount {
			return Sale{}, errors.New("pricing totals overflow")
		}
		itemCOGS := int64(math.Round(baseQty * float64(avgUnitCost)))
		if itemCOGS < 0 || cogsTotal > math.MaxInt64-itemCOGS {
			return Sale{}, errors.New("cogs overflow")
		}
		grossTotal += grossLine
		discountTotal += discount
		total += line
		cogsTotal += itemCOGS
		var overrideActorUserID *uuid.UUID
		if (priceOverride || belowMarginGuard) && cmd.ActorUserID != uuid.Nil {
			id := cmd.ActorUserID
			overrideActorUserID = &id
		}
		prepared = append(prepared, preparedSaleItem{
			Input: item, Unit: unit, BaseQty: baseQty, AvgUnitCost: avgUnitCost, LineTotal: line, GrossLineTotal: grossLine, DiscountAmount: discount, COGS: itemCOGS,
			PriceListID: appliedListID, ListUnitPrice: listUnitPrice, PriceSource: source,
			PriceOverride: priceOverride, OverrideReason: reason, OverrideActorUserID: overrideActorUserID, MarginBPS: pricing.MarginBPS(item.UnitPrice, commercialCost), MarginGuardBPS: minMarginBPS, BelowMarginGuard: belowMarginGuard,
		})
	}

	payments, paid, due, err := normalizePayments(total, cmd.PaymentMethod, cmd.Payments)
	if err != nil {
		return Sale{}, err
	}
	if due > 0 && cmd.CustomerID == nil {
		return Sale{}, errors.New("partial or credit sale requires customer_id")
	}

	saleID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO sales(id,tenant_id,store_id,warehouse_id,customer_id,status,gross_amount,discount_amount,total_amount,paid_amount,due_amount,idempotency_key,source,edge_device_id,edge_local_operation_id,edge_occurred_at) VALUES($1,$2,$3,$4,$5,'posted',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, saleID, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, cmd.CustomerID, grossTotal, discountTotal, total, paid, due, cmd.IdempotencyKey, cmd.Source, cmd.EdgeDeviceID, strings.TrimSpace(cmd.EdgeLocalOperationID), cmd.EdgeOccurredAt)
	if err != nil {
		return Sale{}, err
	}

	for _, item := range prepared {
		_, err = tx.Exec(ctx, `INSERT INTO sale_items(tenant_id,sale_id,product_id,qty,product_unit_id,commercial_qty,commercial_unit_code,commercial_unit_name,conversion_factor,unit_price,unit_cost,line_total,gross_line_total,discount_amount,price_list_id,list_unit_price,price_source,price_override,override_reason,override_actor_user_id,margin_bps,margin_guard_bps,below_margin_guard) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NULLIF($19,''),$20,$21,$22,$23)`, cmd.TenantID, saleID, item.Input.ProductID, item.BaseQty, item.Unit.ID, item.Input.Qty, item.Unit.Code, item.Unit.Name, item.Unit.FactorToBase, item.Input.UnitPrice, item.AvgUnitCost, item.LineTotal, item.GrossLineTotal, item.DiscountAmount, item.PriceListID, item.ListUnitPrice, item.PriceSource, item.PriceOverride, item.OverrideReason, item.OverrideActorUserID, item.MarginBPS, item.MarginGuardBPS, item.BelowMarginGuard)
		if err != nil {
			return Sale{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=on_hand-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, cmd.WarehouseID, item.Input.ProductID, item.BaseQty)
		if err != nil {
			return Sale{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'sale',$4,$5,$6,'sale',$7)`, cmd.TenantID, cmd.WarehouseID, item.Input.ProductID, -item.BaseQty, item.AvgUnitCost, -item.COGS, saleID)
		if err != nil {
			return Sale{}, err
		}
	}

	accounts, err := ensureDefaultAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Sale{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'sale',$3)`, journalID, cmd.TenantID, saleID); err != nil {
		return Sale{}, err
	}
	for _, p := range payments {
		code := "CASH"
		if p.Method == "card" {
			code = "BANK_CARD"
		}
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts[code], p.Amount, 0); err != nil {
			return Sale{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO payments(tenant_id,store_id,sale_id,method,amount) VALUES($1,$2,$3,$4,$5)`, cmd.TenantID, cmd.StoreID, saleID, p.Method, p.Amount); err != nil {
			return Sale{}, err
		}
	}
	if due > 0 {
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["AR"], due, 0); err != nil {
			return Sale{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'sale',$4,$5,0)`, cmd.TenantID, cmd.StoreID, cmd.CustomerID, saleID, due); err != nil {
			return Sale{}, err
		}
	}
	if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["SALES"], 0, total); err != nil {
		return Sale{}, err
	}
	if cogsTotal > 0 {
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["COGS"], cogsTotal, 0); err != nil {
			return Sale{}, err
		}
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], 0, cogsTotal); err != nil {
			return Sale{}, err
		}
	}

	payload, _ := json.Marshal(map[string]any{"sale_id": saleID, "gross_amount": grossTotal, "discount_amount": discountTotal, "total_amount": total, "paid_amount": paid, "due_amount": due, "warehouse_id": cmd.WarehouseID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'sale',$2,'sale.created',$3)`, cmd.TenantID, saleID, payload); err != nil {
		return Sale{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Sale{}, err
	}
	return Sale{ID: saleID, GrossAmount: grossTotal, DiscountAmount: discountTotal, TotalAmount: total, PaidAmount: paid, DueAmount: due, Status: "posted"}, nil
}

type preparedSaleItem struct {
	Input               CreateSaleItem
	Unit                productunit.Unit
	BaseQty             float64
	AvgUnitCost         int64
	LineTotal           int64
	GrossLineTotal      int64
	DiscountAmount      int64
	COGS                int64
	PriceListID         *uuid.UUID
	ListUnitPrice       *int64
	PriceSource         string
	PriceOverride       bool
	OverrideReason      string
	OverrideActorUserID *uuid.UUID
	MarginBPS           int
	MarginGuardBPS      int
	BelowMarginGuard    bool
}

func salePricingContext(ctx context.Context, tx pgx.Tx, cmd CreateSaleCommand) (int, bool, uuid.UUID, uuid.UUID, error) {
	minMarginBPS, cashierMayOverride := 1000, true
	err := tx.QueryRow(ctx, `SELECT min_margin_bps,cashier_may_override FROM store_pricing_settings WHERE tenant_id=$1 AND store_id=$2`, cmd.TenantID, cmd.StoreID).Scan(&minMarginBPS, &cashierMayOverride)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, uuid.Nil, uuid.Nil, err
	}
	var defaultListID uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active ORDER BY created_at LIMIT 1`, cmd.TenantID, cmd.StoreID).Scan(&defaultListID); err != nil {
		return 0, false, uuid.Nil, uuid.Nil, errors.New("default price list is not configured")
	}
	selectedListID := defaultListID
	if cmd.CustomerID != nil {
		var customerListID *uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT pl.id FROM customers c LEFT JOIN price_lists pl ON pl.id=c.price_list_id AND pl.tenant_id=c.tenant_id AND pl.store_id=c.store_id AND pl.active WHERE c.id=$1 AND c.tenant_id=$2 AND c.store_id=$3`, *cmd.CustomerID, cmd.TenantID, cmd.StoreID).Scan(&customerListID); err != nil {
			return 0, false, uuid.Nil, uuid.Nil, err
		}
		if customerListID != nil {
			selectedListID = *customerListID
		}
	}
	return minMarginBPS, cashierMayOverride, defaultListID, selectedListID, nil
}

func resolveSalePrice(ctx context.Context, tx pgx.Tx, cmd CreateSaleCommand, productID uuid.UUID, unit productunit.Unit, commercialQty, baseQty float64, selectedListID, defaultListID uuid.UUID) (int64, *uuid.UUID, string, bool, error) {
	price, found, err := saleBreakPrice(ctx, tx, cmd.TenantID, cmd.StoreID, productID, selectedListID, unit.ID, commercialQty)
	if err != nil {
		return 0, nil, "", false, err
	}
	if found {
		id := selectedListID
		return price, &id, "price_list", true, nil
	}
	if selectedListID != defaultListID {
		price, found, err = saleBreakPrice(ctx, tx, cmd.TenantID, cmd.StoreID, productID, defaultListID, unit.ID, commercialQty)
		if err != nil {
			return 0, nil, "", false, err
		}
		if found {
			id := defaultListID
			return price, &id, "default_fallback", true, nil
		}
	}
	// A package without a dedicated price inherits the base-unit tier at the equivalent base quantity.
	if !unit.IsBase {
		base, err := productunit.Resolve(ctx, tx, cmd.TenantID, productID, nil)
		if err != nil {
			return 0, nil, "", false, err
		}
		price, found, err = saleBreakPrice(ctx, tx, cmd.TenantID, cmd.StoreID, productID, selectedListID, base.ID, baseQty)
		if err != nil {
			return 0, nil, "", false, err
		}
		if found {
			id := selectedListID
			return productunit.CommercialMoney(price, unit), &id, "base_unit_derived", true, nil
		}
		if selectedListID != defaultListID {
			price, found, err = saleBreakPrice(ctx, tx, cmd.TenantID, cmd.StoreID, productID, defaultListID, base.ID, baseQty)
			if err != nil {
				return 0, nil, "", false, err
			}
			if found {
				id := defaultListID
				return productunit.CommercialMoney(price, unit), &id, "base_unit_default_derived", true, nil
			}
		}
	}
	err = tx.QueryRow(ctx, `SELECT selling_price FROM store_product_offers WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND product_id=$4 ORDER BY updated_at DESC LIMIT 1`, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, productID).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, "manual", false, nil
	}
	if err != nil {
		return 0, nil, "", false, err
	}
	return productunit.CommercialMoney(price, unit), nil, "legacy_offer_fallback", true, nil
}

func saleBreakPrice(ctx context.Context, tx pgx.Tx, tenantID, storeID, productID, listID, unitID uuid.UUID, qty float64) (int64, bool, error) {
	var price int64
	err := tx.QueryRow(ctx, `SELECT unit_price FROM product_price_breaks WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND price_list_id=$4 AND product_unit_id=$5 AND min_qty<=$6 ORDER BY min_qty DESC LIMIT 1`, tenantID, storeID, productID, listID, unitID, qty).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return price, err == nil, err
}

func (s *Service) Detail(ctx context.Context, tenantID, storeID, saleID uuid.UUID) (SaleDetail, error) {
	var out SaleDetail
	var created time.Time
	err := s.db.QueryRow(ctx, `SELECT s.id,s.customer_id,COALESCE(c.name,''),s.warehouse_id,s.gross_amount,s.discount_amount,s.total_amount,s.paid_amount,s.due_amount,s.status,s.created_at FROM sales s LEFT JOIN customers c ON c.id=s.customer_id WHERE s.id=$1 AND s.tenant_id=$2 AND s.store_id=$3`, saleID, tenantID, storeID).Scan(&out.ID, &out.CustomerID, &out.CustomerName, &out.WarehouseID, &out.GrossAmount, &out.DiscountAmount, &out.TotalAmount, &out.PaidAmount, &out.DueAmount, &out.Status, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return SaleDetail{}, errors.New("sale not found")
	}
	if err != nil {
		return SaleDetail{}, err
	}
	out.CreatedAt = created.Format(time.RFC3339)
	rows, err := s.db.Query(ctx, `SELECT si.id,si.product_id,p.title,si.product_unit_id,COALESCE(si.commercial_unit_code,p.unit),COALESCE(si.commercial_unit_name,p.unit),si.conversion_factor::float8,COALESCE(si.commercial_qty,si.qty)::float8,si.qty::float8,COALESCE(SUM(sri.qty),0)::float8/NULLIF(si.conversion_factor,0),GREATEST(si.qty-COALESCE(SUM(sri.qty),0),0)::float8/NULLIF(si.conversion_factor,0),si.unit_price,si.unit_cost,si.line_total,si.gross_line_total,si.discount_amount,si.price_list_id,si.list_unit_price,si.price_source,si.price_override,si.override_reason,si.override_actor_user_id,si.margin_bps,si.margin_guard_bps,si.below_margin_guard FROM sale_items si JOIN products p ON p.id=si.product_id LEFT JOIN sales_return_items sri ON sri.sale_item_id=si.id WHERE si.tenant_id=$1 AND si.sale_id=$2 GROUP BY si.id,p.title,p.unit ORDER BY si.created_at`, tenantID, saleID)
	if err != nil {
		return SaleDetail{}, err
	}
	defer rows.Close()
	out.Items = []SaleLine{}
	for rows.Next() {
		var x SaleLine
		if err := rows.Scan(&x.ID, &x.ProductID, &x.Title, &x.ProductUnitID, &x.UnitCode, &x.UnitName, &x.ConversionFactor, &x.Qty, &x.BaseQty, &x.ReturnedQty, &x.ReturnableQty, &x.UnitPrice, &x.UnitCost, &x.LineTotal, &x.GrossLineTotal, &x.DiscountAmount, &x.PriceListID, &x.ListUnitPrice, &x.PriceSource, &x.PriceOverride, &x.OverrideReason, &x.OverrideActorUserID, &x.MarginBPS, &x.MarginGuardBPS, &x.BelowMarginGuard); err != nil {
			return SaleDetail{}, err
		}
		out.Items = append(out.Items, x)
	}
	return out, rows.Err()
}

func normalizePayments(total int64, legacy string, parts []PaymentPart) ([]PaymentPart, int64, int64, error) {
	if total == 0 {
		return nil, 0, 0, nil
	}

	if total < 0 {
		return nil, 0, 0, errors.New("invalid total")
	}
	if len(parts) == 0 {
		switch strings.TrimSpace(legacy) {
		case "cash", "card":
			parts = []PaymentPart{{Method: legacy, Amount: total}}
		case "credit":
			parts = nil
		default:
			return nil, 0, 0, errors.New("payment_method must be cash, card, or credit, or payments must be provided")
		}
	}
	paid := int64(0)
	for _, p := range parts {
		if p.Method != "cash" && p.Method != "card" {
			return nil, 0, 0, errors.New("payment parts support cash or card only")
		}
		if p.Amount <= 0 {
			return nil, 0, 0, errors.New("payment amount must be greater than zero")
		}
		if paid > math.MaxInt64-p.Amount {
			return nil, 0, 0, errors.New("payment total overflow")
		}
		paid += p.Amount
	}
	if paid > total {
		return nil, 0, 0, errors.New("payments exceed sale total")
	}
	return parts, paid, total - paid, nil
}

func ensureDefaultAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typeName string }{{"CASH", "Cash", "asset"}, {"BANK_CARD", "Card Clearing", "asset"}, {"AR", "Accounts Receivable", "asset"}, {"SALES", "Sales Revenue", "revenue"}, {"INVENTORY", "Inventory", "asset"}, {"COGS", "Cost of Goods Sold", "expense"}}
	result := make(map[string]uuid.UUID, len(defs))
	for _, d := range defs {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typeName).Scan(&id)
		if err != nil {
			return nil, err
		}
		result[d.code] = id
	}
	return result, nil
}
func insertEntry(ctx context.Context, tx pgx.Tx, tenantID, journalID, accountID uuid.UUID, debit, credit int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,$5)`, tenantID, journalID, accountID, debit, credit)
	return err
}
