# Phase 8 runbook — Reservation + Mechanic Orders

## Upgrade the existing Phase 7 environment

Do **not** delete the PostgreSQL volume.

```bash
cp .env.example .env
docker compose up --build
```

`db-prepare` applies `006_network_reservations.sql` on the existing `autoparts` database.

Verify:

```bash
docker compose exec postgres psql -U autoparts -d autoparts -c "TABLE schema_migrations;"
```

Expected newest migration:

```text
006_network_reservations.sql
```

## Test the mechanic reservation flow

Open:

```text
http://localhost:3000/mechanic
```

Search:

```text
لنت 206
```

Use an offer tagged `قابل رزرو` and click `رزرو قطعه`.

Reservation requires a mechanic/consumer identity. Fresh Keycloak installations include:

```text
mechanic@example.com
ChangeMe123!
```

Choose quantity `1` and submit.

Expected behavior:

1. API creates a `pending` reservation.
2. `inventory_balances.reserved` increases in the seller warehouse.
3. public search immediately shows one fewer available item.
4. the reservation appears at `/mechanic/orders`.
5. default pending reservation expiry is 30 minutes.

## Test the store workflow

Log in as:

```text
owner@example.com
ChangeMe123!
```

Open:

```text
http://localhost:3000/store/orders
```

Expected sequence:

```text
pending -> accepted -> ready
```

After `ready`, the pickup window becomes 2 hours. If the mechanic does not collect the item before expiry, the reservation is marked `expired` and reserved inventory is released lazily on the next search/reservation-list operation.

The store can reject a `pending` or `accepted` reservation. Rejection releases reserved inventory immediately.

## Test buyer cancellation

Open:

```text
http://localhost:3000/mechanic/orders
```

A mechanic/consumer can cancel a `pending`, `accepted`, or `ready` reservation. Cancellation releases the stock immediately.

## Verify inventory reservation directly

Before and after a reservation:

```bash
docker compose exec postgres psql -U autoparts -d autoparts -c \
"SELECT p.title,b.on_hand,b.reserved,(b.on_hand-b.reserved) AS available
 FROM inventory_balances b
 JOIN products p ON p.id=b.product_id AND p.tenant_id=b.tenant_id
 WHERE p.sku='BRK-206-TXT';"
```

## Direct API flow

Obtain a mechanic access token through the browser/Keycloak flow, then create a reservation:

```http
POST /v1/network/reservations
Authorization: Bearer <mechanic access token>
Idempotency-Key: <uuid>
Content-Type: application/json

{
  "offer_id": "<offer uuid>",
  "qty": 1
}
```

Store transitions:

```http
PATCH /v1/network/reservations/<reservation uuid>
Authorization: Bearer <store access token>
Content-Type: application/json

{"status":"accepted"}
```

then:

```json
{"status":"ready"}
```

## Intentionally not implemented in Phase 8

`ready` does **not** post a sale or accounting journal. Phase 8 only owns inventory reservation and request tracking. The next commerce slice should convert/attach a ready reservation to an actual store sale so inventory, payment, COGS and accounting remain one atomic source of truth.
