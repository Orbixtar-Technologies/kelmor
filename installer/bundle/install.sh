#!/usr/bin/env bash
# Kelmor host installer. Copy this extracted tree onto a fresh Ubuntu 24.04
# machine and run as root. It never contacts a remote VM or update feed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DEV=0
ROOT="${PANEL_INSTALL_ROOT:-}"
ARGS=()

usage() {
	cat <<'EOF'
Usage: sudo ./install.sh [panel-install flags]

Stages Kelmor binaries and portals from this tree, then runs panel-install.
Ubuntu 24.04 LTS (amd64 or arm64) is required unless --dev is set.

Examples:
  sudo ./install.sh --hostname panel.example.net --admin-email ops@example.net --non-interactive
  sudo ./install.sh --config ./install.yaml --non-interactive
  ./install.sh --dev --non-interactive --hostname localhost
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
			echo "kelmor-install: --root requires a path" >&2
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
	*)
		ARGS+=("$1")
		shift
		;;
	esac
done

if [[ "$DEV" -eq 0 && -z "$ROOT" && "$(id -u)" -ne 0 ]]; then
	echo "kelmor-install: must run as root on Ubuntu 24.04" >&2
	exit 1
fi

if [[ "$DEV" -eq 0 ]]; then
	os_release=/etc/os-release
	if [[ -n "$ROOT" ]]; then
		os_release="$ROOT/etc/os-release"
	fi
	if [[ ! -f "$os_release" ]] || ! grep -q Ubuntu "$os_release" || ! grep -q 24.04 "$os_release"; then
		echo "kelmor-install: Ubuntu 24.04 LTS required" >&2
		exit 1
	fi
fi

if [[ ! -x "$HERE/bin/panel-install" ]]; then
	echo "kelmor-install: missing $HERE/bin/panel-install" >&2
	exit 1
fi
if [[ ! -f "$HERE/share/portals/server/index.html" || ! -f "$HERE/share/portals/account/index.html" ]]; then
	echo "kelmor-install: built Director/Control portals are required under share/portals" >&2
	exit 1
fi

install_tree() {
	local dest_bin dest_share
	if [[ "$DEV" -eq 1 && -z "$ROOT" ]]; then
		return 0
	fi
	dest_bin="${ROOT}/usr/local/panel/bin"
	dest_share="${ROOT}/usr/local/panel/share"
	mkdir -p "$dest_bin" "$dest_share"
	cp -a "$HERE/bin/." "$dest_bin/"
	cp -a "$HERE/share/." "$dest_share/"
	chmod 0755 "$dest_bin/"* 2>/dev/null || true
}

install_tree

CONFIG_ARGS=()
has_config=0
if [[ ${#ARGS[@]} -gt 0 ]]; then
	for arg in "${ARGS[@]}"; do
		if [[ "$arg" == --config || "$arg" == --config=* ]]; then
			has_config=1
		fi
	done
fi
if [[ "$has_config" -eq 0 && -f "$HERE/install.yaml" ]]; then
	CONFIG_ARGS=(--config "$HERE/install.yaml")
fi

if [[ "$DEV" -eq 1 && -z "$ROOT" ]]; then
	exec "$HERE/bin/panel-install" "${CONFIG_ARGS[@]+"${CONFIG_ARGS[@]}"}" "${ARGS[@]+"${ARGS[@]}"}"
fi

exec "${ROOT}/usr/local/panel/bin/panel-install" "${CONFIG_ARGS[@]+"${CONFIG_ARGS[@]}"}" "${ARGS[@]+"${ARGS[@]}"}"
