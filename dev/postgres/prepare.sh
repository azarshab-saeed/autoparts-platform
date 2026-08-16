#!/bin/sh
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD:-autoparts}"
HOST="${POSTGRES_HOST:-postgres}"
PORT="${POSTGRES_PORT:-5432}"
ADMIN_USER="${POSTGRES_ADMIN_USER:-autoparts}"
ADMIN_DB="${POSTGRES_ADMIN_DB:-autoparts}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
psql_base="psql -h $HOST -p $PORT -U $ADMIN_USER"

if ! $psql_base -d "$ADMIN_DB" -tAc "SELECT 1 FROM pg_roles WHERE rolname='keycloak'" | grep -q 1; then
  $psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -c "CREATE ROLE keycloak LOGIN PASSWORD 'keycloak';"
fi
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

$psql_base -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -f /dev/postgres/seed.sql
