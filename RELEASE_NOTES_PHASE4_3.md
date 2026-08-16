# Phase 4.3 - Keycloak Integration

## Completed
- Added Keycloak 26.7.0 to Docker Compose.
- Added dedicated Keycloak PostgreSQL database in the same local PostgreSQL instance.
- Added repeatable local DB preparation service for existing as well as fresh dev volumes.
- Added `autoparts` realm import.
- Added public SPA client `autoparts-web` using Authorization Code flow and PKCE.
- Added API audience `autoparts-api`.
- Added realm roles for store, mechanic, and consumer identities.
- Added token mappers for `tenant_id` and `store_id`.
- Added a deterministic local owner account and matching demo tenant/store/warehouse data.
- Replaced Go runtime authentication wiring with Keycloak JWT/JWKS verification.
- Validates issuer, audience, expiry, not-before, RSA signature, tenant/store UUIDs, and application roles.
- Frontend uses `keycloak-js`; tokens are kept in memory rather than localStorage.
- Removed runtime routes `/v1/auth/login` and `/v1/auth/bootstrap`.
- Updated `/v1/me` to return identity display name, email, and roles from the verified token context.
- Updated OpenAPI security scheme to Keycloak OIDC discovery.

## Local credentials

Application:
- user: `owner@example.com`
- password: `ChangeMe123!`

Keycloak admin console defaults:
- user: `admin`
- password: `admin-change-me`

These credentials are local-development defaults only.

## Security boundary

Keycloak authenticates identity. Go remains authoritative for business authorization and tenant/store data isolation.

The API intentionally uses two Keycloak URLs in local Docker:
- public issuer: `http://localhost:8081/realms/autoparts`
- internal JWKS URL: `http://keycloak:8080/realms/autoparts/protocol/openid-connect/certs`

This allows browser-visible token issuer validation while the API fetches signing keys through the Docker network.

## Deferred
- Production TLS/reverse-proxy topology.
- Separate admin hostname/firewall policy for Keycloak.
- User provisioning workflow from store administration UI.
- Identity linking/migration strategy for legacy `users` and `memberships` tables.
- Custom Keycloak Persian login theme.
- MFA/passkeys/social login.


## Phase 4.3.1 registry hotfix
- Default Keycloak image changed from Quay to `keycloak/keycloak:26.7.0` on Docker Hub for networks where `quay.io` times out.
- Added `KEYCLOAK_IMAGE` environment override.
- No authentication or realm behavior changed.
