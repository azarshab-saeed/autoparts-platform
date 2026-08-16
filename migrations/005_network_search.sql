BEGIN;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE stores
  ADD COLUMN IF NOT EXISTS network_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS public_address text,
  ADD COLUMN IF NOT EXISTS public_phone text,
  ADD COLUMN IF NOT EXISTS city text,
  ADD COLUMN IF NOT EXISTS latitude numeric(9,6),
  ADD COLUMN IF NOT EXISTS longitude numeric(9,6),
  ADD COLUMN IF NOT EXISTS network_updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE stores
  DROP CONSTRAINT IF EXISTS stores_latitude_check,
  DROP CONSTRAINT IF EXISTS stores_longitude_check;
ALTER TABLE stores
  ADD CONSTRAINT stores_latitude_check CHECK (latitude IS NULL OR (latitude BETWEEN -90 AND 90)),
  ADD CONSTRAINT stores_longitude_check CHECK (longitude IS NULL OR (longitude BETWEEN -180 AND 180));

CREATE TABLE IF NOT EXISTS store_product_offers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  warehouse_id uuid NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  selling_price bigint NOT NULL CHECK (selling_price > 0),
  visible boolean NOT NULL DEFAULT true,
  allow_reservation boolean NOT NULL DEFAULT false,
  last_verified_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, store_id, warehouse_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_network_offers_visible
  ON store_product_offers(product_id, store_id)
  WHERE visible;
CREATE INDEX IF NOT EXISTS idx_network_store_visible
  ON store_product_offers(store_id, updated_at DESC)
  WHERE visible;
CREATE INDEX IF NOT EXISTS idx_products_network_title_trgm
  ON products USING gin (normalized_title gin_trgm_ops)
  WHERE active AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_network_oem_trgm
  ON products USING gin (lower(coalesce(oem_code,'')) gin_trgm_ops)
  WHERE active AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_network_brand_trgm
  ON products USING gin (lower(coalesce(brand,'')) gin_trgm_ops)
  WHERE active AND deleted_at IS NULL;

COMMIT;
