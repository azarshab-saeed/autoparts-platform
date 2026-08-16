BEGIN;

-- V1 business money unit is Iranian toman. IRT is an application code (not ISO-4217)
-- used deliberately to avoid the historic IRR/toman factor-of-10 ambiguity.
ALTER TABLE sales ALTER COLUMN currency SET DEFAULT 'IRT';
UPDATE sales SET currency='IRT' WHERE currency='IRR';
ALTER TABLE purchases ALTER COLUMN currency SET DEFAULT 'IRT';
UPDATE purchases SET currency='IRT' WHERE currency='IRR';


ALTER TABLE sales
  ADD COLUMN IF NOT EXISTS paid_amount bigint NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
  ADD COLUMN IF NOT EXISTS due_amount bigint NOT NULL DEFAULT 0 CHECK (due_amount >= 0),
  ADD CONSTRAINT sales_paid_due_check CHECK (paid_amount + due_amount = total_amount) NOT VALID;

ALTER TABLE purchases
  ADD COLUMN IF NOT EXISTS paid_amount bigint NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
  ADD COLUMN IF NOT EXISTS due_amount bigint NOT NULL DEFAULT 0 CHECK (due_amount >= 0),
  ADD CONSTRAINT purchases_paid_due_check CHECK (paid_amount + due_amount = total_amount) NOT VALID;

ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_payment_method_check;
ALTER TABLE purchases ADD CONSTRAINT purchases_payment_method_check CHECK (payment_method IN ('cash','card','credit','mixed'));

ALTER TABLE sale_items
  ADD COLUMN IF NOT EXISTS unit_cost bigint NOT NULL DEFAULT 0 CHECK (unit_cost >= 0);

-- Backfill document payment state from existing immediate-payment rows.
UPDATE sales s
SET paid_amount = COALESCE(p.paid, 0),
    due_amount  = s.total_amount - COALESCE(p.paid, 0)
FROM (
  SELECT sale_id, SUM(amount)::bigint AS paid
  FROM payments
  WHERE sale_id IS NOT NULL
  GROUP BY sale_id
) p
WHERE s.id = p.sale_id;
UPDATE sales SET due_amount = total_amount - paid_amount WHERE paid_amount + due_amount <> total_amount;

UPDATE purchases pch
SET paid_amount = COALESCE(p.paid, 0),
    due_amount  = pch.total_amount - COALESCE(p.paid, 0)
FROM (
  SELECT purchase_id, SUM(amount)::bigint AS paid
  FROM payments
  WHERE purchase_id IS NOT NULL
  GROUP BY purchase_id
) p
WHERE pch.id = p.purchase_id;
UPDATE purchases SET due_amount = total_amount - paid_amount WHERE paid_amount + due_amount <> total_amount;

-- Preserve the weighted-average unit cost used when each historical sale reduced stock.
UPDATE sale_items si
SET unit_cost = COALESCE((
  SELECT im.unit_cost
  FROM inventory_movements im
  WHERE im.tenant_id = si.tenant_id
    AND im.reference_type = 'sale'
    AND im.reference_id = si.sale_id
    AND im.product_id = si.product_id
    AND im.movement_type = 'sale'
  ORDER BY im.created_at ASC
  LIMIT 1
), 0)
WHERE si.unit_cost = 0;

ALTER TABLE sales VALIDATE CONSTRAINT sales_paid_due_check;
ALTER TABLE purchases VALIDATE CONSTRAINT purchases_paid_due_check;

CREATE TABLE party_ledger_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  party_type text NOT NULL CHECK (party_type IN ('customer','supplier')),
  customer_id uuid REFERENCES customers(id),
  supplier_id uuid REFERENCES suppliers(id),
  entry_type text NOT NULL CHECK (entry_type IN ('sale','purchase','receipt','payment','sale_return','purchase_return')),
  reference_id uuid NOT NULL,
  debit bigint NOT NULL DEFAULT 0 CHECK (debit >= 0),
  credit bigint NOT NULL DEFAULT 0 CHECK (credit >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((debit = 0 AND credit > 0) OR (credit = 0 AND debit > 0)),
  CHECK (
    (party_type='customer' AND customer_id IS NOT NULL AND supplier_id IS NULL)
    OR
    (party_type='supplier' AND supplier_id IS NOT NULL AND customer_id IS NULL)
  )
);
CREATE INDEX idx_party_ledger_customer ON party_ledger_entries(tenant_id, store_id, customer_id, created_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_party_ledger_supplier ON party_ledger_entries(tenant_id, store_id, supplier_id, created_at DESC) WHERE supplier_id IS NOT NULL;

-- Backfill unpaid balances from documents created before Phase 6.
INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit,created_at)
SELECT tenant_id,store_id,'customer',customer_id,'sale',id,due_amount,0,created_at
FROM sales
WHERE customer_id IS NOT NULL AND due_amount > 0
  AND NOT EXISTS (SELECT 1 FROM party_ledger_entries e WHERE e.entry_type='sale' AND e.reference_id=sales.id);

INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit,created_at)
SELECT tenant_id,store_id,'supplier',supplier_id,'purchase',id,0,due_amount,created_at
FROM purchases
WHERE due_amount > 0
  AND NOT EXISTS (SELECT 1 FROM party_ledger_entries e WHERE e.entry_type='purchase' AND e.reference_id=purchases.id);

CREATE TABLE settlements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  party_type text NOT NULL CHECK (party_type IN ('customer','supplier')),
  customer_id uuid REFERENCES customers(id),
  supplier_id uuid REFERENCES suppliers(id),
  method text NOT NULL CHECK (method IN ('cash','card')),
  amount bigint NOT NULL CHECK (amount > 0),
  note text,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key),
  CHECK (
    (party_type='customer' AND customer_id IS NOT NULL AND supplier_id IS NULL)
    OR
    (party_type='supplier' AND supplier_id IS NOT NULL AND customer_id IS NULL)
  )
);
CREATE INDEX idx_settlements_customer ON settlements(tenant_id,store_id,customer_id,created_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_settlements_supplier ON settlements(tenant_id,store_id,supplier_id,created_at DESC) WHERE supplier_id IS NOT NULL;

CREATE TABLE sales_returns (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  sale_id uuid NOT NULL REFERENCES sales(id),
  customer_id uuid REFERENCES customers(id),
  refund_method text NOT NULL CHECK (refund_method IN ('cash','card','credit_balance')),
  total_amount bigint NOT NULL CHECK (total_amount > 0),
  total_cost bigint NOT NULL CHECK (total_cost >= 0),
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);
CREATE TABLE sales_return_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  sales_return_id uuid NOT NULL REFERENCES sales_returns(id) ON DELETE CASCADE,
  sale_item_id uuid NOT NULL REFERENCES sale_items(id),
  product_id uuid NOT NULL REFERENCES products(id),
  qty numeric(18,3) NOT NULL CHECK (qty > 0),
  unit_price bigint NOT NULL CHECK (unit_price >= 0),
  unit_cost bigint NOT NULL CHECK (unit_cost >= 0),
  line_total bigint NOT NULL CHECK (line_total >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sales_returns_sale ON sales_returns(tenant_id,sale_id,created_at DESC);
CREATE INDEX idx_sales_return_items_source ON sales_return_items(tenant_id,sale_item_id);

CREATE TABLE purchase_returns (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  warehouse_id uuid NOT NULL REFERENCES warehouses(id),
  purchase_id uuid NOT NULL REFERENCES purchases(id),
  supplier_id uuid NOT NULL REFERENCES suppliers(id),
  refund_method text NOT NULL CHECK (refund_method IN ('cash','card','credit_balance')),
  total_amount bigint NOT NULL CHECK (total_amount > 0),
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);
CREATE TABLE purchase_return_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  purchase_return_id uuid NOT NULL REFERENCES purchase_returns(id) ON DELETE CASCADE,
  purchase_item_id uuid NOT NULL REFERENCES purchase_items(id),
  product_id uuid NOT NULL REFERENCES products(id),
  qty numeric(18,3) NOT NULL CHECK (qty > 0),
  unit_cost bigint NOT NULL CHECK (unit_cost >= 0),
  line_total bigint NOT NULL CHECK (line_total >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_purchase_returns_purchase ON purchase_returns(tenant_id,purchase_id,created_at DESC);
CREATE INDEX idx_purchase_return_items_source ON purchase_return_items(tenant_id,purchase_item_id);

COMMIT;
