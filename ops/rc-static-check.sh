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

printf '%s\n' 'PASS RC static safety checks'
