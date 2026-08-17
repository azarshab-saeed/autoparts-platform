package procurement

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

const (
	requestedTTL = 12 * time.Hour
	acceptedTTL  = 24 * time.Hour
	readyTTL     = 48 * time.Hour
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (Order, error) {
	if cmd.BuyerTenantID == uuid.Nil || cmd.BuyerStoreID == uuid.Nil || cmd.BuyerWarehouseID == uuid.Nil || cmd.BuyerProductID == uuid.Nil || cmd.ActorUserID == uuid.Nil || cmd.OfferID == uuid.Nil {
		return Order{}, errors.New("authenticated buyer store, warehouse, product and offer are required")
	}
	if cmd.Qty <= 0 || cmd.Qty > 1000 {
		return Order{}, errors.New("qty must be greater than zero and at most 1000")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Order{}, errors.New("idempotency key is required")
	}
	_, _ = s.ExpireDue(ctx, 100)

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	if existing, found, err := getByIdempotency(ctx, tx, cmd.BuyerTenantID, cmd.IdempotencyKey); err != nil {
		return Order{}, err
	} else if found {
		return existing, tx.Commit(ctx)
	}

	var buyerProductTitle, buyerSKU, buyerOEM, buyerNormalized string
	var warehouseOK bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, cmd.BuyerWarehouseID, cmd.BuyerTenantID, cmd.BuyerStoreID).Scan(&warehouseOK); err != nil {
		return Order{}, err
	}
	if !warehouseOK {
		return Order{}, errors.New("destination warehouse does not belong to authenticated store")
	}
	if err = tx.QueryRow(ctx, `SELECT title,COALESCE(sku,''),COALESCE(oem_code,''),normalized_title FROM products WHERE id=$1 AND tenant_id=$2 AND active AND deleted_at IS NULL`, cmd.BuyerProductID, cmd.BuyerTenantID).
		Scan(&buyerProductTitle, &buyerSKU, &buyerOEM, &buyerNormalized); errors.Is(err, pgx.ErrNoRows) {
		return Order{}, errors.New("destination product does not belong to authenticated tenant")
	} else if err != nil {
		return Order{}, err
	}

	var out Order
	var sellerTenantID uuid.UUID
	var sellerSKU, sellerOEM, sellerNormalized string
	var available float64
	var allowProcurement bool
	err = tx.QueryRow(ctx, `
		SELECT o.id,o.tenant_id,o.store_id,s.name,o.warehouse_id,o.product_id,p.title,
		       COALESCE(p.sku,''),COALESCE(p.oem_code,''),p.normalized_title,o.selling_price,
		       (ib.on_hand-ib.reserved)::float8,o.allow_procurement
		FROM store_product_offers o
		JOIN stores s ON s.id=o.store_id AND s.tenant_id=o.tenant_id
		JOIN products p ON p.id=o.product_id AND p.tenant_id=o.tenant_id
		JOIN inventory_balances ib ON ib.tenant_id=o.tenant_id AND ib.warehouse_id=o.warehouse_id AND ib.product_id=o.product_id
		WHERE o.id=$1 AND o.visible AND s.network_enabled AND s.active AND p.active AND p.deleted_at IS NULL
		FOR UPDATE OF o,ib`, cmd.OfferID).
		Scan(&out.OfferID, &sellerTenantID, &out.SellerStoreID, &out.SellerStoreName, &out.SellerWarehouseID, &out.SellerProductID, &out.SellerProductTitle,
			&sellerSKU, &sellerOEM, &sellerNormalized, &out.UnitPrice, &available, &allowProcurement)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, errors.New("network offer is not available")
	}
	if err != nil {
		return Order{}, err
	}
	if !allowProcurement {
		return Order{}, errors.New("seller has disabled store-to-store procurement for this offer")
	}
	if out.SellerStoreID == cmd.BuyerStoreID {
		return Order{}, errors.New("a store cannot procure from itself")
	}
	if available < cmd.Qty {
		return Order{}, fmt.Errorf("only %.3f item(s) are currently available", available)
	}
	if !productsCompatible(buyerSKU, buyerOEM, buyerNormalized, sellerSKU, sellerOEM, sellerNormalized) {
		return Order{}, errors.New("selected network offer does not match the destination product")
	}

	totalFloat := cmd.Qty * float64(out.UnitPrice)
	if totalFloat <= 0 || totalFloat > math.MaxInt64 {
		return Order{}, errors.New("procurement total is invalid")
	}
	out.ID = uuid.New()
	out.BuyerStoreID = cmd.BuyerStoreID
	out.BuyerWarehouseID = cmd.BuyerWarehouseID
	out.BuyerProductID = cmd.BuyerProductID
	out.BuyerProductTitle = buyerProductTitle
	out.Qty = cmd.Qty
	out.TotalAmount = int64(math.Round(totalFloat))
	out.Status = StatusRequested
	out.ExpiresAt = time.Now().Add(requestedTTL)

	if err = tx.QueryRow(ctx, `SELECT name FROM stores WHERE id=$1 AND tenant_id=$2 AND active`, cmd.BuyerStoreID, cmd.BuyerTenantID).Scan(&out.BuyerStoreName); err != nil {
		return Order{}, errors.New("authenticated buyer store not found")
	}
	ct, err := tx.Exec(ctx, `UPDATE inventory_balances SET reserved=reserved+$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND on_hand-reserved >= $4`, sellerTenantID, out.SellerWarehouseID, out.SellerProductID, cmd.Qty)
	if err != nil {
		return Order{}, err
	}
	if ct.RowsAffected() != 1 {
		return Order{}, errors.New("network inventory changed; refresh and try again")
	}
	if err = tx.QueryRow(ctx, `
		INSERT INTO network_procurements(
		  id,buyer_tenant_id,buyer_store_id,buyer_warehouse_id,buyer_product_id,
		  seller_tenant_id,seller_store_id,seller_warehouse_id,seller_product_id,offer_id,
		  requested_by_user_id,qty,unit_price,total_amount,status,idempotency_key,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'requested',$15,$16)
		RETURNING created_at,updated_at`, out.ID, cmd.BuyerTenantID, cmd.BuyerStoreID, cmd.BuyerWarehouseID, cmd.BuyerProductID,
		sellerTenantID, out.SellerStoreID, out.SellerWarehouseID, out.SellerProductID, out.OfferID,
		cmd.ActorUserID, out.Qty, out.UnitPrice, out.TotalAmount, cmd.IdempotencyKey, out.ExpiresAt).
		Scan(&out.CreatedAt, &out.UpdatedAt); err != nil {
		return Order{}, err
	}
	if err = addEvent(ctx, tx, out.ID, cmd.BuyerTenantID, cmd.ActorUserID, "buyer", StatusRequested); err != nil {
		return Order{}, err
	}
	if err = emitForBoth(ctx, tx, cmd.BuyerTenantID, sellerTenantID, out.ID, "network.procurement.requested", map[string]any{"procurement_id": out.ID, "buyer_store_id": cmd.BuyerStoreID, "seller_store_id": out.SellerStoreID, "qty": out.Qty, "total_amount": out.TotalAmount}); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return out, nil
}

func (s *Service) ListBuyer(ctx context.Context, tenantID, storeID uuid.UUID, status string) ([]Order, error) {
	_, _ = s.ExpireDue(ctx, 100)
	return s.list(ctx, "buyer", tenantID, storeID, status)
}

func (s *Service) ListSeller(ctx context.Context, tenantID, storeID uuid.UUID, status string) ([]Order, error) {
	_, _ = s.ExpireDue(ctx, 100)
	return s.list(ctx, "seller", tenantID, storeID, status)
}

func (s *Service) list(ctx context.Context, side string, tenantID, storeID uuid.UUID, status string) ([]Order, error) {
	where := "p.buyer_tenant_id=$1 AND p.buyer_store_id=$2"
	if side == "seller" {
		where = "p.seller_tenant_id=$1 AND p.seller_store_id=$2"
	}
	args := []any{tenantID, storeID}
	if status != "" && status != "all" {
		args = append(args, status)
		where += " AND p.status=$3"
	}
	rows, err := s.db.Query(ctx, orderSelect+" WHERE "+where+` ORDER BY CASE p.status WHEN 'requested' THEN 0 WHEN 'accepted' THEN 1 WHEN 'ready' THEN 2 ELSE 3 END,p.created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var x Order
		if err := scanOrder(rows, &x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) SellerTransition(ctx context.Context, tenantID, storeID, actorID, procurementID uuid.UUID, next Status) (Order, error) {
	if next != StatusAccepted && next != StatusReady && next != StatusRejected {
		return Order{}, errors.New("seller status must be accepted, ready, or rejected")
	}
	return s.transition(ctx, procurementID, actorID, tenantID, storeID, "seller", next)
}

func (s *Service) BuyerCancel(ctx context.Context, tenantID, storeID, actorID, procurementID uuid.UUID) (Order, error) {
	return s.transition(ctx, procurementID, actorID, tenantID, storeID, "buyer", StatusCancelled)
}

func (s *Service) transition(ctx context.Context, procurementID, actorID, tenantID, storeID uuid.UUID, side string, next Status) (Order, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	var current Order
	var buyerTenantID, sellerTenantID uuid.UUID
	err = tx.QueryRow(ctx, orderSelect+` WHERE p.id=$1 FOR UPDATE OF p`, procurementID).Scan(orderScan(&current, &buyerTenantID, &sellerTenantID)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, errors.New("procurement order not found")
	}
	if err != nil {
		return Order{}, err
	}
	if side == "seller" && (sellerTenantID != tenantID || current.SellerStoreID != storeID) {
		return Order{}, errors.New("procurement order does not belong to authenticated seller store")
	}
	if side == "buyer" && (buyerTenantID != tenantID || current.BuyerStoreID != storeID) {
		return Order{}, errors.New("procurement order does not belong to authenticated buyer store")
	}
	if isActive(current.Status) && !current.ExpiresAt.After(time.Now()) {
		if err := expireLocked(ctx, tx, current, buyerTenantID, sellerTenantID); err != nil {
			return Order{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Order{}, err
		}
		return Order{}, errors.New("procurement order has expired")
	}
	if !canTransition(current.Status, next, side) {
		return Order{}, fmt.Errorf("cannot change procurement from %s to %s", current.Status, next)
	}

	release := next == StatusRejected || next == StatusCancelled
	if release {
		if err := releaseHold(ctx, tx, sellerTenantID, current.SellerWarehouseID, current.SellerProductID, current.Qty); err != nil {
			return Order{}, err
		}
	}
	nextExpiry := current.ExpiresAt
	sqlExtra := ""
	switch next {
	case StatusAccepted:
		nextExpiry = time.Now().Add(acceptedTTL)
		sqlExtra = ",accepted_at=now(),expires_at=$3"
	case StatusReady:
		nextExpiry = time.Now().Add(readyTTL)
		sqlExtra = ",ready_at=now(),expires_at=$3"
	case StatusRejected, StatusCancelled:
		sqlExtra = ",closed_at=now()"
	}
	if next == StatusAccepted || next == StatusReady {
		_, err = tx.Exec(ctx, `UPDATE network_procurements SET status=$2,updated_at=now()`+sqlExtra+` WHERE id=$1`, current.ID, next, nextExpiry)
	} else {
		_, err = tx.Exec(ctx, `UPDATE network_procurements SET status=$2,updated_at=now()`+sqlExtra+` WHERE id=$1`, current.ID, next)
	}
	if err != nil {
		return Order{}, err
	}
	eventTenant := buyerTenantID
	if side == "seller" {
		eventTenant = sellerTenantID
	}
	if err = addEvent(ctx, tx, current.ID, eventTenant, actorID, side, next); err != nil {
		return Order{}, err
	}
	if err = emitForBoth(ctx, tx, buyerTenantID, sellerTenantID, current.ID, "network.procurement."+string(next), map[string]any{"procurement_id": current.ID, "status": next}); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	current.Status = next
	current.ExpiresAt = nextExpiry
	current.UpdatedAt = time.Now()
	return current, nil
}

func (s *Service) Receive(ctx context.Context, cmd ReceiveCommand) (ReceiveResult, error) {
	if cmd.BuyerTenantID == uuid.Nil || cmd.BuyerStoreID == uuid.Nil || cmd.ActorUserID == uuid.Nil || cmd.ProcurementID == uuid.Nil {
		return ReceiveResult{}, errors.New("authenticated buyer store, actor and procurement are required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return ReceiveResult{}, errors.New("idempotency key is required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ReceiveResult{}, err
	}
	defer tx.Rollback(ctx)

	var order Order
	var buyerTenantID, sellerTenantID uuid.UUID
	err = tx.QueryRow(ctx, orderSelect+` WHERE p.id=$1 FOR UPDATE OF p`, cmd.ProcurementID).Scan(orderScan(&order, &buyerTenantID, &sellerTenantID)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReceiveResult{}, errors.New("procurement order not found")
	}
	if err != nil {
		return ReceiveResult{}, err
	}
	if buyerTenantID != cmd.BuyerTenantID || order.BuyerStoreID != cmd.BuyerStoreID {
		return ReceiveResult{}, errors.New("procurement order does not belong to authenticated buyer store")
	}
	if order.Status == StatusReceived && order.SellerSaleID != nil && order.BuyerPurchaseID != nil {
		return ReceiveResult{ProcurementID: order.ID, SellerSaleID: *order.SellerSaleID, BuyerPurchaseID: *order.BuyerPurchaseID, TotalAmount: order.TotalAmount, Status: StatusReceived}, tx.Commit(ctx)
	}
	if order.Status != StatusReady {
		return ReceiveResult{}, fmt.Errorf("procurement must be ready before receiving; current status is %s", order.Status)
	}
	if !order.ExpiresAt.After(time.Now()) {
		if err = expireLocked(ctx, tx, order, buyerTenantID, sellerTenantID); err != nil {
			return ReceiveResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return ReceiveResult{}, err
		}
		return ReceiveResult{}, errors.New("procurement order has expired")
	}

	var sellerOnHand, sellerReserved float64
	var sellerAvgCost int64
	if err = tx.QueryRow(ctx, `SELECT on_hand::float8,reserved::float8,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, sellerTenantID, order.SellerWarehouseID, order.SellerProductID).
		Scan(&sellerOnHand, &sellerReserved, &sellerAvgCost); err != nil {
		return ReceiveResult{}, fmt.Errorf("seller inventory lookup: %w", err)
	}
	if sellerOnHand < order.Qty || sellerReserved < order.Qty {
		return ReceiveResult{}, errors.New("seller reserved inventory invariant violated")
	}

	var buyerOnHand float64
	var buyerAvgCost int64
	err = tx.QueryRow(ctx, `SELECT on_hand::float8,avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 FOR UPDATE`, buyerTenantID, order.BuyerWarehouseID, order.BuyerProductID).
		Scan(&buyerOnHand, &buyerAvgCost)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES($1,$2,$3,0,0,0)`, buyerTenantID, order.BuyerWarehouseID, order.BuyerProductID); err != nil {
			return ReceiveResult{}, err
		}
		buyerOnHand, buyerAvgCost = 0, 0
	} else if err != nil {
		return ReceiveResult{}, err
	}

	customerID, err := ensureNetworkCustomer(ctx, tx, sellerTenantID, order.SellerStoreID, order.BuyerStoreID)
	if err != nil {
		return ReceiveResult{}, err
	}
	supplierID, err := ensureNetworkSupplier(ctx, tx, buyerTenantID, order.BuyerStoreID, order.SellerStoreID)
	if err != nil {
		return ReceiveResult{}, err
	}

	sellerSaleID := uuid.New()
	buyerPurchaseID := uuid.New()
	sellerKey := "network-procurement-seller:" + order.ID.String()
	buyerKey := "network-procurement-buyer:" + order.ID.String()

	if _, err = tx.Exec(ctx, `INSERT INTO sales(id,tenant_id,store_id,warehouse_id,customer_id,status,total_amount,paid_amount,due_amount,idempotency_key) VALUES($1,$2,$3,$4,$5,'posted',$6,0,$6,$7)`, sellerSaleID, sellerTenantID, order.SellerStoreID, order.SellerWarehouseID, customerID, order.TotalAmount, sellerKey); err != nil {
		return ReceiveResult{}, err
	}
	sellerCOGS := int64(math.Round(order.Qty * float64(sellerAvgCost)))
	if _, err = tx.Exec(ctx, `INSERT INTO sale_items(tenant_id,sale_id,product_id,qty,unit_price,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6,$7)`, sellerTenantID, sellerSaleID, order.SellerProductID, order.Qty, order.UnitPrice, sellerAvgCost, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}
	ct, err := tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=on_hand-$4,reserved=reserved-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND on_hand >= $4 AND reserved >= $4`, sellerTenantID, order.SellerWarehouseID, order.SellerProductID, order.Qty)
	if err != nil {
		return ReceiveResult{}, err
	}
	if ct.RowsAffected() != 1 {
		return ReceiveResult{}, errors.New("could not consume seller reserved inventory")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'sale',$4,$5,$6,'sale',$7)`, sellerTenantID, order.SellerWarehouseID, order.SellerProductID, -order.Qty, sellerAvgCost, -sellerCOGS, sellerSaleID); err != nil {
		return ReceiveResult{}, err
	}

	sellerAccounts, err := ensureAccounts(ctx, tx, sellerTenantID)
	if err != nil {
		return ReceiveResult{}, err
	}
	sellerJournal := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'sale',$3)`, sellerJournal, sellerTenantID, sellerSaleID); err != nil {
		return ReceiveResult{}, err
	}
	if err = addEntry(ctx, tx, sellerTenantID, sellerJournal, sellerAccounts["AR"], order.TotalAmount, 0); err != nil {
		return ReceiveResult{}, err
	}
	if err = addEntry(ctx, tx, sellerTenantID, sellerJournal, sellerAccounts["SALES"], 0, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}
	if sellerCOGS > 0 {
		if err = addEntry(ctx, tx, sellerTenantID, sellerJournal, sellerAccounts["COGS"], sellerCOGS, 0); err != nil {
			return ReceiveResult{}, err
		}
		if err = addEntry(ctx, tx, sellerTenantID, sellerJournal, sellerAccounts["INVENTORY"], 0, sellerCOGS); err != nil {
			return ReceiveResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'sale',$4,$5,0)`, sellerTenantID, order.SellerStoreID, customerID, sellerSaleID, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO purchases(id,tenant_id,store_id,warehouse_id,supplier_id,status,payment_method,total_amount,paid_amount,due_amount,idempotency_key) VALUES($1,$2,$3,$4,$5,'posted','credit',$6,0,$6,$7)`, buyerPurchaseID, buyerTenantID, order.BuyerStoreID, order.BuyerWarehouseID, supplierID, order.TotalAmount, buyerKey); err != nil {
		return ReceiveResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO purchase_items(tenant_id,purchase_id,product_id,qty,unit_cost,line_total) VALUES($1,$2,$3,$4,$5,$6)`, buyerTenantID, buyerPurchaseID, order.BuyerProductID, order.Qty, order.UnitPrice, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}
	newBuyerQty := buyerOnHand + order.Qty
	newBuyerAvg := weightedAverage(buyerOnHand, buyerAvgCost, order.Qty, order.UnitPrice)
	if _, err = tx.Exec(ctx, `UPDATE inventory_balances SET on_hand=$4,avg_unit_cost=$5,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, buyerTenantID, order.BuyerWarehouseID, order.BuyerProductID, newBuyerQty, newBuyerAvg); err != nil {
		return ReceiveResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(tenant_id,warehouse_id,product_id,movement_type,qty_delta,unit_cost,cost_delta,reference_type,reference_id) VALUES($1,$2,$3,'purchase',$4,$5,$6,'purchase',$7)`, buyerTenantID, order.BuyerWarehouseID, order.BuyerProductID, order.Qty, order.UnitPrice, order.TotalAmount, buyerPurchaseID); err != nil {
		return ReceiveResult{}, err
	}
	buyerAccounts, err := ensureAccounts(ctx, tx, buyerTenantID)
	if err != nil {
		return ReceiveResult{}, err
	}
	buyerJournal := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'purchase',$3)`, buyerJournal, buyerTenantID, buyerPurchaseID); err != nil {
		return ReceiveResult{}, err
	}
	if err = addEntry(ctx, tx, buyerTenantID, buyerJournal, buyerAccounts["INVENTORY"], order.TotalAmount, 0); err != nil {
		return ReceiveResult{}, err
	}
	if err = addEntry(ctx, tx, buyerTenantID, buyerJournal, buyerAccounts["AP"], 0, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'purchase',$4,0,$5)`, buyerTenantID, order.BuyerStoreID, supplierID, buyerPurchaseID, order.TotalAmount); err != nil {
		return ReceiveResult{}, err
	}

	if _, err = tx.Exec(ctx, `UPDATE network_procurements SET status='received',seller_sale_id=$2,buyer_purchase_id=$3,received_at=now(),received_by_user_id=$4,closed_at=now(),updated_at=now() WHERE id=$1`, order.ID, sellerSaleID, buyerPurchaseID, cmd.ActorUserID); err != nil {
		return ReceiveResult{}, err
	}
	if err = addEvent(ctx, tx, order.ID, buyerTenantID, cmd.ActorUserID, "buyer", StatusReceived); err != nil {
		return ReceiveResult{}, err
	}
	if err = emitForBoth(ctx, tx, buyerTenantID, sellerTenantID, order.ID, "network.procurement.received", map[string]any{"procurement_id": order.ID, "seller_sale_id": sellerSaleID, "buyer_purchase_id": buyerPurchaseID, "total_amount": order.TotalAmount}); err != nil {
		return ReceiveResult{}, err
	}
	salePayload, _ := json.Marshal(map[string]any{"sale_id": sellerSaleID, "procurement_id": order.ID, "total_amount": order.TotalAmount, "paid_amount": 0, "due_amount": order.TotalAmount, "warehouse_id": order.SellerWarehouseID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'sale',$2,'sale.created',$3)`, sellerTenantID, sellerSaleID, salePayload); err != nil {
		return ReceiveResult{}, err
	}
	purchasePayload, _ := json.Marshal(map[string]any{"purchase_id": buyerPurchaseID, "procurement_id": order.ID, "total_amount": order.TotalAmount, "paid_amount": 0, "due_amount": order.TotalAmount, "warehouse_id": order.BuyerWarehouseID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'purchase',$2,'purchase.created',$3)`, buyerTenantID, buyerPurchaseID, purchasePayload); err != nil {
		return ReceiveResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ReceiveResult{}, err
	}
	return ReceiveResult{ProcurementID: order.ID, SellerSaleID: sellerSaleID, BuyerPurchaseID: buyerPurchaseID, TotalAmount: order.TotalAmount, Status: StatusReceived}, nil
}

func (s *Service) ExpireDue(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id FROM network_procurements WHERE status IN ('requested','accepted','ready') AND expires_at<=now() ORDER BY expires_at FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		var o Order
		var buyerTenantID, sellerTenantID uuid.UUID
		if err = tx.QueryRow(ctx, orderSelect+` WHERE p.id=$1`, id).Scan(orderScan(&o, &buyerTenantID, &sellerTenantID)...); err != nil {
			return 0, err
		}
		if err = expireLocked(ctx, tx, o, buyerTenantID, sellerTenantID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func expireLocked(ctx context.Context, tx pgx.Tx, o Order, buyerTenantID, sellerTenantID uuid.UUID) error {
	if err := releaseHold(ctx, tx, sellerTenantID, o.SellerWarehouseID, o.SellerProductID, o.Qty); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE network_procurements SET status='expired',closed_at=now(),updated_at=now() WHERE id=$1 AND status IN ('requested','accepted','ready')`, o.ID); err != nil {
		return err
	}
	if err := addEvent(ctx, tx, o.ID, buyerTenantID, uuid.Nil, "system", StatusExpired); err != nil {
		return err
	}
	return emitForBoth(ctx, tx, buyerTenantID, sellerTenantID, o.ID, "network.procurement.expired", map[string]any{"procurement_id": o.ID, "status": StatusExpired})
}

func releaseHold(ctx context.Context, tx pgx.Tx, tenantID, warehouseID, productID uuid.UUID, qty float64) error {
	ct, err := tx.Exec(ctx, `UPDATE inventory_balances SET reserved=reserved-$4,updated_at=now() WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3 AND reserved >= $4`, tenantID, warehouseID, productID, qty)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return errors.New("reserved inventory invariant violated while releasing procurement hold")
	}
	return nil
}

func canTransition(current Status, next Status, side string) bool {
	if side == "buyer" {
		return next == StatusCancelled && (current == StatusRequested || current == StatusAccepted || current == StatusReady)
	}
	switch next {
	case StatusAccepted:
		return current == StatusRequested
	case StatusReady:
		return current == StatusAccepted
	case StatusRejected:
		return current == StatusRequested || current == StatusAccepted
	default:
		return false
	}
}

func isActive(status Status) bool {
	return status == StatusRequested || status == StatusAccepted || status == StatusReady
}

func productsCompatible(buyerSKU, buyerOEM, buyerNormalized, sellerSKU, sellerOEM, sellerNormalized string) bool {
	buyerSKU, sellerSKU = strings.TrimSpace(strings.ToLower(buyerSKU)), strings.TrimSpace(strings.ToLower(sellerSKU))
	buyerOEM, sellerOEM = strings.TrimSpace(strings.ToLower(buyerOEM)), strings.TrimSpace(strings.ToLower(sellerOEM))
	buyerNormalized, sellerNormalized = strings.TrimSpace(strings.ToLower(buyerNormalized)), strings.TrimSpace(strings.ToLower(sellerNormalized))
	return (buyerSKU != "" && sellerSKU != "" && buyerSKU == sellerSKU) ||
		(buyerOEM != "" && sellerOEM != "" && buyerOEM == sellerOEM) ||
		(buyerNormalized != "" && buyerNormalized == sellerNormalized)
}

func ensureNetworkCustomer(ctx context.Context, tx pgx.Tx, tenantID, storeID, networkStoreID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM customers WHERE tenant_id=$1 AND store_id=$2 AND network_store_id=$3 AND deleted_at IS NULL LIMIT 1`, tenantID, storeID, networkStoreID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var name, phone string
	if err = tx.QueryRow(ctx, `SELECT name,COALESCE(public_phone,'') FROM stores WHERE id=$1 AND active`, networkStoreID).Scan(&name, &phone); err != nil {
		return uuid.Nil, err
	}
	id = uuid.New()
	code := "NET-" + strings.ToUpper(strings.ReplaceAll(networkStoreID.String()[:8], "-", ""))
	_, err = tx.Exec(ctx, `INSERT INTO customers(id,tenant_id,store_id,code,name,phone,notes,network_store_id) VALUES($1,$2,$3,$4,$5,$6,'طرف حساب خودکار شبکه',$7)`, id, tenantID, storeID, code, name, phone, networkStoreID)
	return id, err
}

func ensureNetworkSupplier(ctx context.Context, tx pgx.Tx, tenantID, storeID, networkStoreID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM suppliers WHERE tenant_id=$1 AND store_id=$2 AND network_store_id=$3 AND deleted_at IS NULL LIMIT 1`, tenantID, storeID, networkStoreID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var name, phone string
	if err = tx.QueryRow(ctx, `SELECT name,COALESCE(public_phone,'') FROM stores WHERE id=$1 AND active`, networkStoreID).Scan(&name, &phone); err != nil {
		return uuid.Nil, err
	}
	id = uuid.New()
	code := "NET-" + strings.ToUpper(strings.ReplaceAll(networkStoreID.String()[:8], "-", ""))
	_, err = tx.Exec(ctx, `INSERT INTO suppliers(id,tenant_id,store_id,code,name,phone,notes,network_store_id) VALUES($1,$2,$3,$4,$5,$6,'تأمین‌کننده خودکار شبکه',$7)`, id, tenantID, storeID, code, name, phone, networkStoreID)
	return id, err
}

func ensureAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typ string }{
		{"AR", "Accounts Receivable", "asset"}, {"SALES", "Sales Revenue", "revenue"}, {"INVENTORY", "Inventory", "asset"},
		{"COGS", "Cost of Goods Sold", "expense"}, {"AP", "Accounts Payable", "liability"},
	}
	out := make(map[string]uuid.UUID, len(defs))
	for _, d := range defs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id); err != nil {
			return nil, err
		}
		out[d.code] = id
	}
	return out, nil
}

func addEntry(ctx context.Context, tx pgx.Tx, tenantID, journalID, accountID uuid.UUID, debit, credit int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,$5)`, tenantID, journalID, accountID, debit, credit)
	return err
}

func weightedAverage(oldQty float64, oldAvg int64, incomingQty float64, incomingCost int64) int64 {
	newQty := oldQty + incomingQty
	if newQty <= 0 {
		return 0
	}
	return int64(math.Round((oldQty*float64(oldAvg) + incomingQty*float64(incomingCost)) / newQty))
}

func addEvent(ctx context.Context, tx pgx.Tx, orderID, tenantID, actorID uuid.UUID, side string, event Status) error {
	var actor any
	if actorID != uuid.Nil {
		actor = actorID
	}
	_, err := tx.Exec(ctx, `INSERT INTO network_procurement_events(procurement_id,tenant_id,actor_user_id,actor_side,event_type) VALUES($1,$2,$3,$4,$5)`, orderID, tenantID, actor, side, event)
	return err
}

func emitForBoth(ctx context.Context, tx pgx.Tx, buyerTenantID, sellerTenantID, orderID uuid.UUID, eventType string, payload any) error {
	body, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_procurement',$2,$3,$4)`, buyerTenantID, orderID, eventType, body); err != nil {
		return err
	}
	if sellerTenantID != buyerTenantID {
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'network_procurement',$2,$3,$4)`, sellerTenantID, orderID, eventType, body); err != nil {
			return err
		}
	}
	return nil
}

const orderSelect = `
	SELECT p.id,p.buyer_tenant_id,p.buyer_store_id,bs.name,p.buyer_warehouse_id,p.buyer_product_id,bp.title,
	       p.seller_tenant_id,p.seller_store_id,ss.name,p.seller_warehouse_id,p.seller_product_id,sp.title,p.offer_id,
	       p.qty::float8,p.unit_price,p.total_amount,p.status,p.expires_at,p.seller_sale_id,p.buyer_purchase_id,p.created_at,p.updated_at
	FROM network_procurements p
	JOIN stores bs ON bs.id=p.buyer_store_id AND bs.tenant_id=p.buyer_tenant_id
	JOIN products bp ON bp.id=p.buyer_product_id AND bp.tenant_id=p.buyer_tenant_id
	JOIN stores ss ON ss.id=p.seller_store_id AND ss.tenant_id=p.seller_tenant_id
	JOIN products sp ON sp.id=p.seller_product_id AND sp.tenant_id=p.seller_tenant_id`

func orderScan(o *Order, buyerTenantID, sellerTenantID *uuid.UUID) []any {
	return []any{&o.ID, buyerTenantID, &o.BuyerStoreID, &o.BuyerStoreName, &o.BuyerWarehouseID, &o.BuyerProductID, &o.BuyerProductTitle,
		sellerTenantID, &o.SellerStoreID, &o.SellerStoreName, &o.SellerWarehouseID, &o.SellerProductID, &o.SellerProductTitle, &o.OfferID,
		&o.Qty, &o.UnitPrice, &o.TotalAmount, &o.Status, &o.ExpiresAt, &o.SellerSaleID, &o.BuyerPurchaseID, &o.CreatedAt, &o.UpdatedAt}
}

type rowScanner interface{ Scan(dest ...any) error }

func scanOrder(row rowScanner, o *Order) error {
	var buyerTenantID, sellerTenantID uuid.UUID
	return row.Scan(orderScan(o, &buyerTenantID, &sellerTenantID)...)
}

func getByIdempotency(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string) (Order, bool, error) {
	var o Order
	var buyerTenantID, sellerTenantID uuid.UUID
	err := tx.QueryRow(ctx, orderSelect+` WHERE p.buyer_tenant_id=$1 AND p.idempotency_key=$2`, tenantID, key).Scan(orderScan(&o, &buyerTenantID, &sellerTenantID)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, nil
	}
	return o, err == nil, err
}
