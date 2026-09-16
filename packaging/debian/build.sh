#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VER="${PANEL_DEB_VERSION:-$(bash "$ROOT/scripts/ci/resolve-release-version.sh")}"
STAGE="$ROOT/dist/deb/hosting-panel_${VER}_amd64"
rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" \
  "$STAGE/usr/local/panel/bin" \
  "$STAGE/usr/local/panel/share/portals" \
  "$STAGE/usr/local/panel/share/testdata" \
  "$STAGE/usr/local/panel/share/scripts" \
  "$STAGE/etc/systemd/system" \
  "$STAGE/usr/share/doc/hosting-panel"
cp "$ROOT/packaging/debian/control" "$STAGE/DEBIAN/control"
sed -i "s/^Version:.*/Version: $VER/" "$STAGE/DEBIAN/control"
make -C "$ROOT" build portals
while IFS=$'\t' read -r src dest mode class; do
	mkdir -p "$STAGE/$(dirname "$dest")"
	if [[ -z "$src" ]]; then
		mkdir -p "$STAGE/$dest"
		continue
	fi
	if [[ -d "$ROOT/$src" ]]; then
		mkdir -p "$STAGE/$dest"
		cp -a "$ROOT/$src/." "$STAGE/$dest/"
	else
		install -m "$mode" "$ROOT/$src" "$STAGE/$dest"
	fi
	_="$class"
done < <(go run "$ROOT/scripts/release-inventory" -format package-copy)
cp "$ROOT/packaging/debian/postinst" "$STAGE/DEBIAN/postinst"
chmod 0755 "$STAGE/DEBIAN/postinst"
chmod 0755 "$STAGE/DEBIAN" "$STAGE/usr/local/panel/bin/"*
dpkg-deb --build "$STAGE" "$ROOT/dist/deb/hosting-panel_${VER}_amd64.deb"
echo "wrote $ROOT/dist/deb/hosting-panel_${VER}_amd64.deb"
