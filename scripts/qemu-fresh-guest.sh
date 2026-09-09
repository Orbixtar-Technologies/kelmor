#!/usr/bin/env bash
# Empty-disk Ubuntu 24.04 + latest panel-install. Leaves the proof VM alone.
set -euo pipefail
export PANEL_QEMU_DIR="${PANEL_QEMU_DIR:-/var/lib/panel/qemu-fresh}"
export PANEL_QEMU_NAME="${PANEL_QEMU_NAME:-panel-fresh}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
export PANEL_QEMU_API_FWD="${PANEL_QEMU_API_FWD:-29080}"
export PANEL_QEMU_MEM="${PANEL_QEMU_MEM:-2048}"
export PANEL_FRESH_HOSTNAME="${PANEL_FRESH_HOSTNAME:-fresh.example.net}"
export PANEL_QEMU_ACCEL="${PANEL_QEMU_ACCEL:-tcg}"
export PANEL_QEMU_CLOUD_IMG="${PANEL_QEMU_CLOUD_IMG:-/var/lib/panel/qemu/noble-minimal.img}"
exec "$(cd "$(dirname "$0")" && pwd)/qemu-ubuntu-install.sh"
