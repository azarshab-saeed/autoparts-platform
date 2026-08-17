#!/bin/sh
set -eu
REMOVE_DATA=false
[ "${1:-}" = "--remove-data" ] && REMOVE_DATA=true
systemctl --user disable --now autoparts-store-edge.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/autoparts-store-edge.service"
systemctl --user daemon-reload 2>/dev/null || true
rm -f "$HOME/.local/bin/autoparts-store-edge"
if [ "$REMOVE_DATA" = true ]; then
  rm -rf "$HOME/.local/share/autoparts-store-edge"
  echo "Store Edge and local data removed."
else
  echo "Store Edge removed; local data preserved under ~/.local/share/autoparts-store-edge."
fi
