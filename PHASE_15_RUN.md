# Phase 15 - Release Candidate & End-to-End QA

Release candidate: `0.15.0-rc.1`
Baseline: `c4866a815f2a82955567cbacb7acc08154550338` (Phase 14)

## Start

```bash
APP_VERSION=0.15.0-rc.1 docker compose up -d --build
```

Phase 15 adds a **development-only** Keycloak client named `autoparts-qa`. It enables password grant only for automated local/CI QA. It is created by `keycloak-qa-sync` and is not present in `docker-compose.prod.yml` or the production bootstrap.

## Fast RC gate

```bash
make rc-check
```

Or run parts separately:

```bash
make rc-smoke
make rc-db-check
make rc-load
```

The smoke suite verifies liveness/readiness/version, Keycloak identities, store tenant isolation, OEM network search, idempotent inventory adjustment, reservation hold/release and procurement hold/release. Mutations are self-reverting. Set `RC_SMOKE_MUTATING=false` for read-only smoke.

## Performance defaults

The baseline sends 100 network-search requests with concurrency 8. Default thresholds:

- error rate <= 1%
- p95 <= 750 ms

Override with `RC_LOAD_REQUESTS`, `RC_LOAD_CONCURRENCY`, `RC_LOAD_MAX_FAILURE_PCT` and `RC_LOAD_MAX_P95_MS`.

## Database invariants

`ops/rc-db-check.sh` fails on negative/over-reserved inventory, hold reconciliation drift, unbalanced journals, invalid paid/due totals, broken reservation/procurement document links or tenant mismatch in document items.

## CI

`.github/workflows/rc.yml` runs unit tests, Next.js production build, full Compose smoke, database invariants and a performance baseline.

Do not delete the PostgreSQL volume for this phase.
