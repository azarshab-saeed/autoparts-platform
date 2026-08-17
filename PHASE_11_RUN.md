# Phase 11 — Store Operations & Reporting Hardening

Phase 11 is an in-place upgrade from Phase 10 / Phase 9.1. Do **not** delete the PostgreSQL volume. `db-prepare` will apply `009_store_operations.sql` once through `schema_migrations`.

## Upgrade

```bash
git status
docker compose up --build
```

The new migration creates `daily_closings` and adds supporting indexes for operational history/reporting. Existing sales, purchases, inventory, journals and reservations are preserved.

## Acceptance flow

1. Sign in as `owner@example.com`.
2. Open `/store` and confirm the dashboard uses live values (not the old static mock dashboard when `NEXT_PUBLIC_MOCK_MODE=false`).
3. Create a sale, then open `/store/sales/history`; filter by date/payment state and open the invoice detail.
4. Print the invoice (browser print dialog) and test the shortcut to returns.
5. Create a purchase and repeat the flow under `/store/purchases/history`.
6. Open `/store/reports/inventory`; verify value, low-stock, sold-30d and dead-stock columns, then download CSV.
7. Open `/store/reports`; download the profit/loss CSV.
8. Open `/store/closing`; verify cash/card inflows/outflows against the transactions you created.
9. Enter opening cash and counted cash and close the day. Repeated submission with the same idempotency key is safe; a second independent close for the same store/date is rejected.
10. After closing, create another same-day cash transaction and reopen `/store/closing`. `changed_after_close` must surface as the red warning.

## API smoke checks

```bash
# all require a valid store bearer token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/dashboard
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/v1/sales?from=2026-08-01&to=2026-08-31&limit=50"
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/v1/purchases?from=2026-08-01&to=2026-08-31&limit=50"
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/v1/reports/inventory?warehouse_id=$WAREHOUSE_ID&sort=value&limit=100"
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/v1/reports/cash?date=2026-08-16"
```

Daily close:

```bash
curl -X POST http://localhost:8080/v1/daily-closings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: close-2026-08-16-1" \
  -d '{"business_date":"2026-08-16","opening_cash":10000000,"actual_cash":12300000,"note":"end of day"}'
```

## Important boundary

A daily close is a **snapshot/reconciliation**, not a database lock. Phase 11 deliberately does not block late/backdated transactions because existing sales/purchase/return flows do not yet carry a formal business-day lock. If later activity changes the day's cash totals, the report returns `changed_after_close=true`. A stricter fiscal lock can be added in a later production-hardening phase.
