#!/usr/bin/env bash
# Start the unprivileged API and worker as the panel user.
set -euo pipefail
ROOT="${PANEL_PREFIX:-/usr/local/panel}"
STATE="${PANEL_STATE_DIR:-/var/lib/panel}"
SOCK="${PANEL_AGENT_SOCK:-/run/panel/agent.sock}"
ADDR="${PANEL_API_ADDR:-127.0.0.1:18080}"
DSN="${PANEL_DATABASE_URL:-postgres:///panel_control?host=/var/run/postgresql}"
PDNS_URL="${PANEL_PDNS_URL:-http://127.0.0.1:8081}"
PDNS_KEY="${PANEL_PDNS_API_KEY:-panel-loopback}"
if [[ -f "$STATE/public.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$STATE/public.env"
  set +a
fi
export PANEL_STATE_DIR="$STATE"
export PANEL_AGENT_SOCK="$SOCK"
export PANEL_API_ADDR="$ADDR"
export PANEL_DATABASE_URL="$DSN"
unset PANEL_DEV PANEL_HOST_ROOT || true
install -d -o root -g panel -m 0751 /run/panel || true
# Dovecot must traverse /var/lib/panel to read mail/passwd. Secrets stay 0750.
install -d -o panel -g panel -m 0755 "$STATE" || true
install -d -o panel -g panel -m 0750 "$STATE/secrets" || true
chmod 0755 "$STATE" || true
exec sudo -u panel -g panel env \
  PANEL_STATE_DIR="$STATE" \
  PANEL_AGENT_SOCK="$SOCK" \
  PANEL_API_ADDR="$ADDR" \
  PANEL_DATABASE_URL="$DSN" \
  PANEL_PDNS_URL="$PDNS_URL" \
  PANEL_PDNS_API_KEY="$PDNS_KEY" \
  "$ROOT/bin/$1"
