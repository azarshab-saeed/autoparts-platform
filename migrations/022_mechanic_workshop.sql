BEGIN;

-- Phase 15.16.2: a vehicle notebook belongs to the vehicle, not to the store
-- that happened to create it. Store linkage stays optional for provenance.
ALTER TABLE vehicle_notebooks ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE vehicle_notebooks ALTER COLUMN store_id DROP NOT NULL;
ALTER TABLE vehicle_notebooks
  ADD COLUMN IF NOT EXISTS origin_role text NOT NULL DEFAULT 'store',
  ADD COLUMN IF NOT EXISTS origin_user_id uuid;

UPDATE vehicle_notebooks
SET origin_role='store', origin_user_id=COALESCE(origin_user_id, created_by)
WHERE origin_role='store';

ALTER TABLE vehicle_notebooks DROP CONSTRAINT IF EXISTS vehicle_notebooks_origin_role_check;
ALTER TABLE vehicle_notebooks ADD CONSTRAINT vehicle_notebooks_origin_role_check
  CHECK (origin_role IN ('store','mechanic'));
CREATE INDEX IF NOT EXISTS idx_vehicle_notebooks_origin_user
  ON vehicle_notebooks(origin_user_id, updated_at DESC)
  WHERE origin_user_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE vehicle_notebook_entries
  ADD COLUMN IF NOT EXISTS product_id uuid REFERENCES products(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_store_id uuid REFERENCES stores(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS workshop_job_id uuid,
  ADD COLUMN IF NOT EXISTS workshop_job_item_id uuid;

CREATE TABLE workshop_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mechanic_user_id uuid NOT NULL,
  vehicle_id uuid NOT NULL REFERENCES vehicle_notebooks(id) ON DELETE RESTRICT,
  customer_name text,
  customer_phone text,
  mileage integer CHECK (mileage IS NULL OR mileage >= 0),
  complaint text,
  diagnosis text,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','cancelled')),
  labor_amount bigint NOT NULL DEFAULT 0 CHECK (labor_amount >= 0),
  parts_amount bigint NOT NULL DEFAULT 0 CHECK (parts_amount >= 0),
  total_amount bigint NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
  paid_amount bigint NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
  due_amount bigint NOT NULL DEFAULT 0 CHECK (due_amount >= 0),
  opened_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (paid_amount <= total_amount),
  CHECK (due_amount = total_amount - paid_amount)
);
CREATE INDEX idx_workshop_jobs_mechanic ON workshop_jobs(mechanic_user_id, updated_at DESC);
CREATE INDEX idx_workshop_jobs_vehicle ON workshop_jobs(vehicle_id, opened_at DESC);

CREATE TABLE workshop_job_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id uuid NOT NULL REFERENCES workshop_jobs(id) ON DELETE CASCADE,
  item_type text NOT NULL CHECK (item_type IN ('service','part','labor')),
  title text NOT NULL,
  product_id uuid REFERENCES products(id) ON DELETE SET NULL,
  source_store_id uuid REFERENCES stores(id) ON DELETE SET NULL,
  reservation_id uuid REFERENCES network_reservations(id) ON DELETE SET NULL,
  qty numeric(18,3) NOT NULL DEFAULT 1 CHECK (qty > 0),
  unit_price bigint NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
  line_total bigint NOT NULL DEFAULT 0 CHECK (line_total >= 0),
  notes text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_workshop_job_items_job ON workshop_job_items(job_id, created_at);
CREATE INDEX idx_workshop_job_items_product ON workshop_job_items(product_id, created_at DESC) WHERE product_id IS NOT NULL;
CREATE INDEX idx_workshop_job_items_store ON workshop_job_items(source_store_id, created_at DESC) WHERE source_store_id IS NOT NULL;

CREATE TABLE workshop_job_payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id uuid NOT NULL REFERENCES workshop_jobs(id) ON DELETE CASCADE,
  mechanic_user_id uuid NOT NULL,
  method text NOT NULL CHECK (method IN ('cash','card','transfer','credit')),
  amount bigint NOT NULL CHECK (amount > 0),
  note text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_workshop_job_payments_job ON workshop_job_payments(job_id, created_at DESC);

ALTER TABLE vehicle_notebook_entries
  DROP CONSTRAINT IF EXISTS vehicle_notebook_entries_workshop_job_id_fkey,
  DROP CONSTRAINT IF EXISTS vehicle_notebook_entries_workshop_job_item_id_fkey;
ALTER TABLE vehicle_notebook_entries
  ADD CONSTRAINT vehicle_notebook_entries_workshop_job_id_fkey
    FOREIGN KEY (workshop_job_id) REFERENCES workshop_jobs(id) ON DELETE SET NULL,
  ADD CONSTRAINT vehicle_notebook_entries_workshop_job_item_id_fkey
    FOREIGN KEY (workshop_job_item_id) REFERENCES workshop_job_items(id) ON DELETE SET NULL;

COMMIT;
