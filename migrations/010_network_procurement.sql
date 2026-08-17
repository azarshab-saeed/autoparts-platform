BEGIN;

ALTER TABLE store_product_offers
  ADD COLUMN IF NOT EXISTS allow_procurement boolean NOT NULL DEFAULT true;

ALTER TABLE customers
  ADD COLUMN IF NOT EXISTS network_store_id uuid REFERENCES stores(id);
ALTER TABLE suppliers
  ADD COLUMN IF NOT EXISTS network_store_id uuid REFERENCES stores(id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_customers_network_store
  ON customers(tenant_id, store_id, network_store_id)
  WHERE network_store_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_suppliers_network_store
  ON suppliers(tenant_id, store_id, network_store_id)
  WHERE network_store_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS network_procurements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_tenant_id uuid NOT NULL REFERENCES tenants(id),
  buyer_store_id uuid NOT NULL REFERENCES stores(id),
  buyer_warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  buyer_product_id uuid NOT NULL REFERENCES products(id),
  seller_tenant_id uuid NOT NULL REFERENCES tenants(id),
  seller_store_id uuid NOT NULL REFERENCES stores(id),
  seller_warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  seller_product_id uuid NOT NULL REFERENCES products(id),
  offer_id uuid NOT NULL REFERENCES store_product_offers(id),
  requested_by_user_id uuid NOT NULL,
  qty numeric(18,3) NOT NULL CHECK (qty > 0),
  unit_price bigint NOT NULL CHECK (unit_price > 0),
  total_amount bigint NOT NULL CHECK (total_amount > 0),
  status text NOT NULL CHECK (status IN ('requested','accepted','ready','received','rejected','cancelled','expired')),
  idempotency_key text NOT NULL,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  ready_at timestamptz,
  received_at timestamptz,
  closed_at timestamptz,
  seller_sale_id uuid REFERENCES sales(id),
  buyer_purchase_id uuid REFERENCES purchases(id),
  received_by_user_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (buyer_tenant_id, idempotency_key),
  CHECK (buyer_store_id <> seller_store_id),
  CHECK ((status='received') = (seller_sale_id IS NOT NULL AND buyer_purchase_id IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS network_procurement_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  procurement_id uuid NOT NULL REFERENCES network_procurements(id) ON DELETE CASCADE,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  actor_user_id uuid,
  actor_side text NOT NULL CHECK (actor_side IN ('buyer','seller','system')),
  event_type text NOT NULL CHECK (event_type IN ('requested','accepted','ready','received','rejected','cancelled','expired')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_procurement_buyer_created
  ON network_procurements(buyer_tenant_id,buyer_store_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_procurement_seller_created
  ON network_procurements(seller_tenant_id,seller_store_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_procurement_active_expiry
  ON network_procurements(expires_at)
  WHERE status IN ('requested','accepted','ready');
CREATE INDEX IF NOT EXISTS idx_procurement_events_order
  ON network_procurement_events(procurement_id,created_at);

COMMIT;
