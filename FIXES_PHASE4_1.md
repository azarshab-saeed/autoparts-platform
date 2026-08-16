# Phase 4.1 build fixes

## Fixed: missing go.sum entries during Docker build

The API image now performs the following sequence:

1. `go mod download` to warm the direct dependency cache.
2. Copy application source.
3. `go mod tidy` to add missing module requirements and `go.sum` checksums.
4. `go mod download all` and `go mod verify`.
5. Compile with `go build -mod=readonly` so the final build cannot silently mutate the module graph.

This specifically fixes errors such as:

- missing go.sum entry for `github.com/google/uuid`
- missing go.sum entry for `github.com/jackc/pgx/v5`
- missing go.sum entry for `github.com/jackc/pgx/v5/pgxpool`

For normal source-control workflow, also run `go mod tidy` once on a networked development machine and commit the generated `go.sum`. The Dockerfile remains defensive so a clean generated checkout still builds.

## PostgreSQL

Docker Compose now pins PostgreSQL to `postgres:18.4`.

PostgreSQL 18+ official images use `/var/lib/postgresql` as the volume target and a version-specific `PGDATA` below it, so the named volume has been updated from the pre-18 `/var/lib/postgresql/data` mount.

## Rebuild

If the old build cache contains the broken module layer:

```bash
docker compose build --no-cache api
docker compose up -d
```

For a normal rebuild after that:

```bash
docker compose up --build
```

### Existing PostgreSQL 16 local volume

Do not point an existing PostgreSQL 16 data volume directly at PostgreSQL 18.4. For development data that can be discarded, remove the old named volume and recreate it. For data that must be preserved, perform a real PostgreSQL major-version migration/backup-restore.
