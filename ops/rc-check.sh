#!/bin/sh
set -eu

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
RC_SKIP_BUILD="${RC_SKIP_BUILD:-false}"
RC_SKIP_LOAD="${RC_SKIP_LOAD:-false}"
RC_SKIP_WEB="${RC_SKIP_WEB:-false}"

log() { printf '\n==> %s\n' "$*"; }

log "Static RC safety"
./ops/rc-static-check.sh

if [ "$RC_SKIP_BUILD" != "true" ]; then
  log "Build and start dev stack"
  APP_VERSION="${APP_VERSION:-0.15.0-rc.1}" \
  APP_COMMIT="${APP_COMMIT:-local}" \
  APP_BUILD_TIME="${APP_BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" \
    docker compose -f "$COMPOSE_FILE" up -d --build
fi

log "Wait for API readiness"
attempt=0
until curl -fsS "${RC_API_URL:-http://localhost:8080}/readyz" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    docker compose -f "$COMPOSE_FILE" ps >&2 || true
    docker compose -f "$COMPOSE_FILE" logs --tail=120 api keycloak keycloak-sync keycloak-qa-sync >&2 || true
    echo "API did not become ready" >&2
    exit 1
  fi
  sleep 2
done

log "Go unit/integration tests"
go test ./...

if [ "$RC_SKIP_WEB" != "true" ]; then
  log "Next.js production build"
  (
    cd web
    if [ ! -d node_modules ]; then
      npm install --no-audit --no-fund
    fi
    npm run build
  )
fi

log "End-to-end smoke"
go run ./cmd/rc-smoke

log "Database invariants"
COMPOSE_FILE="$COMPOSE_FILE" ./ops/rc-db-check.sh

if [ "$RC_SKIP_LOAD" != "true" ]; then
  log "Performance baseline"
  go run ./cmd/rc-load
fi

log "Release metadata"
curl -fsS "${RC_API_URL:-http://localhost:8080}/version"
printf '\n'

printf '\n%s\n' 'RC CHECK PASSED'
