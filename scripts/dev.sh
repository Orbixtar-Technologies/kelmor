#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PANEL_DEV=1
export PANEL_STATE_DIR="${PANEL_STATE_DIR:-$PWD/var/panel}"
export PANEL_API_ADDR="${PANEL_API_ADDR:-127.0.0.1:18080}"
mkdir -p "$PANEL_STATE_DIR"
go run ./cmd/panel-install --dev --non-interactive --hostname localhost --admin-email admin@localhost >/tmp/panel-install.out || true
go run ./cmd/panel-dev
