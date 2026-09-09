#!/usr/bin/env bash
# Host orchestrator: tools → build → empty-disk Ubuntu 24.04 guest →
# panel-install → focused Kelmor MVP smoke.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

bash "$ROOT/scripts/qemu-host-ready.sh"

if [[ ! -x "$ROOT/dist/bin/panel-install" ]]; then
  make -C "$ROOT" build
fi
if [[ ! -f "$ROOT/portals/server/dist/index.html" || ! -f "$ROOT/portals/account/dist/index.html" ]]; then
  if [[ -f "$ROOT/portals/server/package.json" ]]; then
    make -C "$ROOT" portals || echo "portals build skipped; Director HTML may be PARTIAL" >&2
  fi
fi

export PANEL_QEMU_DIR="${PANEL_QEMU_DIR:-/var/lib/panel/qemu-fresh}"
export PANEL_QEMU_NAME="${PANEL_QEMU_NAME:-panel-fresh}"
export PANEL_QEMU_SSH="${PANEL_QEMU_SSH:-2322}"
export PANEL_QEMU_API_FWD="${PANEL_QEMU_API_FWD:-29080}"
export PANEL_QEMU_KEY="${PANEL_QEMU_KEY:-$PANEL_QEMU_DIR/id_ed25519}"
export PANEL_QEMU_MEM="${PANEL_QEMU_MEM:-3072}"
export PANEL_FRESH_HOSTNAME="${PANEL_FRESH_HOSTNAME:-fresh.example.net}"
# qemu-fresh-guest.sh defaults to TCG (older hosts BUG'd KVM). Prefer KVM
# here when kvm-ok says the device works.
if [[ -z "${PANEL_QEMU_ACCEL:-}" ]] && command -v kvm-ok >/dev/null && kvm-ok >/dev/null 2>&1; then
  export PANEL_QEMU_ACCEL=kvm
fi
export PANEL_QEMU_ACCEL="${PANEL_QEMU_ACCEL:-}"

bash "$ROOT/scripts/qemu-fresh-guest.sh"
bash "$ROOT/scripts/qemu-kelmor-mvp.sh"
echo KELMOR_QEMU_PATH_OK
