# Phase 4.2 - Go module TLS timeout fix

## Symptom

Docker build fails during Go dependency verification, for example:

```text
Get "https://sum.golang.org/lookup/...": net/http: TLS handshake timeout
```

This is a network path / TLS timeout to the public Go checksum service, not a compile error in the application.

## Changes

1. The Docker build no longer runs an early standalone `go mod download`.
   Dependency resolution is performed once with `go mod tidy`, which also completes `go.sum` inside the build stage.
2. Dependency resolution retries up to 5 times with incremental backoff.
3. `GOPROXY` defaults to:

```text
https://proxy.golang.org|https://goproxy.cn|direct
```

The pipe (`|`) makes Go fall back after any error, including a timeout.
4. `GOSUMDB` defaults to:

```text
sum.golang.google.cn
```

This is an official alias for `sum.golang.org` and retains checksum verification.
5. Both values are configurable through `.env` / Docker build args.
6. `GOSUMDB=off` is deliberately NOT the default because it disables the public checksum security guarantee for unknown modules.

## Rebuild

```bash
docker compose build --no-cache api
docker compose up -d
```

## If your network still cannot reach the primary module proxy

Create `.env` beside `docker-compose.yml`:

```env
GO_MODULE_PROXY=https://goproxy.cn|https://proxy.golang.org|direct
GO_SUMDB=sum.golang.google.cn
```

Then rebuild:

```bash
docker compose build --no-cache api
```

## Development-only emergency fallback

Only if every checksum endpoint is inaccessible and you understand the tradeoff, you can temporarily build with:

```bash
GO_SUMDB=off docker compose build --no-cache api
```

Do not use this as the normal CI/production configuration.
