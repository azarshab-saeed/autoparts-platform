BEGIN;

CREATE TABLE vehicle_notebooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  public_token uuid NOT NULL DEFAULT gen_random_uuid(),
  owner_customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  owner_name text,
  owner_phone text,
  plate text,
  vin text,
  make text,
  model text,
  trim text,
  model_year integer,
  owner_code_hash text NOT NULL,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE(public_token),
  CHECK (model_year IS NULL OR model_year BETWEEN 1200 AND 2200)
);

CREATE UNIQUE INDEX ux_vehicle_notebooks_store_plate
  ON vehicle_notebooks(tenant_id, store_id, lower(plate))
  WHERE deleted_at IS NULL AND plate IS NOT NULL AND btrim(plate) <> '';
CREATE INDEX idx_vehicle_notebooks_search
  ON vehicle_notebooks(tenant_id, store_id, updated_at DESC);
CREATE INDEX idx_vehicle_notebooks_owner_customer
  ON vehicle_notebooks(owner_customer_id)
  WHERE owner_customer_id IS NOT NULL;

CREATE TABLE vehicle_notebook_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  vehicle_id uuid NOT NULL REFERENCES vehicle_notebooks(id) ON DELETE CASCADE,
  tenant_id uuid REFERENCES tenants(id) ON DELETE SET NULL,
  store_id uuid REFERENCES stores(id) ON DELETE SET NULL,
  actor_user_id uuid,
  actor_role text NOT NULL,
  actor_name text NOT NULL DEFAULT '',
  kind text NOT NULL CHECK (kind IN ('service','part','mileage','note')),
  title text NOT NULL,
  mileage integer,
  occurred_on date NOT NULL DEFAULT CURRENT_DATE,
  next_due_mileage integer,
  next_due_date date,
  notes text,
  owner_reported boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (mileage IS NULL OR mileage >= 0),
  CHECK (next_due_mileage IS NULL OR next_due_mileage >= 0)
);

CREATE INDEX idx_vehicle_notebook_entries_vehicle
  ON vehicle_notebook_entries(vehicle_id, occurred_on DESC, created_at DESC);
CREATE INDEX idx_vehicle_notebook_entries_store
  ON vehicle_notebook_entries(tenant_id, store_id, created_at DESC)
  WHERE store_id IS NOT NULL;

COMMIT;
