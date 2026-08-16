# Phase 5 — Inventory + Purchase UI

## Scope
Phase 5 turns the Phase 3 inventory/purchase backend into usable store workflows.

## Added
- Store inventory screen at `/store/inventory`.
- Inventory overview cards for SKU count, low stock, available units, and approximate inventory value.
- Low-stock-only filter and local title/SKU search.
- Physical inventory adjustment dialog.
- Reorder minimum/target editor.
- Purchase entry screen at `/store/purchases`.
- Product search and multi-line purchase editor.
- Supplier search and selection.
- Cash, card, and credit purchase methods.
- Weighted-average cost shown as the suggested purchase unit cost.
- Purchase submission to the existing Go `POST /v1/purchases` endpoint.
- Inventory refresh after a successful purchase.
- Mock inventory that mutates in-memory for purchase, adjustment, and reorder demos.
- Dashboard and store navigation links for Inventory and Purchase.
- Demo suppliers and reorder points in `dev/postgres/seed.sql`.

## Architecture
No new business rules were duplicated in the frontend. The UI only collects user intent. Inventory invariants, weighted-average valuation, tenant/store isolation, idempotency, and journal posting remain enforced by the Go API and PostgreSQL transaction.

## Demo flow
1. Sign in through Keycloak.
2. Open **خرید**.
3. Select a supplier.
4. Add one or more products and enter quantity/unit cost.
5. Submit the purchase.
6. Open **انبار و کالاها** and verify on-hand quantity and average cost.
7. Open **اصلاح موجودی** for a product and post a physical correction.
8. Edit **حد سفارش** and verify the low-stock state.

## Current limitations
- Purchase history/list endpoint is not implemented yet, so Phase 5 focuses on purchase creation.
- Inventory page loads up to 100 rows; server-side search/pagination UX comes later.
- Supplier creation remains backend-capable but does not yet have a dedicated frontend screen.
- Dashboard KPIs are still demo data; live reporting is planned for a later phase.

## Version
Frontend package version: `0.5.0`.
