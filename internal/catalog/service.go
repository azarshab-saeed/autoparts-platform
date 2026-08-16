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
	q = "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `SELECT id,sku,title,brand,oem_code,barcode,unit,active FROM products WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2='%%' OR normalized_title LIKE $2 OR lower(COALESCE(sku,'')) LIKE $2 OR lower(COALESCE(oem_code,'')) LIKE $2 OR lower(COALESCE(barcode,'')) LIKE $2) ORDER BY title,id LIMIT $3 OFFSET $4`, tenantID, q, limit+1, offset)
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
