#!/bin/sh
set -eu

KCADM="/opt/keycloak/bin/kcadm.sh"
SERVER="${KEYCLOAK_SYNC_SERVER:-http://keycloak:8080}"
REALM="${KEYCLOAK_REALM:-autoparts}"
ADMIN_USER="${KEYCLOAK_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin-change-me}"
CLIENT_ID="autoparts-qa"

export HOME="${KEYCLOAK_QA_SYNC_HOME:-/tmp/keycloak-qa-sync-home}"
mkdir -p "$HOME"

log() { printf '%s\n' "[keycloak-qa-sync] $*"; }

attempt=0
while [ "$attempt" -lt 60 ]; do
  if "$KCADM" config credentials --server "$SERVER" --realm master --user "$ADMIN_USER" --password "$ADMIN_PASSWORD" >/tmp/keycloak-qa-login.log 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$attempt" -ge 60 ]; then
  cat /tmp/keycloak-qa-login.log >&2 || true
  exit 1
fi

client_uuid() {
  "$KCADM" get clients -r "$REALM" -q "clientId=$1" --fields id --format csv --noquotes 2>/dev/null | head -n 1
}

QA_UUID="$(client_uuid "$CLIENT_ID")"
if [ -z "$QA_UUID" ]; then
  log "creating dev-only password-grant QA client"
  QA_UUID="$("$KCADM" create clients -r "$REALM" \
    -s "clientId=$CLIENT_ID" \
    -s enabled=true \
    -s publicClient=true \
    -s standardFlowEnabled=false \
    -s implicitFlowEnabled=false \
    -s directAccessGrantsEnabled=true \
    -s serviceAccountsEnabled=false \
    -s fullScopeAllowed=true \
    -i)"
else
  "$KCADM" update "clients/$QA_UUID" -r "$REALM" \
    -s enabled=true \
    -s publicClient=true \
    -s standardFlowEnabled=false \
    -s implicitFlowEnabled=false \
    -s directAccessGrantsEnabled=true \
    -s serviceAccountsEnabled=false \
    -s fullScopeAllowed=true >/dev/null
fi

mapper_id() {
  mapper_name="$1"
  "$KCADM" get "clients/$QA_UUID/protocol-mappers/models" -r "$REALM" \
    --fields id,name --format csv --noquotes 2>/dev/null | \
    while IFS=, read -r id name; do
      if [ "$name" = "$mapper_name" ]; then
        printf '%s\n' "$id"
        break
      fi
    done
}

sync_mapper() {
  name="$1"
  file="$2"
  mid="$(mapper_id "$name")"
  if [ -n "$mid" ]; then
    update_file="/tmp/keycloak-qa-mapper-$mid.json"
    {
      printf '{\n  "id": "%s",\n' "$mid"
      sed '1d' "$file"
    } >"$update_file"
    "$KCADM" update "clients/$QA_UUID/protocol-mappers/models/$mid" -r "$REALM" -f "$update_file" >/dev/null
  else
    "$KCADM" create "clients/$QA_UUID/protocol-mappers/models" -r "$REALM" -f "$file" >/dev/null
  fi
}

cat >/tmp/qa-tenant-id.json <<'JSON'
{
  "name": "tenant-id",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "consentRequired": false,
  "config": {
    "user.attribute": "tenant_id",
    "claim.name": "tenant_id",
    "jsonType.label": "String",
    "access.token.claim": "true",
    "id.token.claim": "false",
    "userinfo.token.claim": "false",
    "introspection.token.claim": "true"
  }
}
JSON

cat >/tmp/qa-store-id.json <<'JSON'
{
  "name": "store-id",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "consentRequired": false,
  "config": {
    "user.attribute": "store_id",
    "claim.name": "store_id",
    "jsonType.label": "String",
    "access.token.claim": "true",
    "id.token.claim": "false",
    "userinfo.token.claim": "false",
    "introspection.token.claim": "true"
  }
}
JSON

cat >/tmp/qa-audience.json <<'JSON'
{
  "name": "autoparts-api-audience",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-audience-mapper",
  "consentRequired": false,
  "config": {
    "included.client.audience": "autoparts-api",
    "access.token.claim": "true",
    "id.token.claim": "false",
    "introspection.token.claim": "true"
  }
}
JSON

sync_mapper tenant-id /tmp/qa-tenant-id.json
sync_mapper store-id /tmp/qa-store-id.json
sync_mapper autoparts-api-audience /tmp/qa-audience.json

DIRECT_GRANT="$("$KCADM" get "clients/$QA_UUID" -r "$REALM" --fields directAccessGrantsEnabled --format csv --noquotes 2>/dev/null | head -n 1)"
if [ "$DIRECT_GRANT" != "true" ]; then
  log "QA client direct grant verification failed"
  exit 1
fi

log "dev-only QA client ready: $CLIENT_ID"
