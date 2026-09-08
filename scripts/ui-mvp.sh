#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/scripts/ui-mvp"
if [[ ! -d node_modules/puppeteer-core ]]; then
  npm install --omit=dev --no-fund --no-audit
fi
export CHROME="${CHROME:-/usr/local/bin/google-chrome}"
export PANEL_SERVER_PORTAL="${PANEL_SERVER_PORTAL:-http://127.0.0.1:8443}"
export PANEL_ACCOUNT_PORTAL="${PANEL_ACCOUNT_PORTAL:-http://127.0.0.1:8444}"
export PANEL_API="${PANEL_API:-http://127.0.0.1:18080}"
exec node "$ROOT/scripts/ui-mvp/index.mjs"
