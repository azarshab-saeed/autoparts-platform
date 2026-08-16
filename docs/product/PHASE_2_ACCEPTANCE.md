# Phase 2 Acceptance - Backend Foundation

Status: Completed in source; dependency download/runtime integration requires a networked Go/Docker environment.

## Delivered
- Signed bearer access tokens with tenant/store/role claims.
- Password hashing using PBKDF2-HMAC-SHA256 with per-user random salt.
- First-install bootstrap flow protected by BOOTSTRAP_SECRET.
- User, membership and role persistence.
- Tenant/store identity derived from authenticated server context, not X-Tenant-ID.
- Catalog create/list/search with pagination.
- Customer create/list/search with pagination.
- Supplier create/list/search with pagination.
- /v1/me context endpoint.
- Standard JSON error envelope.
- Cursor pagination helpers.
- PostgreSQL migration 002_foundation.sql.
- Dockerfile and docker-compose.yml.
- Unit tests for password, token and pagination primitives.
- Updated OpenAPI contract.

## Security boundary
Every protected handler resolves tenant_id/store_id from token claims. New handlers do not accept tenant identity from request bodies or custom tenant headers.

## Known deliberate deferrals
- Refresh tokens/session revocation.
- MFA.
- Fine-grained permissions below role level.
- Account recovery/email verification.
- Rate limiting and brute-force protection.
These belong to hardening/production auth, not the initial foundation.
