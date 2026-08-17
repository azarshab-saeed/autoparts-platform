# Phase 14 Acceptance

- `GET /healthz` remains 200 when the database is unavailable (process liveness).
- `GET /readyz` is 200 only when PostgreSQL is reachable and migration 012 is recorded.
- Every API response includes `X-Request-ID`; caller-provided safe IDs are propagated.
- Requests over the configured per-IP rate limit return HTTP 429 plus `Retry-After`.
- A browser Origin not listed in `ALLOWED_ORIGINS` receives 403.
- Mutating authenticated requests create one store-scoped audit row; an audit-write failure is logged and does not replace the business response.
- Owner/admin can read `/v1/audit-logs`; other store roles receive 403.
- SIGTERM performs graceful HTTP shutdown.
- Production Compose does not run demo seeds or Keycloak dev-user sync.
- Production preflight rejects placeholder/short passwords and non-HTTPS public URLs.
- Backup creates a PostgreSQL custom-format dump and checksum when `sha256sum` is available.
- Restore requires explicit `CONFIRM_RESTORE=YES` and pauses API/Web traffic.
