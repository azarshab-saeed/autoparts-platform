//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test assumes migrations 001 and 002 have already been applied to TEST_DATABASE_URL.
// Example: TEST_DATABASE_URL=postgres://autoparts:autoparts@localhost:5432/autoparts?sslmode=disable go test -tags=integration ./tests/integration
func TestTenantScopedCatalogIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tenantA, tenantB := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO tenants(id,name) VALUES($1,'Tenant A'),($2,'Tenant B')`, tenantA, tenantB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM tenants WHERE id IN ($1,$2)`, tenantA, tenantB)
	})

	if _, err = db.Exec(ctx, `INSERT INTO products(tenant_id,title,normalized_title) VALUES($1,'Shared Name','shared name'),($2,'Shared Name','shared name')`, tenantA, tenantB); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM products WHERE tenant_id=$1`, tenantA).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one tenant A product, got %d", count)
	}
}
