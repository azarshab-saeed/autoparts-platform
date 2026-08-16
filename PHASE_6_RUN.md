# Phase 6 run and acceptance flow

## Upgrade existing Phase 5 database
Do **not** delete the PostgreSQL volume.

```bash
cp .env.example .env
docker compose up --build
```

`db-prepare` will create/adopt `schema_migrations`, identify migrations 001–003 on an existing database, and apply `004_settlements_returns.sql` once.

To inspect migration state:

```bash
docker compose exec postgres psql -U autoparts -d autoparts -c "TABLE schema_migrations;"
```

## Login
- App: http://localhost:3000
- Keycloak: http://localhost:8081
- Demo store user: `owner@example.com` / `ChangeMe123!`

## Acceptance scenario A — partial sale + receipt
1. Open `/store/sales`.
2. Select a customer and add a product.
3. Choose **ترکیبی**.
4. Enter a card/cash amount lower than invoice total.
5. Submit; verify a non-zero remaining balance is shown.
6. Open `/store/accounts` → Customers.
7. Verify the customer's balance increased.
8. Click **دریافت پول**, settle part or all of the balance.
9. Verify balance decreases and cannot be overpaid.

## Acceptance scenario B — partial purchase + supplier payment
1. Open `/store/purchases`.
2. Select a supplier and add products.
3. Use a partial mixed payment or pure credit.
4. Submit.
5. Open `/store/accounts` → Suppliers and verify payable balance.
6. Record a supplier payment and verify the balance decreases.

## Acceptance scenario C — sale return
1. Create a sale and copy the returned `id` shown by the UI.
2. Open `/store/returns` → Sale return.
3. Paste the sale ID and load it.
4. Choose a return quantity less than or equal to returnable quantity.
5. Choose cash/card/account balance and submit.
6. Verify inventory rises and a repeated return cannot exceed remaining quantity.

## Acceptance scenario D — purchase return
1. Create a purchase and copy the returned `id`.
2. Open `/store/returns` → Purchase return.
3. Load the purchase and return part of a line.
4. Verify inventory decreases and supplier accounting is adjusted.

## Mock mode IDs
When `NEXT_PUBLIC_MOCK_MODE=true`:
- Sample sale: `99999999-9999-9999-9999-999999999991`
- Sample purchase: `99999999-9999-9999-9999-999999999992`
