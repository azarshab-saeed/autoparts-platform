package pricing

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

type PriceList struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"is_default"`
	Active    bool      `json:"active"`
}

type CreatePriceList struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

type UpdatePriceList struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Active    bool   `json:"active"`
}

type Settings struct {
	MinMarginBPS       int  `json:"min_margin_bps"`
	CashierMayOverride bool `json:"cashier_may_override"`
}

type ProductPricing struct {
	ProductID uuid.UUID `json:"product_id"`
	Title     string    `json:"title"`
	SKU       *string   `json:"sku,omitempty"`
	Brand     *string   `json:"brand,omitempty"`
	Breaks    []Break   `json:"breaks"`
}

type QuoteRequestLine struct {
	ProductID uuid.UUID `json:"product_id"`
	Qty       float64   `json:"qty"`
}

type QuoteLine struct {
	ProductID       uuid.UUID  `json:"product_id"`
	Qty             float64    `json:"qty"`
	PriceListID     *uuid.UUID `json:"price_list_id,omitempty"`
	PriceListName   string     `json:"price_list_name,omitempty"`
	UnitPrice       int64      `json:"unit_price"`
	PriceSource     string     `json:"price_source"`
	MinAllowedPrice int64      `json:"min_allowed_price"`
}

type QuoteResult struct {
	PriceListID        uuid.UUID   `json:"price_list_id"`
	PriceListName      string      `json:"price_list_name"`
	MinMarginBPS       int         `json:"min_margin_bps"`
	CashierMayOverride bool        `json:"cashier_may_override"`
	Items              []QuoteLine `json:"items"`
}

func (s *Service) ensureBase(ctx context.Context, tenantID, storeID uuid.UUID) error {
	if tenantID == uuid.Nil || storeID == uuid.Nil {
		return errors.New("authenticated store is required")
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active)
		SELECT $1,$2,'retail','خرده / مصرف‌کننده',true,true
		WHERE NOT EXISTS (SELECT 1 FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active)
		ON CONFLICT DO NOTHING`, tenantID, storeID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `INSERT INTO store_pricing_settings(tenant_id,store_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, tenantID, storeID)
	return err
}

func (s *Service) ListPriceLists(ctx context.Context, tenantID, storeID uuid.UUID) ([]PriceList, error) {
	if err := s.ensureBase(ctx, tenantID, storeID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,code,name,is_default,active FROM price_lists WHERE tenant_id=$1 AND store_id=$2 ORDER BY is_default DESC,active DESC,name,id`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PriceList{}
	for rows.Next() {
		var x PriceList
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.IsDefault, &x.Active); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) CreatePriceList(ctx context.Context, tenantID, storeID uuid.UUID, in CreatePriceList) (PriceList, error) {
	if err := s.ensureBase(ctx, tenantID, storeID); err != nil {
		return PriceList{}, err
	}
	code := strings.ToLower(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	if code == "" || name == "" {
		return PriceList{}, errors.New("code and name are required")
	}
	if len(code) > 40 || len(name) > 120 {
		return PriceList{}, errors.New("price list code or name is too long")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PriceList{}, err
	}
	defer tx.Rollback(ctx)
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE price_lists SET is_default=false,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 AND is_default`, tenantID, storeID); err != nil {
			return PriceList{}, err
		}
	}
	var out PriceList
	err = tx.QueryRow(ctx, `INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active) VALUES($1,$2,$3,$4,$5,true) RETURNING id,code,name,is_default,active`, tenantID, storeID, code, name, in.IsDefault).Scan(&out.ID, &out.Code, &out.Name, &out.IsDefault, &out.Active)
	if err != nil {
		return PriceList{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PriceList{}, err
	}
	return out, nil
}

func (s *Service) UpdatePriceList(ctx context.Context, tenantID, storeID, id uuid.UUID, in UpdatePriceList) (PriceList, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return PriceList{}, errors.New("name is required")
	}
	if !in.Active && in.IsDefault {
		return PriceList{}, errors.New("default price list must stay active")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PriceList{}, err
	}
	defer tx.Rollback(ctx)
	var currentDefault bool
	if err = tx.QueryRow(ctx, `SELECT is_default FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3 FOR UPDATE`, id, tenantID, storeID).Scan(&currentDefault); errors.Is(err, pgx.ErrNoRows) {
		return PriceList{}, errors.New("price list not found")
	} else if err != nil {
		return PriceList{}, err
	}
	if currentDefault && (!in.IsDefault || !in.Active) {
		return PriceList{}, errors.New("default price list cannot be disabled or unset; choose another default first")
	}
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE price_lists SET is_default=false,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 AND id<>$3 AND is_default`, tenantID, storeID, id); err != nil {
			return PriceList{}, err
		}
	}
	var out PriceList
	err = tx.QueryRow(ctx, `UPDATE price_lists SET name=$4,is_default=$5,active=$6,updated_at=now() WHERE id=$3 AND tenant_id=$1 AND store_id=$2 RETURNING id,code,name,is_default,active`, tenantID, storeID, id, name, in.IsDefault, in.Active).Scan(&out.ID, &out.Code, &out.Name, &out.IsDefault, &out.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return PriceList{}, errors.New("price list not found")
	}
	if err != nil {
		return PriceList{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PriceList{}, err
	}
	return out, nil
}

func (s *Service) GetSettings(ctx context.Context, tenantID, storeID uuid.UUID) (Settings, error) {
	if err := s.ensureBase(ctx, tenantID, storeID); err != nil {
		return Settings{}, err
	}
	var out Settings
	err := s.db.QueryRow(ctx, `SELECT min_margin_bps,cashier_may_override FROM store_pricing_settings WHERE tenant_id=$1 AND store_id=$2`, tenantID, storeID).Scan(&out.MinMarginBPS, &out.CashierMayOverride)
	return out, err
}

func (s *Service) UpdateSettings(ctx context.Context, tenantID, storeID uuid.UUID, in Settings) (Settings, error) {
	if err := s.ensureBase(ctx, tenantID, storeID); err != nil {
		return Settings{}, err
	}
	if in.MinMarginBPS < 0 || in.MinMarginBPS > 9000 {
		return Settings{}, errors.New("min_margin_bps must be between 0 and 9000")
	}
	var out Settings
	err := s.db.QueryRow(ctx, `INSERT INTO store_pricing_settings(tenant_id,store_id,min_margin_bps,cashier_may_override) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,store_id) DO UPDATE SET min_margin_bps=EXCLUDED.min_margin_bps,cashier_may_override=EXCLUDED.cashier_may_override,updated_at=now() RETURNING min_margin_bps,cashier_may_override`, tenantID, storeID, in.MinMarginBPS, in.CashierMayOverride).Scan(&out.MinMarginBPS, &out.CashierMayOverride)
	return out, err
}

func (s *Service) ListProductPricing(ctx context.Context, tenantID, storeID, priceListID uuid.UUID, q string, limit int) ([]ProductPricing, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var listOK bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, priceListID, tenantID, storeID).Scan(&listOK); err != nil {
		return nil, err
	}
	if !listOK {
		return nil, errors.New("price list not found")
	}
	pat := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `SELECT id,title,sku,brand FROM products WHERE tenant_id=$1 AND deleted_at IS NULL AND active AND ($2='%%' OR lower(title) LIKE $2 OR lower(COALESCE(sku,'')) LIKE $2 OR lower(COALESCE(brand,'')) LIKE $2 OR lower(COALESCE(oem_code,'')) LIKE $2 OR lower(COALESCE(barcode,'')) LIKE $2) ORDER BY lower(title),id LIMIT $3`, tenantID, pat, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductPricing{}
	for rows.Next() {
		var x ProductPricing
		if err := rows.Scan(&x.ProductID, &x.Title, &x.SKU, &x.Brand); err != nil {
			return nil, err
		}
		br, err := s.breaks(ctx, tenantID, storeID, x.ProductID, priceListID)
		if err != nil {
			return nil, err
		}
		x.Breaks = br
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) breaks(ctx context.Context, tenantID, storeID, productID, priceListID uuid.UUID) ([]Break, error) {
	rows, err := s.db.Query(ctx, `SELECT min_qty::float8,unit_price FROM product_price_breaks WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND price_list_id=$4 ORDER BY min_qty`, tenantID, storeID, productID, priceListID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Break{}
	for rows.Next() {
		var b Break
		if err := rows.Scan(&b.MinQty, &b.UnitPrice); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) ReplaceProductBreaks(ctx context.Context, tenantID, storeID, productID, priceListID uuid.UUID, in []Break) ([]Break, error) {
	if len(in) > 30 {
		return nil, errors.New("too many quantity breaks")
	}
	seen := map[int64]bool{}
	for _, b := range in {
		if b.MinQty <= 0 || b.UnitPrice < 0 {
			return nil, errors.New("min_qty must be positive and unit_price must be non-negative")
		}
		scaled := b.MinQty * 1000
		if math.Abs(scaled-math.Round(scaled)) > 1e-9 {
			return nil, errors.New("min_qty supports at most 3 decimal places")
		}
		key := int64(math.Round(scaled))
		if seen[key] {
			return nil, errors.New("duplicate min_qty in price breaks")
		}
		seen[key] = true
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var listOK, productOK bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active)`, priceListID, tenantID, storeID).Scan(&listOK); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND active AND deleted_at IS NULL)`, productID, tenantID).Scan(&productOK); err != nil {
		return nil, err
	}
	if !listOK || !productOK {
		return nil, errors.New("price list or product not found")
	}
	if _, err = tx.Exec(ctx, `DELETE FROM product_price_breaks WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND price_list_id=$4`, tenantID, storeID, productID, priceListID); err != nil {
		return nil, err
	}
	for _, b := range in {
		if _, err = tx.Exec(ctx, `INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price) VALUES($1,$2,$3,$4,$5,$6)`, tenantID, storeID, productID, priceListID, b.MinQty, b.UnitPrice); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.breaks(ctx, tenantID, storeID, productID, priceListID)
}

func (s *Service) Quote(ctx context.Context, tenantID, storeID, warehouseID uuid.UUID, customerID *uuid.UUID, lines []QuoteRequestLine) (QuoteResult, error) {
	if len(lines) == 0 || len(lines) > 100 {
		return QuoteResult{}, errors.New("between 1 and 100 quote lines are required")
	}
	if err := s.ensureBase(ctx, tenantID, storeID); err != nil {
		return QuoteResult{}, err
	}
	var warehouseOK bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, warehouseID, tenantID, storeID).Scan(&warehouseOK); err != nil {
		return QuoteResult{}, err
	}
	if !warehouseOK {
		return QuoteResult{}, errors.New("warehouse does not belong to authenticated store")
	}
	settings, err := s.GetSettings(ctx, tenantID, storeID)
	if err != nil {
		return QuoteResult{}, err
	}
	defaultList, err := s.defaultList(ctx, tenantID, storeID)
	if err != nil {
		return QuoteResult{}, err
	}
	selected := defaultList
	if customerID != nil {
		var id *uuid.UUID
		var name *string
		err = s.db.QueryRow(ctx, `SELECT c.price_list_id,pl.name FROM customers c LEFT JOIN price_lists pl ON pl.id=c.price_list_id AND pl.tenant_id=c.tenant_id AND pl.store_id=c.store_id AND pl.active WHERE c.id=$1 AND c.tenant_id=$2 AND c.store_id=$3 AND c.deleted_at IS NULL`, *customerID, tenantID, storeID).Scan(&id, &name)
		if errors.Is(err, pgx.ErrNoRows) {
			return QuoteResult{}, errors.New("customer does not belong to authenticated store")
		}
		if err != nil {
			return QuoteResult{}, err
		}
		if id != nil && name != nil {
			selected.ID, selected.Name = *id, *name
		}
	}
	out := QuoteResult{PriceListID: selected.ID, PriceListName: selected.Name, MinMarginBPS: settings.MinMarginBPS, CashierMayOverride: settings.CashierMayOverride, Items: make([]QuoteLine, 0, len(lines))}
	for _, line := range lines {
		if line.ProductID == uuid.Nil || line.Qty <= 0 {
			return QuoteResult{}, errors.New("product_id and positive qty are required")
		}
		q, err := s.quoteLine(ctx, tenantID, storeID, warehouseID, selected, defaultList, settings, line)
		if err != nil {
			return QuoteResult{}, err
		}
		out.Items = append(out.Items, q)
	}
	return out, nil
}

func (s *Service) quoteLine(ctx context.Context, tenantID, storeID, warehouseID uuid.UUID, selected, fallback PriceList, settings Settings, line QuoteRequestLine) (QuoteLine, error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND active AND deleted_at IS NULL)`, line.ProductID, tenantID).Scan(&exists); err != nil {
		return QuoteLine{}, err
	}
	if !exists {
		return QuoteLine{}, fmt.Errorf("product %s not found", line.ProductID)
	}
	var cost int64
	err := s.db.QueryRow(ctx, `SELECT avg_unit_cost FROM inventory_balances WHERE tenant_id=$1 AND warehouse_id=$2 AND product_id=$3`, tenantID, warehouseID, line.ProductID).Scan(&cost)
	if errors.Is(err, pgx.ErrNoRows) {
		cost = 0
	} else if err != nil {
		return QuoteLine{}, err
	}

	price, list, source, found, err := s.resolveBreak(ctx, tenantID, storeID, line.ProductID, selected, line.Qty)
	if err != nil {
		return QuoteLine{}, err
	}
	if !found && fallback.ID != selected.ID {
		price, list, source, found, err = s.resolveBreak(ctx, tenantID, storeID, line.ProductID, fallback, line.Qty)
		if err != nil {
			return QuoteLine{}, err
		}
		if found {
			source = "default_fallback"
		}
	}
	if !found {
		err = s.db.QueryRow(ctx, `SELECT selling_price FROM store_product_offers WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND product_id=$4 ORDER BY updated_at DESC LIMIT 1`, tenantID, storeID, warehouseID, line.ProductID).Scan(&price)
		if errors.Is(err, pgx.ErrNoRows) {
			price, source = 0, "manual"
		} else if err != nil {
			return QuoteLine{}, err
		} else {
			source = "network_offer_fallback"
		}
		list = PriceList{}
	}
	minPrice := MinimumPriceForMargin(cost, settings.MinMarginBPS)
	q := QuoteLine{ProductID: line.ProductID, Qty: line.Qty, UnitPrice: price, PriceSource: source, MinAllowedPrice: minPrice}
	if list.ID != uuid.Nil {
		q.PriceListID = &list.ID
		q.PriceListName = list.Name
	}
	return q, nil
}

func (s *Service) resolveBreak(ctx context.Context, tenantID, storeID, productID uuid.UUID, list PriceList, qty float64) (int64, PriceList, string, bool, error) {
	var price int64
	err := s.db.QueryRow(ctx, `SELECT unit_price FROM product_price_breaks WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND price_list_id=$4 AND min_qty<=$5 ORDER BY min_qty DESC LIMIT 1`, tenantID, storeID, productID, list.ID, qty).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, PriceList{}, "", false, nil
	}
	if err != nil {
		return 0, PriceList{}, "", false, err
	}
	return price, list, "price_list", true, nil
}

func (s *Service) defaultList(ctx context.Context, tenantID, storeID uuid.UUID) (PriceList, error) {
	var out PriceList
	err := s.db.QueryRow(ctx, `SELECT id,code,name,is_default,active FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active ORDER BY created_at LIMIT 1`, tenantID, storeID).Scan(&out.ID, &out.Code, &out.Name, &out.IsDefault, &out.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return PriceList{}, errors.New("default price list is not configured")
	}
	return out, err
}
