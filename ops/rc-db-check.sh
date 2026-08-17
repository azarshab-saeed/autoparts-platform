#!/bin/sh
set -eu

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-autoparts}"
POSTGRES_DB="${POSTGRES_DB:-autoparts}"

psql_scalar() {
  docker compose -f "$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atq -v ON_ERROR_STOP=1 -c "$1"
}

assert_zero() {
  name="$1"
  sql="$2"
  value="$(psql_scalar "$sql" | tr -d '[:space:]')"
  if [ "$value" != "0" ]; then
    printf 'FAIL %-34s violations=%s\n' "$name" "$value" >&2
    exit 1
  fi
  printf 'PASS %-34s\n' "$name"
}

printf '%s\n' 'AutoParts RC database invariants'

assert_zero "inventory non-negative + reserved<=on_hand" \
  "SELECT count(*) FROM inventory_balances WHERE on_hand < 0 OR reserved < 0 OR reserved > on_hand;"

assert_zero "reservation/procurement hold reconciliation" \
  "WITH holds AS (
     SELECT tenant_id, warehouse_id, product_id, sum(qty) qty
     FROM (
       SELECT tenant_id, warehouse_id, product_id, qty
       FROM network_reservations
       WHERE status IN ('pending','accepted','ready')
       UNION ALL
       SELECT seller_tenant_id, seller_warehouse_id, seller_product_id, qty
       FROM network_procurements
       WHERE status IN ('requested','accepted','ready')
     ) x
     GROUP BY tenant_id, warehouse_id, product_id
   )
   SELECT count(*)
   FROM inventory_balances b
   FULL JOIN holds h
     ON h.tenant_id=b.tenant_id AND h.warehouse_id=b.warehouse_id AND h.product_id=b.product_id
   WHERE COALESCE(b.reserved,0) <> COALESCE(h.qty,0);"

assert_zero "balanced journals" \
  "SELECT count(*) FROM (
     SELECT journal_id FROM journal_entries GROUP BY journal_id
     HAVING sum(debit) <> sum(credit)
   ) q;"

assert_zero "sales paid + due = total" \
  "SELECT count(*) FROM sales WHERE paid_amount + due_amount <> total_amount;"

assert_zero "purchases paid + due = total" \
  "SELECT count(*) FROM purchases WHERE paid_amount + due_amount <> total_amount;"

assert_zero "fulfilled reservation has sale" \
  "SELECT count(*) FROM network_reservations WHERE status='fulfilled' AND sale_id IS NULL;"

assert_zero "received procurement has both documents" \
  "SELECT count(*) FROM network_procurements WHERE status='received' AND (seller_sale_id IS NULL OR buyer_purchase_id IS NULL);"

assert_zero "sale item tenant consistency" \
  "SELECT count(*) FROM sale_items i JOIN sales s ON s.id=i.sale_id WHERE i.tenant_id <> s.tenant_id;"

assert_zero "purchase item tenant consistency" \
  "SELECT count(*) FROM purchase_items i JOIN purchases p ON p.id=i.purchase_id WHERE i.tenant_id <> p.tenant_id;"

assert_zero "catalog import row reconciliation" \
  "SELECT count(*) FROM catalog_import_batches b WHERE b.row_count <> (SELECT count(*) FROM catalog_import_row_results r WHERE r.batch_id=b.id);"

assert_zero "catalog import batch totals" \
  "SELECT count(*) FROM catalog_import_batches b WHERE b.created_count + b.updated_count <> b.row_count OR b.inventory_initialized_count + b.inventory_preserved_count <> b.row_count OR b.offers_upserted_count > b.row_count;"

assert_zero "catalog import movement trace" \
  "SELECT count(*) FROM inventory_movements m WHERE m.reference_type='catalog_import' AND NOT EXISTS (SELECT 1 FROM catalog_import_batches b WHERE b.id=m.reference_id AND b.tenant_id=m.tenant_id);"

assert_zero "catalog import opening value" \
  "SELECT count(*) FROM catalog_import_batches b WHERE b.opening_inventory_value <> COALESCE((SELECT SUM(m.cost_delta)::bigint FROM inventory_movements m WHERE m.tenant_id=b.tenant_id AND m.reference_type='catalog_import' AND m.reference_id=b.id),0);"

assert_zero "catalog import opening journal" \
  "SELECT count(*) FROM catalog_import_batches b WHERE b.opening_inventory_value > 0 AND NOT EXISTS (SELECT 1 FROM journals j JOIN journal_entries e ON e.journal_id=j.id AND e.tenant_id=j.tenant_id WHERE j.tenant_id=b.tenant_id AND j.reference_type='catalog_import' AND j.reference_id=b.id GROUP BY j.id HAVING SUM(e.debit)=b.opening_inventory_value AND SUM(e.credit)=b.opening_inventory_value);"

printf '%s\n' 'PASS all database invariants'
