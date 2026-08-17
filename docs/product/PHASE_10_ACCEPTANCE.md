# Phase 10 acceptance checklist

- [ ] Migration 008 applies on an existing database without deleting the volume.
- [ ] Default expense categories are available for the authenticated tenant.
- [ ] Cash expense posts debit Operating Expenses / credit Cash.
- [ ] Card expense posts debit Operating Expenses / credit Card Clearing.
- [ ] Repeating the same expense idempotency key returns the existing expense.
- [ ] Expense from another tenant/category cannot be used.
- [ ] Profit/loss is scoped to authenticated tenant and store.
- [ ] Sales returns reduce net sales.
- [ ] Sales-return cost reversals reduce net COGS.
- [ ] Operating expenses reduce net profit.
- [ ] Customer statement shows sale/receipt/return movements and running balance.
- [ ] Supplier statement shows purchase/payment/return movements and running balance.
- [ ] Store UI builds successfully with TypeScript.
