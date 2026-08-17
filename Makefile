.PHONY: rc-static-check deps fmt test build run compose-up compose-rebuild compose-down web-dev web-build auth-logs migrations prod-preflight prod-up prod-down backup rc-smoke rc-db-check rc-load rc-check edge-run edge-test edge-build edge-build-windows

deps:
	go mod tidy
	go mod download all
	go mod verify

fmt:
	gofmt -w cmd internal tests

test:
	go test ./...

build: deps
	CGO_ENABLED=0 go build -mod=readonly -trimpath -o bin/api ./cmd/api

run:
	go run ./cmd/api

compose-up:
	docker compose up --build

compose-rebuild:
	docker compose build --no-cache api
	docker compose up -d

compose-down:
	docker compose down

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

auth-logs:
	docker compose logs -f keycloak

migrations:
	docker compose run --rm db-prepare

prod-preflight:
	ENV_FILE=.env.production ./ops/preflight.sh

prod-up:
	docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build

prod-down:
	docker compose --env-file .env.production -f docker-compose.prod.yml down

backup:
	COMPOSE_FILE=docker-compose.prod.yml ./ops/backup.sh

rc-smoke:
	go run ./cmd/rc-smoke

rc-static-check:
	./ops/rc-static-check.sh

rc-db-check:
	./ops/rc-db-check.sh

rc-load:
	go run ./cmd/rc-load

rc-check:
	./ops/rc-check.sh

edge-run:
	go run ./cmd/store-edge

edge-test:
	go test ./internal/storeedge

edge-build:
	CGO_ENABLED=0 go build -trimpath -o bin/store-edge ./cmd/store-edge

edge-build-windows:
	powershell -ExecutionPolicy Bypass -File edge/windows/build.ps1
