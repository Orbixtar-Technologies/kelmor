#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/feed/stable" "$TMP/bin"
echo '{}' > "$TMP/feed/stable/manifest.json"
for command in ssh scp rsync; do
	cat > "$TMP/bin/$command" <<EOF
#!/usr/bin/env bash
echo "$command" >> "$TMP/network-calls"
exit 99
EOF
	chmod +x "$TMP/bin/$command"
done
export PATH="$TMP/bin:$PATH"
export PANEL_UPDATE_FEED_ROOT="$TMP/feed"
export VM_HOST=203.0.113.10
export VM_USER=ubuntu
export VM_KEY_PATH="$TMP/id"
printf 'dummy\n' > "$TMP/id"
export GITHUB_ACTIONS=true
export PANEL_UPDATE_PUBLISH_URL=deploy@example:/srv/updates
unset PUBLISH_UPDATE_FEED REQUIRE_FEED_PUBLISH || true

publish_input="$(
	grep -A4 '^[[:space:]]*publish_feed:' "$ROOT/.github/workflows/release.yml"
)"
[[ "$publish_input" == *"default: false"* ]] || {
	echo "release publish input must default to false" >&2
	exit 1
}
grep -Fq \
	"if: \${{ github.event_name == 'workflow_dispatch' && inputs.publish_feed == true }}" \
	"$ROOT/.github/workflows/release.yml" || {
	echo "release publish step must require explicit manual dispatch" >&2
	exit 1
}

if ! out="$(bash "$ROOT/scripts/ci/publish-update-feed.sh" 2>&1)"; then
	echo "expected artifact-only mode to succeed: $out" >&2
	exit 1
fi
[[ "$out" == *"Publishing disabled"* ]] || {
	echo "missing publishing-disabled copy: $out" >&2
	exit 1
}
if [[ -s "$TMP/network-calls" ]]; then
	echo "artifact-only mode invoked network commands: $(tr '\n' ' ' < "$TMP/network-calls")" >&2
	exit 1
fi

if PUBLISH_UPDATE_FEED=true bash "$ROOT/scripts/ci/publish-update-feed.sh" >/dev/null 2>&1; then
	echo "expected explicit publish to propagate rsync failure" >&2
	exit 1
fi
[[ "$(cat "$TMP/network-calls")" == "rsync" ]] || {
	echo "explicit publish did not invoke only rsync: $(cat "$TMP/network-calls")" >&2
	exit 1
}

: > "$TMP/network-calls"
unset PANEL_UPDATE_PUBLISH_URL
if PUBLISH_UPDATE_FEED=true REQUIRE_FEED_PUBLISH=true \
	bash "$ROOT/scripts/ci/publish-update-feed.sh" >/dev/null 2>&1; then
	echo "expected explicit VM publish to propagate SSH failure" >&2
	exit 1
fi
[[ "$(cat "$TMP/network-calls")" == "ssh" ]] || {
	echo "explicit VM publish did not invoke only ssh: $(cat "$TMP/network-calls")" >&2
	exit 1
}

: > "$TMP/network-calls"
unset VM_HOST
if PUBLISH_UPDATE_FEED=true REQUIRE_FEED_PUBLISH=true \
	bash "$ROOT/scripts/ci/publish-update-feed.sh" >/dev/null 2>&1; then
	echo "expected explicit publish without a target to fail" >&2
	exit 1
fi
[[ ! -s "$TMP/network-calls" ]] || {
	echo "target-less publish invoked network commands: $(cat "$TMP/network-calls")" >&2
	exit 1
}

echo "publish-update-feed gating behavior ok"
