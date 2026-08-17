# Release Notes — Phase 11

## Store operations
- Added paginated/filterable `GET /v1/sales` and `GET /v1/purchases` history endpoints.
- Added sale/purchase history screens, detail links and printable documents.
- Added client-side UTF-8/BOM CSV export for spreadsheet-friendly reports.

## Management dashboard
- Replaced the static store dashboard with live backend metrics: net sales, gross profit, purchases, AR/AP, inventory value, low-stock count, open reservations, recent sales and a seven-day sales series.

## Inventory analytics
- Added warehouse-scoped inventory valuation and health analytics.
- Exposes available stock, average cost, inventory value, 30-day sold quantity, last-sale age, low-stock and 90-day dead-stock flags.

## Daily cash control
- Added cash/card movement reporting across sales, settlements, purchases, expenses and returns.
- Added `daily_closings` snapshots with opening cash, expected cash, counted cash and variance.
- Close operations are idempotent and unique per tenant/store/business date.
- Late activity is detected with `changed_after_close` rather than silently changing the historical snapshot.

## Security / tenancy
- All new history/reporting queries are tenant/store scoped.
- Warehouse analytics verifies the requested warehouse belongs to the authenticated store.
- New routes use coarse role guards in Go in addition to tenant/store query predicates.
