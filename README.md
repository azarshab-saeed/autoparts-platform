# Auto Parts Platform

**Current milestone: Phase 8 — Reservation + Mechanic Orders**

The repository now contains a runnable store frontend, Keycloak authentication, Go API, PostgreSQL inventory/accounting core, purchases, sales, settlement/returns, cross-store network search, and inventory-backed mechanic reservations. Monetary integer values use **Iranian toman** (`IRT` application code).

See `PHASE_8_RUN.md` for the current upgrade/test flow and `RELEASE_NOTES_PHASE8.md` for implementation details.

---

# Auto Parts Platform - Phase 4.3

Multi-tenant auto-parts store/network platform with a Go core, PostgreSQL, Persian RTL Next.js frontend, and Keycloak identity provider.

## Current stack
- Go 1.23 API
- PostgreSQL 18.4
- Keycloak 26.7.0
- Next.js 16 + React 19.2 + TypeScript
- `keycloak-js` 26.2.4
- pgx/v5
- Modular monolith
- Standard `net/http`

## Authentication architecture

Keycloak owns authentication, passwords, browser login sessions, password reset, and realm roles.

The Go API does **not** receive user passwords. It accepts Keycloak access tokens and verifies:
- RS256 signature against Keycloak JWKS
- exact issuer
- `autoparts-api` audience
- expiration / not-before
- `tenant_id` claim
- `store_id` claim
- application realm roles

Business authorization remains in Go. Tenant/store IDs are never accepted from a client header as a trusted security context.

### Realm roles
- `owner`
- `admin`
- `cashier`
- `warehouse`
- `accountant`
- `mechanic`
- `consumer`

## Start everything

```bash
cp .env.example .env
docker compose up --build
```

Services:
- Web: http://localhost:3000
- API: http://localhost:8080
- Keycloak: http://localhost:8081
- Keycloak Admin Console: http://localhost:8081/admin
- PostgreSQL: localhost:5432

### Development login

Store application user:

```text
owner@example.com
ChangeMe123!
```

Keycloak admin console defaults:

```text
admin
admin-change-me
```

Change the admin password in `.env` before using the environment beyond local development.

## What happens on startup

1. PostgreSQL starts and applies the existing SQL init migrations on a fresh volume.
2. `db-prepare` idempotently creates the dedicated `keycloak` database/user and inserts a small demo store dataset.
3. Keycloak starts and imports `keycloak/autoparts-realm.json` if the `autoparts` realm does not already exist.
4. The Go API validates Keycloak bearer tokens. It no longer exposes `/v1/auth/login` or `/v1/auth/bootstrap`.
5. The Next.js app uses Authorization Code flow + PKCE through `keycloak-js`.

## Keycloak claims used by the API

The realm maps these user attributes into access tokens:

```text
tenant_id
store_id
```

Realm roles are read from:

```text
realm_access.roles
```

The web client adds the API audience:

```text
autoparts-api
```

## Important development behavior

The startup realm import is intentionally convenient for local development. If the `autoparts` realm already exists, Keycloak skips importing it. If you modify the realm JSON and want a clean re-import during development, either update the realm through the Admin Console or recreate the local Keycloak database/volume intentionally.

Do not delete a production database/volume to apply realm configuration changes.

## Frontend routes
- `/login` - redirects authentication to Keycloak
- `/store` - store dashboard
- `/store/sales` - operational sale screen
- `/store/purchases` - purchase entry and receiving screen
- `/store/inventory` - inventory, low-stock, adjustment, and reorder-point screen
- `/store/accounts` - customer/supplier balances and settlement
- `/store/returns` - sale/purchase returns
- `/store/network` - publish stock/prices to the cross-store network
- `/store/orders` - incoming mechanic reservations
- `/mechanic` - public network search
- `/mechanic/orders` - authenticated mechanic/consumer reservation tracking

## Legacy authentication code

The old HMAC token/password implementation is still present temporarily under `internal/platform/auth` so earlier migrations/tests remain understandable, but it is no longer wired into `cmd/api`. The `users` and `memberships` tables are also retained for a later migration/identity-linking decision.

## Go module network fallback

Docker builds use a retrying Go proxy chain and checksum verification. If your network has trouble with the defaults, edit `.env`:

```text
GO_MODULE_PROXY=https://goproxy.cn|https://proxy.golang.org|direct
GO_SUMDB=sum.golang.google.cn
```

## Phase 4.3 files
- `keycloak/autoparts-realm.json`
- `dev/postgres/prepare.sh`
- `dev/postgres/seed.sql`
- `internal/platform/auth/keycloak.go`
- `web/components/auth-provider.tsx`
- `web/lib/keycloak.ts`
- `RELEASE_NOTES_PHASE4_3.md`

## Phase 5 files
- `web/app/store/inventory/page.tsx`
- `web/app/store/purchases/page.tsx`
- `web/lib/api.ts`
- `web/lib/mock.ts`
- `docs/product/PHASE_5_ACCEPTANCE.md`
- `RELEASE_NOTES_PHASE5.md`


## Phase 8 reservation boundary

Reservations lock `inventory_balances.reserved` and therefore reduce public availability immediately. They do not post accounting entries and do not reduce `on_hand`. Store confirmation is a logistics state, not a financial sale. A later fulfillment slice should bind a reservation to the existing sale transaction atomically.

Development mechanic login for a fresh realm:

```text
mechanic@example.com
ChangeMe123!
```
