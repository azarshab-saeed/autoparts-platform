BEGIN;

ALTER TABLE inventory_balances
  ADD COLUMN IF NOT EXISTS avg_unit_cost bigint NOT NULL DEFAULT 0 CHECK (avg_unit_cost >= 0);

ALTER TABLE inventory_movements
  ADD COLUMN IF NOT EXISTS unit_cost bigint NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
  ADD COLUMN IF NOT EXISTS cost_delta bigint NOT NULL DEFAULT 0;

CREATE TABLE purchases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  supplier_id uuid NOT NULL REFERENCES suppliers(id),
  status text NOT NULL CHECK (status IN ('posted','voided')),
  payment_method text NOT NULL CHECK (payment_method IN ('cash','card','credit')),
  total_amount bigint NOT NULL CHECK (total_amount >= 0),
  currency char(3) NOT NULL DEFAULT 'IRR',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);

CREATE TABLE purchase_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  purchase_id uuid NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id),
  qty numeric(18,3) NOT NULL CHECK (qty > 0),
  unit_cost bigint NOT NULL CHECK (unit_cost >= 0),
  line_total bigint NOT NULL CHECK (line_total >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE payments ADD COLUMN IF NOT EXISTS purchase_id uuid REFERENCES purchases(id);
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check CHECK (method IN ('cash','card','credit','mixed'));

CREATE TABLE inventory_reorder_points (
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  product_id uuid NOT NULL REFERENCES products(id),
  min_qty numeric(18,3) NOT NULL DEFAULT 0 CHECK (min_qty >= 0),
  target_qty numeric(18,3) NOT NULL DEFAULT 0 CHECK (target_qty >= min_qty),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, warehouse_id, product_id)
);

CREATE TABLE inventory_adjustments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  product_id uuid NOT NULL REFERENCES products(id),
  qty_delta numeric(18,3) NOT NULL CHECK (qty_delta <> 0),
  reason text NOT NULL,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX idx_purchases_tenant_store_created ON purchases(tenant_id, store_id, created_at DESC);
CREATE INDEX idx_purchase_items_purchase ON purchase_items(tenant_id, purchase_id);
CREATE INDEX idx_inventory_balances_warehouse_stock ON inventory_balances(tenant_id, warehouse_id, on_hand);
CREATE INDEX idx_reorder_points_low_stock ON inventory_reorder_points(tenant_id, warehouse_id, min_qty);

COMMIT;
