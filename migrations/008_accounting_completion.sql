BEGIN;

CREATE TABLE IF NOT EXISTS expense_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  code text NOT NULL,
  name text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS expenses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES expense_categories(id) ON DELETE RESTRICT,
  method text NOT NULL CHECK (method IN ('cash','card')),
  amount bigint NOT NULL CHECK (amount > 0),
  note text,
  occurred_on date NOT NULL DEFAULT CURRENT_DATE,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_expenses_store_date
  ON expenses(tenant_id,store_id,occurred_on DESC,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_category_date
  ON expenses(tenant_id,category_id,occurred_on DESC);

INSERT INTO expense_categories(tenant_id,code,name)
SELECT t.id, x.code, x.name
FROM tenants t
CROSS JOIN (VALUES
  ('RENT','اجاره'),
  ('PAYROLL','حقوق و دستمزد'),
  ('UTILITIES','آب، برق، گاز و اینترنت'),
  ('TRANSPORT','حمل و رفت‌وآمد'),
  ('SUPPLIES','ملزومات فروشگاه'),
  ('MARKETING','تبلیغات و بازاریابی'),
  ('OTHER','سایر هزینه‌ها')
) AS x(code,name)
ON CONFLICT (tenant_id,code) DO NOTHING;

COMMIT;
