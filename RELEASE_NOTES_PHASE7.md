# Phase 7 — Network Search + Mechanic UI

Phase 7 implements the first cross-store capability that differentiates this product from a conventional inventory/accounting application.

## Delivered

- Public `GET /v1/network/search` endpoint.
- Cross-tenant search over **only explicitly published** store offers.
- Search by part title, SKU, brand and OEM code.
- Persian/Arabic digit normalization for searches such as `206` vs `۲۰۶`.
- Sorting by best match, price, distance or inventory freshness.
- Optional latitude/longitude and Haversine distance calculation.
- Freshness labels based on live inventory / offer verification timestamps.
- Store-controlled public network profile: address, phone, city and location.
- Store-controlled product offer publishing and selling price.
- `/store/network` admin UI for publishing inventory.
- `/mechanic` mobile-first public search UI.
- Mechanic UI supports current-location distance, phone calls and map navigation.
- `mechanic` / `consumer` Keycloak identities no longer require tenant/store claims.
- Added a mechanic user to the realm import for fresh installations.
- Migration `005_network_search.sql` with network fields, offers and trigram indexes.
- Multi-store demo seed across three different tenants.
- OpenAPI bumped to 0.7.0.

## Privacy boundary

Network search never exposes purchase cost, ledger/accounting data, reserved quantity, supplier data or other internal store state. A result appears only when:

1. the store is `network_enabled`,
2. the product offer is `visible`,
3. the product is active, and
4. available stock (`on_hand - reserved`) is greater than zero.

## Deliberately deferred

- Reservations / order workflow between mechanic and store.
- Canonical global part catalog and compatibility graph.
- Vehicle fitment filters.
- Search ranking with click/order conversion signals.
- Elasticsearch/OpenSearch/Meilisearch. PostgreSQL + pg_trgm is sufficient for the current scale.
