package customers

import (
	"context"
	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Customer struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Phone *string   `json:"phone,omitempty"`
	Code  *string   `json:"code,omitempty"`
	Notes *string   `json:"notes,omitempty"`
}
type CreateCustomer struct {
	Name  string  `json:"name"`
	Phone *string `json:"phone,omitempty"`
	Code  *string `json:"code,omitempty"`
	Notes *string `json:"notes,omitempty"`
}
type Page struct {
	Items      []Customer `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

func (s *Service) Create(ctx context.Context, tenantID, storeID uuid.UUID, in CreateCustomer) (Customer, error) {
	var c Customer
	err := s.db.QueryRow(ctx, `INSERT INTO customers(tenant_id,store_id,name,phone,code,notes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,name,phone,code,notes`, tenantID, storeID, strings.TrimSpace(in.Name), in.Phone, in.Code, in.Notes).Scan(&c.ID, &c.Name, &c.Phone, &c.Code, &c.Notes)
	return c, err
}
func (s *Service) List(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit, offset int) (Page, error) {
	pat := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `SELECT id,name,phone,code,notes FROM customers WHERE tenant_id=$1 AND store_id=$2 AND deleted_at IS NULL AND ($3='%%' OR lower(name) LIKE $3 OR lower(COALESCE(phone,'')) LIKE $3 OR lower(COALESCE(code,'')) LIKE $3) ORDER BY name,id LIMIT $4 OFFSET $5`, tenantID, storeID, pat, limit+1, offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	out := Page{Items: make([]Customer, 0, limit)}
	for rows.Next() {
		var c Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Code, &c.Notes); err != nil {
			return Page{}, err
		}
		out.Items = append(out.Items, c)
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = pagination.EncodeOffset(offset + limit)
	}
	return out, rows.Err()
}
