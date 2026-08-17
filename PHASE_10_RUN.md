# Phase 10 - Accounting Completion

Phase 10 adds operating expenses, party-ledger statements and a store-level profit/loss report. It is designed to upgrade an existing Phase 9 database in place.

## Upgrade

Do not delete the PostgreSQL volume.

```bash
cp .env.example .env   # only if you do not already have .env
docker compose up --build
```

`db-prepare` applies `008_accounting_completion.sql` automatically through `schema_migrations`.

## UI smoke test

1. Login as the store owner.
2. Open `/store/expenses`.
3. Register an expense such as rent using cash or card.
4. Confirm the row appears in the expense list.
5. Open `/store/reports` and confirm operating expenses and net profit change for the selected date range.
6. Open `/store/accounts`, choose a customer or supplier and click `گردش حساب`.
7. Confirm credit sales/purchases and settlements appear in chronological order with a running balance.

## API smoke test

Get categories:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/expenses/categories
```

Post expense (replace category UUID):

```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: expense-demo-1" \
  -d '{"category_id":"CATEGORY_UUID","method":"card","amount":8500000,"note":"rent","occurred_on":"2026-08-16"}' \
  http://localhost:8080/v1/expenses
```

Profit/loss:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/v1/reports/profit-loss?from=2026-08-01&to=2026-08-31"
```

## Accounting invariants

- Expense posting is idempotent by `(tenant_id, idempotency_key)`.
- Expense journal: debit Operating Expenses; credit Cash or Card Clearing.
- Profit/loss is store scoped.
- Purchases are not expensed directly because inventory uses perpetual weighted-average valuation; COGS is recognized when sales consume inventory.
- Sales returns reduce revenue and reverse the corresponding COGS in the report.
- Party statements are isolated by tenant + store + party ownership.
