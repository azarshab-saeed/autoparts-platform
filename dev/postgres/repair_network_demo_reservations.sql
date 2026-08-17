-- Phase 11.1.4 dev-data repair.
-- Historical Phase 7 seed data overwrote inventory_balances.reserved on every
-- db-prepare run. Rebuild the reserved quantity for only the two cross-store
-- demo warehouses from reservations that still own an active hold.
--
-- This intentionally does NOT change on_hand. It only restores the invariant:
-- inventory_balances.reserved == sum(active network reservation holds)
-- for the four seeded cross-store demo SKUs.

UPDATE inventory_balances ib
SET reserved = COALESCE((
      SELECT SUM(r.qty)
      FROM network_reservations r
      WHERE r.tenant_id = ib.tenant_id
        AND r.warehouse_id = ib.warehouse_id
        AND r.product_id = ib.product_id
        AND r.status IN ('pending','accepted','ready')
    ), 0),
    updated_at = now()
WHERE (ib.tenant_id, ib.warehouse_id, ib.product_id) IN (
  ('11111111-1111-1111-1111-111111111112'::uuid,'33333333-3333-3333-3333-333333333334'::uuid,'55555555-5555-5555-5555-555555555561'::uuid),
  ('11111111-1111-1111-1111-111111111112'::uuid,'33333333-3333-3333-3333-333333333334'::uuid,'55555555-5555-5555-5555-555555555562'::uuid),
  ('11111111-1111-1111-1111-111111111113'::uuid,'33333333-3333-3333-3333-333333333335'::uuid,'55555555-5555-5555-5555-555555555571'::uuid),
  ('11111111-1111-1111-1111-111111111113'::uuid,'33333333-3333-3333-3333-333333333335'::uuid,'55555555-5555-5555-5555-555555555572'::uuid)
);
