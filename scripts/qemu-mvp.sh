#!/usr/bin/env bash
# Drive the MVP acceptance path inside the QEMU Ubuntu 24.04 guest.
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2222}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"

as_root=()
if [[ ! -r "$KEY" ]]; then
  as_root=(sudo)
fi

ssh_cmd() {
  "${as_root[@]}" ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=20 -o ServerAliveInterval=30 -o ServerAliveCountMax=240 \
    -p "$PORT" ubuntu@127.0.0.1 "$@"
}

scp_cmd() {
  "${as_root[@]}" scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$@"
}

ssh_cmd 'set -e
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq bind9-dnsutils
  sudo systemctl enable --now nginx php8.3-fpm postgresql postfix dovecot pdns || true
  sudo systemctl start dovecot nginx php8.3-fpm pdns postfix || true
  systemctl is-active panel-agent panel-api panel-worker dovecot nginx
'
ssh_cmd 'sudo mkdir -p /usr/local/panel/share/scripts /usr/local/panel/share/testdata && sudo chown -R ubuntu:ubuntu /usr/local/panel/share'
scp_cmd -P "$PORT" \
  "$SRC/scripts/live-e2e.sh" "$SRC/scripts/cli-mvp.sh" \
  ubuntu@127.0.0.1:/usr/local/panel/share/scripts/
scp_cmd -P "$PORT" -r \
  "$SRC/testdata/cpanel-acme42" \
  ubuntu@127.0.0.1:/usr/local/panel/share/testdata/
if [[ -f "$SRC/portals/server/dist/index.html" && -f "$SRC/portals/account/dist/index.html" ]]; then
  ssh_cmd 'sudo mkdir -p /usr/local/panel/share/portals/server /usr/local/panel/share/portals/account && sudo chown -R ubuntu:ubuntu /usr/local/panel/share/portals'
  scp_cmd -P "$PORT" -r \
    "$SRC/portals/server/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/server/
  scp_cmd -P "$PORT" -r \
    "$SRC/portals/account/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/account/
  ssh_cmd 'sudo nginx -t && sudo systemctl reload nginx || true'
fi
ssh_cmd 'set -e
  export PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026
  export PANEL_JOB_WAIT_ITERS=240
  sudo -E env PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026 PANEL_JOB_WAIT_ITERS=240 \
    bash /usr/local/panel/share/scripts/live-e2e.sh
  sudo -E env PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026 \
    bash /usr/local/panel/share/scripts/cli-mvp.sh
  echo QEMU_MVP_OK
'
