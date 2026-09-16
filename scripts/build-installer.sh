#!/usr/bin/env bash
# Build a self-contained Kelmor installer tarball for a new Ubuntu 24.04 host.
# Never publishes to a VM or update feed.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
ROOT="${PANEL_INSTALLER_ROOT:-$REPO}"
VER="${PANEL_INSTALLER_VERSION:-$(bash "$REPO/scripts/ci/resolve-release-version.sh")}"
ARCH="$(uname -m)"
case "$ARCH" in
x86_64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
esac
NAME="kelmor-installer_${VER}_linux_${ARCH}"
STAGE="${PANEL_INSTALLER_STAGE:-$ROOT/dist/installer/$NAME}"
OUT="${PANEL_INSTALLER_OUT:-$ROOT/dist/installer/${NAME}.tar.gz}"

rm -rf "$STAGE"
mkdir -p "$STAGE/bin" "$STAGE/share"

while IFS=$'\t' read -r src dest mode class; do
	if [[ -z "$src" ]]; then
		mkdir -p "$STAGE/$(basename "$dest")"
		continue
	fi
	src_path="$ROOT/$src"
	if [[ ! -e "$src_path" ]]; then
		continue
	fi
	# Map package paths into the portable installer tree.
	rel="$dest"
	rel="${rel#usr/local/panel/}"
	rel="${rel#etc/systemd/system/}"
	if [[ "$dest" == etc/systemd/system/* ]]; then
		rel="share/systemd/$(basename "$dest")"
	fi
	target="$STAGE/$rel"
	mkdir -p "$(dirname "$target")"
	if [[ -d "$src_path" ]]; then
		mkdir -p "$target"
		cp -a "$src_path/." "$target/"
	else
		install -m "$mode" "$src_path" "$target"
	fi
	_="$class"
done < <(go run "$REPO/scripts/release-inventory" -format package-copy)

if [[ ! -x "$STAGE/bin/panel-install" ]]; then
	echo "build-installer: panel-install missing (run make build first)" >&2
	exit 1
fi
if [[ ! -f "$STAGE/share/portals/server/index.html" || ! -f "$STAGE/share/portals/account/index.html" ]]; then
	echo "build-installer: built portals missing (run make portals first)" >&2
	exit 1
fi

install -m 0755 "$REPO/installer/bundle/install.sh" "$STAGE/install.sh"
install -m 0644 "$REPO/installer/bundle/install.yaml.example" "$STAGE/install.yaml.example"
if [[ -f "$REPO/installer/bundle/README.md" ]]; then
	install -m 0644 "$REPO/installer/bundle/README.md" "$STAGE/README.md"
fi

mkdir -p "$(dirname "$OUT")"
tar -C "$(dirname "$STAGE")" -czf "$OUT" "$(basename "$STAGE")"
(cd "$(dirname "$OUT")" && sha256sum "$(basename "$OUT")" >"$(basename "$OUT").sha256")

INSTALLER_DIR="$(dirname "$OUT")"
STABLE="$INSTALLER_DIR/kelmor-installer-linux-${ARCH}.tar.gz"
cp -f "$OUT" "$STABLE"
(cd "$INSTALLER_DIR" && sha256sum "$(basename "$STABLE")" >"$(basename "$STABLE").sha256")
install -m 0755 "$REPO/scripts/get-kelmor.sh" "$INSTALLER_DIR/get-kelmor.sh"
echo "wrote $OUT"
echo "wrote $STABLE"
echo "wrote $INSTALLER_DIR/get-kelmor.sh"
