BEGIN;

CREATE TABLE IF NOT EXISTS catalog_import_batches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  requested_by_user_id uuid NOT NULL,
  idempotency_key text NOT NULL,
  row_count integer NOT NULL CHECK (row_count >= 0),
  created_count integer NOT NULL DEFAULT 0 CHECK (created_count >= 0),
  updated_count integer NOT NULL DEFAULT 0 CHECK (updated_count >= 0),
  inventory_initialized_count integer NOT NULL DEFAULT 0 CHECK (inventory_initialized_count >= 0),
  inventory_preserved_count integer NOT NULL DEFAULT 0 CHECK (inventory_preserved_count >= 0),
  offers_upserted_count integer NOT NULL DEFAULT 0 CHECK (offers_upserted_count >= 0),
  opening_inventory_value bigint NOT NULL DEFAULT 0 CHECK (opening_inventory_value >= 0),
  status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS catalog_import_row_results (
  batch_id uuid NOT NULL REFERENCES catalog_import_batches(id) ON DELETE CASCADE,
  row_number integer NOT NULL CHECK (row_number > 0),
  product_id uuid NOT NULL REFERENCES products(id),
  product_action text NOT NULL CHECK (product_action IN ('created','updated')),
  inventory_action text NOT NULL CHECK (inventory_action IN ('initialized','preserved','none')),
  offer_action text NOT NULL CHECK (offer_action IN ('upserted','none')),
  note text,
  PRIMARY KEY (batch_id, row_number)
);

CREATE INDEX IF NOT EXISTS idx_catalog_import_batches_store_time
  ON catalog_import_batches(tenant_id, store_id, created_at DESC);

COMMIT;
