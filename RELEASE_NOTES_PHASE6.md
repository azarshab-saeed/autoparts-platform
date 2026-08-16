# Phase 6 — Settlement + Returns

This phase closes the first operational finance loop across sales, purchases, inventory and party balances.

## Delivered

### Split and partial payments
- Sales and purchases accept multiple real payment parts (`cash`, `card`).
- Any unpaid remainder is posted to the customer/supplier party ledger.
- `paid_amount` and `due_amount` are stored on the originating document as posting-time facts.
- Credit is no longer treated as a money payment.

### Customer / supplier balances
- New per-party ledger (`party_ledger_entries`).
- `GET /v1/accounts/customers`
- `GET /v1/accounts/suppliers`
- Positive customer balance = customer owes the store.
- Positive supplier balance = store owes the supplier.

### Settlements
- `POST /v1/settlements/customer-receipts`
- `POST /v1/settlements/supplier-payments`
- Cash/card only; over-settlement is rejected in V1.
- Accounting entries and party-ledger entries are committed atomically.
- Idempotency is required.

### Returns
- `GET /v1/sales/{id}` and `GET /v1/purchases/{id}` expose returnable quantities.
- `POST /v1/returns/sales`
- `POST /v1/returns/purchases`
- Prevents returning more than the remaining source quantity.
- Sale returns restore inventory at the original sale cost and reverse COGS.
- Purchase returns subtract inventory at the original purchase cost and reject valuation conflicts.
- Refund options: cash, card, or party balance.

### Frontend
- `/store/accounts` — customer receivables and supplier payables with settlement modal.
- `/store/returns` — sale/purchase return workflow.
- Sales UI supports cash, card, credit, and mixed/partial payment.
- Purchase UI supports cash, card, credit, and mixed/partial payment.
- Store navigation and quick actions now link to accounts and returns.

### Migration runner
Previous phases relied on PostgreSQL entrypoint migrations, which only run on a fresh volume. Phase 6 moves migration execution into `db-prepare` with a `schema_migrations` table.

Existing Phase 1–5 databases are adopted by inspecting known schema objects, then migration 004 is applied without deleting the volume.

### Money unit correction
The UI and historical sample values were expressed in toman while document currency was previously marked `IRR`. Phase 6 standardizes the V1 business amount unit as **Iranian toman** and uses application code `IRT` to remove the factor-of-10 ambiguity.

## Database migration
`migrations/004_settlements_returns.sql`

## Important limitations
- Settlements are party-level; V1 does not allocate a receipt/payment to individual invoices.
- Overpayments/customer advances are intentionally rejected by the settlement API.
- Purchase-return valuation uses the original purchase unit cost and rejects a return if that would make current weighted-average inventory value invalid.
- Full end-to-end Docker build could not be run in the artifact environment because external Go/npm registries are not reachable. Go files were formatted/parsed, TypeScript/TSX syntax was transpile-checked, YAML/JSON/shell files were validated.
