#!/usr/bin/env bash
# Build and sign a stable update feed from dist/bin and portal assets.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE="${PANEL_UPDATE_RELEASE:-0.2.0}"
CHANNEL="${PANEL_UPDATE_CHANNEL:-stable}"
PRIV="${PANEL_UPDATE_SIGNING_KEY:-$ROOT/.run/validation/update-signing.priv}"
FEED_ROOT="${PANEL_UPDATE_FEED_ROOT:-$ROOT/dist/update-feed}"
STAGE="$FEED_ROOT/$CHANNEL/$RELEASE"

if [[ ! -f "$PRIV" ]]; then
	echo "missing signing key at $PRIV" >&2
	exit 1
fi
if [[ ! -x "$ROOT/dist/bin/panel-api" ]]; then
	( cd "$ROOT" && make build portals )
fi

rm -rf "$STAGE"
mkdir -p "$STAGE/bin" "$STAGE/share/portals/server" "$STAGE/share/portals/account"

for bin in panel-api panel-worker panel-agent panel-cli panel-updater panel-backup panel-smtp-policy panel-object-store; do
	install -m 0755 "$ROOT/dist/bin/$bin" "$STAGE/bin/$bin"
done
cp -a "$ROOT/dist/share/portals/server/." "$STAGE/share/portals/server/"
cp -a "$ROOT/dist/share/portals/account/." "$STAGE/share/portals/account/"

go run "$ROOT/scripts/build-update-feed" \
	-stage "$STAGE" \
	-release "$RELEASE" \
	-channel "$CHANNEL" \
	-priv "$PRIV" \
	-feed-root "$FEED_ROOT"

echo "feed ready under $FEED_ROOT/$CHANNEL/$RELEASE"
