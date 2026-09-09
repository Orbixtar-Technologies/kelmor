#!/usr/bin/env bash
# Copy the focused Kelmor MVP smoke onto the empty-disk guest.
# Full live-e2e (cPanel/WordPress) remains scripts/qemu-mvp.sh.
set -euo pipefail
export PANEL_QEMU_KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu-fresh/id_ed25519}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
exec "$(cd "$(dirname "$0")" && pwd)/qemu-kelmor-mvp.sh"
