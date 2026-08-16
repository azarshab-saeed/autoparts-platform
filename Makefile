.PHONY: deps fmt test build run compose-up compose-rebuild compose-down web-dev web-build auth-logs migrations

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
