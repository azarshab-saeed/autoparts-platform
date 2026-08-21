BEGIN;

ALTER TABLE sales
  ADD COLUMN IF NOT EXISTS actor_user_id uuid,
  ADD COLUMN IF NOT EXISTS actor_role text;
CREATE INDEX IF NOT EXISTS idx_sales_actor_performance
  ON sales(tenant_id, store_id, actor_user_id, created_at DESC)
  WHERE actor_user_id IS NOT NULL;

ALTER TABLE sales_returns
  ADD COLUMN IF NOT EXISTS actor_user_id uuid,
  ADD COLUMN IF NOT EXISTS actor_role text;
CREATE INDEX IF NOT EXISTS idx_sales_returns_actor_performance
  ON sales_returns(tenant_id, store_id, actor_user_id, created_at DESC)
  WHERE actor_user_id IS NOT NULL;

COMMIT;
