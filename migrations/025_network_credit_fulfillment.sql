BEGIN;

ALTER TABLE network_reservations
  ADD COLUMN IF NOT EXISTS buyer_role text;

ALTER TABLE network_reservations
  DROP CONSTRAINT IF EXISTS network_reservations_buyer_role_check;
ALTER TABLE network_reservations
  ADD CONSTRAINT network_reservations_buyer_role_check
  CHECK (buyer_role IS NULL OR buyer_role IN ('mechanic','consumer'));

CREATE INDEX IF NOT EXISTS idx_network_reservations_buyer_role
  ON network_reservations(buyer_role, created_at DESC)
  WHERE buyer_role IS NOT NULL;

COMMIT;
