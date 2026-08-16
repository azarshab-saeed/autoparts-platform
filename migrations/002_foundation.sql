BEGIN;

ALTER TABLE stores ADD COLUMN IF NOT EXISTS code text;
ALTER TABLE stores ADD COLUMN IF NOT EXISTS active boolean NOT NULL DEFAULT true;
ALTER TABLE stores ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE UNIQUE INDEX IF NOT EXISTS ux_stores_tenant_code ON stores(tenant_id, code) WHERE code IS NOT NULL;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_users_email_ci ON users(lower(email));

CREATE TABLE memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid REFERENCES stores(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('owner','admin','cashier','warehouse','accountant','viewer')),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, tenant_id, store_id, role)
);
CREATE INDEX idx_memberships_user ON memberships(user_id) WHERE active;
CREATE INDEX idx_memberships_tenant_store ON memberships(tenant_id, store_id) WHERE active;

CREATE TABLE brands (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_brands_scope_name ON brands(COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), normalized_name);

CREATE TABLE product_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES product_categories(id),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE products ADD COLUMN IF NOT EXISTS brand_id uuid REFERENCES brands(id);
ALTER TABLE products ADD COLUMN IF NOT EXISTS category_id uuid REFERENCES product_categories(id);
ALTER TABLE products ADD COLUMN IF NOT EXISTS barcode text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS normalized_title text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE products ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
UPDATE products SET normalized_title=lower(trim(title)) WHERE normalized_title IS NULL;
ALTER TABLE products ALTER COLUMN normalized_title SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_products_tenant_normalized_title ON products(tenant_id, normalized_title) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_products_tenant_barcode ON products(tenant_id, barcode) WHERE barcode IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE customers ADD COLUMN IF NOT EXISTS code text;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS notes text;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE customers ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_customers_tenant_store_name ON customers(tenant_id, store_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_tenant_store_phone ON customers(tenant_id, store_id, phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE suppliers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  code text,
  name text NOT NULL,
  phone text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE INDEX idx_suppliers_tenant_store_name ON suppliers(tenant_id, store_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_suppliers_tenant_store_phone ON suppliers(tenant_id, store_id, phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;

COMMIT;
