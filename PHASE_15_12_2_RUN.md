# Phase 15.12.2 — Web Build Hotfix

## Purpose
Fix the production Next.js TypeScript build blocker introduced in the mock sales-history fixtures after Phase 15.11 pricing history fields became required.

## Changes
- Remove the duplicate `below_margin_count` key from the first mock sale-history record.
- Add the required `below_margin_count` value to the second mock sale-history record.
- No API, database, pricing, inventory, or runtime behavior changes.
- No migration.

## Acceptance
- `npm run build` in `web/` completes TypeScript checking past the previous `TS1117` / `TS2741` errors.
- `git diff --check` passes.

## Roadmap
This is a build hotfix only. The next product phase remains **Phase 15.13 — Bank Reconciliation & Check Intelligence**.
