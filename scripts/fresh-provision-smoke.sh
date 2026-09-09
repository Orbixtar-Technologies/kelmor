#!/usr/bin/env bash
set -euo pipefail
# Empty-disk guest smoke: login → provision → HTTP/PHP isolation → mail → backup → suspend → audit.

API="${PANEL_API_ADDR:-127.0.0.1:18080}"
BASE="http://$API"
USER="${PANEL_ADMIN_USER:-admin}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
UNAME="${PANEL_SMOKE_USER:-freshhost}"
DOMAIN="${PANEL_SMOKE_DOMAIN:-freshhost.test}"
WAIT_ITERS="${PANEL_JOB_WAIT_ITERS:-180}"

login=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
token=$(echo "$login" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))')
if [[ -z "$token" ]]; then
  echo "login failed: $login" >&2
  exit 1
fi
AUTH="Authorization: Bearer $token"
echo "login-ok"

wait_job() {
  local jid="$1" label="${2:-job}"
  [[ -z "$jid" ]] && return 0
  local st=""
  for _ in $(seq 1 "$WAIT_ITERS"); do
    st=$(curl -sS "$BASE/api/v1/jobs/$jid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("state",""))')
    echo "$label job=$st"
    if [[ "$st" == "succeeded" ]]; then
      return 0
    fi
    if [[ "$st" == "failed" ]]; then
      echo "$label failed" >&2
      curl -sS "$BASE/api/v1/jobs/$jid" -H "$AUTH" >&2
      return 1
    fi
    sleep 1
  done
  echo "$label timeout state=$st" >&2
  return 1
}

host_fetch() {
  local host="$1" out="${2:-/tmp/host-fetch.body}" path="${3:-/}"
  local code
  code=$(curl -sS -o "$out" -w '%{http_code}' -H "Host: $host" "http://127.0.0.1$path" || true)
  if [[ "$code" == "301" || "$code" == "302" ]]; then
    code=$(curl -sk -o "$out" -w '%{http_code}' --resolve "$host:443:127.0.0.1" "https://$host$path" || true)
  fi
  printf '%s' "$code"
}

pkgs=$(curl -sS "$BASE/api/v1/packages" -H "$AUTH")
pkg=$(echo "$pkgs" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d
starter=next((i for i in items if i.get("name")=="Starter"), None)
pick=starter or max(items, key=lambda i: i.get("disk_bytes") or 0)
print(pick["id"])')

existing=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
aid=$(echo "$existing" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('username')=='$UNAME'), ''))")

if [[ -z "$aid" ]]; then
  created=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"username\":\"$UNAME\",\"primary_domain\":\"$DOMAIN\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$DOMAIN\",\"owner_password\":\"TenantPass!2026\"}")
  echo "$created"
  aid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
fi

st=""
for _ in $(seq 1 90); do
  acc=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH")
  st=$(echo "$acc" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "account status=$st"
  [[ "$st" == "active" ]] && break
  sleep 2
done
[[ "$st" == "active" ]] || { echo "account not active" >&2; exit 1; }

getent passwd "$UNAME"
dnsjob=$(curl -sS -X PATCH "$BASE/api/v1/accounts/$aid" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"package_id\":\"$pkg\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$dnsjob" public-dns

code=$(host_fetch "$DOMAIN" /tmp/live-site.html)
echo "http $code"
phpcode=$(host_fetch "$DOMAIN" /tmp/live-php.html /index.php)
php=$(cat /tmp/live-php.html || true)
echo "php $phpcode $php"
[[ "$code" == "200" ]] || { echo "expected HTTP 200 for $DOMAIN, got $code" >&2; exit 1; }
pool="/etc/php/8.3/fpm/pool.d/panel-${UNAME}.conf"
sudo test -f "$pool" || { echo "missing php-fpm pool $pool" >&2; exit 1; }
sudo grep -q "user = ${UNAME}" "$pool" || { echo "php-fpm pool not isolated to ${UNAME}" >&2; exit 1; }
sudo grep -q 'open_basedir' "$pool" || { echo "php-fpm pool missing open_basedir" >&2; exit 1; }
stat -c '%a %U:%G' "/home/${UNAME}" | grep -Eq '751 root:root' || { echo "home perms not isolated" >&2; exit 1; }
echo "linux-isolation ok"

mdoms=$(curl -sS "$BASE/api/v1/accounts/$aid/mail/domains" -H "$AUTH")
mdid=$(echo "$mdoms" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i['id'] for i in items if '$DOMAIN' in (i.get('name') or i.get('ascii_fqdn') or '')), items[0]['id'] if items else ''))")
if [[ -n "$mdid" ]]; then
  mb=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/mail/mailboxes" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"local_part\":\"hello\",\"domain_id\":\"$mdid\",\"password\":\"MailboxPass!2026\"}")
  echo "$mb"
  mop=$(echo "$mb" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$mop" mailbox
fi

bk=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"destination":"local"}')
echo "$bk"
bop=$(echo "$bk" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$bop" backup

sus=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/suspend" -H "$AUTH" -H 'content-type: application/json' -d '{}')
echo "$sus"
sop=$(echo "$sus" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$sop" suspend
uns=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/unsuspend" -H "$AUTH" -H 'content-type: application/json' -d '{}')
echo "$uns"
uop=$(echo "$uns" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$uop" unsuspend

audit=$(curl -sS "$BASE/api/v1/audit-events" -H "$AUTH")
echo "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or [];
print("audit", len(items));
assert items, d'

echo "QEMU_FRESH_INSTALL_OK"
echo "FRESH_PROVISION_SMOKE_OK"
