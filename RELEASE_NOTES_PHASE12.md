# Release Notes — Phase 12

## Network procurement

- Added authenticated procurement search that excludes the current store and respects per-offer procurement opt-in.
- Added inventory-backed store-to-store procurement requests with idempotency and seller-side holds.
- Added buyer and seller order lists and transitions: requested, accepted, ready, received, rejected, cancelled, expired.
- Added 12h/24h/48h lifecycle TTLs with lazy expiry and safe hold release.

## Atomic receive

Receiving a ready procurement now performs the entire inter-store movement in one SERIALIZABLE PostgreSQL transaction:

- consume seller held inventory;
- post seller credit sale and perpetual COGS;
- create/link seller customer for the buyer store;
- post seller AR and party ledger;
- post buyer credit purchase;
- create/link buyer supplier for the seller store;
- update buyer inventory using perpetual weighted average;
- post buyer AP and party ledger;
- link seller sale + buyer purchase to the procurement;
- emit outbox events for both tenants.

## UI

- Added `/store/procurement` for local-product selection, network offer comparison, procurement request creation, buyer tracking, seller fulfillment, and buyer receiving.
- Added **تأمین از شبکه** to the store navigation.
- Added independent **تأمین همکار** toggle to network offers.

## Data integrity

- Migration: `010_network_procurement.sql`.
- Demo reserved-stock repair now includes active procurement holds as well as mechanic reservations.
- No database volume reset is required.
