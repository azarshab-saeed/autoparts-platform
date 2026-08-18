package returns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) CreateSaleReturn(ctx context.Context, cmd SaleReturnCommand) (Result, error) {
	if cmd.SaleID == uuid.Nil || len(cmd.Items) == 0 {
		return Result{}, errors.New("sale_id and return items are required")
	}
	if !validRefundMethod(cmd.RefundMethod) {
		return Result{}, errors.New("refund_method must be cash, card, or credit_balance")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	var existing Result
	err = tx.QueryRow(ctx, `SELECT id,total_amount,'posted' FROM sales_returns WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing.ID, &existing.TotalAmount, &existing.Status)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	var warehouseID uuid.UUID
	var customerID *uuid.UUID
	var status string
	if err = tx.QueryRow(ctx, `SELECT warehouse_id,customer_id,status FROM sales WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, cmd.SaleID, cmd.TenantID, cmd.StoreID).Scan(&warehouseID, &customerID, &status); errors.Is(err, pgx.ErrNoRows) {
		return Result{}, errors.New("sale not found")
	}
	if err != nil {
		return Result{}, err
	}
	if status != "posted" {
		return Result{}, errors.New("only posted sales can be returned")
	}
	if cmd.RefundMethod == "credit_balance" && customerID == nil {
		return Result{}, errors.New("credit_balance return requires a customer")
	}
	returnID := uuid.New()
	total, totalCost := int64(0), int64(0)
	if _, err = tx.Exec(ctx, `INSERT INTO sales_returns(id,tenant_id,store_id,warehouse_id,sale_id,customer_id,refund_method,total_amount,total_cost,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,1,0,$8)`, returnID, cmd.TenantID, cmd.StoreID, warehouseID, cmd.SaleID, customerID, cmd.RefundMethod, cmd.IdempotencyKey); err != nil {
		return Result{}, err
	}
	for _, ri := range cmd.Items {
		if ri.SourceItemID == uuid.Nil || ri.Qty <= 0 {
			return Result{}, errors.New("invalid return item")
		}
		var productID uuid.UUID
		var soldQty, factor float64
		var unitPrice, unitCost int64
		err = tx.QueryRow(ctx, `SELECT product_id,qty,unit_price,unit_cost,conversion_factor::float8 FROM sale_items WHERE id=$1 AND tenant_id=$2 AND sale_id=$3 FOR UPDATE`, ri.SourceItemID, cmd.TenantID, cmd.SaleID).Scan(&productID, &soldQty, &unitPrice, &unitCost, &factor)
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, fmt.Errorf("sale item %s not found", ri.SourceItemID)
		}
		if err != nil {
			return Result{}, err
		}
		var returned float64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(qty),0) FROM sales_return_items WHERE tenant_id=$1 AND sale_item_id=$2`, cmd.TenantID, ri.SourceItemID).Scan(&returned); err != nil {
			return Result{}, err
		}
		if factor <= 0 {
			factor = 1
		}
		returnBaseQty := ri.Qty * factor
		if returnBaseQty > soldQty-returned+1e-9 {
			return Result{}, errors.New("return quantity exceeds remaining sale quantity")
		}
		line := int64(math.Round(ri.Qty * float64(unitPrice)))
		cost := int64(math.Round(returnBaseQty * float64(unitCost)))
		if total > math.MaxInt64-line || totalCost > math.MaxInt64-cost {
			return Result{}, errors.New("return total overflow")
		}
		total += line
		totalCost += cost
		var oldQty float64
		var oldAvg int64
		err = tx.QueryRow(ctx, `SELECT on_hand,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, warehouseID, productID).Scan(&oldQty, &oldAvg)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err = tx.Exec(ctx, `INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES($1,$2,$3,0,0,0)`, cmd.TenantID, warehouseID, productID); err != nil {
				return Result{}, err
			}
			oldQty, oldAvg = 0, 0
		} else if err != nil {
			return Result{}, err
		}
		newQty := oldQty + returnBaseQty
		newAvg := weightedAverage(oldQty, oldAvg, returnBaseQty, unitCost)
		if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,avg_unit_cost=$5,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, warehouseID, productID, newQty, newAvg); err != nil {
			return Result{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO sales_return_items(tenant_id,sales_return_id,sale_item_id,product_id,qty,unit_price,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, cmd.TenantID, returnID, ri.SourceItemID, productID, returnBaseQty, unitPrice, unitCost, line); err != nil {
			return Result{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'return_in',$4,$5,$6,'sale_return',$7)`, cmd.TenantID, warehouseID, productID, returnBaseQty, unitCost, cost, returnID); err != nil {
			return Result{}, err
		}
	}
	if total <= 0 {
		return Result{}, errors.New("return total must be positive")
	}
	if _, err = tx.Exec(ctx, `UPDATE sales_returns SET total_amount=$2,total_cost=$3 WHERE id=$1`, returnID, total, totalCost); err != nil {
		return Result{}, err
	}
	accounts, err := ensureAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Result{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'sale_return',$3)`, journalID, cmd.TenantID, returnID); err != nil {
		return Result{}, err
	}
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["SALES"], total, 0); err != nil {
		return Result{}, err
	}
	creditCode := refundAccount(cmd.RefundMethod, "AR")
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[creditCode], 0, total); err != nil {
		return Result{}, err
	}
	if totalCost > 0 {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], totalCost, 0); err != nil {
			return Result{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["COGS"], 0, totalCost); err != nil {
			return Result{}, err
		}
	}
	if cmd.RefundMethod == "credit_balance" {
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'sale_return',$4,0,$5)`, cmd.TenantID, cmd.StoreID, customerID, returnID, total); err != nil {
			return Result{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{"sales_return_id": returnID, "sale_id": cmd.SaleID, "total_amount": total})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'sale_return',$2,'sale.returned',$3)`, cmd.TenantID, returnID, payload); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{ID: returnID, TotalAmount: total, Status: "posted"}, nil
}

func (s *Service) CreatePurchaseReturn(ctx context.Context, cmd PurchaseReturnCommand) (Result, error) {
	if cmd.PurchaseID == uuid.Nil || len(cmd.Items) == 0 {
		return Result{}, errors.New("purchase_id and return items are required")
	}
	if !validRefundMethod(cmd.RefundMethod) {
		return Result{}, errors.New("refund_method must be cash, card, or credit_balance")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	var existing Result
	err = tx.QueryRow(ctx, `SELECT id,total_amount,'posted' FROM purchase_returns WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing.ID, &existing.TotalAmount, &existing.Status)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	var warehouseID, supplierID uuid.UUID
	var status string
	if err = tx.QueryRow(ctx, `SELECT warehouse_id,supplier_id,status FROM purchases WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, cmd.PurchaseID, cmd.TenantID, cmd.StoreID).Scan(&warehouseID, &supplierID, &status); errors.Is(err, pgx.ErrNoRows) {
		return Result{}, errors.New("purchase not found")
	}
	if err != nil {
		return Result{}, err
	}
	if status != "posted" {
		return Result{}, errors.New("only posted purchases can be returned")
	}
	returnID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO purchase_returns(id,tenant_id,store_id,warehouse_id,purchase_id,supplier_id,refund_method,total_amount,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,1,$8)`, returnID, cmd.TenantID, cmd.StoreID, warehouseID, cmd.PurchaseID, supplierID, cmd.RefundMethod, cmd.IdempotencyKey); err != nil {
		return Result{}, err
	}
	total := int64(0)
	for _, ri := range cmd.Items {
		if ri.SourceItemID == uuid.Nil || ri.Qty <= 0 {
			return Result{}, errors.New("invalid return item")
		}
		var productID uuid.UUID
		var boughtQty, factor float64
		var unitCost int64
		err = tx.QueryRow(ctx, `SELECT product_id,qty,unit_cost,conversion_factor::float8 FROM purchase_items WHERE id=$1 AND tenant_id=$2 AND purchase_id=$3 FOR UPDATE`, ri.SourceItemID, cmd.TenantID, cmd.PurchaseID).Scan(&productID, &boughtQty, &unitCost, &factor)
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, fmt.Errorf("purchase item %s not found", ri.SourceItemID)
		}
		if err != nil {
			return Result{}, err
		}
		var returned float64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(qty),0) FROM purchase_return_items WHERE tenant_id=$1 AND purchase_item_id=$2`, cmd.TenantID, ri.SourceItemID).Scan(&returned); err != nil {
			return Result{}, err
		}
		if factor <= 0 {
			factor = 1
		}
		returnBaseQty := ri.Qty * factor
		if returnBaseQty > boughtQty-returned+1e-9 {
			return Result{}, errors.New("return quantity exceeds remaining purchase quantity")
		}
		line := int64(math.Round(ri.Qty * float64(unitCost)))
		baseUnitCost := int64(math.Round(float64(unitCost) / factor))
		if total > math.MaxInt64-line {
			return Result{}, errors.New("return total overflow")
		}
		total += line
		var oldQty, reserved float64
		var oldAvg int64
		if err = tx.QueryRow(ctx, `SELECT on_hand,reserved,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, warehouseID, productID).Scan(&oldQty, &reserved, &oldAvg); err != nil {
			return Result{}, err
		}
		if oldQty-reserved < returnBaseQty {
			return Result{}, errors.New("insufficient available stock for purchase return")
		}
		newQty := oldQty - returnBaseQty
		newAvg := int64(0)
		if newQty > 0 {
			newValue := oldQty*float64(oldAvg) - returnBaseQty*float64(baseUnitCost)
			if newValue < -0.5 {
				return Result{}, errors.New("purchase return conflicts with current weighted-average inventory value")
			}
			if newValue < 0 {
				newValue = 0
			}
			newAvg = int64(math.Round(newValue / newQty))
		}
		if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,avg_unit_cost=$5,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, warehouseID, productID, newQty, newAvg); err != nil {
			return Result{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO purchase_return_items(tenant_id,purchase_return_id,purchase_item_id,product_id,qty,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6,$7)`, cmd.TenantID, returnID, ri.SourceItemID, productID, returnBaseQty, unitCost, line); err != nil {
			return Result{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'return_out',$4,$5,$6,'purchase_return',$7)`, cmd.TenantID, warehouseID, productID, -returnBaseQty, baseUnitCost, -line, returnID); err != nil {
			return Result{}, err
		}
	}
	if total <= 0 {
		return Result{}, errors.New("return total must be positive")
	}
	if _, err = tx.Exec(ctx, `UPDATE purchase_returns SET total_amount=$2 WHERE id=$1`, returnID, total); err != nil {
		return Result{}, err
	}
	accounts, err := ensureAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Result{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'purchase_return',$3)`, journalID, cmd.TenantID, returnID); err != nil {
		return Result{}, err
	}
	debitCode := refundAccount(cmd.RefundMethod, "AP")
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[debitCode], total, 0); err != nil {
		return Result{}, err
	}
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], 0, total); err != nil {
		return Result{}, err
	}
	if cmd.RefundMethod == "credit_balance" {
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'purchase_return',$4,$5,0)`, cmd.TenantID, cmd.StoreID, supplierID, returnID, total); err != nil {
			return Result{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{"purchase_return_id": returnID, "purchase_id": cmd.PurchaseID, "total_amount": total})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'purchase_return',$2,'purchase.returned',$3)`, cmd.TenantID, returnID, payload); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{ID: returnID, TotalAmount: total, Status: "posted"}, nil
}

func validRefundMethod(v string) bool { return v == "cash" || v == "card" || v == "credit_balance" }
func refundAccount(method, balanceAccount string) string {
	if method == "cash" {
		return "CASH"
	}
	if method == "card" {
		return "BANK_CARD"
	}
	return balanceAccount
}
func weightedAverage(oldQty float64, oldAvg int64, incomingQty float64, incomingCost int64) int64 {
	newQty := oldQty + incomingQty
	if newQty <= 0 {
		return 0
	}
	return int64(math.Round((oldQty*float64(oldAvg) + incomingQty*float64(incomingCost)) / newQty))
}
func ensureAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typ string }{{"CASH", "Cash", "asset"}, {"BANK_CARD", "Card Clearing", "asset"}, {"AR", "Accounts Receivable", "asset"}, {"AP", "Accounts Payable", "liability"}, {"SALES", "Sales Revenue", "revenue"}, {"INVENTORY", "Inventory", "asset"}, {"COGS", "Cost of Goods Sold", "expense"}}
	out := map[string]uuid.UUID{}
	for _, d := range defs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id); err != nil {
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
