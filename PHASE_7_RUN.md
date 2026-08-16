# Phase 7 runbook

## Upgrade existing Phase 6 volume

Do **not** delete the PostgreSQL volume.

```bash
cp .env.example .env
docker compose up --build
```

The `db-prepare` service applies `005_network_search.sql` and then refreshes idempotent demo data.

Check migrations:

```bash
docker compose exec postgres psql -U autoparts -d autoparts -c "TABLE schema_migrations;"
```

You should see `005_network_search.sql`.

## Test mechanic search

Open:

```text
http://localhost:3000/mechanic
```

Search:

```text
لنت 206
```

The demo should return offers from three stores in separate tenants. Try `ارزان‌ترین` and enable browser location to test distance sorting.

The mechanic search itself is public and does not require login.

## Store network management

Login as the existing store owner:

```text
owner@example.com
ChangeMe123!
```

Open:

```text
http://localhost:3000/store/network
```

Change a public selling price, toggle visibility, save it, then refresh the mechanic search page.

## Mechanic Keycloak user

Fresh Keycloak realm imports include:

```text
mechanic@example.com
ChangeMe123!
```

If you upgraded an **existing** Keycloak database, `--import-realm` intentionally does not overwrite an existing realm, so the new demo user may not exist. This does not block public network search. For a dev-only test, create `mechanic@example.com` in Keycloak Admin and assign the `mechanic` realm role; do not give it tenant/store attributes.

## Direct API test

```bash
curl 'http://localhost:8080/v1/network/search?q=%D9%84%D9%86%D8%AA%20206&sort=price&limit=20'
```

With coordinates:

```bash
curl 'http://localhost:8080/v1/network/search?q=%D9%84%D9%86%D8%AA%20206&sort=distance&lat=35.7007&lng=51.3716'
```
