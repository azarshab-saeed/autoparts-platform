# Phase 13 — Advanced Search & Vehicle Fitment

## What changed

Phase 13 adds a shared vehicle catalog, product-to-vehicle fitments, alternate search terms/OEM equivalents, vehicle-filtered public network search, and relevance scoring.

## Run

```bash
docker compose up --build
```

Do not delete the PostgreSQL volume. `011_vehicle_fitment_search.sql` upgrades the current database and the dev seed adds repeat-safe demo aliases/fitments.

## Smoke test

1. Sign in as `owner@example.com` and open `/store/fitment`.
2. Select `لنت جلو پژو ۲۰۶ تیپ ۵`.
3. Verify the demo alias `لنت 206`, alternate OEM `4254.97`, and Peugeot 206 Type 5 / TU5 fitment are shown.
4. Add/remove a harmless alias and save; reload the page and confirm it persists.
5. Open `/mechanic`.
6. Search `4254.97`; brake-pad offers should be returned and tagged as an exact code/alias match.
7. Select Peugeot → 206 → Type 5 / TU5 and search `لنت` (or clear text and search by vehicle only). Only products with matching fitment should remain.
8. Set a model year inside the fitment range and confirm results remain; use an outside year and confirm incompatible results are removed.
9. Reserve a matching offer to confirm Phase 8/9 reservation behavior remains intact.
10. In a store product search, search an alias such as `لنت 206`; the tenant-owned product should be discoverable.

## New endpoints

- `GET /v1/vehicles/catalog`
- `GET /v1/products/{product_id}/search-metadata`
- `PUT /v1/products/{product_id}/search-metadata`
- `GET /v1/network/search` now accepts `vehicle_variant_id` and optional `year`.

## Search ranking

Default `sort=best` prioritizes exact SKU/OEM codes, exact aliases/equivalents, title matches, vehicle fitment, freshness, distance, then price. Explicit price/distance/fresh sorts remain available.
