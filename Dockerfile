FROM golang:1.23-alpine AS build
WORKDIR /src

# Network-resilient Go module settings.
# - The pipe separator makes GOPROXY fall back on ANY error, including timeouts.
# - sum.golang.google.cn is an official alias for sum.golang.org and keeps
#   checksum verification enabled.
ARG GOPROXY="https://proxy.golang.org|https://goproxy.cn|direct"
ARG GOSUMDB="sum.golang.google.cn"
ARG VERSION="dev"
ARG COMMIT="unknown"
ARG BUILD_TIME="unknown"
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB} \
    GOTOOLCHAIN=local

COPY . .

# Resolve the graph once, with retry/backoff for flaky TLS/network paths.
# Avoid a separate early `go mod download` so the build does not perform
# duplicate network resolution before go.sum has been completed.
RUN set -eux; \
    attempt=1; \
    max_attempts=5; \
    until go mod tidy; do \
      if [ "$attempt" -ge "$max_attempts" ]; then \
        echo "go mod tidy failed after ${max_attempts} attempts" >&2; \
        exit 1; \
      fi; \
      sleep_seconds=$((attempt * 4)); \
      echo "Go module download failed; retrying in ${sleep_seconds}s..." >&2; \
      sleep "$sleep_seconds"; \
      attempt=$((attempt + 1)); \
    done; \
    go mod verify

# The build must not mutate go.mod/go.sum after the dependency step.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" -o /out/api ./cmd/api

FROM alpine:3.22
RUN adduser -D -H -u 10001 app
USER app
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
