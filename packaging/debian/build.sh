#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VER="${PANEL_DEB_VERSION:-0.1.0}"
STAGE="$ROOT/dist/deb/hosting-panel_${VER}_amd64"
rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" \
  "$STAGE/usr/local/panel/bin" \
  "$STAGE/usr/local/panel/share/portals" \
  "$STAGE/etc/systemd/system" \
  "$STAGE/usr/share/doc/hosting-panel"
cp "$ROOT/packaging/debian/control" "$STAGE/DEBIAN/control"
sed -i "s/^Version:.*/Version: $VER/" "$STAGE/DEBIAN/control"
make -C "$ROOT" build portals
cp "$ROOT/dist/bin/"panel-{api,worker,agent,cli,updater,backup,install,smtp-policy,object-store} "$STAGE/usr/local/panel/bin/"
cp -a "$ROOT/dist/share/portals/." "$STAGE/usr/local/panel/share/portals/"
cp "$ROOT/installer/phases/units/"*.service "$STAGE/etc/systemd/system/"
cp "$ROOT/packaging/debian/postinst" "$STAGE/DEBIAN/postinst"
chmod 0755 "$STAGE/DEBIAN/postinst"
cp "$ROOT/README.md" "$STAGE/usr/share/doc/hosting-panel/"
chmod 0755 "$STAGE/DEBIAN" "$STAGE/usr/local/panel/bin/"*
dpkg-deb --build "$STAGE" "$ROOT/dist/deb/hosting-panel_${VER}_amd64.deb"
echo "wrote $ROOT/dist/deb/hosting-panel_${VER}_amd64.deb"
