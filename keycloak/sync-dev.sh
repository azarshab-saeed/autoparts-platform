#!/bin/sh
set -eu

KCADM="/opt/keycloak/bin/kcadm.sh"
SERVER="${KEYCLOAK_SYNC_SERVER:-http://keycloak:8080}"
REALM="${KEYCLOAK_REALM:-autoparts}"
ADMIN_USER="${KEYCLOAK_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin-change-me}"
DEV_PASSWORD="${KEYCLOAK_DEV_PASSWORD:-ChangeMe123!}"

# Keep Admin CLI tokens in a writable ephemeral directory inside the sync container.
export HOME="${KEYCLOAK_SYNC_HOME:-/tmp/keycloak-sync-home}"
mkdir -p "$HOME"

log() {
  printf '%s\n' "[keycloak-sync] $*"
}

login_admin() {
  attempts=0
  while [ "$attempts" -lt 60 ]; do
    if "$KCADM" config credentials \
      --server "$SERVER" \
      --realm master \
      --user "$ADMIN_USER" \
      --password "$ADMIN_PASSWORD" >/tmp/keycloak-sync-login.log 2>&1; then
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 2
  done
  log "could not authenticate to Keycloak after 120 seconds"
  cat /tmp/keycloak-sync-login.log >&2 || true
  return 1
}

ensure_role() {
  role="$1"
  if "$KCADM" get "roles/$role" -r "$REALM" >/dev/null 2>&1; then
    return 0
  fi
  log "creating realm role: $role"
  "$KCADM" create roles -r "$REALM" -s "name=$role" >/dev/null
}

user_id() {
  username="$1"
  "$KCADM" get users -r "$REALM" -q "username=$username" -q exact=true \
    --fields id --format csv --noquotes 2>/dev/null | head -n 1
}

ensure_user() {
  username="$1"
  email="$2"
  first_name="$3"
  last_name="$4"
  role="$5"
  tenant_id="${6:-}"
  store_id="${7:-}"

  uid="$(user_id "$username")"
  if [ -z "$uid" ]; then
    log "creating dev user: $username"
    "$KCADM" create users -r "$REALM" \
      -s "username=$username" \
      -s "email=$email" \
      -s emailVerified=true \
      -s enabled=true \
      -s "firstName=$first_name" \
      -s "lastName=$last_name" >/dev/null
    uid="$(user_id "$username")"
  fi

  if [ -z "$uid" ]; then
    log "failed to resolve user id for $username"
    return 1
  fi

  if [ -n "$tenant_id" ] && [ -n "$store_id" ]; then
    "$KCADM" update "users/$uid" -r "$REALM" \
      -s enabled=true \
      -s emailVerified=true \
      -s "email=$email" \
      -s "firstName=$first_name" \
      -s "lastName=$last_name" \
      -s "attributes={\"tenant_id\":[\"$tenant_id\"],\"store_id\":[\"$store_id\"]}" >/dev/null
  else
    "$KCADM" update "users/$uid" -r "$REALM" \
      -s enabled=true \
      -s emailVerified=true \
      -s "email=$email" \
      -s "firstName=$first_name" \
      -s "lastName=$last_name" >/dev/null
  fi

  "$KCADM" set-password -r "$REALM" --username "$username" --new-password "$DEV_PASSWORD" >/dev/null

  # Keep the test identity deterministic. Best-effort removal of other app
  # roles is fine, but assigning the expected role must never be silent.
  for app_role in owner admin cashier warehouse accountant mechanic consumer; do
    if [ "$app_role" != "$role" ]; then
      "$KCADM" remove-roles -r "$REALM" --uusername "$username" --rolename "$app_role" >/dev/null 2>&1 || true
    fi
  done

  # Use the generic Admin REST endpoint instead of relying on the add-roles
  # convenience command. This is explicit and stable across Keycloak 26.x.
  role_file="/tmp/keycloak-sync-role-$role.json"
  role_array_file="/tmp/keycloak-sync-role-$role-array.json"
  "$KCADM" get "roles/$role" -r "$REALM" >"$role_file"
  {
    printf '[\n'
    cat "$role_file"
    printf '\n]\n'
  } >"$role_array_file"
  "$KCADM" create "users/$uid/role-mappings/realm" -r "$REALM" -f "$role_array_file" >/dev/null

  # A green sync must mean the identity is really usable by the Go verifier.
  if ! "$KCADM" get "users/$uid/role-mappings/realm" -r "$REALM" \
      --fields name --format csv --noquotes 2>/dev/null | grep -Fxq "$role"; then
    log "realm role verification failed for $username: expected $role"
    return 1
  fi

  profile_file="/tmp/keycloak-sync-user-$uid.json"
  "$KCADM" get "users/$uid" -r "$REALM" >"$profile_file"
  if [ -n "$tenant_id" ] && [ -n "$store_id" ]; then
    if ! grep -Fq "\"$tenant_id\"" "$profile_file"; then
      log "tenant_id verification failed for $username"
      return 1
    fi
    if ! grep -Fq "\"$store_id\"" "$profile_file"; then
      log "store_id verification failed for $username"
      return 1
    fi
  fi
  log "verified dev identity: $username role=$role"
}

client_uuid() {
  client_id="$1"
  "$KCADM" get clients -r "$REALM" -q "clientId=$client_id" \
    --fields id --format csv --noquotes 2>/dev/null | head -n 1
}

mapper_id() {
  client_uuid_value="$1"
  mapper_name="$2"
  "$KCADM" get "clients/$client_uuid_value/protocol-mappers/models" -r "$REALM" \
    --fields id,name --format csv --noquotes 2>/dev/null | \
    while IFS=, read -r id name; do
      if [ "$name" = "$mapper_name" ]; then
        printf '%s\n' "$id"
        break
      fi
    done
}

sync_mapper() {
  client_uuid_value="$1"
  mapper_name="$2"
  json_file="$3"
  mid="$(mapper_id "$client_uuid_value" "$mapper_name")"
  if [ -n "$mid" ]; then
    # Keycloak 26.x requires ProtocolMapperRepresentation.id in the update body
    # as well as in the request path. The import JSON intentionally has no id,
    # so build a temporary update representation that includes the existing id.
    update_file="/tmp/keycloak-sync-mapper-$mid.json"
    {
      printf '{\n  "id": "%s",\n' "$mid"
      sed '1d' "$json_file"
    } >"$update_file"
    "$KCADM" update "clients/$client_uuid_value/protocol-mappers/models/$mid" -r "$REALM" -f "$update_file" >/dev/null
  else
    log "creating protocol mapper: $mapper_name"
    "$KCADM" create "clients/$client_uuid_value/protocol-mappers/models" -r "$REALM" -f "$json_file" >/dev/null
  fi
}

log "waiting for Keycloak admin API"
login_admin

if ! "$KCADM" get "realms/$REALM" >/dev/null 2>&1; then
  log "realm '$REALM' does not exist; realm import must complete first"
  exit 1
fi

for role in owner admin cashier warehouse accountant mechanic consumer; do
  ensure_role "$role"
done

WEB_CLIENT_UUID="$(client_uuid autoparts-web)"
API_CLIENT_UUID="$(client_uuid autoparts-api)"
if [ -z "$WEB_CLIENT_UUID" ] || [ -z "$API_CLIENT_UUID" ]; then
  log "required clients autoparts-web/autoparts-api are missing"
  exit 1
fi

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

cat >/tmp/autoparts-api-audience-mapper.json <<'JSON'
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
sync_mapper "$WEB_CLIENT_UUID" autoparts-api-audience /tmp/autoparts-api-audience-mapper.json

ensure_user owner@example.com owner@example.com "رضا" "رضایی" owner \
  11111111-1111-1111-1111-111111111111 22222222-2222-2222-2222-222222222222
ensure_user central@example.com central@example.com "علی" "مرکزی" owner \
  11111111-1111-1111-1111-111111111112 22222222-2222-2222-2222-222222222223
ensure_user pars@example.com pars@example.com "سارا" "پارس" owner \
  11111111-1111-1111-1111-111111111113 22222222-2222-2222-2222-222222222224
ensure_user mechanic@example.com mechanic@example.com "مهدی" "مکانیک" mechanic
ensure_user consumer@example.com consumer@example.com "کاربر" "مصرف‌کننده" consumer

log "Keycloak dev sync completed"
log "store owners: owner@example.com, central@example.com, pars@example.com"
log "external users: mechanic@example.com, consumer@example.com"
