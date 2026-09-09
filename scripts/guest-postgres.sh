#!/usr/bin/env bash
# Provision a tenant PostgreSQL database on a live panel host.
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

aid=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH" | python3 -c 'import json,sys
items=json.load(sys.stdin).get("items") or []
print(next((i["id"] for i in items if i.get("username")=="livehost" and i.get("status")=="active"), ""))')
[[ -n "$aid" ]] || { echo "livehost missing" >&2; exit 1; }
uname=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("username",""))')
haspg=$(curl -sS "$BASE/api/v1/accounts/$aid/databases" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('engine')=='postgres' for i in items))")
if [[ "$haspg" != "True" ]]; then
  pgdb=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/databases" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"name":"pgd","engine":"postgres"}')
  wait_job "$(echo "$pgdb" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')" postgres-db
fi
PGNAME="${uname}_pgd"
sudo -u postgres psql -d "$PGNAME" -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS panel_pg(k text); DELETE FROM panel_pg; INSERT INTO panel_pg VALUES ('postgres-ok');"
sudo -u postgres psql -d "$PGNAME" -At -c "SELECT k FROM panel_pg;" | grep -q postgres-ok
echo "postgres-db $PGNAME ok"
echo GUEST_POSTGRES_OK
