# Phase 15 Release Notes - RC1

Phase 15 turns the Phase 14 production-ready stack into a repeatable release candidate gate.

- RC identifier `0.15.0-rc.1`.
- Automated authenticated E2E smoke with deterministic dev users.
- Development-only `autoparts-qa` Keycloak client for CI/password-grant automation; production remains unchanged.
- Cross-tenant isolation and cross-tenant network search verification.
- Self-reverting inventory idempotency test.
- Self-reverting reservation and procurement hold/release tests.
- PostgreSQL invariant checker for inventory, holds, accounting journals, sales/purchases and network document links.
- Concurrent network-search performance baseline with p50/p95/p99 and threshold enforcement.
- GitHub Actions RC gate and operator checklist.

No schema migration is introduced in Phase 15. Latest required migration remains `012_production_hardening.sql`.
