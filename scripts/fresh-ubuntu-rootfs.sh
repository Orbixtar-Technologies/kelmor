#!/usr/bin/env bash
# Bootstrap a clean Ubuntu 24.04 userspace and run panel-install into it.
# This does not replace a bare-metal ISO, but it exercises the installer
# against an empty Noble root instead of this overlay's already-provisioned /.
set -euo pipefail
ROOT="${PANEL_FRESH_ROOT:-/var/lib/panel/fresh-ubuntu}"
MIRROR="${PANEL_UBUNTU_MIRROR:-http://archive.ubuntu.com/ubuntu}"
HOST="${PANEL_FRESH_HOSTNAME:-panel.example.net}"
SRC="$(cd "$(dirname "$0")/.." && pwd)"

if [[ "$(id -u)" -ne 0 ]]; then
  exec sudo --preserve-env=PANEL_FRESH_ROOT,PANEL_UBUNTU_MIRROR,PANEL_FRESH_HOSTNAME,DEBIAN_FRONTEND "$0" "$@"
fi

export DEBIAN_FRONTEND=noninteractive
if ! command -v debootstrap >/dev/null; then
  apt-get update -qq
  apt-get install -y -qq debootstrap
fi

if [[ ! -f "$ROOT/etc/os-release" ]]; then
  mkdir -p "$ROOT"
  debootstrap --variant=minbase noble "$ROOT" "$MIRROR"
fi
grep -q '24.04' "$ROOT/etc/os-release"
grep -q Ubuntu "$ROOT/etc/os-release"

cleanup() {
  umount "$ROOT/dev" 2>/dev/null || true
  umount "$ROOT/sys" 2>/dev/null || true
  umount "$ROOT/proc" 2>/dev/null || true
}
trap cleanup EXIT
mountpoint -q "$ROOT/proc" || mount -t proc proc "$ROOT/proc"
mountpoint -q "$ROOT/sys" || mount -t sysfs sys "$ROOT/sys"
mountpoint -q "$ROOT/dev" || mount --bind /dev "$ROOT/dev"

mkdir -p "$ROOT/usr/local/panel/bin" "$ROOT/tmp"
if [[ -x "$SRC/dist/bin/panel-install" ]]; then
  cp -a "$SRC/dist/bin/." "$ROOT/usr/local/panel/bin/"
elif [[ -x /usr/local/panel/bin/panel-install ]]; then
  cp -a /usr/local/panel/bin/. "$ROOT/usr/local/panel/bin/"
else
  echo "panel-install binary missing; run make build first" >&2
  exit 1
fi

# Run the host installer so it does not overwrite its own running binary
# inside the target tree.
INSTALLER=/usr/local/panel/bin/panel-install
if [[ ! -x "$INSTALLER" ]]; then
  INSTALLER="$SRC/dist/bin/panel-install"
fi
PANEL_INSTALL_ROOT="$ROOT" "$INSTALLER" \
  --non-interactive \
  --hostname "$HOST" \
  --admin-email "admin@$HOST" \
  --acme letsencrypt \
  --root "$ROOT"

test -f "$ROOT/var/lib/panel/install-state.json"
grep -q 'letsencrypt.org' "$ROOT/var/lib/panel/acme.directory"
test -f "$ROOT/etc/nginx/panel-sites/00-acme.conf"
test -f "$ROOT/etc/systemd/system/panel-agent.service"
test -L "$ROOT/etc/systemd/system/multi-user.target.wants/panel-agent.service"
test -L "$ROOT/etc/systemd/system/multi-user.target.wants/panel-worker.service"
if [[ ! -x "$ROOT/usr/lib/systemd/systemd" ]]; then
  chroot "$ROOT" bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq systemd systemd-sysv dbus'
fi
test -x "$ROOT/usr/lib/systemd/systemd"
echo FRESH_UBUNTU_ROOTFS_OK "$ROOT"
