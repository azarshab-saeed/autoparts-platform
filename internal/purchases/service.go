package purchases

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

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (Purchase, error) {
	if cmd.WarehouseID == uuid.Nil || cmd.SupplierID == uuid.Nil {
		return Purchase{}, errors.New("warehouse_id and supplier_id are required")
	}
	if len(cmd.Items) == 0 {
		return Purchase{}, errors.New("purchase requires at least one item")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Purchase{}, err
	}
	defer tx.Rollback(ctx)

	var ok bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, cmd.WarehouseID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
		return Purchase{}, err
	}
	if !ok {
		return Purchase{}, errors.New("warehouse does not belong to authenticated store")
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM suppliers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, cmd.SupplierID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
		return Purchase{}, err
	}
	if !ok {
		return Purchase{}, errors.New("supplier does not belong to authenticated store")
	}

	var existing Purchase
	err = tx.QueryRow(ctx, `SELECT id,total_amount,paid_amount,due_amount,status FROM purchases WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing.ID, &existing.TotalAmount, &existing.PaidAmount, &existing.DueAmount, &existing.Status)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Purchase{}, err
	}

	total := int64(0)
	for _, item := range cmd.Items {
		if item.ProductID == uuid.Nil || item.Qty <= 0 || item.UnitCost < 0 {
			return Purchase{}, errors.New("invalid purchase item")
		}
		line := int64(math.Round(item.Qty * float64(item.UnitCost)))
		if line < 0 || total > math.MaxInt64-line {
			return Purchase{}, errors.New("purchase total overflow")
		}
		total += line
	}
	payments, paid, due, err := normalizePayments(total, cmd.PaymentMethod, cmd.Payments)
	if err != nil {
		return Purchase{}, err
	}
	summaryMethod := "credit"
	if due == 0 && len(payments) == 1 {
		summaryMethod = payments[0].Method
	} else if len(payments) > 0 {
		summaryMethod = "mixed"
	}

	purchaseID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO purchases(id,tenant_id,store_id,warehouse_id,supplier_id,status,payment_method,total_amount,paid_amount,due_amount,idempotency_key) VALUES($1,$2,$3,$4,$5,'posted',$6,$7,$8,$9,$10)`, purchaseID, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, cmd.SupplierID, summaryMethod, total, paid, due, cmd.IdempotencyKey)
	if err != nil {
		return Purchase{}, err
	}

	for _, item := range cmd.Items {
		line := int64(math.Round(item.Qty * float64(item.UnitCost)))
		var productOK bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND active)`, item.ProductID, cmd.TenantID).Scan(&productOK); err != nil {
			return Purchase{}, err
		}
		if !productOK {
			return Purchase{}, fmt.Errorf("product %s is not available in tenant", item.ProductID)
		}
		var oldQty float64
		var oldAvg int64
		err = tx.QueryRow(ctx, `SELECT on_hand,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, cmd.WarehouseID, item.ProductID).Scan(&oldQty, &oldAvg)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES($1,$2,$3,0,0,0)`, cmd.TenantID, cmd.WarehouseID, item.ProductID)
			if err != nil {
				return Purchase{}, err
			}
			oldQty, oldAvg = 0, 0
		} else if err != nil {
			return Purchase{}, err
		}
		newQty := oldQty + item.Qty
		newAvg := weightedAverage(oldQty, oldAvg, item.Qty, item.UnitCost)
		if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,avg_unit_cost=$5,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, cmd.WarehouseID, item.ProductID, newQty, newAvg); err != nil {
			return Purchase{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO purchase_items(tenant_id,purchase_id,product_id,qty,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6)`, cmd.TenantID, purchaseID, item.ProductID, item.Qty, item.UnitCost, line); err != nil {
			return Purchase{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'purchase',$4,$5,$6,'purchase',$7)`, cmd.TenantID, cmd.WarehouseID, item.ProductID, item.Qty, item.UnitCost, line, purchaseID); err != nil {
			return Purchase{}, err
		}
	}

	accounts, err := ensureAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Purchase{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'purchase',$3)`, journalID, cmd.TenantID, purchaseID); err != nil {
		return Purchase{}, err
	}
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], total, 0); err != nil {
		return Purchase{}, err
	}
	for _, p := range payments {
		code := "CASH"
		if p.Method == "card" {
			code = "BANK_CARD"
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[code], 0, p.Amount); err != nil {
			return Purchase{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO payments(tenant_id,store_id,purchase_id,method,amount) VALUES($1,$2,$3,$4,$5)`, cmd.TenantID, cmd.StoreID, purchaseID, p.Method, p.Amount); err != nil {
			return Purchase{}, err
		}
	}
	if due > 0 {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["AP"], 0, due); err != nil {
			return Purchase{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'purchase',$4,0,$5)`, cmd.TenantID, cmd.StoreID, cmd.SupplierID, purchaseID, due); err != nil {
			return Purchase{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{"purchase_id": purchaseID, "warehouse_id": cmd.WarehouseID, "supplier_id": cmd.SupplierID, "total_amount": total, "paid_amount": paid, "due_amount": due})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'purchase',$2,'purchase.created',$3)`, cmd.TenantID, purchaseID, payload); err != nil {
		return Purchase{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Purchase{}, err
	}
	return Purchase{ID: purchaseID, TotalAmount: total, PaidAmount: paid, DueAmount: due, Status: "posted"}, nil
}

func (s *Service) Detail(ctx context.Context, tenantID, storeID, purchaseID uuid.UUID) (PurchaseDetail, error) {
	var out PurchaseDetail
	var created time.Time
	err := s.db.QueryRow(ctx, `SELECT p.id,p.supplier_id,sp.name,p.warehouse_id,p.total_amount,p.paid_amount,p.due_amount,p.status,p.created_at FROM purchases p JOIN suppliers sp ON sp.id=p.supplier_id WHERE p.id=$1 AND p.tenant_id=$2 AND p.store_id=$3`, purchaseID, tenantID, storeID).Scan(&out.ID, &out.SupplierID, &out.SupplierName, &out.WarehouseID, &out.TotalAmount, &out.PaidAmount, &out.DueAmount, &out.Status, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return PurchaseDetail{}, errors.New("purchase not found")
	}
	if err != nil {
		return PurchaseDetail{}, err
	}
	out.CreatedAt = created.Format(time.RFC3339)
	rows, err := s.db.Query(ctx, `SELECT pi.id,pi.product_id,pr.title,pi.qty,COALESCE(SUM(pri.qty),0),pi.qty-COALESCE(SUM(pri.qty),0),pi.unit_cost,pi.line_total FROM purchase_items pi JOIN products pr ON pr.id=pi.product_id LEFT JOIN purchase_return_items pri ON pri.purchase_item_id=pi.id WHERE pi.tenant_id=$1 AND pi.purchase_id=$2 GROUP BY pi.id,pr.title ORDER BY pi.created_at`, tenantID, purchaseID)
	if err != nil {
		return PurchaseDetail{}, err
	}
	defer rows.Close()
	out.Items = []PurchaseLine{}
	for rows.Next() {
		var x PurchaseLine
		if err := rows.Scan(&x.ID, &x.ProductID, &x.Title, &x.Qty, &x.ReturnedQty, &x.ReturnableQty, &x.UnitCost, &x.LineTotal); err != nil {
			return PurchaseDetail{}, err
		}
		out.Items = append(out.Items, x)
	}
	return out, rows.Err()
}

func normalizePayments(total int64, legacy string, parts []PaymentPart) ([]PaymentPart, int64, int64, error) {
	if total == 0 {
		return nil, 0, 0, nil
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
		return nil, 0, 0, errors.New("payments exceed purchase total")
	}
	return parts, paid, total - paid, nil
}
func weightedAverage(oldQty float64, oldAvg int64, incomingQty float64, incomingCost int64) int64 {
	newQty := oldQty + incomingQty
	if newQty <= 0 {
		return 0
	}
	return int64(math.Round((oldQty*float64(oldAvg) + incomingQty*float64(incomingCost)) / newQty))
}
func ensureAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typ string }{{"CASH", "Cash", "asset"}, {"BANK_CARD", "Card Clearing", "asset"}, {"AP", "Accounts Payable", "liability"}, {"INVENTORY", "Inventory", "asset"}}
	out := map[string]uuid.UUID{}
	for _, d := range defs {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id)
		if err != nil {
			return nil, err
		}
		out[d.code] = id
	}
	return out, nil
}
func entry(ctx context.Context, tx pgx.Tx, tenantID, journalID, accountID uuid.UUID, debit, credit int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,$5)`, tenantID, journalID, accountID, debit, credit)
	return err
}
