#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/feed/stable" "$TMP/bin"
echo '{}' > "$TMP/feed/stable/manifest.json"
printf '%s\n' '#!/usr/bin/env bash' 'echo "ssh: connect to host example port 22: Connection timed out" >&2' 'exit 255' > "$TMP/bin/ssh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 255' > "$TMP/bin/scp"
chmod +x "$TMP/bin/ssh" "$TMP/bin/scp"
export PATH="$TMP/bin:$PATH"
export PANEL_UPDATE_FEED_ROOT="$TMP/feed"
export VM_HOST=203.0.113.10
export VM_USER=ubuntu
export VM_KEY_PATH="$TMP/id"
printf 'dummy\n' > "$TMP/id"
export GITHUB_ACTIONS=true
unset REQUIRE_FEED_PUBLISH || true

if ! out="$(bash "$ROOT/scripts/ci/publish-update-feed.sh" 2>&1)"; then
	echo "expected skip to succeed: $out" >&2
	exit 1
fi
[[ "$out" == *timed\ out* ]] || { echo "missing timeout copy: $out" >&2; exit 1; }

if REQUIRE_FEED_PUBLISH=true bash "$ROOT/scripts/ci/publish-update-feed.sh" >/dev/null 2>&1; then
	echo "expected required publish to fail when SSH is down" >&2
	exit 1
fi

echo "publish-update-feed skip behavior ok"
