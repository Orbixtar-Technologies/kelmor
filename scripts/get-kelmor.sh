#!/usr/bin/env bash
# Single-command Kelmor installer. Intended as:
#   curl -fsSL https://github.com/Orbixtar-Technologies/kelmor/releases/latest/download/get-kelmor.sh | sudo bash
# Downloads the published Ubuntu 24.04 installer tarball from GitHub Releases.
# Does not publish to or require the lab VM.
set -euo pipefail

REPO="${KELMOR_GITHUB_REPO:-Orbixtar-Technologies/kelmor}"
RELEASE_BASE="https://github.com/${REPO}/releases/latest/download"
BASE="${KELMOR_DOWNLOAD_BASE:-$RELEASE_BASE}"
DEV=0
ROOT="${PANEL_INSTALL_ROOT:-}"
HOSTNAME_VALUE="${KELMOR_HOSTNAME:-}"
ADMIN_EMAIL="${KELMOR_ADMIN_EMAIL:-}"
HAS_HOSTNAME=0
HAS_ADMIN=0
HAS_NONINTERACTIVE=0
ARGS=()

usage() {
	cat <<EOF
Usage: curl -fsSL ${RELEASE_BASE}/get-kelmor.sh | sudo bash
   or: curl -fsSL ${RELEASE_BASE}/get-kelmor.sh | sudo bash -s -- --hostname panel.example.net --admin-password 'your-password'

Environment:
  KELMOR_GITHUB_REPO     GitHub owner/name (default: ${REPO})
  KELMOR_DOWNLOAD_BASE   Override installer download URL
  KELMOR_HOSTNAME        Default hostname when --hostname is omitted
  KELMOR_ADMIN_EMAIL     Default administrator email
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	--dev)
		DEV=1
		ARGS+=("$1")
		shift
		;;
	--root)
		if [[ $# -lt 2 ]]; then
			echo "get-kelmor: --root requires a path" >&2
			exit 2
		fi
		ROOT="$2"
		ARGS+=("$1" "$2")
		shift 2
		;;
	--root=*)
		ROOT="${1#--root=}"
		ARGS+=("$1")
		shift
		;;
	--hostname)
		HAS_HOSTNAME=1
		HOSTNAME_VALUE="$2"
		ARGS+=("$1" "$2")
		shift 2
		;;
	--hostname=*)
		HAS_HOSTNAME=1
		HOSTNAME_VALUE="${1#--hostname=}"
		ARGS+=("$1")
		shift
		;;
	--admin-email)
		HAS_ADMIN=1
		ADMIN_EMAIL="$2"
		ARGS+=("$1" "$2")
		shift 2
		;;
	--admin-email=*)
		HAS_ADMIN=1
		ADMIN_EMAIL="${1#--admin-email=}"
		ARGS+=("$1")
		shift
		;;
	--admin-password)
		if [[ $# -lt 2 ]]; then
			echo "get-kelmor: --admin-password requires a value" >&2
			exit 2
		fi
		ARGS+=("$1" "$2")
		shift 2
		;;
	--admin-password=*)
		ARGS+=("$1")
		shift
		;;
	--non-interactive)
		HAS_NONINTERACTIVE=1
		ARGS+=("$1")
		shift
		;;
	*)
		ARGS+=("$1")
		shift
		;;
	esac
done

case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*)
	echo "get-kelmor: unsupported architecture $(uname -m)" >&2
	exit 1
	;;
esac

if [[ "$DEV" -eq 0 && -z "$ROOT" && "$(id -u)" -ne 0 ]]; then
	echo "get-kelmor: must run as root on Ubuntu 24.04" >&2
	exit 1
fi

if [[ "$DEV" -eq 0 ]]; then
	os_release=/etc/os-release
	if [[ -n "$ROOT" ]]; then
		os_release="$ROOT/etc/os-release"
	fi
	if [[ ! -f "$os_release" ]] || ! grep -q Ubuntu "$os_release" || ! grep -q 24.04 "$os_release"; then
		echo "get-kelmor: Ubuntu 24.04 LTS required" >&2
		exit 1
	fi
fi

if [[ "$HAS_HOSTNAME" -eq 0 ]]; then
	if [[ -z "$HOSTNAME_VALUE" ]]; then
		HOSTNAME_VALUE="$(hostname -f 2>/dev/null || hostname)"
	fi
	ARGS+=(--hostname "$HOSTNAME_VALUE")
fi
if [[ "$HAS_ADMIN" -eq 0 ]]; then
	if [[ -z "$ADMIN_EMAIL" ]]; then
		ADMIN_EMAIL="admin@${HOSTNAME_VALUE}"
	fi
	ARGS+=(--admin-email "$ADMIN_EMAIL")
fi
if [[ "$HAS_NONINTERACTIVE" -eq 0 && "$DEV" -eq 0 ]]; then
	ARGS+=(--non-interactive)
fi

TAR="kelmor-installer-linux-${ARCH}.tar.gz"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

echo "get-kelmor: downloading ${BASE}/${TAR}"
curl -fsSL "${BASE}/${TAR}" -o "${WORKDIR}/${TAR}"
curl -fsSL "${BASE}/${TAR}.sha256" -o "${WORKDIR}/${TAR}.sha256"
(
	cd "$WORKDIR"
	sha256sum -c "${TAR}.sha256"
)

echo "get-kelmor: extracting installer (this can take a minute)"
tar -xzf "${WORKDIR}/${TAR}" -C "$WORKDIR"
EXTRACT="$(find "$WORKDIR" -mindepth 1 -maxdepth 1 -type d -name 'kelmor-installer_*' | head -n 1)"
if [[ -z "$EXTRACT" || ! -x "$EXTRACT/install.sh" ]]; then
	echo "get-kelmor: downloaded archive is missing install.sh" >&2
	exit 1
fi

echo "get-kelmor: extracted $(basename "$EXTRACT") — this replaces the running panel"
echo "get-kelmor: starting host install — SSH will stay up; apt output follows"
trap - EXIT
exec "$EXTRACT/install.sh" "${ARGS[@]+"${ARGS[@]}"}"
