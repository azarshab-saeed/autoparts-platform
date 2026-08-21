package reservations

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

const reservationTTL = 30 * time.Minute
const readyTTL = 2 * time.Hour

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (Reservation, error) {
	if cmd.BuyerUserID == uuid.Nil || cmd.OfferID == uuid.Nil {
		return Reservation{}, errors.New("offer_id and authenticated buyer are required")
	}
	if cmd.Qty <= 0 || cmd.Qty > 1000 {
		return Reservation{}, errors.New("qty must be greater than zero and at most 1000")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Reservation{}, errors.New("idempotency key is required")
	}
	if cmd.BuyerRole != "mechanic" && cmd.BuyerRole != "consumer" {
		return Reservation{}, errors.New("buyer role must be mechanic or consumer")
	}
	// Free expired holds before evaluating currently available stock. A failed
	// sweep should not make reservation creation unsafe; the locked balance and
	// availability check below are still the source of truth.
	_, _ = s.ExpireDue(ctx, 100)

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)

	var existing Reservation
	err = tx.QueryRow(ctx, `
      SELECT r.id,r.offer_id,r.product_id,p.title,r.store_id,s.name,COALESCE(s.public_address,''),COALESCE(s.public_phone,''),
             r.buyer_user_id,r.buyer_name,r.buyer_email,COALESCE(r.buyer_role,''),r.sale_id,COALESCE(sale.paid_amount,0),COALESCE(sale.due_amount,0),r.qty::float8,r.unit_price,r.total_amount,r.status,r.expires_at,r.created_at,r.updated_at
      FROM network_reservations r
      JOIN stores s ON s.id=r.store_id AND s.tenant_id=r.tenant_id
      JOIN products p ON p.id=r.product_id AND p.tenant_id=r.tenant_id
      LEFT JOIN sales sale ON sale.id=r.sale_id
      WHERE r.buyer_user_id=$1 AND r.idempotency_key=$2`, cmd.BuyerUserID, cmd.IdempotencyKey).
		Scan(&existing.ID, &existing.OfferID, &existing.ProductID, &existing.ProductTitle, &existing.StoreID, &existing.StoreName, &existing.Address, &existing.Phone,
			&existing.BuyerUserID, &existing.BuyerName, &existing.BuyerEmail, &existing.BuyerRole, &existing.SaleID, &existing.PaidAmount, &existing.DueAmount, &existing.Qty, &existing.UnitPrice, &existing.TotalAmount, &existing.Status, &existing.ExpiresAt, &existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, err
	}

	var r Reservation
	var tenantID, warehouseID uuid.UUID
	var available float64
	var allow bool
	err = tx.QueryRow(ctx, `
      SELECT o.id,o.tenant_id,o.store_id,o.warehouse_id,o.product_id,p.title,s.name,
             COALESCE(s.public_address,''),COALESCE(s.public_phone,''),o.selling_price,
             (ib.on_hand-ib.reserved)::float8,o.allow_reservation
      FROM store_product_offers o
      JOIN stores s ON s.id=o.store_id AND s.tenant_id=o.tenant_id
      JOIN products p ON p.id=o.product_id AND p.tenant_id=o.tenant_id
      JOIN inventory_balances ib ON ib.tenant_id=o.tenant_id AND ib.warehouse_id=o.warehouse_id AND ib.product_id=o.product_id
      WHERE o.id=$1 AND o.visible AND s.network_enabled AND s.active AND p.active AND p.deleted_at IS NULL
      FOR UPDATE OF o,ib`, cmd.OfferID).
		Scan(&r.OfferID, &tenantID, &r.StoreID, &warehouseID, &r.ProductID, &r.ProductTitle, &r.StoreName, &r.Address, &r.Phone, &r.UnitPrice, &available, &allow)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, errors.New("offer is not available")
	}
	if err != nil {
		return Reservation{}, err
	}
	if !allow {
		return Reservation{}, errors.New("this offer does not allow reservation")
	}
	if available < cmd.Qty {
		return Reservation{}, fmt.Errorf("only %.3f item(s) are currently available", available)
	}

	totalFloat := cmd.Qty * float64(r.UnitPrice)
	if totalFloat <= 0 || totalFloat > math.MaxInt64 {
		return Reservation{}, errors.New("reservation total is invalid")
	}
	r.ID = uuid.New()
	r.BuyerUserID = cmd.BuyerUserID
	r.BuyerName = strings.TrimSpace(cmd.BuyerName)
	r.BuyerEmail = strings.TrimSpace(cmd.BuyerEmail)
	r.BuyerRole = cmd.BuyerRole
	r.Qty = cmd.Qty
	r.TotalAmount = int64(math.Round(totalFloat))
	r.Status = "pending"
	r.ExpiresAt = time.Now().Add(reservationTTL)

	if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET reserved=reserved+$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, tenantID, warehouseID, r.ProductID, r.Qty); err != nil {
		return Reservation{}, err
	}
	err = tx.QueryRow(ctx, `
      INSERT INTO network_reservations(id,tenant_id,store_id,warehouse_id,offer_id,product_id,buyer_user_id,buyer_name,buyer_email,buyer_role,qty,unit_price,total_amount,status,idempotency_key,expires_at)
      VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'pending',$14,$15)
      RETURNING created_at,updated_at`, r.ID, tenantID, r.StoreID, warehouseID, r.OfferID, r.ProductID, r.BuyerUserID, r.BuyerName, r.BuyerEmail, r.BuyerRole, r.Qty, r.UnitPrice, r.TotalAmount, cmd.IdempotencyKey, r.ExpiresAt).
		Scan(&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Reservation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,actor_user_id,event_type) VALUES($1,$2,$3,'created')`, r.ID, tenantID, r.BuyerUserID); err != nil {
		return Reservation{}, err
	}
	payload, _ := json.Marshal(map[string]any{"reservation_id": r.ID, "store_id": r.StoreID, "product_id": r.ProductID, "qty": r.Qty, "status": r.Status})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,'network.reservation.created',$3)`, tenantID, r.ID, payload); err != nil {
		return Reservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return r, nil
}

func (s *Service) ListBuyer(ctx context.Context, buyerID uuid.UUID) ([]Reservation, error) {
	if _, err := s.ExpireDue(ctx, 100); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, reservationSelect+` WHERE r.buyer_user_id=$1 ORDER BY r.created_at DESC LIMIT 100`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *Service) ListStore(ctx context.Context, tenantID, storeID uuid.UUID, status string) ([]Reservation, error) {
	if _, err := s.ExpireDue(ctx, 100); err != nil {
		return nil, err
	}
	args := []any{tenantID, storeID}
	where := ` WHERE r.tenant_id=$1 AND r.store_id=$2`
	if status != "" && status != "all" {
		args = append(args, status)
		where += ` AND r.status=$3`
	}
	rows, err := s.db.Query(ctx, reservationSelect+where+` ORDER BY CASE r.status WHEN 'pending' THEN 0 WHEN 'accepted' THEN 1 WHEN 'ready' THEN 2 ELSE 3 END,r.created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *Service) CancelBuyer(ctx context.Context, buyerID, reservationID uuid.UUID) (Reservation, error) {
	return s.transition(ctx, reservationID, buyerID, uuid.Nil, uuid.Nil, "cancelled", true)
}

func (s *Service) StoreTransition(ctx context.Context, tenantID, storeID, actorID, reservationID uuid.UUID, next string) (Reservation, error) {
	if next != "accepted" && next != "ready" && next != "rejected" {
		return Reservation{}, errors.New("status must be accepted, ready, or rejected")
	}
	return s.transition(ctx, reservationID, actorID, tenantID, storeID, next, false)
}

func (s *Service) transition(ctx context.Context, reservationID, actorID, tenantID, storeID uuid.UUID, next string, buyerAction bool) (Reservation, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)
	var current Reservation
	var rowTenant, rowWarehouse uuid.UUID
	query := reservationSelect + ` WHERE r.id=$1 FOR UPDATE OF r`
	err = tx.QueryRow(ctx, query, reservationID).Scan(reservationScan(&current, &rowTenant, &rowWarehouse)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, errors.New("reservation not found")
	}
	if err != nil {
		return Reservation{}, err
	}
	if buyerAction && current.BuyerUserID != actorID {
		return Reservation{}, errors.New("reservation does not belong to authenticated user")
	}
	if !buyerAction && (rowTenant != tenantID || current.StoreID != storeID) {
		return Reservation{}, errors.New("reservation does not belong to authenticated store")
	}
	if current.ExpiresAt.Before(time.Now()) && (current.Status == "pending" || current.Status == "accepted" || current.Status == "ready") {
		if err = releaseStock(ctx, tx, rowTenant, rowWarehouse, current.ProductID, current.Qty); err != nil {
			return Reservation{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE network_reservations SET status='expired',closed_at=now(),updated_at=now() WHERE id=$1`, current.ID); err != nil {
			return Reservation{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,event_type) VALUES($1,$2,'expired')`, current.ID, rowTenant); err != nil {
			return Reservation{}, err
		}
		payload, _ := json.Marshal(map[string]any{"reservation_id": current.ID, "status": "expired"})
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,'network.reservation.expired',$3)`, rowTenant, current.ID, payload); err != nil {
			return Reservation{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Reservation{}, err
		}
		return Reservation{}, errors.New("reservation has expired")
	}
	if !canTransition(current.Status, next, buyerAction) {
		return Reservation{}, fmt.Errorf("cannot change reservation from %s to %s", current.Status, next)
	}
	release := next == "cancelled" || next == "rejected"
	if release {
		if err = releaseStock(ctx, tx, rowTenant, rowWarehouse, current.ProductID, current.Qty); err != nil {
			return Reservation{}, err
		}
	}
	extra := ""
	if next == "accepted" {
		extra = ",accepted_at=now()"
	}
	var readyExpires time.Time
	if next == "ready" {
		readyExpires = time.Now().Add(readyTTL)
		extra = ",ready_at=now(),expires_at=$3"
	}
	if release {
		extra = ",closed_at=now()"
	}
	if next == "ready" {
		if _, err = tx.Exec(ctx, `UPDATE network_reservations SET status=$2,updated_at=now()`+extra+` WHERE id=$1`, current.ID, next, readyExpires); err != nil {
			return Reservation{}, err
		}
		current.ExpiresAt = readyExpires
	} else if _, err = tx.Exec(ctx, `UPDATE network_reservations SET status=$2,updated_at=now()`+extra+` WHERE id=$1`, current.ID, next); err != nil {
		return Reservation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,actor_user_id,event_type) VALUES($1,$2,$3,$4)`, current.ID, rowTenant, actorID, next); err != nil {
		return Reservation{}, err
	}
	payload, _ := json.Marshal(map[string]any{"reservation_id": current.ID, "status": next})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,$3,$4)`, rowTenant, current.ID, "network.reservation."+next, payload); err != nil {
		return Reservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	current.Status = next
	current.UpdatedAt = time.Now()
	if next == "cancelled" || next == "rejected" {
		current.UpdatedAt = time.Now()
	}
	return current, nil
}

func canTransition(current, next string, buyerAction bool) bool {
	if buyerAction {
		return next == "cancelled" && (current == "pending" || current == "accepted" || current == "ready")
	}
	switch next {
	case "accepted":
		return current == "pending"
	case "ready":
		return current == "accepted"
	case "rejected":
		return current == "pending" || current == "accepted"
	default:
		return false
	}
}

func (s *Service) ExpireDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,tenant_id,warehouse_id,product_id,qty::float8 FROM network_reservations WHERE status IN ('pending','accepted','ready') AND expires_at<=now() ORDER BY expires_at FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type due struct {
		id, tenant, warehouse, product uuid.UUID
		qty                            float64
	}
	var all []due
	for rows.Next() {
		var d due
		if err = rows.Scan(&d.id, &d.tenant, &d.warehouse, &d.product, &d.qty); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, d)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, d := range all {
		if err = releaseStock(ctx, tx, d.tenant, d.warehouse, d.product, d.qty); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `UPDATE network_reservations SET status='expired',closed_at=now(),updated_at=now() WHERE id=$1`, d.id); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO network_reservation_events(reservation_id,tenant_id,event_type) VALUES($1,$2,'expired')`, d.id, d.tenant); err != nil {
			return 0, err
		}
		payload, _ := json.Marshal(map[string]any{"reservation_id": d.id, "status": "expired"})
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_reservation',$2,'network.reservation.expired',$3)`, d.tenant, d.id, payload); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(all), nil
}

func releaseStock(ctx context.Context, tx pgx.Tx, tenantID, warehouseID, productID uuid.UUID, qty float64) error {
	ct, err := tx.Exec(ctx, `UPDATE inventory_balances SET reserved=reserved-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND reserved >= $4`, tenantID, warehouseID, productID, qty)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return errors.New("reserved inventory invariant violated")
	}
	return nil
}

const reservationSelect = `
 SELECT r.id,r.offer_id,r.product_id,p.title,r.store_id,s.name,COALESCE(s.public_address,''),COALESCE(s.public_phone,''),
        r.buyer_user_id,r.buyer_name,r.buyer_email,COALESCE(r.buyer_role,''),r.sale_id,COALESCE(sale.paid_amount,0),COALESCE(sale.due_amount,0),r.qty::float8,r.unit_price,r.total_amount,r.status,r.expires_at,r.created_at,r.updated_at,
        r.tenant_id,r.warehouse_id
 FROM network_reservations r
 JOIN stores s ON s.id=r.store_id AND s.tenant_id=r.tenant_id
 JOIN products p ON p.id=r.product_id AND p.tenant_id=r.tenant_id
 LEFT JOIN sales sale ON sale.id=r.sale_id`

func reservationScan(r *Reservation, tenantID, warehouseID *uuid.UUID) []any {
	return []any{&r.ID, &r.OfferID, &r.ProductID, &r.ProductTitle, &r.StoreID, &r.StoreName, &r.Address, &r.Phone, &r.BuyerUserID, &r.BuyerName, &r.BuyerEmail, &r.BuyerRole, &r.SaleID, &r.PaidAmount, &r.DueAmount, &r.Qty, &r.UnitPrice, &r.TotalAmount, &r.Status, &r.ExpiresAt, &r.CreatedAt, &r.UpdatedAt, tenantID, warehouseID}
}

type rowScanner interface{ Scan(...any) error }

func scanReservation(row rowScanner) (Reservation, error) {
	var r Reservation
	var t, w uuid.UUID
	err := row.Scan(reservationScan(&r, &t, &w)...)
	return r, err
}
func scanRows(rows pgx.Rows) ([]Reservation, error) {
	out := []Reservation{}
	for rows.Next() {
		r, err := scanReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
