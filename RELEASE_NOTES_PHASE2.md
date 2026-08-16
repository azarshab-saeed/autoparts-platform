# Phase 2 Release Notes

## Release scope
Backend foundation for authenticated multi-tenant store operations.

## New API surface
- `POST /v1/auth/bootstrap`
- `POST /v1/auth/login`
- `GET /v1/me`
- `GET|POST /v1/products`
- `GET|POST /v1/customers`
- `GET|POST /v1/suppliers`
- Existing `POST /v1/sales` now derives tenant/store from authenticated claims.

## Database
Adds `002_foundation.sql` for users, memberships/roles, suppliers, brand/category foundations and catalog/party metadata.

## Security changes
- Client-supplied tenant header is removed from new request flow.
- Tenant/store/role are signed into access tokens and verified server-side.
- Sale warehouse and customer are checked against authenticated tenant/store.
- Bootstrap is protected by a separate deployment secret.

## Verification performed in this environment
- `gofmt` clean across cmd/internal/tests.
- `go test ./internal/platform/pagination` passes.
- Full `go test ./...` cannot download `github.com/google/uuid` and `github.com/jackc/pgx/v5` because outbound DNS/network access to proxy.golang.org is blocked in the execution container.

## Run full verification in a networked environment
```sh
go mod download
go test ./...
TEST_DATABASE_URL='postgres://...' go test -tags=integration ./tests/integration
```
