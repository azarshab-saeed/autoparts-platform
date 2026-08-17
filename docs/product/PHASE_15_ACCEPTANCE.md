# Phase 15 Acceptance - Release Candidate RC1

Phase 15 is accepted when all of the following are green on a clean Phase 14 database upgraded in place:

1. `docker compose up -d --build` completes and `keycloak-qa-sync` exits 0.
2. `/healthz`, `/readyz` and `/version` return HTTP 200.
3. `go test ./...` passes.
4. `web/npm run build` passes.
5. `go run ./cmd/rc-smoke` passes all identity, isolation, search and self-reverting mutation checks.
6. `./ops/rc-db-check.sh` reports zero invariant violations.
7. `go run ./cmd/rc-load` stays under configured error-rate and p95 thresholds.
8. Full browser regression succeeds for store sale/purchase, mechanic reservation, store fulfillment, network procurement receive, fitment search, reports and audit log.
9. Production preflight from Phase 14 still passes and no `autoparts-qa` client is created by production bootstrap.
10. The candidate can be reverted with one Git revert commit and the existing database remains valid because Phase 15 has no migration.
