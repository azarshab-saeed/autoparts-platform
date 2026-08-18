BEGIN;

CREATE TABLE price_lists (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  code text NOT NULL,
  name text NOT NULL,
  is_default boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, store_id, code)
);
CREATE INDEX idx_price_lists_store ON price_lists(tenant_id,store_id,active,name);
CREATE UNIQUE INDEX ux_price_lists_default ON price_lists(tenant_id,store_id) WHERE is_default AND active;

CREATE TABLE store_pricing_settings (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  min_margin_bps integer NOT NULL DEFAULT 1000 CHECK (min_margin_bps BETWEEN 0 AND 9000),
  cashier_may_override boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, store_id)
);

CREATE TABLE product_price_breaks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
  min_qty numeric(18,3) NOT NULL CHECK (min_qty > 0),
  unit_price bigint NOT NULL CHECK (unit_price >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, store_id, product_id, price_list_id, min_qty)
);
CREATE INDEX idx_product_price_breaks_lookup
  ON product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty DESC);

ALTER TABLE customers ADD COLUMN IF NOT EXISTS price_list_id uuid REFERENCES price_lists(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_customers_price_list ON customers(tenant_id,store_id,price_list_id) WHERE price_list_id IS NOT NULL;

ALTER TABLE sales
  ADD COLUMN IF NOT EXISTS gross_amount bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discount_amount bigint NOT NULL DEFAULT 0;
UPDATE sales SET gross_amount=total_amount WHERE gross_amount=0 AND total_amount<>0;

ALTER TABLE sale_items
  ADD COLUMN IF NOT EXISTS price_list_id uuid REFERENCES price_lists(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS list_unit_price bigint,
  ADD COLUMN IF NOT EXISTS price_source text,
  ADD COLUMN IF NOT EXISTS price_override boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS override_reason text,
  ADD COLUMN IF NOT EXISTS override_actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS margin_bps integer,
  ADD COLUMN IF NOT EXISTS margin_guard_bps integer,
  ADD COLUMN IF NOT EXISTS below_margin_guard boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS gross_line_total bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discount_amount bigint NOT NULL DEFAULT 0;
UPDATE sale_items SET gross_line_total=line_total WHERE gross_line_total=0 AND line_total<>0;

-- Existing stores get a canonical retail list. New stores are provisioned by the app.
INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active)
SELECT s.tenant_id,s.id,'retail','خرده / مصرف‌کننده',true,true
FROM stores s
WHERE NOT EXISTS (
  SELECT 1 FROM price_lists pl WHERE pl.tenant_id=s.tenant_id AND pl.store_id=s.id AND pl.is_default AND pl.active
);

INSERT INTO store_pricing_settings(tenant_id,store_id)
SELECT s.tenant_id,s.id FROM stores s
ON CONFLICT (tenant_id,store_id) DO NOTHING;

-- Bootstrap local retail prices from the existing offer price without changing network pricing.
INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price)
SELECT DISTINCT ON (o.tenant_id,o.store_id,o.product_id)
       o.tenant_id,o.store_id,o.product_id,pl.id,1,o.selling_price
FROM store_product_offers o
JOIN price_lists pl ON pl.tenant_id=o.tenant_id AND pl.store_id=o.store_id AND pl.is_default AND pl.active
ORDER BY o.tenant_id,o.store_id,o.product_id,o.updated_at DESC,o.id DESC
ON CONFLICT (tenant_id,store_id,product_id,price_list_id,min_qty)
DO NOTHING;

COMMIT;
