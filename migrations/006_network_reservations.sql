BEGIN;

CREATE TABLE IF NOT EXISTS network_reservations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  warehouse_id uuid NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
  offer_id uuid NOT NULL REFERENCES store_product_offers(id) ON DELETE RESTRICT,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  buyer_user_id uuid NOT NULL,
  buyer_name text NOT NULL DEFAULT '',
  buyer_email text NOT NULL DEFAULT '',
  qty numeric(18,3) NOT NULL CHECK (qty > 0),
  unit_price bigint NOT NULL CHECK (unit_price > 0),
  total_amount bigint NOT NULL CHECK (total_amount > 0),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','ready','rejected','cancelled','expired')),
  idempotency_key text NOT NULL,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  ready_at timestamptz,
  closed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (buyer_user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_network_reservations_store_status
  ON network_reservations(tenant_id,store_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_network_reservations_buyer
  ON network_reservations(buyer_user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_network_reservations_expiry
  ON network_reservations(expires_at)
  WHERE status IN ('pending','accepted','ready');

CREATE TABLE IF NOT EXISTS network_reservation_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id uuid NOT NULL REFERENCES network_reservations(id) ON DELETE CASCADE,
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_user_id uuid,
  event_type text NOT NULL CHECK (event_type IN ('created','accepted','ready','rejected','cancelled','expired')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_network_reservation_events_reservation
  ON network_reservation_events(reservation_id,created_at);

COMMIT;
