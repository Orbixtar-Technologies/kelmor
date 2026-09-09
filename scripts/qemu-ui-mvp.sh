#!/usr/bin/env bash
# Drive the portal MVP against the QEMU guest through SSH tunnels.
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2222}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"

export PANEL_SERVER_PORTAL="http://127.0.0.1:38443"
export PANEL_ACCOUNT_PORTAL="http://127.0.0.1:38444"
export PANEL_API="http://127.0.0.1:38080"
export PANEL_TENANT_HTTP_PORT=38081
export PANEL_TENANT_HTTPS_PORT=38445

SSH=(ssh)
if [[ ! -r "$KEY" ]]; then
  SSH=(sudo ssh)
fi
"${SSH[@]}" -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 \
  -L 38443:127.0.0.1:8443 \
  -L 38444:127.0.0.1:8444 \
  -L 38080:127.0.0.1:18080 \
  -L 38081:127.0.0.1:80 \
  -L 38445:127.0.0.1:443 \
  -p "$PORT" -N -f ubuntu@127.0.0.1

exec "$SRC/scripts/ui-mvp.sh"
