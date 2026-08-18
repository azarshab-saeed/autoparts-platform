package edge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pairingTTL = 10 * time.Minute

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) CreatePairing(ctx context.Context, tenantID, storeID, warehouseID, userID uuid.UUID) (Pairing, error) {
	if tenantID == uuid.Nil || storeID == uuid.Nil || warehouseID == uuid.Nil || userID == uuid.Nil {
		return Pairing{}, errors.New("authenticated store and warehouse are required")
	}
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, warehouseID, tenantID, storeID).Scan(&ok); err != nil {
		return Pairing{}, err
	}
	if !ok {
		return Pairing{}, errors.New("warehouse does not belong to authenticated store")
	}
	code, err := randomToken(24)
	if err != nil {
		return Pairing{}, err
	}
	expires := time.Now().Add(pairingTTL)
	_, err = s.db.Exec(ctx, `INSERT INTO store_edge_pairings(tenant_id,store_id,warehouse_id,requested_by_user_id,code_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, tenantID, storeID, warehouseID, userID, hashToken(code), expires)
	if err != nil {
		return Pairing{}, err
	}
	return Pairing{Code: code, ExpiresAt: expires}, nil
}

func (s *Service) PairDevice(ctx context.Context, code, name string) (PairResult, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" || len(name) > 120 {
		return PairResult{}, errors.New("pair_code and device_name are required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PairResult{}, err
	}
	defer tx.Rollback(ctx)
	var tenantID, storeID, warehouseID, pairedBy uuid.UUID
	err = tx.QueryRow(ctx, `SELECT tenant_id,store_id,warehouse_id,requested_by_user_id FROM store_edge_pairings WHERE code_hash=$1 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, hashToken(code)).Scan(&tenantID, &storeID, &warehouseID, &pairedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return PairResult{}, errors.New("pair code is invalid, expired, or already used")
	}
	if err != nil {
		return PairResult{}, err
	}
	secret, err := randomToken(32)
	if err != nil {
		return PairResult{}, err
	}
	deviceID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO store_edge_devices(id,tenant_id,store_id,warehouse_id,name,secret_hash,paired_by_user_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, deviceID, tenantID, storeID, warehouseID, name, hashToken(secret), pairedBy); err != nil {
		return PairResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE store_edge_pairings SET consumed_at=now() WHERE code_hash=$1`, hashToken(code)); err != nil {
		return PairResult{}, err
	}
	var storeName string
	if err = tx.QueryRow(ctx, `SELECT name FROM stores WHERE id=$1 AND tenant_id=$2`, storeID, tenantID).Scan(&storeName); err != nil {
		return PairResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PairResult{}, err
	}
	return PairResult{DeviceID: deviceID, DeviceSecret: secret, StoreID: storeID, WarehouseID: warehouseID, StoreName: storeName}, nil
}

func (s *Service) Authenticate(ctx context.Context, deviceRaw, secret string) (Device, error) {
	deviceID, err := uuid.Parse(strings.TrimSpace(deviceRaw))
	if err != nil || strings.TrimSpace(secret) == "" {
		return Device{}, errors.New("edge device credentials are invalid")
	}
	var d Device
	var stored string
	err = s.db.QueryRow(ctx, `SELECT id,tenant_id,store_id,warehouse_id,name,active,last_seen_at,created_at,secret_hash FROM store_edge_devices WHERE id=$1 AND active AND revoked_at IS NULL`, deviceID).
		Scan(&d.ID, &d.TenantID, &d.StoreID, &d.WarehouseID, &d.Name, &d.Active, &d.LastSeenAt, &d.CreatedAt, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, errors.New("edge device is not active")
	}
	if err != nil {
		return Device{}, err
	}
	a, _ := hex.DecodeString(stored)
	b, _ := hex.DecodeString(hashToken(secret))
	if len(a) != len(b) || subtle.ConstantTimeCompare(a, b) != 1 {
		return Device{}, errors.New("edge device credentials are invalid")
	}
	_, _ = s.db.Exec(ctx, `UPDATE store_edge_devices SET last_seen_at=now() WHERE id=$1`, d.ID)
	return d, nil
}

func (s *Service) Snapshot(ctx context.Context, d Device) (Snapshot, error) {
	var out Snapshot
	out.GeneratedAt = time.Now()
	out.StoreID, out.WarehouseID = d.StoreID, d.WarehouseID
	if err := s.db.QueryRow(ctx, `SELECT name FROM stores WHERE id=$1 AND tenant_id=$2`, d.StoreID, d.TenantID).Scan(&out.StoreName); err != nil {
		return Snapshot{}, err
	}
	var cashierMayOverride bool
	err := s.db.QueryRow(ctx, `SELECT cashier_may_override FROM store_pricing_settings WHERE tenant_id=$1 AND store_id=$2`, d.TenantID, d.StoreID).Scan(&cashierMayOverride)
	if errors.Is(err, pgx.ErrNoRows) {
		cashierMayOverride = true
	} else if err != nil {
		return Snapshot{}, err
	}
	out.PricingPolicy = &SnapshotPricingPolicy{CashierMayOverride: cashierMayOverride}
	rows, err := s.db.Query(ctx, `
		SELECT p.id,p.title,COALESCE(p.sku,''),COALESCE(p.brand,''),COALESCE(p.oem_code,''),COALESCE(p.barcode,''),
		       ib.on_hand::float8,ib.reserved::float8,(ib.on_hand-ib.reserved)::float8,
		       COALESCE(local_price.unit_price,o.selling_price,0)::bigint,p.allow_fractional_base_qty,ib.updated_at
		FROM products p
		JOIN inventory_balances ib ON ib.tenant_id=p.tenant_id AND ib.product_id=p.id AND ib.warehouse_id=$3
		LEFT JOIN LATERAL (
		  SELECT ppb.unit_price
		  FROM price_lists pl
		  JOIN product_units pu ON pu.tenant_id=p.tenant_id AND pu.product_id=p.id AND pu.is_base AND pu.active
		  JOIN product_price_breaks ppb ON ppb.price_list_id=pl.id AND ppb.tenant_id=pl.tenant_id AND ppb.store_id=pl.store_id AND ppb.product_id=p.id AND ppb.product_unit_id=pu.id
		  WHERE pl.tenant_id=p.tenant_id AND pl.store_id=$2 AND pl.is_default AND pl.active AND ppb.min_qty<=1
		  ORDER BY ppb.min_qty DESC LIMIT 1
		) local_price ON true
		LEFT JOIN store_product_offers o ON o.tenant_id=p.tenant_id AND o.store_id=$2 AND o.warehouse_id=$3 AND o.product_id=p.id
		WHERE p.tenant_id=$1 AND p.active AND p.deleted_at IS NULL
		ORDER BY lower(p.title),p.id`, d.TenantID, d.StoreID, d.WarehouseID)
	if err != nil {
		return Snapshot{}, err
	}
	out.Products = make([]SnapshotProduct, 0)
	for rows.Next() {
		var p SnapshotProduct
		if err := rows.Scan(&p.ProductID, &p.Title, &p.SKU, &p.Brand, &p.OEMCode, &p.Barcode, &p.OnHand, &p.Reserved, &p.Available, &p.SellingPrice, &p.AllowFractionalBaseQty, &p.UpdatedAt); err != nil {
			rows.Close()
			return Snapshot{}, err
		}
		out.Products = append(out.Products, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Snapshot{}, err
	}
	rows.Close()

	unitRows, err := s.db.Query(ctx, `
		SELECT pu.product_id,pu.id,pu.code,pu.name,pu.factor_to_base::float8,
		       COALESCE(pu.barcode,''),pu.is_base
		FROM product_units pu
		JOIN products p ON p.id=pu.product_id AND p.tenant_id=pu.tenant_id
		WHERE pu.tenant_id=$1 AND p.deleted_at IS NULL AND p.active AND pu.active AND pu.allow_sale
		ORDER BY pu.product_id,pu.is_base DESC,pu.factor_to_base,lower(pu.name),pu.id`, d.TenantID)
	if err != nil {
		return Snapshot{}, err
	}
	unitsByProduct := make(map[uuid.UUID][]SnapshotProductUnit)
	for unitRows.Next() {
		var productID uuid.UUID
		var unit SnapshotProductUnit
		if err := unitRows.Scan(&productID, &unit.ProductUnitID, &unit.Code, &unit.Name, &unit.FactorToBase, &unit.Barcode, &unit.IsBase); err != nil {
			unitRows.Close()
			return Snapshot{}, err
		}
		unitsByProduct[productID] = append(unitsByProduct[productID], unit)
	}
	if err := unitRows.Err(); err != nil {
		unitRows.Close()
		return Snapshot{}, err
	}
	unitRows.Close()

	type unitBreakKey struct {
		ProductID uuid.UUID
		UnitID    uuid.UUID
	}
	breakRows, err := s.db.Query(ctx, `
		SELECT ppb.product_id,ppb.product_unit_id,ppb.min_qty::float8,ppb.unit_price
		FROM price_lists pl
		JOIN product_price_breaks ppb ON ppb.price_list_id=pl.id AND ppb.tenant_id=pl.tenant_id AND ppb.store_id=pl.store_id
		JOIN product_units pu ON pu.id=ppb.product_unit_id AND pu.tenant_id=ppb.tenant_id AND pu.product_id=ppb.product_id AND pu.active AND pu.allow_sale
		WHERE pl.tenant_id=$1 AND pl.store_id=$2 AND pl.is_default AND pl.active
		ORDER BY ppb.product_id,ppb.product_unit_id,ppb.min_qty`, d.TenantID, d.StoreID)
	if err != nil {
		return Snapshot{}, err
	}
	byUnit := make(map[unitBreakKey][]SnapshotPriceBreak)
	for breakRows.Next() {
		var productID, unitID uuid.UUID
		var b SnapshotPriceBreak
		if err := breakRows.Scan(&productID, &unitID, &b.MinQty, &b.UnitPrice); err != nil {
			breakRows.Close()
			return Snapshot{}, err
		}
		key := unitBreakKey{ProductID: productID, UnitID: unitID}
		byUnit[key] = append(byUnit[key], b)
	}
	if err := breakRows.Err(); err != nil {
		breakRows.Close()
		return Snapshot{}, err
	}
	breakRows.Close()
	for i := range out.Products {
		units := unitsByProduct[out.Products[i].ProductID]
		for u := range units {
			units[u].PriceBreaks = byUnit[unitBreakKey{ProductID: out.Products[i].ProductID, UnitID: units[u].ProductUnitID}]
			if units[u].IsBase {
				out.Products[i].PriceBreaks = units[u].PriceBreaks
			}
		}
		out.Products[i].Units = units
	}
	return out, nil
}

func (s *Service) ListDevices(ctx context.Context, tenantID, storeID uuid.UUID) ([]Device, error) {
	rows, err := s.db.Query(ctx, `SELECT id,tenant_id,store_id,warehouse_id,name,active,last_seen_at,created_at FROM store_edge_devices WHERE tenant_id=$1 AND store_id=$2 ORDER BY created_at DESC`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.TenantID, &d.StoreID, &d.WarehouseID, &d.Name, &d.Active, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) Revoke(ctx context.Context, tenantID, storeID, deviceID uuid.UUID) error {
	ct, err := s.db.Exec(ctx, `UPDATE store_edge_devices SET active=false,revoked_at=now() WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active`, deviceID, tenantID, storeID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return errors.New("active edge device was not found")
	}
	return nil
}

func (s *Service) RecordSync(ctx context.Context, d Device, localID string, serverID *uuid.UUID, status, detail string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO store_edge_sync_events(device_id,tenant_id,store_id,local_operation_id,operation_type,server_reference_id,status,detail) VALUES($1,$2,$3,$4,'sale',$5,$6,$7) ON CONFLICT(device_id,local_operation_id) DO UPDATE SET server_reference_id=EXCLUDED.server_reference_id,status=EXCLUDED.status,detail=EXCLUDED.detail,received_at=now()`, d.ID, d.TenantID, d.StoreID, localID, serverID, status, detail)
	return err
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(v string) string {
	s := sha256.Sum256([]byte(v))
	return hex.EncodeToString(s[:])
}
