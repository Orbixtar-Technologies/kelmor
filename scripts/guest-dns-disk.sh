#!/usr/bin/env bash
# Runs on the Ubuntu 24.04 guest (or any live panel host).
set -euo pipefail
BASE="${PANEL_API:-http://127.0.0.1:18080}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
TOKEN=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
AUTH="authorization: Bearer $TOKEN"

wait_job() {
  local jid="$1" label="${2:-job}"
  [[ -n "$jid" ]] || { echo "$label missing operation_id" >&2; exit 1; }
  local iters="${PANEL_JOB_WAIT_ITERS:-240}"
  for _ in $(seq 1 "$iters"); do
    local body st
    body=$(curl -sS "$BASE/api/v1/jobs/$jid" -H "$AUTH")
    st=$(echo "$body" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("state",""))')
    echo "$label $st"
    [[ "$st" == "succeeded" ]] && return 0
    if [[ "$st" == "failed" ]]; then
      echo "$body" >&2
      exit 1
    fi
    sleep 1
  done
  echo "$label timed out" >&2
  exit 1
}

systemctl is-active vsftpd >/dev/null
ss -lnt | grep -q ':21 '
echo vsftpd-ok

accs=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
aid=$(echo "$accs" | python3 -c 'import json,sys
items=json.load(sys.stdin).get("items") or []
print(next((i["id"] for i in items if i.get("username")=="livehost" and i.get("status")=="active"), ""))')
if [[ -z "$aid" ]]; then
  pkg=$(curl -sS "$BASE/api/v1/packages" -H "$AUTH" | python3 -c 'import json,sys
items=json.load(sys.stdin).get("items") or []
print(next((i["id"] for i in items if i.get("name")=="Starter"), items[0]["id"] if items else ""))')
  created=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"username\":\"livehost\",\"primary_domain\":\"livehost.test\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@livehost.test\",\"owner_password\":\"TenantPass!2026\"}")
  aid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
  wait_job "$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" dns-disk-provision
  for _ in $(seq 1 40); do
    st=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
    [[ "$st" == "active" ]] && break
    sleep 1
  done
fi
[[ -n "$aid" ]] || { echo "no account for DNSSEC" >&2; exit 1; }

acc=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH")
DOMAIN=$(echo "$acc" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("primary_domain",""))')
zid=$(curl -sS "$BASE/api/v1/accounts/$aid/dns/zones" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i['id'] for i in items if i.get('name')=='$DOMAIN'), ''))")
[[ -n "$zid" ]] || { echo "zone missing for $DOMAIN" >&2; exit 1; }
sec=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/dns/zones/$zid/dnssec" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"enabled":true}')
wait_job "$(echo "$sec" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" dnssec
curl -sS "$BASE/api/v1/accounts/$aid/dns/zones/$zid/ds" -H "$AUTH" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or [];
assert items, d
print("dnssec-ds", len(items), items[0].get("content","")[:48])'
echo DNSSEC_OK

DUSER="dq$(date +%s)"
DDOM="${DUSER}.test"
dpkg=$(curl -sS -X POST "$BASE/api/v1/packages" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"name\":\"TinyDisk ${DUSER}\",\"disk_bytes\":256,\"bandwidth_bytes_monthly\":1073741824,\"domains\":2,\"subdomains\":2,\"alias_domains\":1,\"databases\":1,\"database_users\":1,\"mailboxes\":1,\"mailbox_storage_bytes\":1048576,\"ftp_users\":1,\"cron_jobs\":1,\"application_instances\":1,\"backup_retention_days\":1,\"cpu_percent\":50,\"memory_bytes\":134217728,\"process_limit\":20,\"io_weight\":100,\"iops\":50,\"concurrent_web_requests\":10,\"email_daily_limit\":10}")
dpkgid=$(echo "$dpkg" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))')
[[ -n "$dpkgid" ]] || { echo "tiny disk package failed: $dpkg" >&2; exit 1; }
dcreated=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"username\":\"$DUSER\",\"primary_domain\":\"$DDOM\",\"package_id\":\"$dpkgid\",\"owner_email\":\"ops@$DDOM\",\"owner_password\":\"TenantPass!2026\"}")
did=$(echo "$dcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
wait_job "$(echo "$dcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" tiny-disk-provision
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$did" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  [[ "$st" == "active" ]] && break
  sleep 1
done
dcode=$(curl -sS -o /tmp/disk-deny.json -w '%{http_code}' -X POST "$BASE/api/v1/accounts/$did/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/too-big.txt","content":"0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"}')
[[ "$dcode" == "403" ]] || { echo "expected PACKAGE_LIMIT 403, got $dcode" >&2; cat /tmp/disk-deny.json >&2; exit 1; }
python3 -c 'import json; d=json.load(open("/tmp/disk-deny.json")); e=d.get("error") or {}; assert e.get("code")=="PACKAGE_LIMIT", d; print("disk-quota", e.get("code"))'
dterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$did/terminate" -H "$AUTH")
wait_job "$(echo "$dterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" tiny-disk-terminate
echo DISK_QUOTA_OK
echo GUEST_DNS_DISK_OK
