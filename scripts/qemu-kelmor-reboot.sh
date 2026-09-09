#!/usr/bin/env bash
# Reboot the QEMU guest and prove the Kelmor MVP stack returns without repair.
set -euo pipefail
KEY="${PANEL_QEMU_KEY:-/var/lib/panel/qemu-fresh/id_ed25519}"
PORT="${PANEL_QEMU_SSH:-2322}"
SRC="$(cd "$(dirname "$0")" && pwd)/.."

as_root=()
if [[ ! -r "$KEY" ]]; then
  as_root=(sudo)
fi

ssh_cmd() {
  "${as_root[@]}" ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=20 -o ServerAliveInterval=30 -o ServerAliveCountMax=240 \
    -p "$PORT" ubuntu@127.0.0.1 "$@"
}

scp_cmd() {
  "${as_root[@]}" scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$@"
}

ssh_cmd true
echo "reboot-guest"
# reboot closes SSH; ignore the dropped connection.
ssh_cmd 'sudo /sbin/shutdown -r now' || true

down=0
for _ in $(seq 1 60); do
  if ! ssh_cmd true >/dev/null 2>&1; then
    down=1
    break
  fi
  sleep 2
done
[[ "$down" == "1" ]] || echo "ssh never dropped; continuing to wait for return"

ok=0
for _ in $(seq 1 180); do
  if ssh_cmd true >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done
[[ "$ok" == "1" ]] || { echo "ssh timeout after reboot" >&2; exit 1; }

ssh_cmd 'set -e
  test "$(cat /proc/1/comm)" = systemd
  echo PID1=$(cat /proc/1/comm)
'
scp_cmd -P "$PORT" \
  "$SRC/scripts/guest-reboot-health.sh" \
  ubuntu@127.0.0.1:/tmp/guest-reboot-health.sh
ssh_cmd 'set -e
  export PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026
  export PANEL_TENANT_PASSWORD=TenantPass!2026
  sudo -E env PANEL_ADMIN_PASSWORD=ChangeMeOnce!2026 PANEL_TENANT_PASSWORD=TenantPass!2026 \
    bash /tmp/guest-reboot-health.sh
'
echo QEMU_KELMOR_REBOOT_OK
