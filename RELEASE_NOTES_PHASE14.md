# Phase 14 — Production Readiness & Security Hardening

- Added `/healthz` liveness and `/readyz` database/schema readiness probes.
- Added `/version` build metadata and Docker build-time version/commit/timestamp injection.
- Added UUID request IDs and `X-Request-ID` response headers.
- Added JSON structured request logs with status, duration, bytes and client IP.
- Added panic recovery, API/browser security headers, strict CORS rejection, and configurable rate limits.
- Added `audit_logs` with tenant/store isolation for authenticated mutating requests.
- Added owner/admin audit API and `/store/audit` UI.
- Added SIGTERM/SIGINT graceful HTTP shutdown.
- Added migration `012_production_hardening.sql`.
- Added production Docker Compose with host-only port binding, healthchecks, no demo seed, and production Keycloak mode.
- Added idempotent production Keycloak realm/client/mapper bootstrap without demo users.
- Added production env template, deployment preflight, PostgreSQL backup and guarded restore scripts.
- Development `db-prepare` now supports `SEED_DEMO_DATA=false` and configurable Keycloak database credentials.
