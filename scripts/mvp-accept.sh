#!/usr/bin/env bash
# Full MVP gate: installer resume → API path → CLI path → portals.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PANEL_API="${PANEL_API:-http://127.0.0.1:18080}"
export PANEL_API_ADDR="${PANEL_API_ADDR:-127.0.0.1:18080}"

echo "== installer resume/verify =="
sudo /usr/local/panel/bin/panel-install --non-interactive \
  --hostname "$(hostname)" --admin-email admin@localhost
sudo test -f /var/lib/panel/installation-report.txt
sudo nft list table inet panel >/dev/null
pgrep -x panel-agent >/dev/null
pgrep -x panel-api >/dev/null
pgrep -x panel-worker >/dev/null

echo "== API live path =="
bash "$ROOT/scripts/live-e2e.sh"

echo "== CLI live path =="
bash "$ROOT/scripts/cli-mvp.sh"

echo "== portals =="
curl -sS -o /dev/null -w 'server:%{http_code}\n' http://127.0.0.1:8443/ | grep -q 200
curl -sS -o /dev/null -w 'account:%{http_code}\n' http://127.0.0.1:8444/ | grep -q 200
curl -sS http://127.0.0.1:8443/ | grep -q 'Server Portal'
curl -sS http://127.0.0.1:8444/ | grep -q 'Account Portal'
curl -sS http://127.0.0.1:8443/healthz | grep -q ok
curl -sS -o /dev/null -w 'dev-server:%{http_code}\n' http://127.0.0.1:18443/ | grep -q 200 || true
curl -sS -o /dev/null -w 'dev-account:%{http_code}\n' http://127.0.0.1:18444/ | grep -q 200 || true
echo "== UI live path =="
bash "$ROOT/scripts/ui-mvp.sh"

echo "== host probes =="
dig +short @127.0.0.1 livehost.test A | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'
code=$(curl -sS -o /dev/null -w '%{http_code}' -H 'Host: livehost.test' http://127.0.0.1/)
if [[ "$code" == "301" || "$code" == "302" ]]; then
  code=$(curl -sk -o /dev/null -w '%{http_code}' --resolve livehost.test:443:127.0.0.1 https://livehost.test/)
fi
[[ "$code" == "200" ]]
ok=0
for _ in $(seq 1 20); do
  if sudo doveadm auth test info@livehost.test 'MailboxPass!2026' 2>/dev/null | grep -q succeeded; then
    ok=1
    break
  fi
  sleep 0.3
done
[[ "$ok" == "1" ]] || { echo "imap auth failed" >&2; exit 1; }
stat -c '%a' /var/lib/panel | grep -q 755

echo MVP_ACCEPT_OK
