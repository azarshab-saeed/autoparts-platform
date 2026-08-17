#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: CONFIRM_RESTORE=YES $0 <backup-directory>" >&2
  exit 2
fi
if [ "${CONFIRM_RESTORE:-}" != "YES" ]; then
  echo "Refusing restore. Set CONFIRM_RESTORE=YES after verifying the target environment." >&2
  exit 2
fi

dir="$1"
[ -d "$dir" ] || { echo "backup directory not found: $dir" >&2; exit 2; }
[ -f "$dir/autoparts.dump" ] || { echo "missing $dir/autoparts.dump" >&2; exit 2; }
[ -f "$dir/keycloak.dump" ] || { echo "missing $dir/keycloak.dump" >&2; exit 2; }
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
POSTGRES_USER="${POSTGRES_USER:-autoparts}"
POSTGRES_DB="${POSTGRES_DB:-autoparts}"
KEYCLOAK_DB="${KEYCLOAK_DB:-keycloak}"

if [ -f "$dir/SHA256SUMS" ] && command -v sha256sum >/dev/null 2>&1; then
  (cd "$dir" && sha256sum -c SHA256SUMS)
fi

echo "Stopping application and identity traffic before restore"
docker compose -f "$COMPOSE_FILE" stop api web keycloak
trap 'docker compose -f "$COMPOSE_FILE" up -d keycloak api web >/dev/null 2>&1 || true' EXIT INT TERM

echo "Restoring application database"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges < "$dir/autoparts.dump"

echo "Restoring Keycloak database"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_restore -U "$POSTGRES_USER" -d "$KEYCLOAK_DB" --clean --if-exists --no-owner --no-privileges < "$dir/keycloak.dump"

echo "Re-applying forward migrations and current Keycloak DB credentials"
docker compose -f "$COMPOSE_FILE" run --rm db-prepare

echo "Starting Keycloak and reconciling production realm/client configuration"
docker compose -f "$COMPOSE_FILE" up -d keycloak
docker compose -f "$COMPOSE_FILE" run --rm keycloak-bootstrap

echo "Starting application"
docker compose -f "$COMPOSE_FILE" up -d api web
trap - EXIT INT TERM
