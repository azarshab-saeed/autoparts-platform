# Phase 15.12.1 Run — Build Hotfix

## Purpose

Fix the API compile blocker discovered immediately after applying Phase 15.12.

## Root cause

`internal/pricing/service.go` still imported the standard-library package `fmt` after the final Phase 15.12 pricing refactor stopped using it. Go treats unused imports as compile errors, so `docker compose up -d --build` failed at `go build`.

## Change

- removed the unused `fmt` import from `internal/pricing/service.go`;
- ran `gofmt` on the file;
- verified `git diff --check` is clean;
- verified no other standard-library import in the Phase 15.12 worktree is syntactically unused.

## Acceptance

On a development machine with Docker/network access:

```bash
docker compose up -d --build
```

The previous error must no longer appear:

```text
internal/pricing/service.go:6:2: "fmt" imported and not used
```

This hotfix does not add a migration and does not change the Phase 15.13 product roadmap.
