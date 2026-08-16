# Phase 7 acceptance

- [ ] Existing Phase 6 DB upgrades without deleting the volume.
- [ ] `schema_migrations` contains `005_network_search.sql`.
- [ ] `/mechanic` loads without authentication.
- [ ] `لنت 206` matches products stored with Persian digits (`۲۰۶`).
- [ ] Search returns multiple stores from separate tenants.
- [ ] Price sort puts the cheapest offer first.
- [ ] Distance sort works when lat/lng are provided.
- [ ] A store with `network_enabled=false` disappears from public results.
- [ ] An offer with `visible=false` disappears from public results.
- [ ] A zero-available inventory row never appears publicly.
- [ ] Purchase cost and reserved quantity are absent from search responses.
- [ ] Store owner can edit price/visibility at `/store/network`.
- [ ] Store changes are visible on the mechanic search after refresh.
- [ ] Existing store roles still require tenant/store claims.
- [ ] Mechanic/consumer tokens can be valid without tenant/store claims.
