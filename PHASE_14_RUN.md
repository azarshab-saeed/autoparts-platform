# Phase 14 Runbook — Production Readiness & Security Hardening

Phase 14 adds production-oriented health/readiness probes, structured access logs, request IDs, panic recovery, in-memory rate limiting, strict CORS handling, security headers, store-scoped audit logs, graceful shutdown, build metadata, production Compose, Keycloak production bootstrap, and PostgreSQL backup/restore tooling.

## Development upgrade

Do not delete the PostgreSQL volume.

```bash
docker compose up --build
```

Check probes:

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i http://localhost:8080/version
```

`/readyz` must report migration `012_production_hardening.sql`.

## Audit smoke test

1. Login as owner/admin.
2. Perform any mutation such as an inventory reorder-point update or network offer update.
3. Open `/store/audit`.
4. Confirm method, path, HTTP status, actor role, request id, client IP and timestamp are present.
5. Verify a cashier cannot call `GET /v1/audit-logs`.

## Production preflight

```bash
cp .env.production.example .env.production
# replace every placeholder with real secrets/URLs
ENV_FILE=.env.production ./ops/preflight.sh
```

Production mode never runs demo seed data and never creates demo Keycloak application users.

Start:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
```

Put a TLS reverse proxy/load balancer in front of ports 3000/8080/8081. The production compose publishes them only on 127.0.0.1.

## Backup / restore

```bash
COMPOSE_FILE=docker-compose.prod.yml ./ops/backup.sh
CONFIRM_RESTORE=YES COMPOSE_FILE=docker-compose.prod.yml ./ops/restore.sh backups/<timestamp>
```

Restore intentionally stops API/Web traffic while PostgreSQL is restored.
