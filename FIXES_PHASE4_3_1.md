# Phase 4.3.1 - Keycloak Registry Fix

## Problem
Docker could not pull `quay.io/keycloak/keycloak:26.7.0` because the connection to `quay.io` timed out during the TLS handshake.

## Fix
The Keycloak service now uses the Keycloak Docker Hub repository by default:

```text
keycloak/keycloak:26.7.0
```

The image is configurable through `KEYCLOAK_IMAGE`, so the Compose file does not need to be edited if a different registry is required later.

## Run

```bash
cp .env.example .env
docker compose pull keycloak
docker compose up --build
```

If an old failed pull is cached, run:

```bash
docker image rm quay.io/keycloak/keycloak:26.7.0 2>/dev/null || true
docker compose pull keycloak
docker compose up --build
```

## Override
To switch back to Quay later:

```env
KEYCLOAK_IMAGE=quay.io/keycloak/keycloak:26.7.0
```
