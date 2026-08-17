-- Phase 12 dev-data repair.
-- Rebuild reserved inventory for the seeded cross-store demo SKUs from every
-- active hold source. This keeps db-prepare idempotent without destroying live
-- mechanic reservations or store-to-store procurement holds.

UPDATE inventory_balances ib
SET reserved = COALESCE((
      SELECT SUM(x.qty)
      FROM (
        SELECT r.qty
        FROM network_reservations r
        WHERE r.tenant_id = ib.tenant_id
          AND r.warehouse_id = ib.warehouse_id
          AND r.product_id = ib.product_id
          AND r.status IN ('pending','accepted','ready')
        UNION ALL
        SELECT p.qty
        FROM network_procurements p
        WHERE p.seller_tenant_id = ib.tenant_id
          AND p.seller_warehouse_id = ib.warehouse_id
          AND p.seller_product_id = ib.product_id
          AND p.status IN ('requested','accepted','ready')
      ) x
    ), 0),
    updated_at = now()
WHERE (ib.tenant_id, ib.warehouse_id, ib.product_id) IN (
  ('11111111-1111-1111-1111-111111111112'::uuid,'33333333-3333-3333-3333-333333333334'::uuid,'55555555-5555-5555-5555-555555555561'::uuid),
  ('11111111-1111-1111-1111-111111111112'::uuid,'33333333-3333-3333-3333-333333333334'::uuid,'55555555-5555-5555-5555-555555555562'::uuid),
  ('11111111-1111-1111-1111-111111111113'::uuid,'33333333-3333-3333-3333-333333333335'::uuid,'55555555-5555-5555-5555-555555555571'::uuid),
  ('11111111-1111-1111-1111-111111111113'::uuid,'33333333-3333-3333-3333-333333333335'::uuid,'55555555-5555-5555-5555-555555555572'::uuid)
);
