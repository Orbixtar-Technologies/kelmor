#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PANEL_DEV=1
export PANEL_STATE_DIR="${PANEL_STATE_DIR:-$PWD/var/panel}"
export PANEL_API_ADDR="${PANEL_API_ADDR:-127.0.0.1:18080}"
export PANEL_DATABASE_URL="${PANEL_DATABASE_URL:-postgres:///panel_control?host=/var/run/postgresql}"
if [[ -S /run/panel/agent.sock ]]; then
  export PANEL_AGENT_SOCK="${PANEL_AGENT_SOCK:-/run/panel/agent.sock}"
fi
mkdir -p "$PANEL_STATE_DIR"
echo "API http://${PANEL_API_ADDR}  (HTML index is Kelmor; SPAs are not in this binary)"
echo "Kelmor Director chrome: npm run dev  → http://127.0.0.1:${SERVER_PORTAL_PORT:-18443}/"
echo "Kelmor Control chrome:  npm run dev  → http://127.0.0.1:${ACCOUNT_PORTAL_PORT:-18444}/"
echo "Installed nginx :8443/:8444 stays stale until: make refresh-portals"
go run ./cmd/panel-install --dev --non-interactive --hostname localhost --admin-email admin@localhost >/tmp/panel-install.out || true
if id -nG | grep -qw panel; then
  exec sg panel -c "./dist/bin/panel-dev"
fi
exec ./dist/bin/panel-dev
