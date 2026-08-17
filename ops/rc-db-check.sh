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

printf '%s\n' 'PASS all database invariants'
