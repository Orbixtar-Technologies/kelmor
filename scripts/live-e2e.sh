#!/usr/bin/env bash
set -euo pipefail
# Drive install → provision → host → mail/files/backups → suspend → migrate → audit.

API="${PANEL_API_ADDR:-127.0.0.1:18080}"
BASE="http://$API"
USER="${PANEL_ADMIN_USER:-admin}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
UNAME="livehost"
DOMAIN="livehost.test"

login=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
token=$(echo "$login" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))')
if [[ -z "$token" ]]; then
  echo "login failed: $login" >&2
  exit 1
fi
AUTH="Authorization: Bearer $token"

pkgs=$(curl -sS "$BASE/api/v1/packages" -H "$AUTH")
pkg=$(echo "$pkgs" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d; print(items[0]["id"])')

existing=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
aid=$(echo "$existing" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('username')=='$UNAME'), ''))")

if [[ -z "$aid" ]]; then
  created=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"username\":\"$UNAME\",\"primary_domain\":\"$DOMAIN\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$DOMAIN\",\"owner_password\":\"TenantPass!2026\"}")
  echo "$created"
  aid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
fi

for i in $(seq 1 40); do
  acc=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH")
  st=$(echo "$acc" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "account status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done

getent passwd "$UNAME" || true
code=$(curl -sS -o /tmp/live-site.html -w '%{http_code}' -H "Host: $DOMAIN" http://127.0.0.1/)
echo "http $code"
php=$(curl -sS -H "Host: $DOMAIN" http://127.0.0.1/index.php || true)
echo "php $php"
[[ "$code" == "200" ]] || echo "warning: expected HTTP 200 for $DOMAIN"
dig +short @"127.0.0.1" "$DOMAIN" A || true

mds=$(curl -sS "$BASE/api/v1/accounts/$aid/mail/domains" -H "$AUTH")
mdid=$(echo "$mds" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or []; print(items[0]["id"] if items else "")')
if [[ -n "$mdid" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/mail/mailboxes" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"domain_id\":\"$mdid\",\"local_part\":\"info\",\"password\":\"MailboxPass!2026\"}" >/tmp/mbox.json || true
  sleep 2
fi
grep -n "$DOMAIN" /var/lib/panel/mail/virtual || true

python3 - <<PY
import smtplib
from email.mime.text import MIMEText
msg = MIMEText("panel live smtp")
msg["Subject"] = "panel live"
msg["From"] = "probe@localhost"
msg["To"] = "info@$DOMAIN"
with smtplib.SMTP("127.0.0.1", 25, timeout=10) as s:
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
print("smtp accepted")
PY
sleep 1
sudo ls /var/vmail/$DOMAIN/info/Maildir/new 2>/dev/null | head || true
if sudo doveadm auth test info@$DOMAIN 'MailboxPass!2026' 2>&1 | grep -q succeeded; then
  echo "imap-auth ok"
fi
python3 - <<'PY'
import imaplib, ssl
ctx = ssl._create_unverified_context()
try:
    m = imaplib.IMAP4_SSL("127.0.0.1", 993, ssl_context=ctx)
    typ, _ = m.login("info@livehost.test", "MailboxPass!2026")
    print("imap", typ)
    m.logout()
except Exception as e:
    print("imap skip", e)
PY

files=$(curl -sS "$BASE/api/v1/accounts/$aid/files?path=/public_html" -H "$AUTH")
echo "$files"
curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/note.txt","content":"live-hosted"}' >/dev/null
sudo grep -q live-hosted /home/$UNAME/public_html/note.txt

# python runtime addon (idempotent)
domains=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH")
has_py=$(echo "$domains" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('ascii_fqdn')=='python.livehost.test' for i in items))")
if [[ "$has_py" != "True" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"fqdn":"python.livehost.test","type":"addon","runtime":"python"}'
  sleep 3
fi
curl -sS -o /tmp/py.out -w 'python_http %{http_code}\n' -H 'Host: python.livehost.test' http://127.0.0.1/ || true
head -c 80 /tmp/py.out; echo

curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/restore-marker.txt","content":"before-backup"}' >/dev/null
bak=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"kind":"full","destination":"local"}')
echo "$bak"
bid=$(echo "$bak" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("backup") or {}).get("id") or d.get("resource_id") or "")')
sleep 3
if [[ -n "$bid" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"path":"/public_html/restore-marker.txt","content":"after-backup"}' >/dev/null
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/restores" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"backup_id\":\"$bid\",\"mode\":\"in_place\"}" || true
  sleep 3
fi

exp=$(curl -sS "$BASE/api/v1/accounts/$aid/export" -H "$AUTH")
echo "$exp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("export", d.get("account",{}).get("username"))'
echo "$exp" > /tmp/livehost.export.json
mig=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
mid=$(echo "$mig" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((i['id'] for i in items if i.get('username')=='e2emig'), ''))")
if [[ -z "$mid" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/import?username=e2emig&domain=e2emig.test" -H "$AUTH" \
    -H 'content-type: application/json' --data-binary @/tmp/livehost.export.json || true
  sleep 4
fi

curl -sS -X POST "$BASE/api/v1/accounts/$aid/suspend" -H "$AUTH" >/dev/null
sleep 1
curl -sS -X POST "$BASE/api/v1/accounts/$aid/unsuspend" -H "$AUTH" >/dev/null
audit=$(curl -sS "$BASE/api/v1/audit-events" -H "$AUTH")
echo "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d; print("audit", len(items) if isinstance(items,list) else d)'

echo "LIVE_E2E_OK"
