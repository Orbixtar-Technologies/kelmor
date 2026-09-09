#!/usr/bin/env bash
# SSH to the public validation VM using .run/validation/vm.env.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/scripts/load-validation-env.sh"
if [[ -z "${VM_HOST:-}" || -z "${VM_USER:-}" ]]; then
	echo "vm.env missing VM_HOST or VM_USER" >&2
	exit 1
fi
if [[ ! -f "${VM_KEY_PATH:-}" ]]; then
	echo "SSH key not found at ${VM_KEY_PATH:-<unset>} — place server-ssh-key_rsa.prv in .run/validation" >&2
	exit 1
fi
chmod 600 "$VM_KEY_PATH" || true
exec ssh -i "$VM_KEY_PATH" -p "${VM_PORT:-22}" \
	-o StrictHostKeyChecking=accept-new \
	-o IdentitiesOnly=yes \
	"${VM_USER}@${VM_HOST}" "$@"
