#!/bin/sh
set -eu

fail() {
  printf 'FAIL %s\n' "$*" >&2
  exit 1
}

printf '%s\n' 'AutoParts RC static safety checks'

grep -Fq 'autoparts-qa' keycloak/sync-qa-client.sh || fail 'dev QA sync script is missing autoparts-qa client'
grep -Fq 'keycloak-qa-sync' docker-compose.yml || fail 'dev compose is missing keycloak-qa-sync service'

if grep -Fq 'autoparts-qa' docker-compose.prod.yml; then
  fail 'development QA password-grant client leaked into production compose'
fi
if grep -Fq 'autoparts-qa' keycloak/bootstrap-prod.sh; then
  fail 'development QA password-grant client leaked into production bootstrap'
fi
if grep -Fq 'autoparts-qa' keycloak/autoparts-realm.json; then
  fail 'development QA password-grant client leaked into imported realm JSON'
fi

grep -Fq 'APP_VERSION=0.15.0-rc.1' .env.production.example || fail 'production example version is not RC1'
grep -Fq 'APP_VERSION:-0.15.0-rc.1' docker-compose.prod.yml || fail 'production compose version default is not RC1'

grep -Fq 'SEED_DEMO_DATA: "false"' docker-compose.prod.yml || fail 'production demo seed must stay disabled'
grep -Fq 'NEXT_PUBLIC_SHOW_DEV_CREDENTIALS: "false"' docker-compose.prod.yml || fail 'production dev credentials must stay hidden'

grep -Fq '127.0.0.1:17624' internal/storeedge/store.go || fail 'Store Edge must bind loopback by default'
grep -Fq 'offline sales support cash or card only' cmd/api/main.go || fail 'offline payment safety guard is missing'
grep -Fq '017_multi_unit_packaging.sql' cmd/api/main.go || fail 'API readiness is not pinned to multi-unit packaging migration'
grep -Fq 'CREATE TABLE checks' migrations/015_checks_banking.sql || fail 'checks migration is missing checks table'
grep -Fq 'CREATE TABLE store_bank_accounts' migrations/015_checks_banking.sql || fail 'checks migration is missing store bank accounts'
grep -Fq 'CREATE TABLE price_lists' migrations/016_advanced_pricing.sql || fail 'advanced pricing migration is missing price lists'
grep -Fq 'CREATE TABLE product_price_breaks' migrations/016_advanced_pricing.sql || fail 'advanced pricing migration is missing quantity breaks'
grep -Fq 'min_margin_bps' migrations/016_advanced_pricing.sql || fail 'advanced pricing migration is missing margin guard settings'
grep -Fq '/v1/pricing/quote' cmd/api/main.go || fail 'server-side pricing quote route is missing'
grep -Fq 'price below minimum margin' internal/sales/service.go || fail 'server-side sale margin guard is missing'
grep -Fq 'margin override reason is required' internal/sales/service.go || fail 'below-margin owner/admin audit reason is missing'
grep -Fq 'gross_amount' migrations/016_advanced_pricing.sql || fail 'historical gross sale amount is missing'
grep -Fq 'discount_amount' migrations/016_advanced_pricing.sql || fail 'historical discount amount is missing'
grep -Fq 'override_actor_user_id' migrations/016_advanced_pricing.sql || fail 'price override actor audit field is missing'
grep -Fq 'below_margin_guard' migrations/016_advanced_pricing.sql || fail 'historical margin guard decision is missing'
grep -Fq 'CREATE TABLE product_units' migrations/017_multi_unit_packaging.sql || fail 'multi-unit migration is missing product units'
grep -Fq 'ux_product_units_tenant_barcode' migrations/017_multi_unit_packaging.sql || fail 'tenant-scoped package barcode uniqueness is missing'
grep -Fq 'commercial_qty' migrations/017_multi_unit_packaging.sql || fail 'commercial quantity history is missing'
grep -Fq 'product_unit_id' internal/sales/model.go || fail 'sales payload is missing selected product unit'
grep -Fq 'product_unit_id' internal/purchases/model.go || fail 'purchase payload is missing selected product unit'
grep -Fq 'BaseQty' internal/productunit/unit.go || fail 'base quantity conversion guard is missing'
test -f web/app/store/products/new/page.tsx || fail 'new-product barcode/unit wizard is missing'
test -f 'web/app/store/products/[id]/units/page.tsx' || fail 'product unit/barcode management page is missing'
grep -Fq 'price_breaks' internal/edge/model.go || fail 'Store Edge snapshot is missing offline quantity breaks'
grep -Fq 'priceForQuantity' internal/storeedge/store.go || fail 'Store Edge offline quantity pricing resolver is missing'
grep -Fq 'BaseQty' internal/storeedge/types.go || fail 'Store Edge local sale is missing canonical base quantity'
grep -Fq 'product_unit_id' internal/storeedge/cloud.go || fail 'Store Edge cloud sync is missing selected product unit'
grep -Fq 'product_unit_id:x.unit.id' web/lib/edge.ts || fail 'web POS offline queue is missing selected product unit'
grep -Fq 'unit.Barcode' internal/storeedge/store.go || fail 'Store Edge catalog search is missing package barcodes'
grep -Fq 'product_unit_id:x.product_unit_id' cmd/store-edge/offline.html || fail 'standalone Store Edge POS is missing package-unit queueing'
grep -Fq 'exactBarcodeUnit' cmd/store-edge/offline.html || fail 'standalone Store Edge POS is missing package-barcode resolution'
grep -Fq 'PricingPolicy' internal/edge/model.go || fail 'Store Edge snapshot is missing cashier pricing policy'
grep -Fq 'preserve_price:true' web/lib/edge.ts || fail 'web POS offline fallback does not preserve resolved tier price'
grep -Fq 'customer_id:customerId||undefined' web/lib/edge.ts || fail 'web POS offline fallback does not carry customer tier identity'
grep -Fq 'cmd.ActorRole == "edge"' internal/sales/service.go || fail 'Store Edge price override is not covered by server-side cashier policy'
grep -Fq 'AUTOPARTS_EDGE_ALLOW_MOCK_POS' internal/storeedge/hardware.go || fail 'mock POS is not explicitly gated'
grep -Fq 'AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT' internal/storeedge/hardware.go || fail 'QA file printer transport is not explicitly gated'
if grep -Fq 'AUTOPARTS_EDGE_ALLOW_MOCK_POS=true' docker-compose.prod.yml; then fail 'mock POS must never be enabled in production compose'; fi
if grep -Fq 'AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT=true' docker-compose.prod.yml; then fail 'file printer transport must never be enabled in production compose'; fi

printf '%s\n' 'PASS RC static safety checks'
