#!/usr/bin/env bash
# Vite HMR for Kelmor Director (:18443) and Kelmor Control (:18444).
# The Go API is not in these binaries; start panel-dev / kelmor-dev on :18080.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cleanup() {
  jobs -p | xargs -r kill 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Kelmor Director (Vite): http://127.0.0.1:18443/"
echo "Kelmor Control (Vite):  http://127.0.0.1:18444/"
echo "API proxy target:       http://127.0.0.1:18080/"
echo "Installed nginx :8443/:8444 stays stale until: make refresh-portals"

(cd "$ROOT/portals/server" && npm install && npm run dev) &
(cd "$ROOT/portals/account" && npm install && npm run dev) &
wait
