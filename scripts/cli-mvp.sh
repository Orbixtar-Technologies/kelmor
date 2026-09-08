#!/usr/bin/env bash
# Drive provision → host → mail → postgres → backup → suspend → audit via panel-cli.
set -euo pipefail
export PANEL_API="${PANEL_API:-http://127.0.0.1:18080}"
CLI="${PANEL_CLI:-/usr/local/panel/bin/panel-cli}"
eval "$($CLI login "${PANEL_ADMIN_USER:-admin}" "${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}")"
$CLI health >/dev/null
pkgs=$($CLI account list >/dev/null; curl -sS "$PANEL_API/api/v1/packages" -H "Authorization: Bearer $PANEL_TOKEN")
pkg=$(echo "$pkgs" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("items") or [])[0]["id"])')
accounts=$($CLI account list)
aid=$(echo "$accounts" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((a['id'] for a in items if a.get('username')=='climvp'), ''))")
if [[ -z "$aid" ]]; then
  created=$($CLI account create climvp climvp.test "$pkg" 'TenantPass!2026')
  aid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
  op=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  [[ -n "$op" ]] && $CLI job wait "$op"
fi
for i in $(seq 1 20); do
  st=$($CLI account get "$aid" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "climvp status=$st"
  if [[ "$st" == "suspended" ]]; then
    uns=$($CLI account unsuspend "$aid")
    uop=$(echo "$uns" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
    [[ -n "$uop" ]] && $CLI job wait "$uop"
    continue
  fi
  [[ "$st" == "active" ]] && break
  sleep 1
done
host_fetch() {
  local host="$1" out="${2:-/tmp/cli-host.body}"
  local code
  code=$(curl -sS -o "$out" -w '%{http_code}' -H "Host: $host" http://127.0.0.1/ || true)
  if [[ "$code" == "301" || "$code" == "302" ]]; then
    code=$(curl -sk -o "$out" -w '%{http_code}' --resolve "$host:443:127.0.0.1" "https://$host/" || true)
  fi
  printf '%s' "$code"
}
code=$(host_fetch climvp.test /tmp/climvp.html)
[[ "$code" == "200" ]]
$CLI file write "$aid" /public_html/cli.txt climvp-ok
$CLI file list "$aid" /public_html | python3 -c 'import json,sys; names=[i["name"] for i in json.load(sys.stdin).get("items") or []];
assert "cli.txt" in names, names'
RDOM="gone.climvp.test"
rdid=$(curl -sS "$PANEL_API/api/v1/accounts/$aid/domains" -H "Authorization: Bearer $PANEL_TOKEN" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$RDOM'), ''))")
if [[ -z "$rdid" ]]; then
  dcreated=$($CLI domain create "$aid" "$RDOM" php subdomain)
  dop=$(echo "$dcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  [[ -n "$dop" ]] && $CLI job wait "$dop"
fi
rwid=$(curl -sS "$PANEL_API/api/v1/accounts/$aid/websites" -H "Authorization: Bearer $PANEL_TOKEN" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((i['id'] for i in items if 'gone.climvp.test' in (i.get('document_root') or '')), ''))")
if [[ -z "$rwid" ]]; then
  rdid=$(curl -sS "$PANEL_API/api/v1/accounts/$aid/domains" -H "Authorization: Bearer $PANEL_TOKEN" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((i['id'] for i in items if i.get('ascii_fqdn')=='gone.climvp.test'), ''))")
  [[ -n "$rdid" ]]
  wcreated=$($CLI website create "$aid" "$rdid" php)
  wcop=$(echo "$wcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  [[ -n "$wcop" ]] && $CLI job wait "$wcop"
  rwid=$(echo "$wcreated" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("website") or {}).get("id",""))')
fi
[[ -n "$rwid" ]]
wdel=$($CLI website delete "$aid" "$rwid")
wop=$(echo "$wdel" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$wop" ]] && $CLI job wait "$wop"
mds=$(curl -sS "$PANEL_API/api/v1/accounts/$aid/mail/domains" -H "Authorization: Bearer $PANEL_TOKEN")
mdid=$(echo "$mds" | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; print(items[0]["id"] if items else "")')
if [[ -n "$mdid" ]]; then
  mbox=$($CLI mailbox create "$aid" "$mdid" info 'MailboxPass!2026')
  mop=$(echo "$mbox" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  [[ -n "$mop" ]] && $CLI job wait "$mop"
fi
sudo doveadm auth test info@climvp.test 'MailboxPass!2026' | grep -q succeeded
if ! sudo -u postgres psql -d climvp_clipg -c 'SELECT 1' >/dev/null 2>&1; then
  db=$($CLI db create "$aid" clipg postgres)
  dop=$(echo "$db" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  [[ -n "$dop" ]] && $CLI job wait "$dop"
  sudo -u postgres psql -d climvp_clipg -c 'SELECT 1' >/dev/null
fi
bak=$($CLI backup create "$aid")
bop=$(echo "$bak" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$bop" ]] && $CLI job wait "$bop"
sus=$($CLI account suspend "$aid")
sop=$(echo "$sus" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$sop" ]] && $CLI job wait "$sop"
code=""
for _ in $(seq 1 20); do
  code=$(host_fetch climvp.test /tmp/climvp-sus.html)
  [[ "$code" == "503" ]] && break
  sleep 0.2
done
[[ "$code" == "503" ]] || { echo "climvp expected 503, got $code" >&2; exit 1; }
uns=$($CLI account unsuspend "$aid")
uop=$(echo "$uns" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$uop" ]] && $CLI job wait "$uop"
code=""
for _ in $(seq 1 20); do
  code=$(host_fetch climvp.test /tmp/climvp.html)
  [[ "$code" == "200" ]] && break
  sleep 0.2
done
[[ "$code" == "200" ]]
$CLI account sftp-password "$aid" 'SftpPass!2026' >/dev/null
$CLI firewall apply | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("ok") is True or d.get("message"), d'
miguser=climig
accounts=$($CLI account list)
mid=$(echo "$accounts" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((a['id'] for a in items if a.get('username')=='climig'), ''))")
if [[ -z "$mid" ]]; then
  imported=$($CLI account migrate "$aid" "$miguser" climig.test)
  mid=$(echo "$imported" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
  hop=$(echo "$imported" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("homedir_job",""))')
  [[ -n "$hop" ]] && $CLI job wait "$hop"
fi
for i in $(seq 1 20); do
  st=$($CLI account get "$mid" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "climig status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
code=$(host_fetch climig.test /tmp/climig.html)
[[ "$code" == "200" ]]
$CLI audit | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; assert len(items)>0'
echo CLI_MVP_OK
