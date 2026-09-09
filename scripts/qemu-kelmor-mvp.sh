#!/usr/bin/env bash
# Drive the focused Kelmor MVP path inside a running QEMU Ubuntu 24.04 guest:
# Director login → create account → Nginx/PHP/DNS/MariaDB/SFTP/mailbox.
# Does not run live-e2e (cPanel import / WordPress).
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu-fresh/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2322}"
SRC="$(cd "$(dirname "$0")" && pwd)/.."

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
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq bind9-dnsutils sshpass || true
  sudo systemctl enable --now nginx php8.3-fpm postgresql postfix dovecot pdns mariadb pebble || true
  sudo systemctl start dovecot nginx php8.3-fpm pdns postfix mariadb pebble || true
  systemctl is-active panel-agent panel-api panel-worker nginx
'
ssh_cmd 'sudo mkdir -p /usr/local/panel/share/scripts && sudo chown -R ubuntu:ubuntu /usr/local/panel/share'
scp_cmd -P "$PORT" \
  "$SRC/scripts/fresh-provision-smoke.sh" \
  ubuntu@127.0.0.1:/usr/local/panel/share/scripts/
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
  export PANEL_TENANT_PASSWORD=TenantPass!2026
  export PANEL_REQUIRE_DIRECTOR=1
  sudo -E env PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026 PANEL_JOB_WAIT_ITERS=240 \
    PANEL_TENANT_PASSWORD=TenantPass!2026 PANEL_REQUIRE_DIRECTOR=1 \
    bash /usr/local/panel/share/scripts/fresh-provision-smoke.sh
  echo QEMU_KELMOR_MVP_OK
'
