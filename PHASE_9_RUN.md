# Phase 9 - Reservation Fulfillment + Sale Conversion

Phase 9 turns a `ready` network reservation into a real posted sale.

## Upgrade

Do not remove the PostgreSQL volume.

```bash
docker compose up --build
```

Migration `007_reservation_fulfillment.sql` is picked up automatically by the existing migration runner.

Verify:

```bash
docker compose exec postgres psql -U autoparts -d autoparts -c "TABLE schema_migrations;"
```

You should see `007_reservation_fulfillment.sql`.

## Happy-path test

1. Login as `mechanic@example.com`.
2. Search for a reservable offer and create a reservation.
3. Login as `owner@example.com`.
4. Open `/store/orders`.
5. Move the reservation from `pending` to `accepted`.
6. Move it from `accepted` to `ready`.
7. Click either `تحویل + فروش نقدی` or `تحویل + کارتخوان`.
8. The row should become `تحویل و فروخته شد`.
9. Login as the mechanic and open `/mechanic/orders`; the same reservation should show `تحویل و خرید انجام شد`.

## Database checks

Replace `<reservation-id>` below.

```sql
SELECT id,status,sale_id,fulfilled_at,fulfilled_by_user_id
FROM network_reservations
WHERE id='<reservation-id>';

SELECT id,total_amount,paid_amount,due_amount,status
FROM sales
WHERE id=(SELECT sale_id FROM network_reservations WHERE id='<reservation-id>');

SELECT on_hand,reserved,avg_unit_cost
FROM inventory_balances
WHERE product_id=(SELECT product_id FROM network_reservations WHERE id='<reservation-id>')
  AND warehouse_id=(SELECT warehouse_id FROM network_reservations WHERE id='<reservation-id>');
```

Expected behavior:

- `network_reservations.status = fulfilled`
- exactly one `sale_id` is linked to the reservation
- `reserved` decreases by the reservation quantity
- `on_hand` decreases by the same quantity
- a `sale` inventory movement exists
- sales revenue and COGS journal entries exist
- cash/card payment exists for immediate payment

## Double-fulfillment test

Repeat the fulfillment request for the same reservation. The API must return the already-linked sale and must not reduce stock a second time.

## Expiry test

Let a `ready` reservation expire and then attempt fulfillment. The operation must be rejected, the reservation becomes `expired`, and held stock is released.

## Credit/partial API test

The fulfillment endpoint supports `payment_method=credit` or `payments` for partial/mixed immediate payments. If `due_amount > 0`, `customer_id` is mandatory so Accounts Receivable and the customer party ledger remain balanced.

Endpoint: `POST /v1/network/reservations/{id}/fulfill`

Required header: `Idempotency-Key`.
