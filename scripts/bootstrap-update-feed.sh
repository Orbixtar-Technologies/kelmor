#!/usr/bin/env bash
# Build and sign a stable update feed from the runtime-asset inventory.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE="${PANEL_UPDATE_RELEASE:-$(bash "$ROOT/scripts/ci/resolve-release-version.sh")}"
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
mkdir -p "$STAGE"

while IFS=$'\t' read -r src dest mode kind; do
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
	_="$kind"
done < <(go run "$ROOT/scripts/release-inventory" -class feed -format feed-copy)

go run "$ROOT/scripts/build-update-feed" \
	-stage "$STAGE" \
	-release "$RELEASE" \
	-channel "$CHANNEL" \
	-priv "$PRIV" \
	-feed-root "$FEED_ROOT"

echo "feed ready under $FEED_ROOT/$CHANNEL/$RELEASE"
