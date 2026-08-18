package catalog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type ProductUnit struct {
	ID            uuid.UUID `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	FactorToBase  float64   `json:"factor_to_base"`
	Barcode       *string   `json:"barcode,omitempty"`
	IsBase        bool      `json:"is_base"`
	AllowSale     bool      `json:"allow_sale"`
	AllowPurchase bool      `json:"allow_purchase"`
	Active        bool      `json:"active"`
}

type Product struct {
	ID                     uuid.UUID     `json:"id"`
	SKU                    *string       `json:"sku,omitempty"`
	Title                  string        `json:"title"`
	Brand                  *string       `json:"brand,omitempty"`
	OEMCode                *string       `json:"oem_code,omitempty"`
	Barcode                *string       `json:"barcode,omitempty"`
	Unit                   string        `json:"unit"`
	AllowFractionalBaseQty bool          `json:"allow_fractional_base_qty"`
	Active                 bool          `json:"active"`
	Units                  []ProductUnit `json:"units,omitempty"`
}

type CreateProductUnit struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	FactorToBase  float64 `json:"factor_to_base"`
	Barcode       *string `json:"barcode,omitempty"`
	AllowSale     bool    `json:"allow_sale"`
	AllowPurchase bool    `json:"allow_purchase"`
	RetailPrice   int64   `json:"retail_price,omitempty"`
}

type CreateProduct struct {
	SKU                    *string             `json:"sku,omitempty"`
	Title                  string              `json:"title"`
	Brand                  *string             `json:"brand,omitempty"`
	OEMCode                *string             `json:"oem_code,omitempty"`
	Barcode                *string             `json:"barcode,omitempty"`
	Unit                   string              `json:"unit"`
	BaseUnitName           string              `json:"base_unit_name,omitempty"`
	AllowFractionalBaseQty bool                `json:"allow_fractional_base_qty"`
	RetailPrice            int64               `json:"retail_price,omitempty"`
	Units                  []CreateProductUnit `json:"units,omitempty"`
}

type ReplaceUnits struct {
	AllowFractionalBaseQty bool                `json:"allow_fractional_base_qty"`
	BaseUnitName           string              `json:"base_unit_name,omitempty"`
	BaseBarcode            *string             `json:"base_barcode,omitempty"`
	Units                  []CreateProductUnit `json:"units"`
}

type Page struct {
	Items      []Product `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

func (s *Service) Create(ctx context.Context, tenantID, storeID uuid.UUID, in CreateProduct) (Product, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Product{}, errors.New("title is required")
	}
	in.Unit = normalizeUnitCode(in.Unit)
	if in.Unit == "" {
		in.Unit = "pcs"
	}
	baseName := strings.TrimSpace(in.BaseUnitName)
	if baseName == "" {
		baseName = defaultUnitName(in.Unit)
	}
	if in.RetailPrice < 0 {
		return Product{}, errors.New("retail_price cannot be negative")
	}
	baseBarcode := cleanOptional(in.Barcode)
	units, err := validateAlternateUnits(in.Unit, baseBarcode, in.AllowFractionalBaseQty, in.Units)
	if err != nil {
		return Product{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)

	var p Product
	err = tx.QueryRow(ctx, `INSERT INTO products(tenant_id,sku,title,brand,oem_code,barcode,unit,allow_fractional_base_qty,normalized_title)
		VALUES($1,NULLIF($2,''),$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,$8,lower(trim($3)))
		RETURNING id,sku,title,brand,oem_code,barcode,unit,allow_fractional_base_qty,active`,
		tenantID, cleanString(in.SKU), in.Title, cleanString(in.Brand), cleanString(in.OEMCode), cleanString(baseBarcode), in.Unit, in.AllowFractionalBaseQty,
	).Scan(&p.ID, &p.SKU, &p.Title, &p.Brand, &p.OEMCode, &p.Barcode, &p.Unit, &p.AllowFractionalBaseQty, &p.Active)
	if err != nil {
		return Product{}, err
	}
	var base ProductUnit
	err = tx.QueryRow(ctx, `INSERT INTO product_units(tenant_id,product_id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active)
		VALUES($1,$2,$3,$4,1,NULLIF($5,''),true,true,true,true)
		RETURNING id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active`, tenantID, p.ID, in.Unit, baseName, cleanString(baseBarcode)).Scan(
		&base.ID, &base.Code, &base.Name, &base.FactorToBase, &base.Barcode, &base.IsBase, &base.AllowSale, &base.AllowPurchase, &base.Active,
	)
	if err != nil {
		return Product{}, err
	}
	p.Units = append(p.Units, base)
	var listID uuid.UUID
	needsRetailList := in.RetailPrice > 0
	if !needsRetailList {
		for _, u := range units {
			if u.RetailPrice > 0 {
				needsRetailList = true
				break
			}
		}
	}
	if needsRetailList && storeID != uuid.Nil {
		if _, err = tx.Exec(ctx, `INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active)
			SELECT $1,$2,'retail','خرده / مصرف‌کننده',true,true
			WHERE NOT EXISTS (SELECT 1 FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active)
			ON CONFLICT DO NOTHING`, tenantID, storeID); err != nil {
			return Product{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO store_pricing_settings(tenant_id,store_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, tenantID, storeID); err != nil {
			return Product{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT id FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active ORDER BY created_at LIMIT 1`, tenantID, storeID).Scan(&listID); err != nil {
			return Product{}, fmt.Errorf("default price list is not configured: %w", err)
		}
	}
	if in.RetailPrice > 0 && listID != uuid.Nil {
		if _, err = tx.Exec(ctx, `INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty,unit_price)
			VALUES($1,$2,$3,$4,$5,1,$6)
			ON CONFLICT(tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty) DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now()`, tenantID, storeID, p.ID, listID, base.ID, in.RetailPrice); err != nil {
			return Product{}, err
		}
	}
	for _, u := range units {
		var row ProductUnit
		err = tx.QueryRow(ctx, `INSERT INTO product_units(tenant_id,product_id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active)
			VALUES($1,$2,$3,$4,$5,NULLIF($6,''),false,$7,$8,true)
			RETURNING id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active`, tenantID, p.ID, u.Code, u.Name, u.FactorToBase, cleanString(u.Barcode), u.AllowSale, u.AllowPurchase).Scan(
			&row.ID, &row.Code, &row.Name, &row.FactorToBase, &row.Barcode, &row.IsBase, &row.AllowSale, &row.AllowPurchase, &row.Active,
		)
		if err != nil {
			return Product{}, err
		}
		p.Units = append(p.Units, row)
		if u.RetailPrice > 0 && listID != uuid.Nil {
			if _, err = tx.Exec(ctx, `INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty,unit_price)
				VALUES($1,$2,$3,$4,$5,1,$6)
				ON CONFLICT(tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty) DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now()`, tenantID, storeID, p.ID, listID, row.ID, u.RetailPrice); err != nil {
				return Product{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q string, limit, offset int) (Page, error) {
	q = "%" + normalizeCatalogSearch(q) + "%"
	rows, err := s.db.Query(ctx, `SELECT id,sku,title,brand,oem_code,barcode,unit,allow_fractional_base_qty,active FROM products p WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2='%%' OR translate(lower(p.normalized_title),'۰۱۲۳۴۵۶۷۸۹٠١٢٣٤٥٦٧٨٩','01234567890123456789') LIKE $2 OR lower(COALESCE(sku,'')) LIKE $2 OR lower(COALESCE(oem_code,'')) LIKE $2 OR lower(COALESCE(barcode,'')) LIKE $2 OR EXISTS (SELECT 1 FROM product_units pu WHERE pu.tenant_id=p.tenant_id AND pu.product_id=p.id AND pu.active AND lower(COALESCE(pu.barcode,'')) LIKE $2) OR EXISTS (SELECT 1 FROM product_search_terms pst WHERE pst.tenant_id=p.tenant_id AND pst.product_id=p.id AND pst.normalized_term LIKE $2)) ORDER BY title,id LIMIT $3 OFFSET $4`, tenantID, q, limit+1, offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	out := Page{Items: make([]Product, 0, limit)}
	for rows.Next() {
		var p Product
		if err = rows.Scan(&p.ID, &p.SKU, &p.Title, &p.Brand, &p.OEMCode, &p.Barcode, &p.Unit, &p.AllowFractionalBaseQty, &p.Active); err != nil {
			return Page{}, err
		}
		out.Items = append(out.Items, p)
	}
	if err = rows.Err(); err != nil {
		return Page{}, err
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = pagination.EncodeOffset(offset + limit)
	}
	for i := range out.Items {
		out.Items[i].Units, err = s.listUnits(ctx, tenantID, out.Items[i].ID)
		if err != nil {
			return Page{}, err
		}
	}
	return out, nil
}

func (s *Service) Export(ctx context.Context, tenantID uuid.UUID) ([]Product, error) {
	page, err := s.List(ctx, tenantID, "", 100000, 0)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *Service) Get(ctx context.Context, tenantID, productID uuid.UUID) (Product, error) {
	var p Product
	err := s.db.QueryRow(ctx, `SELECT id,sku,title,brand,oem_code,barcode,unit,allow_fractional_base_qty,active FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, productID, tenantID).Scan(&p.ID, &p.SKU, &p.Title, &p.Brand, &p.OEMCode, &p.Barcode, &p.Unit, &p.AllowFractionalBaseQty, &p.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, errors.New("product not found")
	}
	if err != nil {
		return Product{}, err
	}
	p.Units, err = s.listUnits(ctx, tenantID, productID)
	return p, err
}

func (s *Service) Units(ctx context.Context, tenantID, productID uuid.UUID) ([]ProductUnit, error) {
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL)`, productID, tenantID).Scan(&ok); err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("product not found")
	}
	return s.listUnits(ctx, tenantID, productID)
}

func (s *Service) ReplaceProductUnits(ctx context.Context, tenantID, productID uuid.UUID, in ReplaceUnits) ([]ProductUnit, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var baseCode string
	if err = tx.QueryRow(ctx, `SELECT unit FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL FOR UPDATE`, productID, tenantID).Scan(&baseCode); errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("product not found")
	} else if err != nil {
		return nil, err
	}
	baseCode = normalizeUnitCode(baseCode)
	baseBarcode := cleanOptional(in.BaseBarcode)
	units, err := validateAlternateUnits(baseCode, baseBarcode, in.AllowFractionalBaseQty, in.Units)
	if err != nil {
		return nil, err
	}
	baseName := strings.TrimSpace(in.BaseUnitName)
	if baseName == "" {
		baseName = defaultUnitName(baseCode)
	}
	if !in.AllowFractionalBaseQty {
		var fractionalStock bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM inventory_balances
			WHERE tenant_id=$1 AND product_id=$2
			  AND (abs(on_hand-round(on_hand))>0.0000001 OR abs(reserved-round(reserved))>0.0000001)
		)`, tenantID, productID).Scan(&fractionalStock); err != nil {
			return nil, err
		}
		if fractionalStock {
			return nil, errors.New("cannot disable fractional stock while this product has fractional inventory")
		}
	}
	// Deactivate alternates first so a barcode can safely move from an old package to the base unit.
	if _, err = tx.Exec(ctx, `UPDATE product_units SET active=false,updated_at=now() WHERE tenant_id=$1 AND product_id=$2 AND NOT is_base`, tenantID, productID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE products SET barcode=NULLIF($3,''),allow_fractional_base_qty=$4,updated_at=now() WHERE id=$1 AND tenant_id=$2`, productID, tenantID, cleanString(baseBarcode), in.AllowFractionalBaseQty); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE product_units SET name=$3,barcode=NULLIF($4,''),updated_at=now(),active=true,allow_sale=true,allow_purchase=true WHERE tenant_id=$1 AND product_id=$2 AND is_base`, tenantID, productID, baseName, cleanString(baseBarcode)); err != nil {
		return nil, err
	}
	for _, u := range units {
		_, err = tx.Exec(ctx, `INSERT INTO product_units(tenant_id,product_id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active)
			VALUES($1,$2,$3,$4,$5,NULLIF($6,''),false,$7,$8,true)
			ON CONFLICT(tenant_id,product_id,code) DO UPDATE SET name=EXCLUDED.name,factor_to_base=EXCLUDED.factor_to_base,barcode=EXCLUDED.barcode,allow_sale=EXCLUDED.allow_sale,allow_purchase=EXCLUDED.allow_purchase,active=true,updated_at=now()`, tenantID, productID, u.Code, u.Name, u.FactorToBase, cleanString(u.Barcode), u.AllowSale, u.AllowPurchase)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.listUnits(ctx, tenantID, productID)
}

func (s *Service) listUnits(ctx context.Context, tenantID, productID uuid.UUID) ([]ProductUnit, error) {
	rows, err := s.db.Query(ctx, `SELECT id,code,name,factor_to_base::float8,barcode,is_base,allow_sale,allow_purchase,active FROM product_units WHERE tenant_id=$1 AND product_id=$2 AND active ORDER BY is_base DESC,factor_to_base,name,id`, tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductUnit{}
	for rows.Next() {
		var u ProductUnit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.FactorToBase, &u.Barcode, &u.IsBase, &u.AllowSale, &u.AllowPurchase, &u.Active); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func validateAlternateUnits(baseCode string, baseBarcode *string, allowFractional bool, units []CreateProductUnit) ([]CreateProductUnit, error) {
	seenCodes := map[string]bool{baseCode: true}
	seenBarcodes := map[string]bool{}
	if baseBarcode != nil && strings.TrimSpace(*baseBarcode) != "" {
		seenBarcodes[strings.ToLower(strings.TrimSpace(*baseBarcode))] = true
	}
	out := make([]CreateProductUnit, 0, len(units))
	for _, raw := range units {
		u := raw
		u.Code = normalizeUnitCode(u.Code)
		u.Name = strings.TrimSpace(u.Name)
		u.Barcode = cleanOptional(u.Barcode)
		if u.Code == "" || u.Name == "" {
			return nil, errors.New("unit code and name are required")
		}
		if seenCodes[u.Code] {
			return nil, fmt.Errorf("duplicate product unit code %q", u.Code)
		}
		if u.FactorToBase <= 0 || math.IsNaN(u.FactorToBase) || math.IsInf(u.FactorToBase, 0) {
			return nil, fmt.Errorf("invalid conversion factor for unit %q", u.Name)
		}
		if !allowFractional && math.Abs(u.FactorToBase-math.Round(u.FactorToBase)) > 1e-9 {
			return nil, fmt.Errorf("unit %q creates fractional base quantity while fractional stock is disabled", u.Name)
		}
		if !u.AllowSale && !u.AllowPurchase {
			return nil, fmt.Errorf("unit %q must be enabled for sale, purchase, or both", u.Name)
		}
		if u.RetailPrice < 0 {
			return nil, fmt.Errorf("retail price for unit %q cannot be negative", u.Name)
		}
		if u.Barcode != nil && strings.TrimSpace(*u.Barcode) != "" {
			key := strings.ToLower(strings.TrimSpace(*u.Barcode))
			if seenBarcodes[key] {
				return nil, fmt.Errorf("duplicate barcode %q in product units", strings.TrimSpace(*u.Barcode))
			}
			seenBarcodes[key] = true
		}
		seenCodes[u.Code] = true
		out = append(out, u)
	}
	return out, nil
}

func cleanOptional(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}
func cleanString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
func normalizeUnitCode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "_")
	return v
}
func defaultUnitName(code string) string {
	switch code {
	case "pcs", "piece":
		return "عدد"
	case "pair":
		return "جفت"
	case "set":
		return "دست"
	case "pack":
		return "بسته"
	case "box":
		return "جعبه"
	case "carton":
		return "کارتن"
	default:
		if strings.TrimSpace(code) == "" {
			return "عدد"
		}
		return code
	}
}

func normalizeCatalogSearch(v string) string {
	r := strings.NewReplacer(
		"ي", "ی", "ى", "ی", "ك", "ک",
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	return strings.ToLower(strings.Join(strings.Fields(r.Replace(strings.TrimSpace(v))), " "))
}
