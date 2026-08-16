# Phase 5 Quick Test

Start the existing stack:

```bash
cp .env.example .env
docker compose up --build
```

Open:
- Store app: `http://localhost:3000`
- Keycloak: `http://localhost:8081`

Development store user:
- Email: `owner@example.com`
- Password: `ChangeMe123!`

## Purchase -> inventory test
1. Sign in.
2. Open **خرید**.
3. Search and select **پخش قطعات ایران**.
4. Search a product, add it, enter quantity and unit cost.
5. Submit with cash/card/credit.
6. Open **انبار و کالاها**.
7. Verify quantity increased and average unit cost changed according to weighted average.

## Low stock test
The demo seed configures reorder points. `واترپمپ EF7` starts below its configured minimum and should appear as low stock.

## Inventory adjustment test
1. Open **انبار و کالاها**.
2. Click **اصلاح** on a row.
3. Enter `-1` and a reason.
4. Submit.
5. Verify on-hand/available quantity decreases.

## Reorder point test
1. Click **حد سفارش**.
2. Set minimum and target.
3. Save.
4. Toggle **فقط رو به اتمام** to verify the state.

## Existing volume note
`db-prepare` runs the demo seed idempotently on startup, so the two demo suppliers and reorder points are added even when the development PostgreSQL volume already exists.
