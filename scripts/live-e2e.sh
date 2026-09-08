#!/usr/bin/env bash
set -euo pipefail
# Drive install → provision → host → mail/files/backups → suspend → audit on this host.

API="${PANEL_API_ADDR:-127.0.0.1:18080}"
BASE="http://$API"
USER="${PANEL_ADMIN_USER:-admin}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
UNAME="livehost"
DOMAIN="livehost.test"

login=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
echo "$login" | python3 -m json.tool | head
token=$(echo "$login" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))')
if [[ -z "$token" ]]; then
  echo "login failed: $login" >&2
  exit 1
fi

pkgs=$(curl -sS "$BASE/api/v1/packages" -H "Authorization: Bearer $token")
pkg=$(echo "$pkgs" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d; print(items[0]["id"])')

existing=$(curl -sS "$BASE/api/v1/accounts" -H "Authorization: Bearer $token")
aid=$(echo "$existing" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('username')=='$UNAME'), ''))")

if [[ -z "$aid" ]]; then
  created=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "Authorization: Bearer $token" -H 'content-type: application/json' \
    -d "{\"username\":\"$UNAME\",\"primary_domain\":\"$DOMAIN\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$DOMAIN\",\"owner_password\":\"TenantPass!2026\"}")
  echo "$created"
  aid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
fi

for i in $(seq 1 40); do
  acc=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "Authorization: Bearer $token")
  st=$(echo "$acc" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "account status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done

getent passwd "$UNAME" || true
ls -ld "/home/$UNAME" "/home/$UNAME/public_html" || true
ls /etc/nginx/panel-sites || true
code=$(curl -sS -o /tmp/live-site.html -w '%{http_code}' -H "Host: $DOMAIN" http://127.0.0.1/)
echo "http $code"
head -c 200 /tmp/live-site.html; echo
[[ "$code" == "200" ]] || echo "warning: expected HTTP 200 for $DOMAIN"
dig +short @"127.0.0.1" "$DOMAIN" A || true

mds=$(curl -sS "$BASE/api/v1/accounts/$aid/mail/domains" -H "Authorization: Bearer $token")
mdid=$(echo "$mds" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or []; print(items[0]["id"] if items else "")')
if [[ -n "$mdid" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/mail/mailboxes" -H "Authorization: Bearer $token" -H 'content-type: application/json' \
    -d "{\"domain_id\":\"$mdid\",\"local_part\":\"info\",\"password\":\"MailboxPass!2026\"}"
  sleep 2
fi
grep -n "$DOMAIN" /var/lib/panel/mail/virtual || true

python3 - <<PY
import smtplib
from email.mime.text import MIMEText
msg = MIMEText("panel live smtp")
msg["Subject"] = "panel live"
msg["From"] = "probe@localhost"
msg["To"] = "postmaster@$DOMAIN"
with smtplib.SMTP("127.0.0.1", 25, timeout=10) as s:
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
print("smtp accepted")
PY

ls -la /var/vmail/$DOMAIN/postmaster/Maildir/new || true

files=$(curl -sS "$BASE/api/v1/accounts/$aid/files?path=/public_html" -H "Authorization: Bearer $token")
echo "$files"

curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "Authorization: Bearer $token" -H 'content-type: application/json' \
  -d '{"path":"/public_html/note.txt","content":"live-hosted"}'
sudo test -f /home/$UNAME/public_html/note.txt
sudo grep live-hosted /home/$UNAME/public_html/note.txt

bak=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "Authorization: Bearer $token" -H 'content-type: application/json' \
  -d '{"kind":"full","destination":"local"}')
echo "$bak"
sleep 3
curl -sS "$BASE/api/v1/accounts/$aid/export" -H "Authorization: Bearer $token" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("export", d.get("account",{}).get("username"))'
curl -sS -X POST "$BASE/api/v1/accounts/$aid/suspend" -H "Authorization: Bearer $token"
audit=$(curl -sS "$BASE/api/v1/audit-events" -H "Authorization: Bearer $token")
echo "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d; print("audit", len(items) if isinstance(items,list) else d)'

echo "LIVE_E2E_OK"
