#!/usr/bin/env bash
set -euo pipefail
export PANEL_SERVER_PORTAL="${PANEL_SERVER_PORTAL:-https://127.0.0.1:38443}"
export PANEL_ACCOUNT_PORTAL="${PANEL_ACCOUNT_PORTAL:-https://127.0.0.1:38444}"
export PANEL_API="http://127.0.0.1:38080"
export PANEL_TENANT_HTTP_PORT=38081
export PANEL_TENANT_HTTPS_PORT=38445
export PANEL_HTTP_TRIES=180
export PANEL_BACKUP_WAIT_MS=180000
exec bash /workspace/scripts/ui-mvp.sh
