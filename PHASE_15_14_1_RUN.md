# Phase 15.14.1 - Tax Invoice Date Range SQL Hotfix

## Purpose
Fix PostgreSQL `SQLSTATE 42883` raised when the tax invoice register applies an inclusive end-date filter.

## Root cause
PostgreSQL could infer an untyped bind parameter in `$4 + interval '1 day'` as an `interval`, producing an invalid comparison between `timestamp with time zone` and `interval`.

## Changes
- Explicitly cast date-range bind parameters to `timestamptz` in the tax invoice register.
- Apply the same fix to sales and purchase history queries that used the same latent pattern.
- Add an RC static gate that rejects uncast bind-parameter interval arithmetic in Go SQL.

## Migration
None.

## Acceptance
- `./ops/rc-static-check.sh`
- `git diff --check`
- Open `/store/tax` and verify the invoice register loads for a date range without SQLSTATE 42883.
- Verify sales and purchase history date filters still load.

## Roadmap
This is a maintenance hotfix. Phase 15.14 remains complete and Phase 15.15 remains next.
