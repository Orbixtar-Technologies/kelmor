#!/usr/bin/env bash
# Boot an official Ubuntu 24.04 cloud image under KVM and run panel-install
# so systemd PID 1 starts the control plane. This is not a mocked overlay.
set -euo pipefail
DIR="${PANEL_QEMU_DIR:-/var/lib/panel/qemu}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"
NAME="${PANEL_QEMU_NAME:-panel-noble}"
IMG="$DIR/noble-minimal.img"
CLOUD_IMG="${PANEL_QEMU_CLOUD_IMG:-/var/lib/panel/qemu/noble-minimal.img}"
DISK="$DIR/${NAME}.qcow2"
SEED="$DIR/seed.iso"
KEY="$DIR/id_ed25519"
SSH_PORT="${PANEL_QEMU_SSH:-2222}"
API_FWD="${PANEL_QEMU_API_FWD:-28080}"
MEM="${PANEL_QEMU_MEM:-3072}"
HOST="${PANEL_FRESH_HOSTNAME:-panel.example.net}"
RAW="$DIR/${NAME}.raw"

if [[ "$(id -u)" -ne 0 ]]; then
  exec sudo --preserve-env=PANEL_QEMU_DIR,PANEL_QEMU_SSH,PANEL_QEMU_NAME,PANEL_QEMU_CLOUD_IMG,PANEL_QEMU_API_FWD,PANEL_QEMU_MEM,PANEL_FRESH_HOSTNAME,PANEL_QEMU_ACCEL "$0" "$@"
fi

mkdir -p "$DIR"
if [[ ! -f "$IMG" && -f "$CLOUD_IMG" && "$CLOUD_IMG" != "$IMG" ]]; then
  cp -a "$CLOUD_IMG" "$IMG"
fi
if [[ ! -f "$IMG" ]]; then
  curl -fL --retry 3 -o "$IMG" \
    https://cloud-images.ubuntu.com/minimal/releases/noble/release/ubuntu-24.04-minimal-cloudimg-amd64.img
fi
if [[ ! -f "$KEY" && -f /var/lib/panel/qemu/id_ed25519 ]]; then
  cp -a /var/lib/panel/qemu/id_ed25519 /var/lib/panel/qemu/id_ed25519.pub "$DIR/" || true
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
printf 'instance-id: %s\nlocal-hostname: %s\n' "$NAME" "$HOST" > "$DIR/meta-data"
cloud-localds "$SEED" "$DIR/user-data" "$DIR/meta-data"
if [[ ! -f "$DISK" && ! -f "$RAW" ]]; then
  qemu-img convert -O qcow2 "$IMG" "$DISK"
  qemu-img resize "$DISK" 20G
fi

inject_ssh() {
  # Minimal cloud waits on snapd.seeded before ssh host keys exist.
  # Inject keys offline so systemd-booted guests are reachable even when
  # KVM is broken and TCG makes first-boot seeding take too long.
  local src="$1" mnt="$DIR/mnt-root" off
  mkdir -p "$mnt"
  off=$(python3 - "$src" <<'PY'
import struct, sys
path = sys.argv[1]
with open(path, "rb") as fh:
    fh.seek(512)
    hdr = fh.read(92)
    if hdr[:8] != b"EFI PART":
        raise SystemExit("not gpt")
    part_lba = struct.unpack_from("<Q", hdr, 72)[0]
    n = struct.unpack_from("<I", hdr, 80)[0]
    esz = struct.unpack_from("<I", hdr, 84)[0]
    fh.seek(part_lba * 512)
    best = None
    for _ in range(n):
        ent = fh.read(esz)
        if ent[:16] == b"\x00" * 16:
            continue
        first, last = struct.unpack_from("<QQ", ent, 32)
        size = (last - first + 1) * 512
        if best is None or size > best[1]:
            best = (first * 512, size)
    if not best:
        raise SystemExit("no gpt partitions")
    print(best[0])
PY
)
  mount -o loop,offset="$off" "$src" "$mnt"
  for t in rsa ecdsa ed25519; do
    if [[ ! -f "$mnt/etc/ssh/ssh_host_${t}_key" ]]; then
      ssh-keygen -t "$t" -f "$mnt/etc/ssh/ssh_host_${t}_key" -N "" -C "panel-qemu"
    fi
  done
  mkdir -p "$mnt/root/.ssh"
  cp "$KEY.pub" "$mnt/root/.ssh/authorized_keys"
  chmod 700 "$mnt/root/.ssh"
  chmod 600 "$mnt/root/.ssh/authorized_keys"
  cat > "$mnt/etc/ssh/sshd_config.d/99-panel-qemu.conf" <<'EOF'
PermitRootLogin prohibit-password
PasswordAuthentication no
EOF
  ln -sfn /dev/null "$mnt/etc/systemd/system/snapd.seeded.service"
  mkdir -p "$mnt/var/lib/cloud/seed/nocloud"
  cp "$DIR/user-data" "$mnt/var/lib/cloud/seed/nocloud/user-data"
  cp "$DIR/meta-data" "$mnt/var/lib/cloud/seed/nocloud/meta-data"
  if ! grep -q '^ubuntu:' "$mnt/etc/passwd"; then
    chroot "$mnt" useradd -m -s /bin/bash -G sudo ubuntu
  fi
  echo 'ubuntu ALL=(ALL) NOPASSWD:ALL' > "$mnt/etc/sudoers.d/ubuntu"
  chmod 440 "$mnt/etc/sudoers.d/ubuntu"
  mkdir -p "$mnt/home/ubuntu/.ssh"
  cp "$KEY.pub" "$mnt/home/ubuntu/.ssh/authorized_keys"
  chroot "$mnt" chown -R ubuntu:ubuntu /home/ubuntu/.ssh
  chmod 700 "$mnt/home/ubuntu/.ssh"
  chmod 600 "$mnt/home/ubuntu/.ssh/authorized_keys"
  mkdir -p "$mnt/etc/systemd/system/multi-user.target.wants"
  ln -sfn /usr/lib/systemd/system/ssh.service \
    "$mnt/etc/systemd/system/multi-user.target.wants/ssh.service"
  umount "$mnt"
}

if [[ ! -f "$DIR/.ssh-injected" ]]; then
  if [[ -f "$RAW" ]]; then
    inject_ssh "$RAW"
  else
    qemu-img convert -O raw "$DISK" "$RAW"
    qemu-img resize "$RAW" 20G
    inject_ssh "$RAW"
  fi
  touch "$DIR/.ssh-injected"
fi
BOOT_DISK="$RAW"
BOOT_FMT=raw
if [[ ! -f "$RAW" ]]; then
  BOOT_DISK="$DISK"
  BOOT_FMT=qcow2
fi

OVMF_CODE="${OVMF_CODE:-/usr/share/OVMF/OVMF_CODE_4M.fd}"
OVMF_VARS="$DIR/OVMF_VARS.fd"
if [[ ! -f "$OVMF_VARS" ]]; then
  cp /usr/share/OVMF/OVMF_VARS_4M.fd "$OVMF_VARS"
fi

# This host's KVM can BUG in kvm_arch_vcpu_create (0% CPU, empty serial).
# Prefer KVM when a 3s probe actually runs a vCPU; otherwise TCG.
ACCEL_ARGS=(-machine q35 -accel tcg,thread=multi -cpu qemu64)
if [[ "${PANEL_QEMU_ACCEL:-}" == "kvm" ]] || {
  [[ "${PANEL_QEMU_ACCEL:-}" != "tcg" && -c /dev/kvm ]] &&
    timeout 3 qemu-system-x86_64 -machine q35,accel=kvm -cpu host -m 64 \
      -display none -serial none >/dev/null 2>&1
}; then
  if ! dmesg 2>/dev/null | grep -q 'kvm_spurious_fault'; then
    ACCEL_ARGS=(-machine q35,accel=kvm -cpu host)
  fi
fi
if [[ "${PANEL_QEMU_ACCEL:-}" == "tcg" ]]; then
  ACCEL_ARGS=(-machine q35 -accel tcg,thread=multi -cpu qemu64)
fi
echo "qemu accel: ${ACCEL_ARGS[*]}"

if ! pgrep -f "qemu-system-x86_64.*${NAME}" >/dev/null; then
  : > "$DIR/serial.log"
  : > "$DIR/qemu.err"
  chmod 666 "$DIR/serial.log" "$DIR/qemu.err" || true
  # Noble cloud images are UEFI/GPT.
  qemu-system-x86_64 -name "$NAME" \
    "${ACCEL_ARGS[@]}" -m "$MEM" -smp 2 \
    -display none -serial file:"$DIR/serial.log" \
    -drive if=pflash,format=raw,readonly=on,file="$OVMF_CODE" \
    -drive if=pflash,format=raw,file="$OVMF_VARS" \
    -drive file="$BOOT_DISK",if=virtio,format="$BOOT_FMT" \
    -cdrom "$SEED" \
    -netdev user,id=n0,hostfwd=tcp:127.0.0.1:${SSH_PORT}-:22,hostfwd=tcp:127.0.0.1:${API_FWD}-:18080 \
    -device virtio-net-pci,netdev=n0 \
    -object rng-random,filename=/dev/urandom,id=rng0 \
    -device virtio-rng-pci,rng=rng0 \
    >/dev/null 2>"$DIR/qemu.err" &
  echo $! > "$DIR/qemu.pid"
fi

ssh_cmd() {
  ssh -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=20 -o ServerAliveInterval=30 -o ServerAliveCountMax=120 \
    -p "$SSH_PORT" ubuntu@127.0.0.1 "$@"
}

ok=0
for _ in $(seq 1 240); do
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
if [[ -f "$SRC/portals/server/dist/index.html" && -f "$SRC/portals/account/dist/index.html" ]]; then
  ssh_cmd 'sudo mkdir -p /usr/local/panel/share/portals/server /usr/local/panel/share/portals/account && sudo chown -R ubuntu:ubuntu /usr/local/panel/share/portals'
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" -r \
    "$SRC/portals/server/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/server/
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" -r \
    "$SRC/portals/account/dist/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/account/
elif [[ -f "$SRC/dist/share/portals/server/index.html" && -f "$SRC/dist/share/portals/account/index.html" ]]; then
  ssh_cmd 'sudo mkdir -p /usr/local/panel/share/portals/server /usr/local/panel/share/portals/account && sudo chown -R ubuntu:ubuntu /usr/local/panel/share/portals'
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" -r \
    "$SRC/dist/share/portals/server/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/server/
  scp -i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P "$SSH_PORT" -r \
    "$SRC/dist/share/portals/account/." ubuntu@127.0.0.1:/usr/local/panel/share/portals/account/
fi

ssh_cmd "sudo /usr/local/panel/bin/panel-install --non-interactive --hostname $HOST --admin-email admin@$HOST --acme pebble"
ssh_cmd 'set -e
  systemctl is-system-running --wait || true
  systemctl is-enabled panel-agent panel-api panel-worker
  systemctl is-active panel-agent panel-api panel-worker
  curl -sS -o /dev/null -w "health:%{http_code}\n" http://127.0.0.1:18080/healthz
  echo QEMU_UBUNTU_INSTALL_OK
'
