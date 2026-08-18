package customers

import (
	"context"
	"errors"
	"strings"

	"github.com/example/autoparts-core/internal/platform/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Customer struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Phone         *string    `json:"phone,omitempty"`
	Code          *string    `json:"code,omitempty"`
	Notes         *string    `json:"notes,omitempty"`
	PriceListID   *uuid.UUID `json:"price_list_id,omitempty"`
	PriceListName *string    `json:"price_list_name,omitempty"`
}
type CreateCustomer struct {
	Name        string     `json:"name"`
	Phone       *string    `json:"phone,omitempty"`
	Code        *string    `json:"code,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	PriceListID *uuid.UUID `json:"price_list_id,omitempty"`
}
type Page struct {
	Items      []Customer `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

func (s *Service) Create(ctx context.Context, tenantID, storeID uuid.UUID, in CreateCustomer) (Customer, error) {
	if in.PriceListID != nil {
		if err := s.validatePriceList(ctx, tenantID, storeID, *in.PriceListID); err != nil {
			return Customer{}, err
		}
	}
	var c Customer
	err := s.db.QueryRow(ctx, `INSERT INTO customers(tenant_id,store_id,name,phone,code,notes,price_list_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,name,phone,code,notes,price_list_id`, tenantID, storeID, strings.TrimSpace(in.Name), in.Phone, in.Code, in.Notes, in.PriceListID).Scan(&c.ID, &c.Name, &c.Phone, &c.Code, &c.Notes, &c.PriceListID)
	if err != nil {
		return Customer{}, err
	}
	if c.PriceListID != nil {
		_ = s.db.QueryRow(ctx, `SELECT name FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, *c.PriceListID, tenantID, storeID).Scan(&c.PriceListName)
	}
	return c, nil
}
func (s *Service) List(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit, offset int) (Page, error) {
	pat := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `SELECT c.id,c.name,c.phone,c.code,c.notes,CASE WHEN pl.id IS NULL THEN NULL ELSE c.price_list_id END,pl.name FROM customers c LEFT JOIN price_lists pl ON pl.id=c.price_list_id AND pl.tenant_id=c.tenant_id AND pl.store_id=c.store_id AND pl.active WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.deleted_at IS NULL AND ($3='%%' OR lower(c.name) LIKE $3 OR lower(COALESCE(c.phone,'')) LIKE $3 OR lower(COALESCE(c.code,'')) LIKE $3) ORDER BY c.name,c.id LIMIT $4 OFFSET $5`, tenantID, storeID, pat, limit+1, offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	out := Page{Items: make([]Customer, 0, limit)}
	for rows.Next() {
		var c Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Code, &c.Notes, &c.PriceListID, &c.PriceListName); err != nil {
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

func (s *Service) AssignPriceList(ctx context.Context, tenantID, storeID, customerID uuid.UUID, priceListID *uuid.UUID) (Customer, error) {
	if priceListID != nil {
		if err := s.validatePriceList(ctx, tenantID, storeID, *priceListID); err != nil {
			return Customer{}, err
		}
	}
	var c Customer
	err := s.db.QueryRow(ctx, `UPDATE customers SET price_list_id=$4,updated_at=now() WHERE id=$3 AND tenant_id=$1 AND store_id=$2 AND deleted_at IS NULL RETURNING id,name,phone,code,notes,price_list_id`, tenantID, storeID, customerID, priceListID).Scan(&c.ID, &c.Name, &c.Phone, &c.Code, &c.Notes, &c.PriceListID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, errors.New("customer not found")
	}
	if err != nil {
		return Customer{}, err
	}
	if c.PriceListID != nil {
		if err = s.db.QueryRow(ctx, `SELECT name FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, *c.PriceListID, tenantID, storeID).Scan(&c.PriceListName); err != nil {
			return Customer{}, err
		}
	}
	return c, nil
}

func (s *Service) validatePriceList(ctx context.Context, tenantID, storeID, id uuid.UUID) error {
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM price_lists WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active)`, id, tenantID, storeID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("price list does not belong to authenticated store")
	}
	return nil
}
