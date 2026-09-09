#!/usr/bin/env bash
# Empty-disk Ubuntu 24.04 + latest panel-install. Leaves the proof VM alone.
set -euo pipefail
export PANEL_QEMU_DIR="${PANEL_QEMU_DIR:-/var/lib/panel/qemu-fresh}"
export PANEL_QEMU_NAME="${PANEL_QEMU_NAME:-panel-fresh}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
export PANEL_QEMU_API_FWD="${PANEL_QEMU_API_FWD:-29080}"
export PANEL_QEMU_MEM="${PANEL_QEMU_MEM:-3072}"
export PANEL_FRESH_HOSTNAME="${PANEL_FRESH_HOSTNAME:-fresh.example.net}"
export PANEL_QEMU_ACCEL="${PANEL_QEMU_ACCEL:-tcg}"
export PANEL_QEMU_CLOUD_IMG="${PANEL_QEMU_CLOUD_IMG:-/var/lib/panel/qemu/noble-minimal.img}"
export PANEL_QEMU_EXTRA_FWD="${PANEL_QEMU_EXTRA_FWD:-hostfwd=tcp:127.0.0.1:39443-:8443,hostfwd=tcp:127.0.0.1:39444-:8444,hostfwd=tcp:127.0.0.1:39081-:80,hostfwd=tcp:127.0.0.1:39445-:443}"
exec "$(cd "$(dirname "$0")" && pwd)/qemu-ubuntu-install.sh"
