#!/usr/bin/env bash
# Build binaries, deb package, and signed stable update feed.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

export PANEL_UPDATE_RELEASE="${PANEL_UPDATE_RELEASE:-$(bash "$ROOT/scripts/ci/resolve-release-version.sh")}"
export PANEL_DEB_VERSION="${PANEL_DEB_VERSION:-$PANEL_UPDATE_RELEASE}"
export PANEL_UPDATE_CHANNEL="${PANEL_UPDATE_CHANNEL:-stable}"
export PANEL_UPDATE_FEED_ROOT="${PANEL_UPDATE_FEED_ROOT:-$ROOT/dist/update-feed}"

if [[ -z "${PANEL_UPDATE_SIGNING_KEY:-}" ]]; then
	default_key="$ROOT/.run/validation/update-signing.priv"
	if [[ -f "$default_key" ]]; then
		export PANEL_UPDATE_SIGNING_KEY="$default_key"
	else
		echo "set PANEL_UPDATE_SIGNING_KEY or place key at $default_key" >&2
		exit 1
	fi
fi

echo "Building Kelmor release $PANEL_UPDATE_RELEASE (channel $PANEL_UPDATE_CHANNEL)"
make lint test build portals package
bash "$ROOT/scripts/bootstrap-update-feed.sh"

"$ROOT/dist/bin/panel-updater" verify \
	"$PANEL_UPDATE_FEED_ROOT/$PANEL_UPDATE_CHANNEL/manifest.json" \
	"$ROOT/installer/phases/release.pub"

mkdir -p "$ROOT/dist/release"
cat > "$ROOT/dist/release/release.env" <<EOF
PANEL_UPDATE_RELEASE=$PANEL_UPDATE_RELEASE
PANEL_DEB_VERSION=$PANEL_DEB_VERSION
PANEL_UPDATE_CHANNEL=$PANEL_UPDATE_CHANNEL
PANEL_UPDATE_FEED_ROOT=$PANEL_UPDATE_FEED_ROOT
EOF

echo "Release $PANEL_UPDATE_RELEASE ready:"
echo "  deb: $(ls -1 "$ROOT"/dist/deb/hosting-panel_"${PANEL_DEB_VERSION}"_amd64.deb)"
echo "  feed: $PANEL_UPDATE_FEED_ROOT/$PANEL_UPDATE_CHANNEL/manifest.json"
