#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/autoparts-store-edge"
UNIT_DIR="${HOME}/.config/systemd/user"
BIN="$BIN_DIR/autoparts-store-edge"
UNIT="$UNIT_DIR/autoparts-store-edge.service"
mkdir -p "$BIN_DIR" "$DATA_DIR" "$UNIT_DIR"
(
  cd "$ROOT"
  CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=0.15.7.1" -o "$BIN" ./cmd/store-edge
)
cat > "$UNIT" <<UNIT
[Unit]
Description=AutoParts Store Edge
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=AUTOPARTS_EDGE_DATA_DIR=$DATA_DIR
ExecStart=$BIN
Restart=always
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=default.target
UNIT
systemctl --user daemon-reload
systemctl --user enable --now autoparts-store-edge.service
printf 'Store Edge installed as a user systemd service.\n'
printf 'Status: systemctl --user status autoparts-store-edge.service\n'
printf 'Local UI: http://127.0.0.1:17624/\n'
