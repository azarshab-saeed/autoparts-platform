BEGIN;

ALTER TABLE network_reservations
  ADD COLUMN IF NOT EXISTS sale_id uuid REFERENCES sales(id) ON DELETE RESTRICT,
  ADD COLUMN IF NOT EXISTS fulfilled_at timestamptz,
  ADD COLUMN IF NOT EXISTS fulfilled_by_user_id uuid;

ALTER TABLE network_reservations
  DROP CONSTRAINT IF EXISTS network_reservations_status_check;
ALTER TABLE network_reservations
  ADD CONSTRAINT network_reservations_status_check
  CHECK (status IN ('pending','accepted','ready','fulfilled','rejected','cancelled','expired'));

ALTER TABLE network_reservation_events
  DROP CONSTRAINT IF EXISTS network_reservation_events_event_type_check;
ALTER TABLE network_reservation_events
  ADD CONSTRAINT network_reservation_events_event_type_check
  CHECK (event_type IN ('created','accepted','ready','fulfilled','rejected','cancelled','expired'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_network_reservations_sale
  ON network_reservations(sale_id)
  WHERE sale_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_network_reservations_fulfilled
  ON network_reservations(tenant_id,store_id,fulfilled_at DESC)
  WHERE status='fulfilled';

COMMIT;
