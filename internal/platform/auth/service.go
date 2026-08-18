package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db     *pgxpool.Pool
	tokens *TokenManager
}

func NewService(db *pgxpool.Pool, tokens *TokenManager) *Service {
	return &Service{db: db, tokens: tokens}
}

type LoginRequest struct {
	Email    string     `json:"email"`
	Password string     `json:"password"`
	StoreID  *uuid.UUID `json:"store_id,omitempty"`
}
type LoginResponse struct {
	AccessToken string         `json:"access_token"`
	ExpiresIn   int64          `json:"expires_in"`
	User        UserSummary    `json:"user"`
	Context     ContextSummary `json:"context"`
}
type UserSummary struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}
type ContextSummary struct {
	TenantID uuid.UUID `json:"tenant_id"`
	StoreID  uuid.UUID `json:"store_id"`
	Role     string    `json:"role"`
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	var user UserSummary
	var hash string
	var active bool
	err := s.db.QueryRow(ctx, `SELECT id,email,display_name,password_hash,active FROM users WHERE lower(email)=lower($1)`, strings.TrimSpace(req.Email)).Scan(&user.ID, &user.Email, &user.DisplayName, &hash, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginResponse{}, errors.New("invalid credentials")
		}
		return LoginResponse{}, err
	}
	if !active || !VerifyPassword(hash, req.Password) {
		return LoginResponse{}, errors.New("invalid credentials")
	}
	var tenantID, storeID uuid.UUID
	var role string
	if req.StoreID != nil {
		err = s.db.QueryRow(ctx, `SELECT tenant_id,store_id,role FROM memberships WHERE user_id=$1 AND store_id=$2 AND active=true ORDER BY CASE role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 ELSE 9 END LIMIT 1`, user.ID, *req.StoreID).Scan(&tenantID, &storeID, &role)
	} else {
		err = s.db.QueryRow(ctx, `SELECT tenant_id,store_id,role FROM memberships WHERE user_id=$1 AND active=true AND store_id IS NOT NULL ORDER BY created_at LIMIT 1`, user.ID).Scan(&tenantID, &storeID, &role)
	}
	if err != nil {
		return LoginResponse{}, errors.New("no active store membership")
	}
	ttl := 12 * time.Hour
	token, err := s.tokens.Sign(Claims{UserID: user.ID, TenantID: tenantID, StoreID: storeID, Role: role}, ttl)
	if err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: token, ExpiresIn: int64(ttl.Seconds()), User: user, Context: ContextSummary{TenantID: tenantID, StoreID: storeID, Role: role}}, nil
}

type BootstrapRequest struct {
	TenantName string `json:"tenant_name"`
	StoreName  string `json:"store_name"`
	StoreCode  string `json:"store_code"`
	AdminName  string `json:"admin_name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}
type BootstrapResponse struct {
	TenantID uuid.UUID `json:"tenant_id"`
	StoreID  uuid.UUID `json:"store_id"`
	UserID   uuid.UUID `json:"user_id"`
}

func (s *Service) Bootstrap(ctx context.Context, req BootstrapRequest) (BootstrapResponse, error) {
	hash, err := HashPassword(req.Password)
	if err != nil {
		return BootstrapResponse{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return BootstrapResponse{}, err
	}
	defer tx.Rollback(ctx)
	var out BootstrapResponse
	if err = tx.QueryRow(ctx, `INSERT INTO tenants(name) VALUES($1) RETURNING id`, strings.TrimSpace(req.TenantName)).Scan(&out.TenantID); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO stores(tenant_id,name,code) VALUES($1,$2,NULLIF($3,'')) RETURNING id`, out.TenantID, strings.TrimSpace(req.StoreName), strings.TrimSpace(req.StoreCode)).Scan(&out.StoreID); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active) VALUES($1,$2,'retail','خرده / مصرف‌کننده',true,true)`, out.TenantID, out.StoreID); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO store_pricing_settings(tenant_id,store_id) VALUES($1,$2)`, out.TenantID, out.StoreID); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO users(email,display_name,password_hash) VALUES(lower($1),$2,$3) RETURNING id`, strings.TrimSpace(req.Email), strings.TrimSpace(req.AdminName), hash).Scan(&out.UserID); err != nil {
		return out, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO memberships(user_id,tenant_id,store_id,role) VALUES($1,$2,$3,'owner')`, out.UserID, out.TenantID, out.StoreID)
	if err != nil {
		return out, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO warehouses(tenant_id,store_id,name) VALUES($1,$2,'انبار اصلی')`, out.TenantID, out.StoreID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
