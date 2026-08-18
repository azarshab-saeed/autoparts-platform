#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/autoparts-store-edge"
LEGACY_DATA="${HOME}/.autoparts-store-edge"
UNIT_DIR="${HOME}/.config/systemd/user"
CONF_DIR="${HOME}/.config/autoparts-store-edge"
ENV_FILE="$CONF_DIR/manager.env"
MANAGER_BIN="$BIN_DIR/autoparts-store-edge-manager"
WORKER_BIN="$BIN_DIR/autoparts-store-edge"
UPDATER_BIN="$BIN_DIR/autoparts-store-edge-updater"
MANAGER_UNIT="$UNIT_DIR/autoparts-store-edge-manager.service"
LEGACY_UNIT="$UNIT_DIR/autoparts-store-edge.service"
VERSION="0.15.8.2"
mkdir -p "$BIN_DIR" "$DATA_DIR" "$UNIT_DIR" "$CONF_DIR"
chmod 700 "$CONF_DIR"

if [ -d "$LEGACY_DATA" ] && [ -z "$(find "$DATA_DIR" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]; then
  cp -a "$LEGACY_DATA/." "$DATA_DIR/"
  printf 'Migrated existing Store Edge local data from %s\n' "$LEGACY_DATA"
fi

(
  cd "$ROOT"
  CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$WORKER_BIN" ./cmd/store-edge
  MANAGER_LDFLAGS="-s -w -X main.version=$VERSION"
  if [ -n "${AUTOPARTS_EDGE_UPDATE_MANIFEST_URL:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultUpdateManifestURL=${AUTOPARTS_EDGE_UPDATE_MANIFEST_URL}"; fi
  if [ -n "${AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultUpdatePublicKey=${AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY}"; fi
  if [ -n "${AUTOPARTS_EDGE_ALLOWED_ORIGINS:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultAllowedOrigins=${AUTOPARTS_EDGE_ALLOWED_ORIGINS}"; fi
  CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "$MANAGER_LDFLAGS" -o "$MANAGER_BIN" ./cmd/store-edge-manager
  CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$UPDATER_BIN" ./cmd/store-edge-updater
)
chmod 755 "$MANAGER_BIN" "$WORKER_BIN" "$UPDATER_BIN"

cat > "$ENV_FILE" <<ENV
AUTOPARTS_EDGE_DATA_DIR=$DATA_DIR
AUTOPARTS_EDGE_WORKER_PATH=$WORKER_BIN
AUTOPARTS_EDGE_UPDATER_PATH=$UPDATER_BIN
AUTOPARTS_EDGE_MANAGER_SERVICE=systemd-user
ENV
if [ -n "${AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT:-}" ]; then printf 'AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT=%s\n' "$AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT" >> "$ENV_FILE"; fi
chmod 600 "$ENV_FILE"

systemctl --user disable --now autoparts-store-edge.service 2>/dev/null || true
rm -f "$LEGACY_UNIT"
cat > "$MANAGER_UNIT" <<UNIT
[Unit]
Description=AutoParts Store Agent Lifecycle Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=$MANAGER_BIN
Restart=always
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=default.target
UNIT
systemctl --user daemon-reload
systemctl --user enable --now autoparts-store-edge-manager.service
printf 'Store Agent Lifecycle Manager installed as a user systemd service.\n'
printf 'Manager: http://127.0.0.1:17623/healthz\n'
printf 'Store Agent: http://127.0.0.1:17624/healthz\n'
printf 'From now on Start/Stop/Restart is controlled from the web Store Edge page.\n'
