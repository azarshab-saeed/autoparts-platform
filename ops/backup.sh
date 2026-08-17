#!/bin/sh
set -eu

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
POSTGRES_USER="${POSTGRES_USER:-autoparts}"
POSTGRES_DB="${POSTGRES_DB:-autoparts}"
KEYCLOAK_DB="${KEYCLOAK_DB:-keycloak}"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
outdir="$BACKUP_DIR/$ts"
mkdir -p "$outdir"

echo "Creating application database backup"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges > "$outdir/autoparts.dump"

echo "Creating Keycloak database backup"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$KEYCLOAK_DB" --format=custom --no-owner --no-privileges > "$outdir/keycloak.dump"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$outdir" && sha256sum autoparts.dump keycloak.dump > SHA256SUMS)
  cat "$outdir/SHA256SUMS"
fi

cat > "$outdir/README.txt" <<TXT
AutoParts production backup
UTC timestamp: $ts
Application database: $POSTGRES_DB
Identity database: $KEYCLOAK_DB
Restore with: CONFIRM_RESTORE=YES COMPOSE_FILE=$COMPOSE_FILE ./ops/restore.sh $outdir
TXT

echo "Backup complete: $outdir"
