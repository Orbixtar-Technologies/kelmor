#!/usr/bin/env bash
# Generate a release signing keypair and optionally install release.pub.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PRIV_OUT="${1:-$ROOT/.run/validation/update-signing.priv}"
INSTALL_PUB=false
if [[ "${2:-}" == "--install-pub" ]]; then
	INSTALL_PUB=true
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
BUNDLE="$TMP/bundle"
mkdir -p "$BUNDLE"
echo sample >"$BUNDLE/sample.txt"

go run "$ROOT/cmd/panel-updater" sign "$BUNDLE" 0.0.0 stable >/dev/null
install -d -m 700 "$(dirname "$PRIV_OUT")"
cp "$BUNDLE/release.priv" "$PRIV_OUT"
chmod 600 "$PRIV_OUT"

echo "Wrote private key to $PRIV_OUT"
echo "Public key (release.pub):"
cat "$BUNDLE/release.pub"
if $INSTALL_PUB; then
	cp "$BUNDLE/release.pub" "$ROOT/installer/phases/release.pub"
	echo "Updated $ROOT/installer/phases/release.pub"
	echo "Redeploy /etc/panel/update.pub on existing hosts after rotating keys."
fi
