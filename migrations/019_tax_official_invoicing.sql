BEGIN;

CREATE TABLE tenant_tax_profiles (
  tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  legal_name text,
  national_id text,
  economic_code text,
  registration_number text,
  postal_code text,
  province text,
  city text,
  address text,
  phone text,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE store_tax_settings (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  tax_enabled boolean NOT NULL DEFAULT false,
  tax_on_normal_sales boolean NOT NULL DEFAULT false,
  calculation_mode text NOT NULL DEFAULT 'exclusive' CHECK (calculation_mode IN ('exclusive','inclusive')),
  default_invoice_mode text NOT NULL DEFAULT 'normal' CHECK (default_invoice_mode IN ('normal','official')),
  default_tax_code text,
  official_series text NOT NULL DEFAULT 'A',
  next_official_number bigint NOT NULL DEFAULT 1 CHECK (next_official_number > 0),
  invoice_number_width integer NOT NULL DEFAULT 6 CHECK (invoice_number_width BETWEEN 1 AND 12),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, store_id)
);
INSERT INTO store_tax_settings(tenant_id,store_id)
SELECT tenant_id,id FROM stores
ON CONFLICT (tenant_id,store_id) DO NOTHING;

CREATE TABLE tax_rates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  code text NOT NULL,
  name text NOT NULL,
  category text NOT NULL DEFAULT 'taxable' CHECK (category IN ('taxable','exempt','non_taxable')),
  rate_bps integer NOT NULL DEFAULT 0 CHECK (rate_bps BETWEEN 0 AND 10000),
  effective_from date NOT NULL,
  effective_to date,
  exemption_reason text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (effective_to IS NULL OR effective_to >= effective_from),
  UNIQUE (tenant_id, store_id, code, effective_from)
);
CREATE INDEX idx_tax_rates_lookup ON tax_rates(tenant_id,store_id,code,effective_from DESC) WHERE active;

CREATE TABLE product_tax_profiles (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  tax_code text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, store_id, product_id)
);
CREATE INDEX idx_product_tax_profiles_code ON product_tax_profiles(tenant_id,store_id,tax_code);

ALTER TABLE customers
  ADD COLUMN IF NOT EXISTS legal_type text CHECK (legal_type IS NULL OR legal_type IN ('natural','legal','other')),
  ADD COLUMN IF NOT EXISTS national_id text,
  ADD COLUMN IF NOT EXISTS economic_code text,
  ADD COLUMN IF NOT EXISTS registration_number text,
  ADD COLUMN IF NOT EXISTS postal_code text,
  ADD COLUMN IF NOT EXISTS address text;
CREATE INDEX IF NOT EXISTS idx_customers_national_id ON customers(tenant_id,store_id,national_id) WHERE national_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE sales
  ADD COLUMN IF NOT EXISTS invoice_mode text NOT NULL DEFAULT 'normal' CHECK (invoice_mode IN ('normal','official')),
  ADD COLUMN IF NOT EXISTS invoice_kind text NOT NULL DEFAULT 'original' CHECK (invoice_kind IN ('original','correction','cancellation')),
  ADD COLUMN IF NOT EXISTS invoice_parent_sale_id uuid REFERENCES sales(id),
  ADD COLUMN IF NOT EXISTS invoice_state text NOT NULL DEFAULT 'not_applicable' CHECK (invoice_state IN ('not_applicable','issued','correction_requested','cancellation_requested')),
  ADD COLUMN IF NOT EXISTS invoice_series text,
  ADD COLUMN IF NOT EXISTS invoice_number bigint,
  ADD COLUMN IF NOT EXISTS invoice_number_display text,
  ADD COLUMN IF NOT EXISTS invoice_issued_at timestamptz,
  ADD COLUMN IF NOT EXISTS seller_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS buyer_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS tax_calculation_mode text NOT NULL DEFAULT 'exclusive' CHECK (tax_calculation_mode IN ('exclusive','inclusive')),
  ADD COLUMN IF NOT EXISTS net_amount bigint NOT NULL DEFAULT 0 CHECK (net_amount >= 0),
  ADD COLUMN IF NOT EXISTS taxable_amount bigint NOT NULL DEFAULT 0 CHECK (taxable_amount >= 0),
  ADD COLUMN IF NOT EXISTS exempt_amount bigint NOT NULL DEFAULT 0 CHECK (exempt_amount >= 0),
  ADD COLUMN IF NOT EXISTS tax_amount bigint NOT NULL DEFAULT 0 CHECK (tax_amount >= 0);
UPDATE sales SET net_amount=total_amount WHERE net_amount=0 AND total_amount<>0;
CREATE UNIQUE INDEX ux_sales_official_number ON sales(tenant_id,store_id,invoice_series,invoice_number) WHERE invoice_mode='official' AND invoice_number IS NOT NULL;
CREATE INDEX idx_sales_invoice_mode_time ON sales(tenant_id,store_id,invoice_mode,created_at DESC);

ALTER TABLE sale_items
  ADD COLUMN IF NOT EXISTS tax_category text NOT NULL DEFAULT 'not_applied' CHECK (tax_category IN ('not_applied','taxable','exempt','non_taxable')),
  ADD COLUMN IF NOT EXISTS tax_code text,
  ADD COLUMN IF NOT EXISTS tax_rate_name text,
  ADD COLUMN IF NOT EXISTS tax_rate_bps integer NOT NULL DEFAULT 0 CHECK (tax_rate_bps BETWEEN 0 AND 10000),
  ADD COLUMN IF NOT EXISTS tax_base_amount bigint NOT NULL DEFAULT 0 CHECK (tax_base_amount >= 0),
  ADD COLUMN IF NOT EXISTS tax_amount bigint NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
  ADD COLUMN IF NOT EXISTS total_with_tax bigint NOT NULL DEFAULT 0 CHECK (total_with_tax >= 0),
  ADD COLUMN IF NOT EXISTS tax_exemption_reason text;
UPDATE sale_items SET total_with_tax=line_total WHERE total_with_tax=0 AND line_total<>0;

ALTER TABLE sales_returns
  ADD COLUMN IF NOT EXISTS net_amount bigint NOT NULL DEFAULT 0 CHECK (net_amount >= 0),
  ADD COLUMN IF NOT EXISTS tax_amount bigint NOT NULL DEFAULT 0 CHECK (tax_amount >= 0);
UPDATE sales_returns SET net_amount=total_amount WHERE net_amount=0 AND total_amount<>0;

ALTER TABLE sales_return_items
  ADD COLUMN IF NOT EXISTS net_amount bigint NOT NULL DEFAULT 0 CHECK (net_amount >= 0),
  ADD COLUMN IF NOT EXISTS tax_amount bigint NOT NULL DEFAULT 0 CHECK (tax_amount >= 0);
UPDATE sales_return_items SET net_amount=line_total WHERE net_amount=0 AND line_total<>0;

CREATE TABLE official_invoice_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  sale_id uuid NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  action_type text NOT NULL CHECK (action_type IN ('correction','cancellation')),
  reason text NOT NULL,
  status text NOT NULL DEFAULT 'requested' CHECK (status IN ('requested','completed','cancelled')),
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  replacement_sale_id uuid REFERENCES sales(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE INDEX idx_official_invoice_actions_sale ON official_invoice_actions(tenant_id,store_id,sale_id,created_at DESC);

COMMIT;
