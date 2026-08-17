#!/bin/sh
set -eu

KCADM="/opt/keycloak/bin/kcadm.sh"
SERVER="${KEYCLOAK_BOOTSTRAP_SERVER:-http://keycloak:8080}"
REALM="${KEYCLOAK_REALM:-autoparts}"
ADMIN_USER="${KEYCLOAK_ADMIN_USERNAME:?KEYCLOAK_ADMIN_USERNAME is required}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:?KEYCLOAK_ADMIN_PASSWORD is required}"
APP_PUBLIC_URL="${APP_PUBLIC_URL:?APP_PUBLIC_URL is required}"

export HOME="${KEYCLOAK_BOOTSTRAP_HOME:-/tmp/keycloak-bootstrap-home}"
mkdir -p "$HOME"

log(){ printf '%s\n' "[keycloak-bootstrap] $*"; }

login_admin(){
  attempts=0
  while [ "$attempts" -lt 60 ]; do
    if "$KCADM" config credentials --server "$SERVER" --realm master --user "$ADMIN_USER" --password "$ADMIN_PASSWORD" >/tmp/keycloak-bootstrap-login.log 2>&1; then return 0; fi
    attempts=$((attempts+1)); sleep 2
  done
  log "could not authenticate to Keycloak after 120 seconds"
  cat /tmp/keycloak-bootstrap-login.log >&2 || true
  return 1
}

ensure_role(){
  role="$1"
  "$KCADM" get "roles/$role" -r "$REALM" >/dev/null 2>&1 || "$KCADM" create roles -r "$REALM" -s "name=$role" >/dev/null
}

client_uuid(){
  "$KCADM" get clients -r "$REALM" -q "clientId=$1" --fields id --format csv --noquotes 2>/dev/null | head -n 1
}

ensure_api_client(){
  id="$(client_uuid autoparts-api)"
  file="/tmp/autoparts-api-client.json"
  if [ -n "$id" ]; then
    cat >"$file" <<JSON
{"id":"$id","clientId":"autoparts-api","name":"AutoParts API","enabled":true,"protocol":"openid-connect","bearerOnly":true,"standardFlowEnabled":false,"directAccessGrantsEnabled":false,"serviceAccountsEnabled":false}
JSON
    "$KCADM" update "clients/$id" -r "$REALM" -f "$file" >/dev/null
  else
    cat >"$file" <<'JSON'
{"clientId":"autoparts-api","name":"AutoParts API","enabled":true,"protocol":"openid-connect","bearerOnly":true,"standardFlowEnabled":false,"directAccessGrantsEnabled":false,"serviceAccountsEnabled":false}
JSON
    "$KCADM" create clients -r "$REALM" -f "$file" >/dev/null
  fi
}

ensure_web_client(){
  id="$(client_uuid autoparts-web)"
  file="/tmp/autoparts-web-client.json"
  if [ -n "$id" ]; then
    cat >"$file" <<JSON
{"id":"$id","clientId":"autoparts-web","name":"AutoParts Web","enabled":true,"protocol":"openid-connect","publicClient":true,"standardFlowEnabled":true,"implicitFlowEnabled":false,"directAccessGrantsEnabled":false,"serviceAccountsEnabled":false,"fullScopeAllowed":true,"redirectUris":["$APP_PUBLIC_URL/*"],"webOrigins":["$APP_PUBLIC_URL"],"attributes":{"pkce.code.challenge.method":"S256","post.logout.redirect.uris":"$APP_PUBLIC_URL/*"}}
JSON
    "$KCADM" update "clients/$id" -r "$REALM" -f "$file" >/dev/null
  else
    cat >"$file" <<JSON
{"clientId":"autoparts-web","name":"AutoParts Web","enabled":true,"protocol":"openid-connect","publicClient":true,"standardFlowEnabled":true,"implicitFlowEnabled":false,"directAccessGrantsEnabled":false,"serviceAccountsEnabled":false,"fullScopeAllowed":true,"redirectUris":["$APP_PUBLIC_URL/*"],"webOrigins":["$APP_PUBLIC_URL"],"attributes":{"pkce.code.challenge.method":"S256","post.logout.redirect.uris":"$APP_PUBLIC_URL/*"}}
JSON
    "$KCADM" create clients -r "$REALM" -f "$file" >/dev/null
  fi
}

mapper_id(){
  cid="$1"; name="$2"
  "$KCADM" get "clients/$cid/protocol-mappers/models" -r "$REALM" --fields id,name --format csv --noquotes 2>/dev/null | while IFS=, read -r id n; do [ "$n" = "$name" ] && { printf '%s\n' "$id"; break; }; done
}

sync_mapper(){
  cid="$1"; name="$2"; file="$3"; mid="$(mapper_id "$cid" "$name")"
  if [ -n "$mid" ]; then
    update_file="/tmp/keycloak-bootstrap-mapper-$mid.json"
    { printf '{\n  "id": "%s",\n' "$mid"; sed '1d' "$file"; } > "$update_file"
    "$KCADM" update "clients/$cid/protocol-mappers/models/$mid" -r "$REALM" -f "$update_file" >/dev/null
  else
    "$KCADM" create "clients/$cid/protocol-mappers/models" -r "$REALM" -f "$file" >/dev/null
  fi
}

log "waiting for Keycloak admin API"
login_admin

if ! "$KCADM" get "realms/$REALM" >/dev/null 2>&1; then
  log "creating realm $REALM"
  "$KCADM" create realms -s "realm=$REALM" -s enabled=true -s registrationAllowed=false -s resetPasswordAllowed=true -s rememberMe=true -s loginWithEmailAllowed=true -s duplicateEmailsAllowed=false -s defaultSignatureAlgorithm=RS256 >/dev/null
fi

"$KCADM" update "realms/$REALM" -s enabled=true -s registrationAllowed=false -s duplicateEmailsAllowed=false -s defaultSignatureAlgorithm=RS256 >/dev/null
"$KCADM" update users/profile -r "$REALM" -s unmanagedAttributePolicy=ADMIN_EDIT >/dev/null

for role in owner admin cashier warehouse accountant mechanic consumer; do ensure_role "$role"; done
ensure_api_client
ensure_web_client
WEB_CLIENT_UUID="$(client_uuid autoparts-web)"
[ -n "$WEB_CLIENT_UUID" ] || { log "autoparts-web client was not created"; exit 1; }

cat >/tmp/tenant-id-mapper.json <<'JSON'
{
  "name": "tenant-id",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "consentRequired": false,
  "config": {
    "user.attribute": "tenant_id",
    "claim.name": "tenant_id",
    "jsonType.label": "String",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true",
    "introspection.token.claim": "true"
  }
}
JSON
cat >/tmp/store-id-mapper.json <<'JSON'
{
  "name": "store-id",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "consentRequired": false,
  "config": {
    "user.attribute": "store_id",
    "claim.name": "store_id",
    "jsonType.label": "String",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true",
    "introspection.token.claim": "true"
  }
}
JSON
cat >/tmp/audience-mapper.json <<'JSON'
{
  "name": "autoparts-api-audience",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-audience-mapper",
  "consentRequired": false,
  "config": {
    "included.client.audience": "autoparts-api",
    "id.token.claim": "false",
    "access.token.claim": "true",
    "introspection.token.claim": "true"
  }
}
JSON

sync_mapper "$WEB_CLIENT_UUID" tenant-id /tmp/tenant-id-mapper.json
sync_mapper "$WEB_CLIENT_UUID" store-id /tmp/store-id-mapper.json
sync_mapper "$WEB_CLIENT_UUID" autoparts-api-audience /tmp/audience-mapper.json

log "production realm/client bootstrap completed"
log "no application users were created; provision users separately with tenant_id/store_id and realm roles"
