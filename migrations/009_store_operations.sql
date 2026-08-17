BEGIN;

CREATE TABLE IF NOT EXISTS daily_closings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  business_date date NOT NULL,
  opening_cash bigint NOT NULL CHECK (opening_cash >= 0),
  cash_in bigint NOT NULL CHECK (cash_in >= 0),
  cash_out bigint NOT NULL CHECK (cash_out >= 0),
  expected_cash bigint NOT NULL,
  actual_cash bigint NOT NULL CHECK (actual_cash >= 0),
  variance bigint NOT NULL,
  closed_by_user_id uuid NOT NULL,
  note text,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, store_id, business_date),
  UNIQUE (tenant_id, store_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_daily_closings_store_date
  ON daily_closings(tenant_id,store_id,business_date DESC);

CREATE INDEX IF NOT EXISTS idx_sales_store_created
  ON sales(tenant_id,store_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_store_created
  ON purchases(tenant_id,store_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_store_created
  ON payments(tenant_id,store_id,created_at DESC);

COMMIT;
