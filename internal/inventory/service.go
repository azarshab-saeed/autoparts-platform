package inventory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) List(ctx context.Context, tenantID, storeID, warehouseID uuid.UUID, lowOnly bool, limit, offset int) ([]Stock, error) {
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, warehouseID, tenantID, storeID).Scan(&ok); err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("warehouse does not belong to authenticated store")
	}
	rows, err := s.db.Query(ctx, `
SELECT p.id,p.title,p.sku,b.on_hand,b.reserved,(b.on_hand-b.reserved),b.avg_unit_cost,
       COALESCE(r.min_qty,0),COALESCE(r.target_qty,0),
       ((b.on_hand-b.reserved) <= COALESCE(r.min_qty,0) AND COALESCE(r.min_qty,0)>0) AS low_stock
FROM inventory_balances b
JOIN products p ON p.id=b.product_id AND p.tenant_id=b.tenant_id
LEFT JOIN inventory_reorder_points r ON r.tenant_id=b.tenant_id AND r.warehouse_id=b.warehouse_id AND r.product_id=b.product_id
WHERE b.tenant_id=$1 AND b.warehouse_id=$2 AND p.deleted_at IS NULL
  AND (NOT $3 OR ((b.on_hand-b.reserved) <= COALESCE(r.min_qty,0) AND COALESCE(r.min_qty,0)>0))
ORDER BY low_stock DESC,p.title,p.id LIMIT $4 OFFSET $5`, tenantID, warehouseID, lowOnly, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Stock, 0, limit)
	for rows.Next() {
		var x Stock
		if err = rows.Scan(&x.ProductID, &x.Title, &x.SKU, &x.OnHand, &x.Reserved, &x.Available, &x.AvgUnitCost, &x.MinQty, &x.TargetQty, &x.LowStock); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) SetReorderPoint(ctx context.Context, tenantID, storeID, warehouseID, productID uuid.UUID, minQty, targetQty float64) error {
	if minQty < 0 || targetQty < minQty {
		return errors.New("target_qty must be >= min_qty and both non-negative")
	}
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, warehouseID, tenantID, storeID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("warehouse does not belong to authenticated store")
	}
	_, err := s.db.Exec(ctx, `INSERT INTO inventory_reorder_points(tenant_id,warehouse_id,product_id,min_qty,target_qty) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,warehouse_id,product_id) DO UPDATE SET min_qty=EXCLUDED.min_qty,target_qty=EXCLUDED.target_qty,updated_at=now()`, tenantID, warehouseID, productID, minQty, targetQty)
	return err
}

func (s *Service) Adjust(ctx context.Context, cmd AdjustmentCommand) (uuid.UUID, error) {
	if cmd.QtyDelta == 0 || strings.TrimSpace(cmd.Reason) == "" {
		return uuid.Nil, errors.New("qty_delta and reason are required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var ok bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, cmd.WarehouseID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, errors.New("warehouse does not belong to authenticated store")
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM inventory_adjustments WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existing)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var onHand, reserved float64
	var avg int64
	err = tx.QueryRow(ctx, `SELECT on_hand,reserved,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, cmd.WarehouseID, cmd.ProductID).Scan(&onHand, &reserved, &avg)
	if errors.Is(err, pgx.ErrNoRows) {
		if cmd.QtyDelta < 0 {
			return uuid.Nil, errors.New("cannot reduce missing inventory balance")
		}
		_, err = tx.Exec(ctx, `INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES($1,$2,$3,0,0,0)`, cmd.TenantID, cmd.WarehouseID, cmd.ProductID)
		if err != nil {
			return uuid.Nil, err
		}
		onHand = 0
		reserved = 0
		avg = 0
	} else if err != nil {
		return uuid.Nil, err
	}
	newOnHand := onHand + cmd.QtyDelta
	if newOnHand < reserved {
		return uuid.Nil, fmt.Errorf("adjustment would make on_hand %.3f lower than reserved %.3f", newOnHand, reserved)
	}
	id := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_adjustments(id,tenant_id,store_id,warehouse_id,product_id,qty_delta,reason,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, cmd.TenantID, cmd.StoreID, cmd.WarehouseID, cmd.ProductID, cmd.QtyDelta, strings.TrimSpace(cmd.Reason), cmd.IdempotencyKey); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, cmd.TenantID, cmd.WarehouseID, cmd.ProductID, newOnHand); err != nil {
		return uuid.Nil, err
	}
	costDelta := int64(math.Round(cmd.QtyDelta * float64(avg)))
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'adjustment',$4,$5,$6,'inventory_adjustment',$7)`, cmd.TenantID, cmd.WarehouseID, cmd.ProductID, cmd.QtyDelta, avg, costDelta, id); err != nil {
		return uuid.Nil, err
	}
	value := int64(math.Round(math.Abs(cmd.QtyDelta) * float64(avg)))
	if value > 0 {
		accounts, e := ensureAdjustmentAccounts(ctx, tx, cmd.TenantID)
		if e != nil {
			return uuid.Nil, e
		}
		journalID := uuid.New()
		if _, e = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'inventory_adjustment',$3)`, journalID, cmd.TenantID, id); e != nil {
			return uuid.Nil, e
		}
		if cmd.QtyDelta < 0 {
			if e = adjustmentEntry(ctx, tx, cmd.TenantID, journalID, accounts["INV_ADJ"], value, 0); e != nil {
				return uuid.Nil, e
			}
			if e = adjustmentEntry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], 0, value); e != nil {
				return uuid.Nil, e
			}
		} else {
			if e = adjustmentEntry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], value, 0); e != nil {
				return uuid.Nil, e
			}
			if e = adjustmentEntry(ctx, tx, cmd.TenantID, journalID, accounts["INV_ADJ"], 0, value); e != nil {
				return uuid.Nil, e
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func ensureAdjustmentAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typ string }{{"INVENTORY", "Inventory", "asset"}, {"INV_ADJ", "Inventory Gain/Loss", "expense"}}
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
func adjustmentEntry(ctx context.Context, tx pgx.Tx, tenantID, journalID, accountID uuid.UUID, debit, credit int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,$5)`, tenantID, journalID, accountID, debit, credit)
	return err
}
