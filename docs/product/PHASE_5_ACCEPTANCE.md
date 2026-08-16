# Phase 5 Acceptance — Inventory + Purchase UI

Phase 5 is accepted when all items below pass.

## Navigation
- [ ] `/store/inventory` opens from the store sidebar.
- [ ] `/store/purchases` opens from the store sidebar.
- [ ] Dashboard quick actions link to both screens.

## Inventory
- [ ] Inventory loads for the authenticated store default warehouse.
- [ ] Low-stock filter only shows products at or below their reorder point.
- [ ] Search filters visible inventory by product title or SKU.
- [ ] Inventory statistics show SKU count, low-stock count, available quantity, and approximate inventory value.
- [ ] Owner/Admin/Warehouse roles can post a physical inventory adjustment.
- [ ] Negative adjustments cannot make on-hand lower than reserved quantity.
- [ ] Reorder minimum and target quantities can be saved.
- [ ] Target quantity cannot be lower than minimum quantity.

## Purchase
- [ ] Products can be searched and added to a purchase document.
- [ ] Existing weighted-average cost is suggested as the unit cost.
- [ ] Supplier can be searched and selected.
- [ ] Payment method supports cash, card, and credit.
- [ ] Purchase cannot be submitted without supplier and at least one line.
- [ ] Successful purchase posts through `POST /v1/purchases`.
- [ ] After successful purchase, inventory is refreshed.
- [ ] Purchase uses `Idempotency-Key`.

## Demo / Mock
- [ ] Mock mode can execute purchase and inventory adjustment without API.
- [ ] Demo PostgreSQL seed contains at least two suppliers.
- [ ] Demo PostgreSQL seed contains reorder points so low-stock UI is testable.

## Security
- [ ] Access token is still supplied by Keycloak integration.
- [ ] Tenant/store IDs are not accepted from browser input.
- [ ] Inventory mutation actions remain role-protected by the Go API.
