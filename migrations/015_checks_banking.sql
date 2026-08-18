BEGIN;

ALTER TABLE party_ledger_entries DROP CONSTRAINT IF EXISTS party_ledger_entries_entry_type_check;
ALTER TABLE party_ledger_entries ADD CONSTRAINT party_ledger_entries_entry_type_check CHECK (
  entry_type IN (
    'sale','purchase','receipt','payment','sale_return','purchase_return',
    'check_receipt','check_bounce','check_payment','check_return'
  )
);

CREATE TABLE store_bank_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
  name text NOT NULL,
  bank_name text NOT NULL,
  account_number text,
  card_number text,
  iban text,
  opening_balance bigint NOT NULL DEFAULT 0 CHECK (opening_balance >= 0),
  active boolean NOT NULL DEFAULT true,
  is_default boolean NOT NULL DEFAULT false,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key),
  UNIQUE (tenant_id, account_id)
);
CREATE INDEX idx_store_bank_accounts_store ON store_bank_accounts(tenant_id,store_id,active,name);
CREATE UNIQUE INDEX ux_store_bank_accounts_default
  ON store_bank_accounts(tenant_id,store_id)
  WHERE is_default AND active;

CREATE TABLE checks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  direction text NOT NULL CHECK (direction IN ('receivable','payable')),
  customer_id uuid REFERENCES customers(id) ON DELETE RESTRICT,
  supplier_id uuid REFERENCES suppliers(id) ON DELETE RESTRICT,
  check_number text NOT NULL,
  sayad_id text,
  bank_name text,
  branch_name text,
  amount bigint NOT NULL CHECK (amount > 0),
  issue_date date NOT NULL,
  due_date date NOT NULL,
  status text NOT NULL,
  bank_account_id uuid REFERENCES store_bank_accounts(id) ON DELETE RESTRICT,
  endorsed_supplier_id uuid REFERENCES suppliers(id) ON DELETE RESTRICT,
  note text,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key),
  CHECK (due_date >= issue_date),
  CHECK (
    (direction='receivable' AND customer_id IS NOT NULL AND supplier_id IS NULL
      AND status IN ('held','deposited','cleared','bounced','endorsed','returned','cancelled'))
    OR
    (direction='payable' AND supplier_id IS NOT NULL AND customer_id IS NULL
      AND status IN ('issued','cleared','returned','cancelled'))
  )
);
CREATE INDEX idx_checks_store_due ON checks(tenant_id,store_id,direction,status,due_date,created_at DESC);
CREATE INDEX idx_checks_customer ON checks(tenant_id,store_id,customer_id,created_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_checks_supplier ON checks(tenant_id,store_id,supplier_id,created_at DESC) WHERE supplier_id IS NOT NULL;
CREATE INDEX idx_checks_number ON checks(tenant_id,store_id,check_number);
CREATE INDEX idx_checks_sayad ON checks(tenant_id,store_id,sayad_id) WHERE sayad_id IS NOT NULL;

CREATE TABLE check_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  check_id uuid NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
  action text NOT NULL,
  idempotency_key text,
  from_status text,
  to_status text NOT NULL,
  bank_account_id uuid REFERENCES store_bank_accounts(id) ON DELETE RESTRICT,
  supplier_id uuid REFERENCES suppliers(id) ON DELETE RESTRICT,
  actor_user_id uuid,
  note text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_check_events_check ON check_events(tenant_id,store_id,check_id,created_at,id);
CREATE UNIQUE INDEX ux_check_events_idempotency ON check_events(tenant_id,idempotency_key) WHERE idempotency_key IS NOT NULL;

COMMIT;
