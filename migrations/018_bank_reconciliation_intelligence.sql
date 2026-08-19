BEGIN;

CREATE TABLE bank_statement_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  bank_account_id uuid NOT NULL REFERENCES store_bank_accounts(id) ON DELETE CASCADE,
  txn_date date NOT NULL,
  amount bigint NOT NULL CHECK (amount <> 0),
  description text,
  reference text,
  external_id text,
  fingerprint text NOT NULL,
  imported_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, bank_account_id, fingerprint)
);
CREATE INDEX idx_bank_statement_lines_account_date ON bank_statement_lines(tenant_id,store_id,bank_account_id,txn_date DESC,created_at DESC);
CREATE INDEX idx_bank_statement_lines_duplicate_assist ON bank_statement_lines(tenant_id,store_id,bank_account_id,txn_date,amount);

CREATE TABLE bank_reconciliation_matches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  statement_line_id uuid NOT NULL REFERENCES bank_statement_lines(id) ON DELETE CASCADE,
  journal_entry_id uuid NOT NULL REFERENCES journal_entries(id) ON DELETE RESTRICT,
  matched_amount bigint NOT NULL CHECK (matched_amount > 0),
  note text,
  matched_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, statement_line_id, journal_entry_id)
);
CREATE INDEX idx_bank_reconciliation_matches_line ON bank_reconciliation_matches(tenant_id,store_id,statement_line_id,created_at,id);
CREATE INDEX idx_bank_reconciliation_matches_entry ON bank_reconciliation_matches(tenant_id,journal_entry_id);

CREATE TABLE bank_reconciliation_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  bank_account_id uuid NOT NULL REFERENCES store_bank_accounts(id) ON DELETE CASCADE,
  statement_line_id uuid REFERENCES bank_statement_lines(id) ON DELETE CASCADE,
  match_id uuid,
  action text NOT NULL CHECK (action IN ('statement_imported','matched','unmatched')),
  actor_user_id uuid,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_bank_reconciliation_events_account ON bank_reconciliation_events(tenant_id,store_id,bank_account_id,created_at DESC,id DESC);

COMMIT;
