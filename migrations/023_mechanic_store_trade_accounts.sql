BEGIN;

CREATE TABLE mechanic_store_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mechanic_user_id uuid NOT NULL,
  mechanic_name text NOT NULL DEFAULT '',
  mechanic_email text,
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(mechanic_user_id, store_id)
);
CREATE INDEX idx_mechanic_store_accounts_mechanic ON mechanic_store_accounts(mechanic_user_id, updated_at DESC);
CREATE INDEX idx_mechanic_store_accounts_store ON mechanic_store_accounts(tenant_id, store_id, updated_at DESC);

CREATE TABLE mechanic_store_trade_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES mechanic_store_accounts(id) ON DELETE CASCADE,
  request_type text NOT NULL CHECK (request_type IN ('charge','payment','return','credit','adjustment_debit','adjustment_credit')),
  amount bigint NOT NULL CHECK (amount > 0),
  method text CHECK (method IS NULL OR method IN ('cash','card','transfer','check','credit','other')),
  reservation_id uuid REFERENCES network_reservations(id) ON DELETE SET NULL,
  reference_type text,
  reference_id uuid,
  note text,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','confirmed','rejected')),
  initiated_by_role text NOT NULL CHECK (initiated_by_role IN ('mechanic','store')),
  initiated_by_user_id uuid NOT NULL,
  confirmed_by_role text CHECK (confirmed_by_role IS NULL OR confirmed_by_role IN ('mechanic','store')),
  confirmed_by_user_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);
CREATE INDEX idx_mechanic_trade_requests_account ON mechanic_store_trade_requests(account_id, created_at DESC);
CREATE INDEX idx_mechanic_trade_requests_pending ON mechanic_store_trade_requests(status, created_at) WHERE status='pending';
CREATE UNIQUE INDEX uq_mechanic_trade_request_reservation_charge
  ON mechanic_store_trade_requests(account_id, reservation_id)
  WHERE reservation_id IS NOT NULL AND request_type='charge' AND status <> 'rejected';

CREATE TABLE mechanic_store_ledger_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES mechanic_store_accounts(id) ON DELETE CASCADE,
  request_id uuid REFERENCES mechanic_store_trade_requests(id) ON DELETE RESTRICT,
  entry_type text NOT NULL CHECK (entry_type IN ('charge','payment','return','credit','adjustment_debit','adjustment_credit')),
  reference_type text,
  reference_id uuid,
  debit bigint NOT NULL DEFAULT 0 CHECK (debit >= 0),
  credit bigint NOT NULL DEFAULT 0 CHECK (credit >= 0),
  note text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0))
);
CREATE UNIQUE INDEX uq_mechanic_store_ledger_request ON mechanic_store_ledger_entries(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX idx_mechanic_store_ledger_account ON mechanic_store_ledger_entries(account_id, created_at DESC, id DESC);

COMMIT;
