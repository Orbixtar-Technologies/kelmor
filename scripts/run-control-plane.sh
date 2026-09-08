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
if [[ -r "$STATE/public.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$STATE/public.env"
  set +a
elif [[ -f "$STATE/public.env" ]]; then
  eval "$(sudo grep -E '^PANEL_PUBLIC_IPV4=' "$STATE/public.env")"
fi
if sudo test -f "$STATE/secrets/backup-sftp.env"; then
  eval "$(sudo grep -E '^PANEL_SFTP_[A-Z0-9_]+=' "$STATE/secrets/backup-sftp.env")"
fi
if sudo test -f "$STATE/secrets/backup-s3.env"; then
  eval "$(sudo grep -E '^PANEL_S3_[A-Z0-9_]+=' "$STATE/secrets/backup-s3.env")"
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
  PANEL_PUBLIC_IPV4="${PANEL_PUBLIC_IPV4:-}" \
  PANEL_SFTP_HOST="${PANEL_SFTP_HOST:-}" \
  PANEL_SFTP_USER="${PANEL_SFTP_USER:-}" \
  PANEL_SFTP_KEY="${PANEL_SFTP_KEY:-}" \
  PANEL_SFTP_HOST_KEY="${PANEL_SFTP_HOST_KEY:-}" \
  PANEL_SFTP_ROOT="${PANEL_SFTP_ROOT:-}" \
  PANEL_S3_ENDPOINT="${PANEL_S3_ENDPOINT:-}" \
  PANEL_S3_REGION="${PANEL_S3_REGION:-}" \
  PANEL_S3_BUCKET="${PANEL_S3_BUCKET:-}" \
  PANEL_S3_ACCESS_KEY="${PANEL_S3_ACCESS_KEY:-}" \
  PANEL_S3_SECRET_KEY="${PANEL_S3_SECRET_KEY:-}" \
  PANEL_S3_PREFIX="${PANEL_S3_PREFIX:-}" \
  "$ROOT/bin/$1"
