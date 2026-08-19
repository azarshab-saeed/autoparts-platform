# Release Notes — Phase 15.13

## Bank Reconciliation & Check Intelligence

Phase 15.13 closes major professional finance gaps for check-heavy stores.

### Checks

- Added weighted maturity averaging (راس‌گیری) for selected checks.
- Added maturity buckets, overdue exposure, future cash-in/out calendar and projected bank balance.
- Added customer check-risk indicators based on actual overdue and bounced history.

### Banking

- Added bank statement import with duplicate protection.
- Added reconciliation suggestions against actual bank-account journal entries.
- Added full and partial matches.
- Added match history and audited undo/reopen.
- Reconciliation is non-destructive: it does not rewrite posted accounting journals.

### UI

- Added **Finance Control / کنترل مالی پیشرفته** for Owner/Admin/Accountant roles.
- Includes dashboard, check maturity averaging and bank reconciliation tabs.

### Database

- Added migration `018_bank_reconciliation_intelligence.sql`.
