# Phase 11.1 - Keycloak Dev Sync & Cross-Store Test Users

Phase 11.1 removes the need to delete the PostgreSQL/Keycloak volume when development realm data drifts behind `keycloak/autoparts-realm.json`.

## What changed

`keycloak-sync` runs after Keycloak starts and uses Keycloak Admin CLI to idempotently enforce the development identity contract:

- realm roles: `owner`, `admin`, `cashier`, `warehouse`, `accountant`, `mechanic`, `consumer`
- `tenant_id` and `store_id` access-token mappers on `autoparts-web`
- `autoparts-api` audience mapper on `autoparts-web`
- permanent development passwords
- deterministic role assignment for development users
- owner users for all three seeded network stores

No PostgreSQL volume reset is required.

## Start

```bash
docker compose up --build
```

To inspect the sync result:

```bash
docker compose logs keycloak-sync
```

Expected final line:

```text
[keycloak-sync] Keycloak dev sync completed
```

If a user was already logged in before the sync, log out and log in again so Keycloak issues a fresh access token.

## Development identities

All users use the default password `ChangeMe123!`. Override it with `KEYCLOAK_DEV_PASSWORD` when needed.

| Identity | Username | Store / role |
|---|---|---|
| Store owner 1 | `owner@example.com` | یدکی رضایی |
| Store owner 2 | `central@example.com` | یدکی مرکزی |
| Store owner 3 | `pars@example.com` | پارس یدک |
| Mechanic | `mechanic@example.com` | mechanic |
| Consumer | `consumer@example.com` | consumer |

## Cross-store reservation smoke test

1. Login as `mechanic@example.com` and search for `لنت جلو ۲۰۶`.
2. Reserve the `پارس یدک` offer (seed price: 1,750,000 toman).
3. Logout and login as `pars@example.com`.
4. Open `/store/orders`; the reservation must be visible there.
5. The same reservation must not appear for `owner@example.com` or `central@example.com`.
6. Accept it, mark it ready, then fulfill it as a sale.

For `یدکی مرکزی`, use a reservable seeded offer such as `واترپمپ EF7`; its seeded brake-pad offer intentionally has reservation disabled.

## Troubleshooting

If `keycloak-sync` cannot authenticate, verify that `KEYCLOAK_ADMIN_USERNAME` and `KEYCLOAK_ADMIN_PASSWORD` match the existing Keycloak admin account. Bootstrap admin environment variables only create the first admin on a fresh Keycloak database; the sync uses the actual current admin credentials.
