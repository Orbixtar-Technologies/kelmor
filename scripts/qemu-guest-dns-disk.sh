#!/usr/bin/env bash
# Prove DNSSEC DS records and package disk limits inside the QEMU guest.
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2222}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"

ssh_cmd() {
  ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=20 -o ServerAliveInterval=30 \
    -p "$PORT" ubuntu@127.0.0.1 "$@"
}

ssh_cmd 'sudo mkdir -p /usr/local/panel/share/scripts && sudo chown -R ubuntu:ubuntu /usr/local/panel/share/scripts'
scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" \
  "$SRC/scripts/live-e2e.sh" ubuntu@127.0.0.1:/usr/local/panel/share/scripts/
scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$PORT" \
  "$SRC/scripts/guest-dns-disk.sh" ubuntu@127.0.0.1:/tmp/guest-dns-disk.sh
ssh_cmd 'sudo bash /tmp/guest-dns-disk.sh'
echo QEMU_DNS_DISK_OK
