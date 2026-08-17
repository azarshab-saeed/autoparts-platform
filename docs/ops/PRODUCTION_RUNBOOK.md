# Production Deployment Runbook

## Topology

The provided production Compose is intended for a single-host first deployment behind a TLS reverse proxy. PostgreSQL stays on the private Docker network. Web, API and Keycloak bind only to loopback so the host reverse proxy is the public ingress.

Suggested public endpoints:

- `https://app.example.com` -> `127.0.0.1:3000`
- `https://api.example.com` -> `127.0.0.1:8080`
- `https://auth.example.com` -> `127.0.0.1:8081`

Forward `X-Forwarded-For`/`X-Forwarded-Proto` from a trusted proxy only. `TRUST_PROXY_HEADERS=true` is enabled only in the production Compose.

## Identity bootstrap

`keycloak-bootstrap` creates/reconciles the `autoparts` realm, application roles, `autoparts-web`, `autoparts-api`, PKCE settings, tenant/store attribute mappers and API audience. It creates **no application users**.

Provision application users through Keycloak administration/automation. Store users need `tenant_id` and `store_id` attributes plus a store realm role. Mechanic/consumer users do not need tenant/store attributes.

## Deployment

1. Copy `.env.production.example` to `.env.production` and replace all placeholders.
2. Run `ENV_FILE=.env.production ./ops/preflight.sh`.
3. Take a database backup before an upgrade.
4. Run `docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build`.
5. Wait for API health to become healthy.
6. Check `/readyz` and `/version` through the public TLS endpoint.
7. Test login and one read-only store request before enabling normal traffic.

## Rollback

Application rollback: redeploy the previous image/commit. Database migrations are forward-only; do not delete volumes. For a destructive/schema rollback, schedule maintenance and restore a verified pre-upgrade backup.

## Operational signals

API logs are one JSON object per line. Track at minimum 5xx rate, 429 rate, readiness failures, `panic_recovered`, `audit_write_failed`, API latency and PostgreSQL disk usage. The audit table is an operational/security record, not an application error log.
