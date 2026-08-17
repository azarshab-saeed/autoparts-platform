BEGIN;

CREATE TABLE IF NOT EXISTS audit_logs (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  store_id uuid NOT NULL REFERENCES stores(id),
  actor_user_id uuid NOT NULL,
  role text NOT NULL,
  request_id text NOT NULL,
  method text NOT NULL,
  path text NOT NULL,
  route text NOT NULL,
  status integer NOT NULL CHECK (status BETWEEN 100 AND 599),
  remote_ip text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_store_time
  ON audit_logs(tenant_id, store_id, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_time
  ON audit_logs(actor_user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id
  ON audit_logs(request_id);

COMMIT;
