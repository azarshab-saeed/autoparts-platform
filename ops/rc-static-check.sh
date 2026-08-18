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
grep -Fq '015_checks_banking.sql' cmd/api/main.go || fail 'API readiness is not pinned to checks/banking migration'
grep -Fq 'CREATE TABLE checks' migrations/015_checks_banking.sql || fail 'checks migration is missing checks table'
grep -Fq 'CREATE TABLE store_bank_accounts' migrations/015_checks_banking.sql || fail 'checks migration is missing store bank accounts'
grep -Fq 'AUTOPARTS_EDGE_ALLOW_MOCK_POS' internal/storeedge/hardware.go || fail 'mock POS is not explicitly gated'
grep -Fq 'AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT' internal/storeedge/hardware.go || fail 'QA file printer transport is not explicitly gated'
if grep -Fq 'AUTOPARTS_EDGE_ALLOW_MOCK_POS=true' docker-compose.prod.yml; then fail 'mock POS must never be enabled in production compose'; fi
if grep -Fq 'AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT=true' docker-compose.prod.yml; then fail 'file printer transport must never be enabled in production compose'; fi

printf '%s\n' 'PASS RC static safety checks'
