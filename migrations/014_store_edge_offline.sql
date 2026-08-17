BEGIN;

CREATE TABLE IF NOT EXISTS store_edge_pairings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  requested_by_user_id uuid NOT NULL,
  code_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_store_edge_pairings_expiry
  ON store_edge_pairings(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS store_edge_devices (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  name text NOT NULL,
  secret_hash text NOT NULL,
  paired_by_user_id uuid NOT NULL,
  active boolean NOT NULL DEFAULT true,
  last_seen_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_store_edge_devices_store
  ON store_edge_devices(tenant_id, store_id, active, created_at DESC);

CREATE TABLE IF NOT EXISTS store_edge_sync_events (
  id bigserial PRIMARY KEY,
  device_id uuid NOT NULL REFERENCES store_edge_devices(id),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  local_operation_id text NOT NULL,
  operation_type text NOT NULL CHECK (operation_type IN ('sale')),
  server_reference_id uuid,
  status text NOT NULL CHECK (status IN ('synced','conflict')),
  detail text,
  received_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(device_id, local_operation_id)
);

ALTER TABLE sales ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'web';
ALTER TABLE sales ADD COLUMN IF NOT EXISTS edge_device_id uuid REFERENCES store_edge_devices(id);
ALTER TABLE sales ADD COLUMN IF NOT EXISTS edge_local_operation_id text;
ALTER TABLE sales ADD COLUMN IF NOT EXISTS edge_occurred_at timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_edge_local_operation
  ON sales(edge_device_id, edge_local_operation_id)
  WHERE edge_device_id IS NOT NULL AND edge_local_operation_id IS NOT NULL;

COMMIT;
