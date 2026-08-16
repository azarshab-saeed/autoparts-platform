# Phase 8 — Reservation + Mechanic Orders

Phase 8 turns network search into a two-sided transaction workflow without prematurely mixing reservation state with accounting.

## Backend

- Added `network_reservations` with seller tenant/store/warehouse/offer/product scope.
- External buyer identity is the Keycloak subject UUID; mechanic/consumer users still do not pretend to belong to a store tenant.
- `POST /v1/network/reservations` for mechanic/consumer users.
- `GET /v1/me/reservations` and buyer cancellation endpoint.
- Store inbox endpoint and controlled store-side state transitions.
- Serializable transaction + row lock on inventory when creating a reservation.
- Creating a reservation increments `inventory_balances.reserved` atomically.
- Reject/cancel/expiry release reserved inventory atomically.
- Public network search already uses `on_hand - reserved`, so availability changes immediately.
- Idempotency key protects reservation creation from duplicate browser retries.
- Reservation event audit table and outbox events for created/status transitions.
- Lazy expiry sweep is invoked by reservation list/create and public network search.

## Lifecycle

```text
pending -> accepted -> ready
   |          |
   +-> rejected
   +-> cancelled (buyer)

pending / accepted expire after the reservation deadline.
ready gets a fresh two-hour pickup deadline and can also expire/cancel.
```

Initial reservation TTL: 30 minutes.
Ready-for-pickup TTL: 2 hours.

## Frontend

- Search cards now have `رزرو قطعه` for reservable offers.
- Quantity + total confirmation modal.
- `/mechanic/orders` for buyer tracking and cancellation.
- `/store/orders` for incoming network reservations and store actions.
- Store navigation includes `سفارش‌های شبکه`.
- Mock mode has a mechanic identity and reservation state so UI can be demonstrated without the Go API.
- Login routing now sends mechanic/consumer users to the mechanic experience instead of briefly routing through the store panel.

## Migration

- `006_network_reservations.sql`
- existing Phase 7 PostgreSQL volume is upgraded by the existing versioned migration runner.

## Important boundary

A reservation is **not a sale**. Marking a reservation ready does not change `on_hand`, create a sale, recognize revenue, post COGS, or take payment. This is intentional. The fulfillment step must later bind the reservation to the existing sale transaction so financial and inventory ledgers remain consistent.
