# Phase 3 Acceptance — Inventory + Purchase

## Completed
- Purchase posting with supplier/store/warehouse isolation.
- Purchase idempotency using `(tenant_id, idempotency_key)`.
- Perpetual weighted-average inventory valuation (`avg_unit_cost`).
- Purchase inventory movements with quantity and cost deltas.
- Purchase journal: Inventory debit; Cash/Card/AP credit.
- Cash/card purchase payment record; credit purchase leaves Accounts Payable open.
- Sale COGS journal using weighted-average cost in the same serializable transaction.
- Sale inventory movement now captures cost basis.
- Inventory listing with on-hand, reserved, available, average cost, reorder thresholds and low-stock state.
- Reorder point update.
- Physical inventory adjustment with idempotency and `on_hand >= reserved` protection.
- Phase 3 migration and OpenAPI contract.

## Chosen valuation rule
The MVP uses **perpetual weighted-average cost** per `(tenant, warehouse, product)`.

On purchase:
`new_avg = ((old_qty * old_avg) + (incoming_qty * incoming_unit_cost)) / (old_qty + incoming_qty)`

On sale:
- quantity is reduced;
- the current weighted-average unit cost is captured on the movement;
- COGS is debited and Inventory is credited atomically.

The average cost is intentionally not changed by a normal sale.

## Accounting behavior implemented
### Purchase paid now
- Dr Inventory
- Cr Cash or Card Clearing

### Purchase on credit
- Dr Inventory
- Cr Accounts Payable

### Sale
- Dr Cash / Card Clearing / Accounts Receivable
- Cr Sales Revenue
- Dr Cost of Goods Sold
- Cr Inventory

## Explicitly deferred
- Purchase return and sale return workflows.
- Void/reversal journals.
- Partial and mixed payments.
- Supplier/customer sub-ledger allocation to individual documents.
- Tax/VAT/discount/freight allocation into inventory cost.
- Stock transfer between warehouses.
- Reservation workflow for mechanic orders.
- Period close / accounting lock dates.

These are planned for later vertical slices; they should not be approximated with destructive updates.
