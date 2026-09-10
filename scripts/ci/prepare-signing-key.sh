#!/usr/bin/env bash
# Normalize and validate PANEL_UPDATE_SIGNING_KEY before release builds.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="${1:-${PANEL_UPDATE_SIGNING_KEY:-}}"
SIGNING_KEY="${SIGNING_KEY:-}"

if [[ -z "$OUT" ]]; then
	echo "usage: prepare-signing-key.sh <output-path>" >&2
	exit 1
fi
if [[ -z "$SIGNING_KEY" ]]; then
	echo "SIGNING_KEY is required (GitHub secret PANEL_UPDATE_SIGNING_KEY)." >&2
	exit 1
fi

install -d -m 700 "$(dirname "$OUT")"
go run "$ROOT/scripts/validate-signing-key" \
	-secret "$SIGNING_KEY" \
	-public "$ROOT/installer/phases/release.pub" \
	-output "$OUT"
chmod 600 "$OUT"
echo "Signing key validated against installer/phases/release.pub"
