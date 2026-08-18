#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
VERSION=${VERSION:-0.15.8.2}
ARCH=${ARCH:-amd64}
case "$ARCH" in
  amd64) GOARCH=amd64 ;;
  arm64) GOARCH=arm64 ;;
  *) echo "Unsupported Debian architecture: $ARCH" >&2; exit 2 ;;
esac
command -v dpkg-deb >/dev/null 2>&1 || { echo "dpkg-deb is required" >&2; exit 1; }
DIST="$ROOT/dist"
WORK="$DIST/deb-${ARCH}"
PKG="$WORK/pkg"
BIN="$WORK/bin"
rm -rf "$WORK"
mkdir -p "$PKG/DEBIAN" "$PKG/usr/lib/autoparts-store-edge" "$PKG/usr/lib/systemd/user" "$BIN" "$DIST"
chmod 0755 "$PKG" "$PKG/DEBIAN"
chmod g-s "$PKG" "$PKG/DEBIAN"

MANAGER_LDFLAGS="-s -w -X main.version=$VERSION"
if [ -n "${AUTOPARTS_EDGE_UPDATE_MANIFEST_URL:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultUpdateManifestURL=${AUTOPARTS_EDGE_UPDATE_MANIFEST_URL}"; fi
if [ -n "${AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultUpdatePublicKey=${AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY}"; fi
if [ -n "${AUTOPARTS_EDGE_ALLOWED_ORIGINS:-}" ]; then MANAGER_LDFLAGS="$MANAGER_LDFLAGS -X main.defaultAllowedOrigins=${AUTOPARTS_EDGE_ALLOWED_ORIGINS}"; fi
(
  cd "$ROOT"
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$BIN/autoparts-store-edge" ./cmd/store-edge
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -buildvcs=false -trimpath -ldflags "$MANAGER_LDFLAGS" -o "$BIN/autoparts-store-edge-manager" ./cmd/store-edge-manager
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$BIN/autoparts-store-edge-updater" ./cmd/store-edge-updater
)
install -m 0755 "$BIN/autoparts-store-edge" "$PKG/usr/lib/autoparts-store-edge/autoparts-store-edge"
install -m 0755 "$BIN/autoparts-store-edge-manager" "$PKG/usr/lib/autoparts-store-edge/autoparts-store-edge-manager"
install -m 0755 "$BIN/autoparts-store-edge-updater" "$PKG/usr/lib/autoparts-store-edge/autoparts-store-edge-updater"
install -m 0755 "$ROOT/edge/linux/package/autoparts-store-edge-launcher" "$PKG/usr/lib/autoparts-store-edge/autoparts-store-edge-launcher"
printf '%s\n' "$VERSION" > "$PKG/usr/lib/autoparts-store-edge/VERSION"
install -m 0644 "$ROOT/edge/linux/package/autoparts-store-edge-manager.service" "$PKG/usr/lib/systemd/user/autoparts-store-edge-manager.service"
install -m 0755 "$ROOT/edge/linux/package/postinst" "$PKG/DEBIAN/postinst"
install -m 0755 "$ROOT/edge/linux/package/prerm" "$PKG/DEBIAN/prerm"
cat > "$PKG/DEBIAN/control" <<CONTROL
Package: autoparts-store-agent
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: AutoParts <support@localhost>
Depends: systemd, coreutils, util-linux
Description: AutoParts Store Agent for offline sales and store hardware
 Installs the user-level Lifecycle Manager, offline Store Edge worker and signed updater.
CONTROL
OUT="$DIST/autoparts-store-agent_${VERSION}_${ARCH}.deb"
dpkg-deb --build --root-owner-group "$PKG" "$OUT" >/dev/null
STABLE="$DIST/autoparts-store-agent-linux-${ARCH}.deb"
cp "$OUT" "$STABLE"
sha256sum "$OUT" > "$OUT.sha256"
sha256sum "$STABLE" > "$STABLE.sha256"
printf 'Built %s\n' "$OUT"
printf 'Stable alias %s\n' "$STABLE"
