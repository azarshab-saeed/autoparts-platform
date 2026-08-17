# Release Notes - Phase 10

## Accounting completion

- Added `expense_categories` and `expenses` with migration 008.
- Default categories: rent, payroll, utilities, transport, supplies, marketing, other.
- Expense posting creates a balanced journal and outbox event atomically.
- Added store-level profit/loss report using posted sales, sales returns, perpetual COGS and expenses.
- Added customer and supplier party-ledger statements with running balances.

## Frontend

- New `/store/expenses` screen.
- New `/store/reports` profit/loss screen.
- `/store/accounts` now exposes `گردش حساب`.
- Reports and expenses are enabled in store navigation.

## API

- `GET /v1/expenses/categories`
- `GET /v1/expenses`
- `POST /v1/expenses`
- `GET /v1/reports/profit-loss`
- `GET /v1/accounts/customers/{id}/statement`
- `GET /v1/accounts/suppliers/{id}/statement`

OpenAPI version: 0.10.0.
