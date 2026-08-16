package stores

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Me struct {
	UserID      uuid.UUID `json:"user_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	StoreID     uuid.UUID `json:"store_id"`
	Role        string    `json:"role"`
	Roles       []string  `json:"roles,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	Email       string    `json:"email,omitempty"`
	StoreName   string    `json:"store_name"`
	WarehouseID uuid.UUID `json:"default_warehouse_id"`
}

func (s *Service) Context(ctx context.Context, userID, tenantID, storeID uuid.UUID, role string) (Me, error) {
	m := Me{UserID: userID, TenantID: tenantID, StoreID: storeID, Role: role}
	err := s.db.QueryRow(ctx, `SELECT s.name,w.id FROM stores s JOIN warehouses w ON w.store_id=s.id AND w.tenant_id=s.tenant_id WHERE s.tenant_id=$1 AND s.id=$2 AND s.active=true ORDER BY w.created_at LIMIT 1`, tenantID, storeID).Scan(&m.StoreName, &m.WarehouseID)
	return m, err
}
