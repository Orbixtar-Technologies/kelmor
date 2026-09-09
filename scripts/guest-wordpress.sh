#!/usr/bin/env bash
# Install WordPress on livehost and prove cron delete on a live panel host.
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

host_fetch() {
  local host="$1" out="${2:-/tmp/host-fetch.body}"
  local code
  code=$(curl -sS -o "$out" -w '%{http_code}' -H "Host: $host" "http://127.0.0.1/" || true)
  if [[ "$code" == "301" || "$code" == "302" ]]; then
    code=$(curl -sk -o "$out" -w '%{http_code}' --resolve "$host:443:127.0.0.1" "https://$host/" || true)
  fi
  printf '%s' "$code"
}

accs=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
aid=$(echo "$accs" | python3 -c 'import json,sys
items=json.load(sys.stdin).get("items") or []
print(next((i["id"] for i in items if i.get("username")=="livehost" and i.get("status")=="active"), ""))')
[[ -n "$aid" ]] || { echo "livehost missing" >&2; exit 1; }

hasblog=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('ascii_fqdn')=='blog.livehost.test' for i in items))")
if [[ "$hasblog" != "True" ]]; then
  blog=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"fqdn":"blog.livehost.test","type":"addon","runtime":"php"}')
  wait_job "$(echo "$blog" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" wordpress-domain
fi
bid=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='blog.livehost.test'), ''))")
wsite=$(curl -sS "$BASE/api/v1/accounts/$aid/websites" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i['id'] for i in items if i.get('domain_id')=='$bid'), ''))")
[[ -n "$wsite" ]] || { echo "blog website missing" >&2; exit 1; }
haswp=$(curl -sS "$BASE/api/v1/accounts/$aid/applications" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('runtime')=='wordpress' for i in items))")
if [[ "$haswp" != "True" ]]; then
  wp=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/wordpress" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"website_id\":\"$wsite\",\"title\":\"Live Blog\",\"admin_user\":\"wpadmin\",\"admin_password\":\"WpAdmin!2026\",\"admin_email\":\"ops@livehost.test\"}")
  wait_job "$(echo "$wp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" wordpress-install
fi
wpcode=""
for _ in $(seq 1 40); do
  wpcode=$(host_fetch blog.livehost.test /tmp/wp.out)
  echo "wordpress_http $wpcode"
  if [[ "$wpcode" == "200" ]] && grep -q 'Live Blog' /tmp/wp.out; then
    break
  fi
  sleep 1
done
[[ "$wpcode" == "200" ]] || { echo "wordpress site down: $wpcode" >&2; head -c 300 /tmp/wp.out >&2; exit 1; }
grep -q 'Live Blog' /tmp/wp.out || { echo "wordpress title missing" >&2; head -c 300 /tmp/wp.out >&2; exit 1; }
echo WORDPRESS_OK

cronj=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/cron" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"schedule":"7 * * * *","command":"true"}')
cid=$(echo "$cronj" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("cron") or {}).get("id",""))')
wait_job "$(echo "$cronj" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" cron-create
cdel=$(curl -sS -X DELETE "$BASE/api/v1/accounts/$aid/cron/$cid" -H "$AUTH")
wait_job "$(echo "$cdel" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" cron-delete
curl -sS "$BASE/api/v1/accounts/$aid/cron" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; assert all(i.get('id')!='$cid' for i in items), items"
echo CRON_DELETE_OK
echo GUEST_WORDPRESS_OK
