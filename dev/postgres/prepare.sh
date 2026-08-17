#!/bin/sh
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD:-autoparts}"
HOST="${POSTGRES_HOST:-postgres}"
PORT="${POSTGRES_PORT:-5432}"
ADMIN_USER="${POSTGRES_ADMIN_USER:-autoparts}"
ADMIN_DB="${POSTGRES_ADMIN_DB:-autoparts}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
KEYCLOAK_DB_PASSWORD="${KEYCLOAK_DB_PASSWORD:-keycloak}"
SEED_DEMO_DATA="${SEED_DEMO_DATA:-true}"
psql_base="psql -h $HOST -p $PORT -U $ADMIN_USER"

# Reconcile the Keycloak DB role without embedding a password directly into
# shell SQL. psql quotes the variable and \gexec executes the generated DDL.
$psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -v kc_password="$KEYCLOAK_DB_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE keycloak LOGIN PASSWORD %L', :'kc_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='keycloak') \gexec
SELECT format('ALTER ROLE keycloak LOGIN PASSWORD %L', :'kc_password') \gexec
SQL

if ! $psql_base -d "$ADMIN_DB" -tAc "SELECT 1 FROM pg_database WHERE datname='keycloak'" | grep -q 1; then
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "CREATE DATABASE keycloak OWNER keycloak;"
fi

$psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

# Adopt databases created by Phase 1-5, where migrations were applied by the
# postgres entrypoint before schema_migrations existed.
if $psql_base -d "$ADMIN_DB" -tAc "SELECT to_regclass('public.tenants') IS NOT NULL" | grep -q t; then
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES('001_init.sql') ON CONFLICT DO NOTHING;"
fi
if $psql_base -d "$ADMIN_DB" -tAc "SELECT to_regclass('public.suppliers') IS NOT NULL" | grep -q t; then
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES('002_foundation.sql') ON CONFLICT DO NOTHING;"
fi
if $psql_base -d "$ADMIN_DB" -tAc "SELECT to_regclass('public.purchases') IS NOT NULL" | grep -q t; then
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES('003_inventory_purchase.sql') ON CONFLICT DO NOTHING;"
fi

for file in "$MIGRATIONS_DIR"/*.sql; do
  [ -f "$file" ] || continue
  version="$(basename "$file")"
  if $psql_base -d "$ADMIN_DB" -tAc "SELECT 1 FROM schema_migrations WHERE version='$version'" | grep -q 1; then
    echo "migration $version already applied"
    continue
  fi
  echo "applying migration $version"
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -f "$file"
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES('$version');"
done

if [ "$SEED_DEMO_DATA" = "true" ]; then
  echo "loading development seed data"
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -f /dev/postgres/seed.sql

  # Older dev seeds overwrote transactional reserved quantities on compose
  # start. Repair only demo rows from active reservations/procurements.
  if $psql_base -d "$ADMIN_DB" -tAc "SELECT to_regclass('public.network_reservations') IS NOT NULL" | grep -q t; then
    $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -f /dev/postgres/repair_network_demo_reservations.sql
  fi
else
  echo "SEED_DEMO_DATA=false; skipping all demo seed/repair SQL"
fi
