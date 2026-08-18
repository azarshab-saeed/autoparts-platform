.PHONY: rc-static-check deps fmt test build run compose-up compose-rebuild compose-down web-dev web-build auth-logs migrations prod-preflight prod-up prod-down backup rc-smoke rc-db-check rc-load rc-check edge-run edge-manager-run edge-test edge-manager-test edge-build edge-manager-build edge-build-windows edge-build-windows-installer edge-linux-install

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

edge-manager-run:
	@mkdir -p bin
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=0.15.8.1" -o bin/store-edge ./cmd/store-edge
	AUTOPARTS_EDGE_WORKER_PATH=$$(pwd)/bin/store-edge go run ./cmd/store-edge-manager

edge-test:
	go test ./cmd/store-edge ./internal/storeedge

edge-manager-test:
	go test ./cmd/store-edge-manager ./cmd/store-edge-updater ./cmd/store-edge-manifest ./internal/storeedgemanager

edge-build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=0.15.8.1" -o bin/store-edge ./cmd/store-edge

edge-manager-build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=0.15.8.1" -o bin/store-edge-manager ./cmd/store-edge-manager
	CGO_ENABLED=0 go build -trimpath -o bin/store-edge-updater ./cmd/store-edge-updater

edge-build-windows:
	powershell -ExecutionPolicy Bypass -File edge/windows/build.ps1 -Version 0.15.8.1

edge-build-windows-installer:
	powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.8.1

edge-linux-install:
	./edge/linux/install-user-service.sh

edge-lifecycle-test: edge-manager-test
	@echo "Store Agent Lifecycle Manager tests passed"
