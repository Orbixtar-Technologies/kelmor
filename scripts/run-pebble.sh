#!/usr/bin/env bash
# Local ACME CA (Pebble) using HTTP-01 on port 80 — same protocol as Let's Encrypt.
set -euo pipefail
BIN="${PEBBLE_BIN:-/usr/local/panel/bin/pebble}"
CFG="${PEBBLE_CONFIG:-/var/lib/panel/pebble/config.json}"
if [[ ! -x "$BIN" ]]; then
  echo "pebble binary missing at $BIN" >&2
  exit 1
fi
if [[ ! -f "$CFG" ]]; then
  echo "run panel-install so TLS phase writes $CFG" >&2
  exit 1
fi
exec env PEBBLE_VA_NOSLEEP=1 PEBBLE_WFE_NONCEREJECT=0 \
  "$BIN" -config "$CFG" -dnsserver 127.0.0.1:53
