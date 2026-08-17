package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

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
	err = tx.QueryRow(ctx, `SELECT id,total_amount,paid_amount,due_amount,status FROM sales WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing.ID, &existing.TotalAmount, &existing.PaidAmount, &existing.DueAmount, &existing.Status)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Sale{}, err
	}

	total := int64(0)
	for _, item := range cmd.Items {
		if item.ProductID == uuid.Nil || item.Qty <= 0 || item.UnitPrice < 0 {
			return Sale{}, errors.New("invalid sale item")
		}
		line := int64(math.Round(item.Qty * float64(item.UnitPrice)))
		if line < 0 || total > math.MaxInt64-line {
			return Sale{}, errors.New("invalid line total")
		}
		total += line
	}
	payments, paid, due, err := normalizePayments(total, cmd.PaymentMethod, cmd.Payments)
	if err != nil {
		return Sale{}, err
	}
	if due > 0 && cmd.CustomerID == nil {
		return Sale{}, errors.New("partial or credit sale requires customer_id")
	}

	saleID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO sales(id,tenant_id,store_id,warehouse_id,customer_id,status,total_amount,paid_amount,due_amount,idempotency_key,source,edge_device_id,edge_local_operation_id,edge_occurred_at) VALUES($1,$2,$3,$4,$5,'posted',$6,$7,$8,$9,$10,$11,$12,$13)`, saleID, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, cmd.CustomerID, total, paid, due, cmd.IdempotencyKey, cmd.Source, cmd.EdgeDeviceID, strings.TrimSpace(cmd.EdgeLocalOperationID), cmd.EdgeOccurredAt)
	if err != nil {
		return Sale{}, err
	}

	cogsTotal := int64(0)
	for _, item := range cmd.Items {
		line := int64(math.Round(item.Qty * float64(item.UnitPrice)))
		var onHand, reserved float64
		var avgUnitCost int64
		err = tx.QueryRow(ctx, `SELECT on_hand,reserved,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, cmd.WarehouseID, item.ProductID).Scan(&onHand, &reserved, &avgUnitCost)
		if err != nil {
			return Sale{}, fmt.Errorf("inventory lookup: %w", err)
		}
		if onHand-reserved < item.Qty {
			return Sale{}, fmt.Errorf("insufficient stock for %s", item.ProductID)
		}
		itemCOGS := int64(math.Round(item.Qty * float64(avgUnitCost)))
		if itemCOGS < 0 || cogsTotal > math.MaxInt64-itemCOGS {
			return Sale{}, errors.New("cogs overflow")
		}
		cogsTotal += itemCOGS
		_, err = tx.Exec(ctx, `INSERT INTO sale_items(tenant_id,sale_id,product_id,qty,unit_price,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6,$7)`, cmd.TenantID, saleID, item.ProductID, item.Qty, item.UnitPrice, avgUnitCost, line)
		if err != nil {
			return Sale{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=on_hand-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, cmd.WarehouseID, item.ProductID, item.Qty)
		if err != nil {
			return Sale{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'sale',$4,$5,$6,'sale',$7)`, cmd.TenantID, cmd.WarehouseID, item.ProductID, -item.Qty, avgUnitCost, -itemCOGS, saleID)
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

	payload, _ := json.Marshal(map[string]any{"sale_id": saleID, "total_amount": total, "paid_amount": paid, "due_amount": due, "warehouse_id": cmd.WarehouseID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'sale',$2,'sale.created',$3)`, cmd.TenantID, saleID, payload); err != nil {
		return Sale{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Sale{}, err
	}
	return Sale{ID: saleID, TotalAmount: total, PaidAmount: paid, DueAmount: due, Status: "posted"}, nil
}

func (s *Service) Detail(ctx context.Context, tenantID, storeID, saleID uuid.UUID) (SaleDetail, error) {
	var out SaleDetail
	var created time.Time
	err := s.db.QueryRow(ctx, `SELECT s.id,s.customer_id,COALESCE(c.name,''),s.warehouse_id,s.total_amount,s.paid_amount,s.due_amount,s.status,s.created_at FROM sales s LEFT JOIN customers c ON c.id=s.customer_id WHERE s.id=$1 AND s.tenant_id=$2 AND s.store_id=$3`, saleID, tenantID, storeID).Scan(&out.ID, &out.CustomerID, &out.CustomerName, &out.WarehouseID, &out.TotalAmount, &out.PaidAmount, &out.DueAmount, &out.Status, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return SaleDetail{}, errors.New("sale not found")
	}
	if err != nil {
		return SaleDetail{}, err
	}
	out.CreatedAt = created.Format(time.RFC3339)
	rows, err := s.db.Query(ctx, `SELECT si.id,si.product_id,p.title,si.qty,COALESCE(SUM(sri.qty),0),si.qty-COALESCE(SUM(sri.qty),0),si.unit_price,si.unit_cost,si.line_total FROM sale_items si JOIN products p ON p.id=si.product_id LEFT JOIN sales_return_items sri ON sri.sale_item_id=si.id WHERE si.tenant_id=$1 AND si.sale_id=$2 GROUP BY si.id,p.title ORDER BY si.created_at`, tenantID, saleID)
	if err != nil {
		return SaleDetail{}, err
	}
	defer rows.Close()
	out.Items = []SaleLine{}
	for rows.Next() {
		var x SaleLine
		if err := rows.Scan(&x.ID, &x.ProductID, &x.Title, &x.Qty, &x.ReturnedQty, &x.ReturnableQty, &x.UnitPrice, &x.UnitCost, &x.LineTotal); err != nil {
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
