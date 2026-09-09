#!/usr/bin/env bash
# Browser pass of Kelmor Control against the QEMU (or local) API.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/scripts/ui-mvp"
if [[ ! -d node_modules/puppeteer-core ]]; then
  npm install --omit=dev --no-fund --no-audit
fi
export CHROME="${CHROME:-/usr/local/bin/google-chrome}"
if [[ ! -x "$CHROME" ]]; then
  echo "chrome missing at $CHROME" >&2
  exit 1
fi
export PANEL_ACCOUNT_PORTAL="${PANEL_ACCOUNT_PORTAL:-https://127.0.0.1:39444}"
# QEMU user-net :18080 forwards have been observed to RST after guest reboot.
# Control nginx on :8444 (host 39444) already proxies /api.
export PANEL_API="${PANEL_API:-$PANEL_ACCOUNT_PORTAL}"
export NODE_TLS_REJECT_UNAUTHORIZED="${NODE_TLS_REJECT_UNAUTHORIZED:-0}"
export PANEL_CONTROL_USER="${PANEL_CONTROL_USER:-freshhost}"
export PANEL_CONTROL_PASSWORD="${PANEL_CONTROL_PASSWORD:-TenantPass!2026}"
export PANEL_SMOKE_DOMAIN="${PANEL_SMOKE_DOMAIN:-freshhost.test}"
exec node "$ROOT/scripts/ui-mvp/control.mjs"
