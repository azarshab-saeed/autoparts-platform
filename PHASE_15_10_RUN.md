# Phase 15.10 — Checks Pagination & Acceptance Hardening

Phase 15.10 continues the runtime hardening started in 15.9.1.

- the checks page consumes `total` and `next_cursor` instead of silently stopping after the first 100 rows
- `نمایش بیشتر` appends the next page without replacing already loaded checks
- search/status changes reset pagination to the first page
- the UI shows total rows vs. currently loaded rows
- check transition errors for outstanding-balance violations are translated to Persian as well
- the first page still respects the shared API maximum (`limit=100`)

No database migration is required.

## Validation

```bash
./ops/rc-static-check.sh
cd web && npm install --no-audit --no-fund && npm run build
```

Runtime acceptance:

1. Create or seed more than 100 receivable/payable checks.
2. Open `/store/checks` and verify the first page loads without an invalid-limit error.
3. Verify total count is greater than the loaded count and `نمایش بیشتر` is visible.
4. Click `نمایش بیشتر`; rows append and the loaded count grows without duplicates.
5. Change status or run a search; verify the list resets to the first page for that filter.
6. Trigger an endorsement/outstanding-balance rejection and verify the error is shown in Persian.
