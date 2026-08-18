#!/bin/sh
set -eu
REMOVE_DATA=false
[ "${1:-}" = "--remove-data" ] && REMOVE_DATA=true
systemctl --user disable --now autoparts-store-edge-manager.service 2>/dev/null || true
systemctl --user disable --now autoparts-store-edge.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/autoparts-store-edge-manager.service" "$HOME/.config/systemd/user/autoparts-store-edge.service"
systemctl --user daemon-reload 2>/dev/null || true
rm -f "$HOME/.local/bin/autoparts-store-edge-manager" "$HOME/.local/bin/autoparts-store-edge" "$HOME/.local/bin/autoparts-store-edge-updater"
rm -rf "$HOME/.config/autoparts-store-edge"
if [ "$REMOVE_DATA" = true ]; then
  rm -rf "$HOME/.local/share/autoparts-store-edge" "$HOME/.autoparts-store-edge"
  echo "Store Agent Manager, worker and local data removed."
else
  echo "Store Agent Manager and worker removed; local offline data was preserved."
fi
