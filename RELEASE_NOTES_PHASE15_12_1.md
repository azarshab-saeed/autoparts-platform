# Release Notes — Phase 15.12.1

## API build hotfix

- Fixed the Go API image build after Phase 15.12 by removing a stale unused `fmt` import from `internal/pricing/service.go`.
- No API contract, database schema, pricing behavior, inventory behavior or UI behavior changed.
- No migration is required.

## Next product phase

Phase 15.13 — Bank Reconciliation & Check Intelligence remains next in `COMPETITIVE_ROADMAP.md`.
