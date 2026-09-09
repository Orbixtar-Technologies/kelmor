#!/usr/bin/env bash
# Drive portal MVP against the empty-disk guest (does not collide with proof-VM tunnels).
set -euo pipefail
export PANEL_QEMU_KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu-fresh/id_ed25519}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
export PANEL_SERVER_PORTAL="${PANEL_SERVER_PORTAL:-https://127.0.0.1:39443}"
export PANEL_ACCOUNT_PORTAL="${PANEL_ACCOUNT_PORTAL:-https://127.0.0.1:39444}"
export PANEL_API="${PANEL_API:-http://127.0.0.1:37080}"
export PANEL_TENANT_HTTP_PORT="${PANEL_TENANT_HTTP_PORT:-39081}"
export PANEL_TENANT_HTTPS_PORT="${PANEL_TENANT_HTTPS_PORT:-39445}"
export PANEL_HTTP_TRIES="${PANEL_HTTP_TRIES:-180}"
export PANEL_BACKUP_WAIT_MS="${PANEL_BACKUP_WAIT_MS:-180000}"

KEY="$PANEL_QEMU_KEY"
PORT="$PANEL_QEMU_SSH"
SSH=(ssh)
if [[ ! -r "$KEY" ]]; then
  SSH=(sudo ssh)
fi
# QEMU user-net hostfwd for 18080 is unreliable; tunnel the API.
# `-N -f` often leaves a dead listener on this slirp; keep a remote sleep.
if ! curl -sS -m 2 http://127.0.0.1:37080/healthz >/dev/null; then
  "${SSH[@]}" -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ServerAliveInterval=15 -o ServerAliveCountMax=240 \
    -L 37080:127.0.0.1:18080 \
    -p "$PORT" ubuntu@127.0.0.1 sleep 7200 >/tmp/fresh-api-tun.log 2>&1 &
  for _ in $(seq 1 20); do
    curl -sS -m 1 http://127.0.0.1:37080/healthz >/dev/null && break
    sleep 0.3
  done
fi

exec "$(cd "$(dirname "$0")" && pwd)/ui-mvp.sh"
