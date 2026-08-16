# Phase 8 acceptance checklist

## Migration
- [ ] Existing Phase 7 volume upgrades without deletion.
- [ ] `schema_migrations` contains `006_network_reservations.sql`.

## Reservation creation
- [ ] Public search still works without login.
- [ ] Non-reservable offer cannot be reserved.
- [ ] mechanic/consumer login can reserve a visible reservable offer.
- [ ] store owner/admin account cannot create a mechanic reservation.
- [ ] duplicate request with same buyer + Idempotency-Key returns the existing reservation.
- [ ] concurrent reservation cannot reserve more than available stock.
- [ ] `reserved` increases and public `available` decreases immediately.

## Store workflow
- [ ] store can only see reservations belonging to its tenant/store.
- [ ] pending -> accepted is allowed.
- [ ] accepted -> ready is allowed.
- [ ] pending/accepted -> rejected releases inventory.
- [ ] invalid transitions are rejected.
- [ ] ready receives a new two-hour pickup deadline.

## Buyer workflow
- [ ] buyer sees only reservations owned by its Keycloak subject UUID.
- [ ] pending/accepted/ready can be cancelled by the owner.
- [ ] cancellation releases inventory.
- [ ] another mechanic cannot cancel someone else's reservation.

## Expiry
- [ ] due pending/accepted/ready reservations become expired.
- [ ] expiry releases reserved quantity exactly once.
- [ ] network search does not continue hiding stock after expiry processing.

## UX
- [ ] mechanic search shows a reservation CTA only when `allow_reservation=true`.
- [ ] reservation modal cannot request more than currently displayed availability.
- [ ] mechanic order tracker shows Persian status labels.
- [ ] store inbox supports filter + accept/reject/ready actions.
