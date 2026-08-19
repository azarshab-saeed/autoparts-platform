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
)

type FulfillReservationCommand struct {
	TenantID       uuid.UUID
	StoreID        uuid.UUID
	ActorUserID    uuid.UUID
	ReservationID  uuid.UUID
	CustomerID     *uuid.UUID
	PaymentMethod  string
	Payments       []PaymentPart
	IdempotencyKey string
}

type ReservationFulfillment struct {
	ReservationID uuid.UUID `json:"reservation_id"`
	SaleID        uuid.UUID `json:"sale_id"`
	TotalAmount   int64     `json:"total_amount"`
	PaidAmount    int64     `json:"paid_amount"`
	DueAmount     int64     `json:"due_amount"`
	Status        string    `json:"status"`
}

func (s *Service) FulfillReservation(ctx context.Context, cmd FulfillReservationCommand) (ReservationFulfillment, error) {
	if cmd.TenantID == uuid.Nil || cmd.StoreID == uuid.Nil || cmd.ActorUserID == uuid.Nil || cmd.ReservationID == uuid.Nil {
		return ReservationFulfillment{}, errors.New("authenticated store, actor and reservation are required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return ReservationFulfillment{}, errors.New("idempotency key is required")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ReservationFulfillment{}, err
	}
	defer tx.Rollback(ctx)

	var warehouseID, productID uuid.UUID
	var qty float64
	var unitPrice, total int64
	var status string
	var expiresAt time.Time
	var existingSaleID *uuid.UUID
	err = tx.QueryRow(ctx, `
      SELECT warehouse_id,product_id,qty::float8,unit_price,total_amount,status,expires_at,sale_id
      FROM network_reservations
      WHERE id=$1 AND tenant_id=$2 AND store_id=$3
      FOR UPDATE`, cmd.ReservationID, cmd.TenantID, cmd.StoreID).
		Scan(&warehouseID, &productID, &qty, &unitPrice, &total, &status, &expiresAt, &existingSaleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReservationFulfillment{}, errors.New("reservation not found for authenticated store")
	}
	if err != nil {
		return ReservationFulfillment{}, err
	}

	if status == "fulfilled" && existingSaleID != nil {
		var out ReservationFulfillment
		out.ReservationID = cmd.ReservationID
		out.SaleID = *existingSaleID
		err = tx.QueryRow(ctx, `SELECT total_amount,paid_amount,due_amount,status FROM sales WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, out.SaleID, cmd.TenantID, cmd.StoreID).
			Scan(&out.TotalAmount, &out.PaidAmount, &out.DueAmount, &out.Status)
		if err != nil {
			return ReservationFulfillment{}, err
		}
		return out, tx.Commit(ctx)
	}
	if status != "ready" {
		return ReservationFulfillment{}, fmt.Errorf("reservation must be ready before fulfillment; current status is %s", status)
	}

	if !expiresAt.After(time.Now()) {
		ct, e := tx.Exec(ctx, `UPDATE inventory_balances SET reserved=reserved-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND reserved >= $4`, cmd.TenantID, warehouseID, productID, qty)
		if e != nil {
			return ReservationFulfillment{}, e
		}
		if ct.RowsAffected() != 1 {
			return ReservationFulfillment{}, errors.New("reserved inventory invariant violated while expiring reservation")
		}
		if _, e = tx.Exec(ctx, `UPDATE network_reservations SET status='expired',closed_at=now(),updated_at=now() WHERE id=$1`, cmd.ReservationID); e != nil {
			return ReservationFulfillment{}, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,actor_user_id,event_type) VALUES($1,$2,$3,'expired')`, cmd.ReservationID, cmd.TenantID, cmd.ActorUserID); e != nil {
			return ReservationFulfillment{}, e
		}
		payload, _ := json.Marshal(map[string]any{"reservation_id": cmd.ReservationID, "status": "expired"})
		if _, e = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,'network.reservation.expired',$3)`, cmd.TenantID, cmd.ReservationID, payload); e != nil {
			return ReservationFulfillment{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return ReservationFulfillment{}, e
		}
		return ReservationFulfillment{}, errors.New("reservation has expired")
	}

	if cmd.CustomerID != nil {
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, *cmd.CustomerID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
			return ReservationFulfillment{}, err
		}
		if !ok {
			return ReservationFulfillment{}, errors.New("customer does not belong to authenticated store")
		}
	}

	payments, paid, due, err := normalizePayments(total, cmd.PaymentMethod, cmd.Payments)
	if err != nil {
		return ReservationFulfillment{}, err
	}
	if due > 0 && cmd.CustomerID == nil {
		return ReservationFulfillment{}, errors.New("partial or credit fulfillment requires customer_id")
	}

	expectedTotal := int64(math.Round(qty * float64(unitPrice)))
	if expectedTotal != total {
		return ReservationFulfillment{}, errors.New("reservation amount invariant violated")
	}

	var onHand, reserved float64
	var avgUnitCost int64
	err = tx.QueryRow(ctx, `SELECT on_hand::float8,reserved::float8,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, cmd.TenantID, warehouseID, productID).
		Scan(&onHand, &reserved, &avgUnitCost)
	if err != nil {
		return ReservationFulfillment{}, fmt.Errorf("inventory lookup: %w", err)
	}
	if onHand < qty || reserved < qty {
		return ReservationFulfillment{}, errors.New("reserved inventory invariant violated")
	}

	saleID := uuid.New()
	saleIdempotencyKey := "network-reservation:" + cmd.ReservationID.String()
	if _, err = tx.Exec(ctx, `INSERT INTO sales(id,tenant_id,store_id,warehouse_id,customer_id,status,gross_amount,discount_amount,net_amount,total_amount,paid_amount,due_amount,idempotency_key) VALUES($1,$2,$3,$4,$5,'posted',$6,0,$6,$6,$7,$8,$9)`, saleID, cmd.TenantID, cmd.StoreID, warehouseID, cmd.CustomerID, total, paid, due, saleIdempotencyKey); err != nil {
		return ReservationFulfillment{}, err
	}

	itemCOGS := int64(math.Round(qty * float64(avgUnitCost)))
	if itemCOGS < 0 {
		return ReservationFulfillment{}, errors.New("invalid cost of goods sold")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sale_items(tenant_id,sale_id,product_id,qty,unit_price,unit_cost,line_total,gross_line_total,discount_amount,price_source,tax_base_amount,total_with_tax) VALUES($1,$2,$3,$4,$5,$6,$7,$7,0,'network_reservation',$7,$7)`, cmd.TenantID, saleID, productID, qty, unitPrice, avgUnitCost, total); err != nil {
		return ReservationFulfillment{}, err
	}
	ct, err := tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=on_hand-$4,reserved=reserved-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND on_hand >= $4 AND reserved >= $4`, cmd.TenantID, warehouseID, productID, qty)
	if err != nil {
		return ReservationFulfillment{}, err
	}
	if ct.RowsAffected() != 1 {
		return ReservationFulfillment{}, errors.New("could not consume reserved inventory")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'sale',$4,$5,$6,'sale',$7)`, cmd.TenantID, warehouseID, productID, -qty, avgUnitCost, -itemCOGS, saleID); err != nil {
		return ReservationFulfillment{}, err
	}

	accounts, err := ensureDefaultAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return ReservationFulfillment{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'sale',$3)`, journalID, cmd.TenantID, saleID); err != nil {
		return ReservationFulfillment{}, err
	}
	for _, p := range payments {
		code := "CASH"
		if p.Method == "card" {
			code = "BANK_CARD"
		}
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts[code], p.Amount, 0); err != nil {
			return ReservationFulfillment{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO payments(tenant_id,store_id,sale_id,method,amount) VALUES($1,$2,$3,$4,$5)`, cmd.TenantID, cmd.StoreID, saleID, p.Method, p.Amount); err != nil {
			return ReservationFulfillment{}, err
		}
	}
	if due > 0 {
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["AR"], due, 0); err != nil {
			return ReservationFulfillment{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'sale',$4,$5,0)`, cmd.TenantID, cmd.StoreID, cmd.CustomerID, saleID, due); err != nil {
			return ReservationFulfillment{}, err
		}
	}
	if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["SALES"], 0, total); err != nil {
		return ReservationFulfillment{}, err
	}
	if itemCOGS > 0 {
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["COGS"], itemCOGS, 0); err != nil {
			return ReservationFulfillment{}, err
		}
		if err = insertEntry(ctx, tx, cmd.TenantID, journalID, accounts["INVENTORY"], 0, itemCOGS); err != nil {
			return ReservationFulfillment{}, err
		}
	}

	if _, err = tx.Exec(ctx, `UPDATE network_reservations SET status='fulfilled',sale_id=$2,fulfilled_at=now(),fulfilled_by_user_id=$3,closed_at=now(),updated_at=now() WHERE id=$1`, cmd.ReservationID, saleID, cmd.ActorUserID); err != nil {
		return ReservationFulfillment{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,actor_user_id,event_type) VALUES($1,$2,$3,'fulfilled')`, cmd.ReservationID, cmd.TenantID, cmd.ActorUserID); err != nil {
		return ReservationFulfillment{}, err
	}

	salePayload, _ := json.Marshal(map[string]any{"sale_id": saleID, "reservation_id": cmd.ReservationID, "total_amount": total, "paid_amount": paid, "due_amount": due, "warehouse_id": warehouseID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'sale',$2,'sale.created',$3)`, cmd.TenantID, saleID, salePayload); err != nil {
		return ReservationFulfillment{}, err
	}
	reservationPayload, _ := json.Marshal(map[string]any{"reservation_id": cmd.ReservationID, "sale_id": saleID, "status": "fulfilled"})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,'network.reservation.fulfilled',$3)`, cmd.TenantID, cmd.ReservationID, reservationPayload); err != nil {
		return ReservationFulfillment{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return ReservationFulfillment{}, err
	}
	return ReservationFulfillment{ReservationID: cmd.ReservationID, SaleID: saleID, TotalAmount: total, PaidAmount: paid, DueAmount: due, Status: "posted"}, nil
}
