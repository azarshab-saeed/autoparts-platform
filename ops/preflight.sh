#!/bin/sh
set -eu

ENV_FILE="${ENV_FILE:-.env.production}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE (copy .env.production.example first)" >&2; exit 2; }

get_env() {
  name="$1"
  awk -v wanted="$name" '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      line=$0
      sub(/^[[:space:]]*/, "", line)
      pos=index(line, "=")
      if (pos==0) next
      key=substr(line,1,pos-1)
      gsub(/[[:space:]]+$/, "", key)
      if (key!=wanted) next
      value=substr(line,pos+1)
      sub(/^[[:space:]]*/, "", value)
      sub(/[[:space:]]*$/, "", value)
      if ((substr(value,1,1)=="\"" && substr(value,length(value),1)=="\"") || (substr(value,1,1)=="\047" && substr(value,length(value),1)=="\047")) {
        value=substr(value,2,length(value)-2)
      }
      print value
      exit
    }
  ' "$ENV_FILE"
}

required='POSTGRES_PASSWORD DATABASE_URL KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD KEYCLOAK_HOSTNAME KEYCLOAK_ISSUER APP_PUBLIC_URL NEXT_PUBLIC_API_URL NEXT_PUBLIC_KEYCLOAK_URL ALLOWED_ORIGINS'
for name in $required; do
  value="$(get_env "$name")"
  [ -n "$value" ] || { echo "missing required value: $name" >&2; exit 2; }
  case "$value" in
    *CHANGE_TO*) echo "placeholder value detected for $name" >&2; exit 2;;
  esac
done

POSTGRES_PASSWORD="$(get_env POSTGRES_PASSWORD)"
KEYCLOAK_DB_PASSWORD="$(get_env KEYCLOAK_DB_PASSWORD)"
KEYCLOAK_ADMIN_PASSWORD="$(get_env KEYCLOAK_ADMIN_PASSWORD)"
KEYCLOAK_HOSTNAME="$(get_env KEYCLOAK_HOSTNAME)"
KEYCLOAK_ISSUER="$(get_env KEYCLOAK_ISSUER)"
APP_PUBLIC_URL="$(get_env APP_PUBLIC_URL)"
NEXT_PUBLIC_API_URL="$(get_env NEXT_PUBLIC_API_URL)"
NEXT_PUBLIC_KEYCLOAK_URL="$(get_env NEXT_PUBLIC_KEYCLOAK_URL)"
ALLOWED_ORIGINS="$(get_env ALLOWED_ORIGINS)"

for value in "$POSTGRES_PASSWORD" "$KEYCLOAK_DB_PASSWORD" "$KEYCLOAK_ADMIN_PASSWORD"; do
  case "$value" in
    autoparts|keycloak|admin-change-me|ChangeMe123!)
      echo "development password detected; refusing production preflight" >&2
      exit 2
      ;;
  esac
  [ "${#value}" -ge 20 ] || { echo "production passwords must be at least 20 characters" >&2; exit 2; }
done

case "$KEYCLOAK_HOSTNAME" in https://*) ;; *) echo "KEYCLOAK_HOSTNAME must use https" >&2; exit 2;; esac
case "$KEYCLOAK_ISSUER" in https://*) ;; *) echo "KEYCLOAK_ISSUER must use https" >&2; exit 2;; esac
case "$APP_PUBLIC_URL" in https://*) ;; *) echo "APP_PUBLIC_URL must use https" >&2; exit 2;; esac
case "$NEXT_PUBLIC_API_URL" in https://*) ;; *) echo "NEXT_PUBLIC_API_URL must use https" >&2; exit 2;; esac
case "$NEXT_PUBLIC_KEYCLOAK_URL" in https://*) ;; *) echo "NEXT_PUBLIC_KEYCLOAK_URL must use https" >&2; exit 2;; esac
case "$ALLOWED_ORIGINS" in *\**) echo "ALLOWED_ORIGINS must not contain wildcard origins" >&2; exit 2;; esac

EXPECTED_ISSUER="${KEYCLOAK_HOSTNAME%/}/realms/autoparts"
[ "$KEYCLOAK_ISSUER" = "$EXPECTED_ISSUER" ] || { echo "KEYCLOAK_ISSUER must equal $EXPECTED_ISSUER" >&2; exit 2; }
[ "${NEXT_PUBLIC_KEYCLOAK_URL%/}" = "${KEYCLOAK_HOSTNAME%/}" ] || { echo "NEXT_PUBLIC_KEYCLOAK_URL must match KEYCLOAK_HOSTNAME" >&2; exit 2; }
ORIGINS_COMPACT="$(printf '%s' "$ALLOWED_ORIGINS" | tr -d '[:space:]')"
case ",$ORIGINS_COMPACT," in
  *",$APP_PUBLIC_URL,"*) ;;
  *) echo "ALLOWED_ORIGINS must include APP_PUBLIC_URL" >&2; exit 2;;
esac

command -v docker >/dev/null 2>&1 || { echo "docker is required for production preflight" >&2; exit 2; }
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config >/dev/null
echo "production preflight passed"
