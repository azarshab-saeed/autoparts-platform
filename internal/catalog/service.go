package catalog

import (
	"context"
	"strings"

	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Product struct {
	ID      uuid.UUID `json:"id"`
	SKU     *string   `json:"sku,omitempty"`
	Title   string    `json:"title"`
	Brand   *string   `json:"brand,omitempty"`
	OEMCode *string   `json:"oem_code,omitempty"`
	Barcode *string   `json:"barcode,omitempty"`
	Unit    string    `json:"unit"`
	Active  bool      `json:"active"`
}
type CreateProduct struct {
	SKU     *string `json:"sku,omitempty"`
	Title   string  `json:"title"`
	Brand   *string `json:"brand,omitempty"`
	OEMCode *string `json:"oem_code,omitempty"`
	Barcode *string `json:"barcode,omitempty"`
	Unit    string  `json:"unit"`
}
type Page struct {
	Items      []Product `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, in CreateProduct) (Product, error) {
	if strings.TrimSpace(in.Unit) == "" {
		in.Unit = "pcs"
	}
	var p Product
	err := s.db.QueryRow(ctx, `INSERT INTO products(tenant_id,sku,title,brand,oem_code,barcode,unit,normalized_title) VALUES($1,$2,$3,$4,$5,$6,$7,lower(trim($3))) RETURNING id,sku,title,brand,oem_code,barcode,unit,active`, tenantID, in.SKU, strings.TrimSpace(in.Title), in.Brand, in.OEMCode, in.Barcode, in.Unit).Scan(&p.ID, &p.SKU, &p.Title, &p.Brand, &p.OEMCode, &p.Barcode, &p.Unit, &p.Active)
	return p, err
}
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q string, limit, offset int) (Page, error) {
	q = "%" + normalizeCatalogSearch(q) + "%"
	rows, err := s.db.Query(ctx, `SELECT id,sku,title,brand,oem_code,barcode,unit,active FROM products p WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2='%%' OR translate(lower(p.normalized_title),'۰۱۲۳۴۵۶۷۸۹٠١٢٣٤٥٦٧٨٩','01234567890123456789') LIKE $2 OR lower(COALESCE(sku,'')) LIKE $2 OR lower(COALESCE(oem_code,'')) LIKE $2 OR lower(COALESCE(barcode,'')) LIKE $2 OR EXISTS (SELECT 1 FROM product_search_terms pst WHERE pst.tenant_id=p.tenant_id AND pst.product_id=p.id AND pst.normalized_term LIKE $2)) ORDER BY title,id LIMIT $3 OFFSET $4`, tenantID, q, limit+1, offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	out := Page{Items: make([]Product, 0, limit)}
	for rows.Next() {
		var p Product
		if err = rows.Scan(&p.ID, &p.SKU, &p.Title, &p.Brand, &p.OEMCode, &p.Barcode, &p.Unit, &p.Active); err != nil {
			return Page{}, err
		}
		out.Items = append(out.Items, p)
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = pagination.EncodeOffset(offset + limit)
	}
	return out, rows.Err()
}

func normalizeCatalogSearch(v string) string {
	r := strings.NewReplacer(
		"ي", "ی", "ى", "ی", "ك", "ک",
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	return strings.ToLower(strings.Join(strings.Fields(r.Replace(strings.TrimSpace(v))), " "))
}
