# Release Notes - Phase 11.1

## Keycloak dev-state reconciliation

- Added an idempotent `keycloak-sync` Compose service.
- Existing Keycloak databases are repaired without deleting the PostgreSQL volume.
- Realm application roles are ensured on every development start.
- `tenant_id`, `store_id`, and API audience protocol mappers are reconciled.
- Added separate owner test users for the two extra seeded network stores.
- Added a consumer test user and enforced the mechanic realm role.
- Updated the login screen to show which development owner belongs to which store.
- Added a cross-store reservation smoke-test guide.

This is development tooling only; production identity provisioning should use a separate controlled administration workflow and production secrets.
