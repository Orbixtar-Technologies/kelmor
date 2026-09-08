#!/usr/bin/env bash
# Boot an official Ubuntu 24.04 cloud image under KVM and run panel-install
# so systemd PID 1 starts the control plane. This is not a mocked overlay.
set -euo pipefail
DIR="${PANEL_QEMU_DIR:-/var/lib/panel/qemu}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"
IMG="$DIR/noble-minimal.img"
DISK="$DIR/panel-noble.qcow2"
SEED="$DIR/seed.iso"
KEY="$DIR/id_ed25519"
SSH_PORT="${PANEL_QEMU_SSH:-2222}"
HOST="${PANEL_FRESH_HOSTNAME:-panel.example.net}"

if [[ "$(id -u)" -ne 0 ]]; then
  exec sudo --preserve-env=PANEL_QEMU_DIR,PANEL_QEMU_SSH,PANEL_FRESH_HOSTNAME "$0" "$@"
fi

mkdir -p "$DIR"
if [[ ! -f "$IMG" ]]; then
  curl -fL --retry 3 -o "$IMG" \
    https://cloud-images.ubuntu.com/minimal/releases/noble/release/ubuntu-24.04-minimal-cloudimg-amd64.img
fi
if [[ ! -f "$KEY" ]]; then
  ssh-keygen -t ed25519 -f "$KEY" -N "" -C "panel-qemu"
fi
PUB=$(cat "$KEY.pub")
cat > "$DIR/user-data" <<EOF
#cloud-config
hostname: $HOST
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - $PUB
ssh_pwauth: false
package_update: false
EOF
printf 'instance-id: panel-noble\nlocal-hostname: %s\n' "$HOST" > "$DIR/meta-data"
cloud-localds "$SEED" "$DIR/user-data" "$DIR/meta-data"
if [[ ! -f "$DISK" ]]; then
  qemu-img convert -O qcow2 "$IMG" "$DISK"
  qemu-img resize "$DISK" 20G
fi

if ! pgrep -f 'qemu-system-x86_64.*panel-noble' >/dev/null; then
  qemu-system-x86_64 -name panel-noble -enable-kvm -m 3072 -smp 2 \
    -display none -serial file:"$DIR/serial.log" \
    -drive file="$DISK",if=virtio,format=qcow2 \
    -drive file="$SEED",if=virtio,format=raw \
    -netdev user,id=n0,hostfwd=tcp:127.0.0.1:${SSH_PORT}-:22,hostfwd=tcp:127.0.0.1:28080-:18080 \
    -device virtio-net-pci,netdev=n0 \
    -object rng-random,filename=/dev/urandom,id=rng0 \
    -device virtio-rng-pci,rng=rng0 \
    >/dev/null 2>"$DIR/qemu.err" &
  echo $! > "$DIR/qemu.pid"
fi

ssh_cmd() {
  ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=5 -p "$SSH_PORT" ubuntu@127.0.0.1 "$@"
}

ok=0
for _ in $(seq 1 90); do
  if ssh_cmd true 2>/dev/null; then
    ok=1
    break
  fi
  sleep 2
done
[[ "$ok" == "1" ]] || { echo "ssh timeout" >&2; tail -50 "$DIR/serial.log" >&2; exit 1; }

ssh_cmd 'set -e
  grep -q 24.04 /etc/os-release
  grep -q Ubuntu /etc/os-release
  test "$(cat /proc/1/comm)" = systemd
  echo PID1=$(cat /proc/1/comm)
'

INSTALLER=/usr/local/panel/bin/panel-install
if [[ ! -x "$INSTALLER" ]]; then
  INSTALLER="$SRC/dist/bin/panel-install"
fi
ssh_cmd 'sudo mkdir -p /usr/local/panel/bin /tmp/panel-in'
scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" \
  "$INSTALLER" /usr/local/panel/bin/panel-agent /usr/local/panel/bin/panel-api \
  /usr/local/panel/bin/panel-worker /usr/local/panel/bin/panel-cli \
  /usr/local/panel/bin/panel-smtp-policy /usr/local/panel/bin/panel-object-store \
  /usr/local/panel/bin/panel-backup /usr/local/panel/bin/pebble \
  ubuntu@127.0.0.1:/tmp/panel-in/ 2>/dev/null || \
scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" \
  "$SRC/dist/bin/"panel-* "$SRC/dist/bin/pebble" \
  ubuntu@127.0.0.1:/tmp/panel-in/
ssh_cmd 'sudo cp -a /tmp/panel-in/. /usr/local/panel/bin/; sudo chmod 0755 /usr/local/panel/bin/panel-* /usr/local/panel/bin/pebble || true'

ssh_cmd "sudo /usr/local/panel/bin/panel-install --non-interactive --hostname $HOST --admin-email admin@$HOST --acme pebble"
ssh_cmd 'set -e
  systemctl is-system-running --wait || true
  systemctl is-enabled panel-agent panel-api panel-worker
  systemctl is-active panel-agent panel-api panel-worker
  curl -sS -o /dev/null -w "health:%{http_code}\n" http://127.0.0.1:18080/healthz
  echo QEMU_UBUNTU_INSTALL_OK
'
