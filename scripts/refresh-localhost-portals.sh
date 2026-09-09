#!/usr/bin/env bash
# Rebuild Director/Control SPAs and copy them where nginx localhost looks.
# Portals are NOT embedded in Go binaries.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f portals/server/dist/index.html || ! -f portals/account/dist/index.html ]]; then
  make portals
fi

check_index() {
  local file="$1" title="$2"
  grep -q "$title" "$file" || { echo "$file missing $title" >&2; exit 1; }
  if grep -E 'Hosting Panel|Server Portal|Account Portal' "$file"; then
    echo "legacy chrome in $file" >&2
    exit 1
  fi
}

check_index "$ROOT/portals/server/dist/index.html" "Kelmor Director"
check_index "$ROOT/portals/account/dist/index.html" "Kelmor Control"

copy_tree() {
  local dest="$1"
  mkdir -p "$dest"
  rm -rf "$dest/server" "$dest/account"
  cp -a "$ROOT/portals/server/dist" "$dest/server"
  cp -a "$ROOT/portals/account/dist" "$dest/account"
  echo "copied Kelmor Director / Kelmor Control into $dest"
}

mkdir -p "$ROOT/dist/share/portals"
copy_tree "$ROOT/dist/share/portals"

DEV_DEST="$ROOT/var/panel/host/usr/local/panel/share/portals"
if [[ -d "$ROOT/var/panel/host" || -d "$DEV_DEST" ]]; then
  copy_tree "$DEV_DEST"
fi

if [[ -d /usr/local/panel/share/portals ]]; then
  if [[ -w /usr/local/panel/share/portals ]]; then
    copy_tree /usr/local/panel/share/portals
  else
    sudo mkdir -p /usr/local/panel/share/portals
    sudo rm -rf /usr/local/panel/share/portals/server /usr/local/panel/share/portals/account
    sudo cp -a "$ROOT/portals/server/dist" /usr/local/panel/share/portals/server
    sudo cp -a "$ROOT/portals/account/dist" /usr/local/panel/share/portals/account
    echo "copied Kelmor Director / Kelmor Control into /usr/local/panel/share/portals"
  fi
  if [[ -x /usr/sbin/nginx ]]; then
    sudo nginx -t && sudo nginx -s reload || true
  fi
fi

echo "Kelmor Director: https://127.0.0.1:8443/  (or Vite http://127.0.0.1:18443/)"
echo "Kelmor Control:  https://127.0.0.1:8444/  (or Vite http://127.0.0.1:18444/)"
