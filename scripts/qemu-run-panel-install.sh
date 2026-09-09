#!/usr/bin/env bash
# Copy panel binaries into the running QEMU guest and run panel-install.
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2222}"
HOST="${PANEL_FRESH_HOSTNAME:-panel.example.net}"
BIN="${PANEL_BIN_DIR:-/usr/local/panel/bin}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"

ssh_cmd() {
  ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=20 -o ServerAliveInterval=30 -o ServerAliveCountMax=120 \
    -p "$PORT" ubuntu@127.0.0.1 "$@"
}

echo "PID1=$(ssh_cmd cat /proc/1/comm)"
ssh_cmd 'mkdir -p /tmp/panel-in && sudo mkdir -p /usr/local/panel/bin && sudo chown ubuntu:ubuntu /tmp/panel-in'
scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" \
  "$BIN/panel-install" "$BIN/panel-agent" "$BIN/panel-api" "$BIN/panel-worker" \
  "$BIN/panel-cli" "$BIN/panel-smtp-policy" "$BIN/panel-object-store" \
  "$BIN/panel-backup" "$BIN/pebble" \
  ubuntu@127.0.0.1:/tmp/panel-in/
ssh_cmd 'sudo cp -a /tmp/panel-in/. /usr/local/panel/bin/; sudo chmod 0755 /usr/local/panel/bin/*'
if [[ -f "$SRC/portals/server/dist/index.html" && -f "$SRC/portals/account/dist/index.html" ]]; then
  ssh_cmd 'sudo mkdir -p /usr/local/panel/share/portals/server /usr/local/panel/share/portals/account && sudo chown -R ubuntu:ubuntu /usr/local/panel/share/portals'
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" -r \
    "$SRC/portals/server/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/server/
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" -r \
    "$SRC/portals/account/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/account/
fi
if [[ -d "$SRC/testdata/cpanel-acme42" ]]; then
  ssh_cmd 'sudo mkdir -p /usr/local/panel/share/testdata /usr/local/panel/share/scripts && sudo chown -R ubuntu:ubuntu /usr/local/panel/share'
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" -r \
    "$SRC/testdata/cpanel-acme42" ubuntu@127.0.0.1:/usr/local/panel/share/testdata/
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" \
    "$SRC/scripts/live-e2e.sh" "$SRC/scripts/cli-mvp.sh" \
    ubuntu@127.0.0.1:/usr/local/panel/share/scripts/
fi
echo BINS_COPIED
ssh_cmd "sudo /usr/local/panel/bin/panel-install --non-interactive --hostname $HOST --admin-email admin@$HOST --acme pebble"
echo INSTALL_DONE
ssh_cmd 'set -e
  systemctl is-enabled panel-agent panel-api panel-worker
  systemctl is-active panel-agent panel-api panel-worker
  curl -sS -o /dev/null -w "health:%{http_code}\n" http://127.0.0.1:18080/healthz
  echo QEMU_UBUNTU_INSTALL_OK
'
