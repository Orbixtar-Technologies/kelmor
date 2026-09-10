#!/usr/bin/env bash
# Publish dist/update-feed to configured hosts or artifact-only mode.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FEED_SRC="${PANEL_UPDATE_FEED_ROOT:-$ROOT/dist/update-feed}"

if [[ ! -f "$FEED_SRC/stable/manifest.json" ]]; then
	echo "missing signed feed at $FEED_SRC/stable/manifest.json — run scripts/ci/release.sh first" >&2
	exit 1
fi

if [[ -n "${PANEL_UPDATE_PUBLISH_URL:-}" ]]; then
	echo "Publishing feed to $PANEL_UPDATE_PUBLISH_URL"
	rsync -az --delete "$FEED_SRC/" "$PANEL_UPDATE_PUBLISH_URL"
	echo "Feed published via rsync."
	exit 0
fi

if [[ -z "${VM_HOST:-}" && -f "$ROOT/.env" ]] && grep -q '^VM_HOST=' "$ROOT/.env"; then
	# shellcheck disable=SC1091
	source "$ROOT/.env"
fi

if [[ -n "${VM_HOST:-}" ]]; then
	if [[ -z "${VM_KEY_PATH:-}" ]]; then
		echo "VM_HOST is set but VM_KEY_PATH is missing" >&2
		exit 1
	fi
	VM_USER="${VM_USER:-ubuntu}"
	SSH=(ssh -i "$VM_KEY_PATH" -p "${VM_PORT:-22}" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)
	SCP=(scp -i "$VM_KEY_PATH" -P "${VM_PORT:-22}" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)
	echo "Publishing feed to ${VM_USER}@${VM_HOST}:/usr/local/panel/share/updates/"
	"${SSH[@]}" "${VM_USER}@${VM_HOST}" 'sudo rm -rf /tmp/kelmor-feed && sudo mkdir -p /tmp/kelmor-feed && sudo chown "$USER:$USER" /tmp/kelmor-feed'
	"${SCP[@]}" -r "$FEED_SRC/." "${VM_USER}@${VM_HOST}:/tmp/kelmor-feed/"
	"${SSH[@]}" "${VM_USER}@${VM_HOST}" 'sudo bash -s' <<'REMOTE'
set -euo pipefail
sudo rm -rf /usr/local/panel/share/updates
sudo cp -a /tmp/kelmor-feed/. /usr/local/panel/share/updates/
sudo chown -R root:root /usr/local/panel/share/updates
sudo cp -a /tmp/kelmor-feed/stable/manifest.json /usr/local/panel/share/updates/stable/manifest.json
REMOTE
	echo "Feed published on VM. Hosts will pick it up via panel-update.timer or Director check."
	exit 0
fi

echo "No publish target configured."
echo "Set PANEL_UPDATE_PUBLISH_URL (rsync target) or VM_HOST in .env for automatic deploy."
echo "Feed remains at $FEED_SRC for manual upload or CI artifacts."
