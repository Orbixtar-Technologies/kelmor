#!/usr/bin/env bash
# Deploy the current tree to the validation VM and bootstrap update automation.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/.env"
SSH=(ssh -i "$VM_KEY_PATH" -p "${VM_PORT:-22}" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)
SCP=(scp -i "$VM_KEY_PATH" -P "${VM_PORT:-22}" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)

cd "$ROOT"
make build portals package
bash "$ROOT/scripts/bootstrap-update-feed.sh"

DEB="$(ls -1 "$ROOT"/dist/deb/hosting-panel_*_amd64.deb | tail -1)"
echo "Using $DEB"

"${SSH[@]}" "${VM_USER}@${VM_HOST}" 'mkdir -p /tmp/kelmor-deploy/update-feed'
"${SCP[@]}" "$DEB" "$ROOT/installer/phases/release.pub" \
	"${VM_USER}@${VM_HOST}:/tmp/kelmor-deploy/"
"${SCP[@]}" -r "$ROOT/dist/update-feed/." \
	"${VM_USER}@${VM_HOST}:/tmp/kelmor-deploy/update-feed/"

"${SSH[@]}" "${VM_USER}@${VM_HOST}" 'sudo bash -s' <<'REMOTE'
set -euo pipefail
DEB="$(ls -1 /tmp/kelmor-deploy/hosting-panel_*_amd64.deb | tail -1)"
sudo dpkg -i "$DEB" || sudo apt-get -f install -y
sudo cp -a /tmp/kelmor-deploy/release.pub /etc/panel/update.pub
sudo rm -rf /usr/local/panel/share/updates
sudo cp -a /tmp/kelmor-deploy/update-feed/. /usr/local/panel/share/updates/
sudo chown -R root:root /usr/local/panel/share/updates
sudo /usr/local/panel/bin/panel-install --non-interactive --hostname lab.kelmor.host --admin-email ops@kelmor.host
sudo systemctl daemon-reload
sudo systemctl restart pdns panel-agent panel-api panel-worker panel-object-store nginx
sudo systemctl enable panel-update.timer
sudo systemctl start panel-update.timer
REMOTE

echo "Running remote verification..."
"${SSH[@]}" "${VM_USER}@${VM_HOST}" 'bash -s' <<'VERIFY'
set -e
curl -fsS -H "X-API-Key: panel-loopback" http://127.0.0.1:8081/api/v1/servers/localhost >/dev/null
curl -fsS http://127.0.0.1:18080/healthz
curl -kfsS https://127.0.0.1:8443/updates/stable/manifest.json >/dev/null
sudo /usr/local/panel/bin/panel-updater check --config /etc/panel/update.env
systemctl is-active pdns panel-api panel-worker
systemctl is-enabled panel-update.timer
VERIFY

echo "Deploy complete."
