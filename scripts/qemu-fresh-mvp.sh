#!/usr/bin/env bash
# Copy acceptance scripts onto the empty-disk guest and run live-e2e + cli-mvp.
set -euo pipefail
export PANEL_QEMU_KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu-fresh/id_ed25519}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
exec "$(cd "$(dirname "$0")" && pwd)/qemu-mvp.sh"
