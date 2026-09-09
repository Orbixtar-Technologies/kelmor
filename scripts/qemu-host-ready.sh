#!/usr/bin/env bash
# Report whether this host can run the QEMU Ubuntu 24.04 MVP path.
# Prints KEY=value lines. Exit 0 when the minimum tools exist; 1 otherwise.
set -euo pipefail

ok=1
report() {
  printf '%s=%s\n' "$1" "$2"
}

if [[ -f /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  report HOST_OS "${PRETTY_NAME:-unknown}"
  report HOST_ID "${ID:-unknown}"
  report HOST_VERSION_ID "${VERSION_ID:-unknown}"
else
  report HOST_OS unknown
  ok=0
fi

if command -v qemu-system-x86_64 >/dev/null; then
  report QEMU_BIN "$(command -v qemu-system-x86_64)"
  report QEMU_VERSION "$(qemu-system-x86_64 --version | head -1 | tr ' ' '_')"
else
  report QEMU_BIN missing
  ok=0
fi

if command -v cloud-localds >/dev/null; then
  report CLOUD_LOCALDS "$(command -v cloud-localds)"
else
  report CLOUD_LOCALDS missing
  ok=0
fi

OVMF_CODE="${OVMF_CODE:-/usr/share/OVMF/OVMF_CODE_4M.fd}"
if [[ -f "$OVMF_CODE" ]]; then
  report OVMF_CODE "$OVMF_CODE"
else
  report OVMF_CODE missing
  ok=0
fi

if [[ -c /dev/kvm ]]; then
  report KVM_DEV /dev/kvm
else
  report KVM_DEV missing
fi

if [[ -r /sys/module/kvm_intel/parameters/nested ]]; then
  report KVM_NESTED "$(cat /sys/module/kvm_intel/parameters/nested)"
elif [[ -r /sys/module/kvm_amd/parameters/nested ]]; then
  report KVM_NESTED "$(cat /sys/module/kvm_amd/parameters/nested)"
else
  report KVM_NESTED unknown
fi

report MEM_MB "$(awk '/MemAvailable:/ {print int($2/1024)}' /proc/meminfo)"
report NPROC "$(nproc)"

if [[ -x /usr/sbin/kvm-ok ]] || command -v kvm-ok >/dev/null; then
  if kvm-ok >/dev/null 2>&1; then
    report KVM_OK yes
  else
    report KVM_OK no
  fi
else
  report KVM_OK unknown
fi

if pgrep -f 'qemu-system-x86_64' >/dev/null; then
  report QEMU_RUNNING yes
else
  report QEMU_RUNNING no
fi

if [[ "$ok" -eq 1 ]]; then
  report QEMU_HOST_READY yes
  echo QEMU_HOST_READY
  exit 0
fi
report QEMU_HOST_READY no
echo QEMU_HOST_NOT_READY >&2
exit 1
