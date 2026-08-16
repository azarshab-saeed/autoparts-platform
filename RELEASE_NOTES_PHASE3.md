# Release Notes — Phase 3

Phase 3 adds the first complete purchasing + perpetual inventory-costing vertical slice.

## Database
Migration `003_inventory_purchase.sql` adds purchases, purchase items, weighted-average unit cost, costed inventory movements, reorder points, and inventory adjustments.

## API
- `POST /v1/purchases`
- `GET /v1/inventory`
- `PUT /v1/inventory/reorder-point`
- `POST /v1/inventory/adjustments`
- `POST /v1/sales` now records COGS and Inventory accounting entries.

## Safety invariants
- Tenant/store ownership is checked server-side from authenticated claims.
- Purchase and adjustment writes are idempotent.
- Purchase, stock mutation, valuation, accounting and outbox writes share one serializable transaction.
- A physical adjustment cannot reduce on-hand below reserved quantity.
- Existing sales cannot consume more available inventory than `on_hand - reserved`.

## Verification commands
```
gofmt -w ./cmd ./internal
GOTOOLCHAIN=local go test ./...
docker compose up --build
```

Full dependency-backed tests require access to the Go module proxy or an already populated module cache.
