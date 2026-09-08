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

wait_job() {
  local jid="$1" label="${2:-job}"
  [[ -z "$jid" ]] && return 0
  local st=""
  for _ in $(seq 1 90); do
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

# Issued certificates enable HTTPS redirects. Follow to the local vhost.
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

for i in $(seq 1 40); do
  acc=$(curl -sS "$BASE/api/v1/accounts/$aid" -H "$AUTH")
  st=$(echo "$acc" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "account status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done

getent passwd "$UNAME" || true
dnsjob=$(curl -sS -X PATCH "$BASE/api/v1/accounts/$aid" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"package_id\":\"$pkg\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$dnsjob" public-dns
pub=$(grep -E '^PANEL_PUBLIC_IPV4=' /var/lib/panel/public.env 2>/dev/null | cut -d= -f2- || true)
a=$(dig +short @"127.0.0.1" "$DOMAIN" A | tail -n1)
echo "public-a $a public-ip $pub"
if [[ -n "$pub" && "$pub" != "127.0.0.1" ]]; then
  [[ "$a" == "$pub" ]] || { echo "expected $DOMAIN A $pub, got $a" >&2; cat /var/lib/panel/dns/zones/$DOMAIN.zone >&2; exit 1; }
fi
code=$(host_fetch "$DOMAIN" /tmp/live-site.html)
echo "http $code"
phpcode=$(host_fetch "$DOMAIN" /tmp/live-php.html /index.php)
php=$(cat /tmp/live-php.html || true)
echo "php $phpcode $php"
[[ "$code" == "200" ]] || { echo "expected HTTP 200 for $DOMAIN, got $code" >&2; exit 1; }
ADOM="www.$DOMAIN"
existing_alias=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$ADOM'), ''))")
if [[ -z "$existing_alias" ]]; then
  aliasj=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"fqdn\":\"$ADOM\",\"type\":\"alias\"}")
  echo "$aliasj"
  aop=$(echo "$aliasj" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$aop" alias-provision
fi
acode=""
for _ in $(seq 1 20); do
  acode=$(host_fetch "$ADOM" /tmp/alias-ok.html)
  [[ "$acode" == "200" ]] && break
  sleep 0.2
done
[[ "$acode" == "200" ]] || { echo "alias $ADOM HTTP $acode" >&2; exit 1; }
sudo grep -q "$ADOM" /etc/nginx/panel-sites/*.conf || { echo "alias missing from nginx server_name" >&2; exit 1; }
echo "alias-ok $ADOM"

pwid=$(curl -sS "$BASE/api/v1/accounts/$aid/websites" -H "$AUTH" | python3 -c "import json,sys
d=json.load(sys.stdin); items=d.get('items') or []
print(next((i['id'] for i in items if i.get('document_root','').endswith('/public_html')), items[0]['id'] if items else ''))")
[[ -n "$pwid" ]] || { echo "primary website missing" >&2; exit 1; }
pdel=$(curl -sS -o /tmp/primary-web-del.json -w '%{http_code}' -X DELETE "$BASE/api/v1/accounts/$aid/websites/$pwid" -H "$AUTH")
[[ "$pdel" == "409" ]] || { echo "expected 409 deleting primary website, got $pdel" >&2; cat /tmp/primary-web-del.json >&2; exit 1; }
echo "primary-website-protected"

RDOM="retire.$DOMAIN"
existing_retire=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$RDOM'), ''))")
if [[ -z "$existing_retire" ]]; then
  retj=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"fqdn\":\"$RDOM\",\"type\":\"subdomain\"}")
  echo "$retj"
  rop=$(echo "$retj" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$rop" retire-domain
fi
rwid=$(curl -sS "$BASE/api/v1/accounts/$aid/websites" -H "$AUTH" | python3 -c "import json,sys
d=json.load(sys.stdin); items=d.get('items') or []
print(next((i['id'] for i in items if '$RDOM' in (i.get('document_root') or '')), ''))")
if [[ -z "$rwid" ]]; then
  rdid=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$RDOM'), ''))")
  [[ -n "$rdid" ]] || { echo "retire domain missing for $RDOM" >&2; exit 1; }
  sitej=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/websites" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"domain_id\":\"$rdid\",\"runtime\":\"php\"}")
  echo "$sitej"
  sop=$(echo "$sitej" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$sop" retire-website-apply
  rwid=$(echo "$sitej" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("website") or {}).get("id",""))')
fi
[[ -n "$rwid" ]] || { echo "retire website missing for $RDOM" >&2; exit 1; }
rcode=""
for _ in $(seq 1 20); do
  rcode=$(host_fetch "$RDOM" /tmp/retire-ok.html)
  [[ "$rcode" == "200" ]] && break
  sleep 0.2
done
[[ "$rcode" == "200" ]] || { echo "retire $RDOM HTTP $rcode before delete" >&2; exit 1; }
sudo test -f "/etc/nginx/panel-sites/${rwid}.conf" || { echo "expected vhost $rwid" >&2; exit 1; }
rdel=$(curl -sS -X DELETE "$BASE/api/v1/accounts/$aid/websites/$rwid" -H "$AUTH")
echo "$rdel"
rdop=$(echo "$rdel" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$rdop" website-delete
gone=""
for _ in $(seq 1 20); do
  gone=$(curl -sS "$BASE/api/v1/accounts/$aid/websites" -H "$AUTH" | python3 -c "import json,sys
items=json.load(sys.stdin).get('items') or []
print('gone' if not any(i.get('id')=='$rwid' for i in items) else 'remain')")
  [[ "$gone" == "gone" ]] && break
  sleep 0.2
done
[[ "$gone" == "gone" ]] || { echo "website $rwid still listed" >&2; exit 1; }
sudo test ! -f "/etc/nginx/panel-sites/${rwid}.conf" || { echo "vhost $rwid remains" >&2; exit 1; }
sudo test -d "/home/$UNAME" || { echo "home removed after website retire" >&2; exit 1; }
sudo test -f "/etc/php/8.3/fpm/pool.d/panel-${UNAME}.conf" || { echo "php pool removed after website retire" >&2; exit 1; }
pcode=$(host_fetch "$DOMAIN" /tmp/live-site-after-retire.html)
[[ "$pcode" == "200" ]] || { echo "primary $DOMAIN HTTP $pcode after addon retire" >&2; exit 1; }
echo "website-retire-ok $RDOM"
rdid=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$RDOM'), ''))")
[[ -n "$rdid" ]] || { echo "retire domain id missing" >&2; exit 1; }
ddel=$(curl -sS -X DELETE "$BASE/api/v1/accounts/$aid/domains/$rdid" -H "$AUTH")
echo "$ddel"
ddop=$(echo "$ddel" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$ddop" domain-delete
dleft=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print('remain' if any(i.get('ascii_fqdn')=='$RDOM' for i in items) else 'gone')")
[[ "$dleft" == "gone" ]] || { echo "domain $RDOM still listed" >&2; exit 1; }
sudo test ! -f "/var/lib/panel/dns/zones/${RDOM}.zone" || { echo "zone $RDOM remains" >&2; exit 1; }
pcode=$(host_fetch "$DOMAIN" /tmp/live-site-after-domain-retire.html)
[[ "$pcode" == "200" ]] || { echo "primary $DOMAIN HTTP $pcode after domain retire" >&2; exit 1; }
echo "domain-retire-ok $RDOM"
dig +short @"127.0.0.1" "$DOMAIN" A || true

mds=$(curl -sS "$BASE/api/v1/accounts/$aid/mail/domains" -H "$AUTH")
mdid=$(echo "$mds" | python3 -c "import json,sys; d=json.load(sys.stdin); items=d.get('items') or [];
print(next((i['id'] for i in items if i.get('ascii_fqdn')=='$DOMAIN'), items[0]['id'] if items else ''))")
if [[ -n "$mdid" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/mail/mailboxes" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"domain_id\":\"$mdid\",\"local_part\":\"info\",\"password\":\"MailboxPass!2026\"}" >/tmp/mbox.json
  mjob=$(python3 -c 'import json; print(json.load(open("/tmp/mbox.json")).get("operation_id",""))')
  for i in $(seq 1 20); do
    if [[ -z "$mjob" ]]; then break; fi
    st=$(curl -sS "$BASE/api/v1/jobs/$mjob" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("state",""))')
    echo "mailbox job=$st"
    [[ "$st" == "succeeded" || "$st" == "failed" ]] && break
    sleep 1
  done
fi
if [[ -n "$mdid" ]]; then
  policy=$(echo "$mds" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i.get('catchall_policy') or 'reject' for i in items if i.get('id')=='$mdid'), 'reject'))")
  maps=$(curl -sS -X PATCH "$BASE/api/v1/accounts/$aid/mail/domains/$mdid" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"catchall_policy\":\"$policy\"}")
  mapsjob=$(echo "$maps" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$mapsjob" mail-maps
fi
grep -n "$DOMAIN" /var/lib/panel/mail/virtual || true
grep -n "info@$DOMAIN" /var/lib/panel/mail/passwd || true
sudo grep -n "info@$DOMAIN" /var/lib/panel/mail/sender-login || true

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
python3 - <<PY
import smtplib
from email.mime.text import MIMEText
msg = MIMEText("panel submission 587")
msg["Subject"] = "panel submission"
msg["From"] = "info@$DOMAIN"
msg["To"] = "info@$DOMAIN"
with smtplib.SMTP("127.0.0.1", 587, timeout=10) as s:
    s.starttls()
    s.login("info@$DOMAIN", "MailboxPass!2026")
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
print("submission accepted")
PY
python3 - <<PY
import smtplib
from email.mime.text import MIMEText
try:
    with smtplib.SMTP("127.0.0.1", 587, timeout=10) as s:
        s.starttls()
        s.login("info@$DOMAIN", "MailboxPass!2026")
        msg = MIMEText("forged from")
        msg["Subject"] = "forged"
        msg["From"] = "forged@$DOMAIN"
        msg["To"] = "info@$DOMAIN"
        s.sendmail("forged@$DOMAIN", ["info@$DOMAIN"], msg.as_string())
    raise SystemExit("sender mismatch was accepted")
except (smtplib.SMTPSenderRefused, smtplib.SMTPDataError, smtplib.SMTPRecipientsRefused) as e:
    print("sender mismatch rejected")
PY
have_sales=$(curl -sS "$BASE/api/v1/accounts/$aid/mail/aliases" -H "$AUTH" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or [];
print(next((i['id'] for i in items if i.get('address')=='sales'), ''))")
if [[ -z "$have_sales" ]]; then
  aliasj=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/mail/aliases" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"domain_id\":\"$mdid\",\"address\":\"sales\",\"destination\":\"info\"}")
  echo "$aliasj"
  ajob=$(echo "$aliasj" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$ajob" mail-alias
fi
sudo grep -q "sales@$DOMAIN info@$DOMAIN" /var/lib/panel/mail/aliases || { echo "alias map missing sales@$DOMAIN" >&2; exit 1; }
sudo grep -q "sales@$DOMAIN info@$DOMAIN" /var/lib/panel/mail/sender-login || { echo "sender-login missing sales@$DOMAIN" >&2; exit 1; }
python3 - <<PY
import smtplib
from email.mime.text import MIMEText
msg = MIMEText("alias as sender")
msg["Subject"] = "alias sender"
msg["From"] = "sales@$DOMAIN"
msg["To"] = "info@$DOMAIN"
with smtplib.SMTP("127.0.0.1", 587, timeout=10) as s:
    s.starttls()
    s.login("info@$DOMAIN", "MailboxPass!2026")
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
print("alias sender accepted")
PY
python3 - <<PY
import smtplib
from email.mime.text import MIMEText
msg = MIMEText("panel alias smtp")
msg["Subject"] = "panel alias"
msg["From"] = "probe@localhost"
msg["To"] = "sales@$DOMAIN"
with smtplib.SMTP("127.0.0.1", 25, timeout=10) as s:
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
print("smtp alias accepted")
PY
if sudo test -f /var/lib/panel/mail/send-limits; then
  sudo grep -q "info@$DOMAIN" /var/lib/panel/mail/send-limits || { echo "send-limits missing info@$DOMAIN" >&2; exit 1; }
fi
if ss -lnt | grep -q ':10031 '; then
  python3 - <<'PY'
import socket
s = socket.create_connection(("127.0.0.1", 10031), 2)
s.sendall(b"request=smtpd_access_policy\nsender=probe@localhost\n\n")
print("smtp-policy", s.recv(256).decode().strip())
s.close()
PY
fi
sleep 1
sudo ls /var/vmail/$DOMAIN/info/Maildir/new 2>/dev/null | head || true
sudo doveadm reload >/dev/null 2>&1 || true
imap_ok=0
for i in $(seq 1 20); do
  if sudo doveadm auth test info@$DOMAIN 'MailboxPass!2026' 2>&1 | grep -q succeeded; then
    echo "imap-auth ok"
    imap_ok=1
    break
  fi
  sleep 1
done
python3 - <<PY
import imaplib, ssl, sys
ctx = ssl._create_unverified_context()
last = None
for _ in range(15):
    try:
        m = imaplib.IMAP4_SSL("127.0.0.1", 993, ssl_context=ctx)
        typ, _ = m.login("info@$DOMAIN", "MailboxPass!2026")
        print("imap", typ)
        m.logout()
        sys.exit(0)
    except Exception as e:
        last = e
        import time; time.sleep(1)
print("imap failed", last)
sys.exit(1)
PY
[[ "$imap_ok" == "1" ]] || { echo "doveadm auth failed" >&2; exit 1; }
sudo test -s /var/lib/panel/dkim/$DOMAIN/default.key || { echo "dkim private key missing" >&2; exit 1; }
dkim=""
for _ in $(seq 1 20); do
  dkim=$(dig +short TXT default._domainkey.$DOMAIN @127.0.0.1 | tr -d '" \n')
  echo "$dkim" | grep -q 'v=DKIM1' && break
  sudo pdns_control bind-reload-now "$DOMAIN" >/dev/null 2>&1 || true
  sleep 0.2
done
echo "dkim $dkim"
echo "$dkim" | grep -q 'v=DKIM1' || { echo "DKIM TXT missing in PowerDNS" >&2; cat /var/lib/panel/dns/zones/$DOMAIN.zone >&2; exit 1; }
sudo grep -q "$DOMAIN" /etc/rspamd/local.d/dkim_signing.conf || { echo "rspamd dkim_signing missing domain" >&2; exit 1; }

files=$(curl -sS "$BASE/api/v1/accounts/$aid/files?path=/public_html" -H "$AUTH")
echo "$files"
curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/note.txt","content":"live-hosted"}' >/dev/null
sudo grep -q live-hosted /home/$UNAME/public_html/note.txt

ftp=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/ftp" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"username":"liveftp","password":"FtpPass!2026"}')
echo "$ftp"
ftpjob=$(echo "$ftp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$ftpjob" ftp
if sudo test -f /var/lib/panel/ftp/passwd; then
  sudo grep -q '^liveftp:' /var/lib/panel/ftp/passwd || { echo "ftp passwd missing liveftp" >&2; exit 1; }
fi
if ss -lnt | grep -q ':21 '; then
  python3 - <<'PY'
from ftplib import FTP
f = FTP()
f.connect("127.0.0.1", 21, timeout=8)
f.login("liveftp", "FtpPass!2026")
print("ftp", f.nlst()[:8])
f.quit()
PY
fi

# python runtime addon (idempotent)
domains=$(curl -sS "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH")
has_py=$(echo "$domains" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('ascii_fqdn')=='python.livehost.test' for i in items))")
if [[ "$has_py" != "True" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/domains" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"fqdn":"python.livehost.test","type":"addon","runtime":"python"}'
  sleep 3
fi
pycode=$(host_fetch python.livehost.test /tmp/py.out)
echo "python_http $pycode"
head -c 80 /tmp/py.out; echo
if [[ "$pycode" != "200" ]]; then
  curl -sS -X PATCH "$BASE/api/v1/accounts/$aid" -H "$AUTH" -H 'content-type: application/json' -d '{}' >/dev/null
  sleep 3
  pycode=$(host_fetch python.livehost.test /tmp/py.out)
  echo "python_http_retry $pycode"
fi
[[ "$pycode" == "200" ]] || { echo "python site down" >&2; exit 1; }

dbs=$(curl -sS "$BASE/api/v1/accounts/$aid/databases" -H "$AUTH")
hasdb=$(echo "$dbs" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(any(i.get('name','').endswith('_e2e') for i in items))")
if [[ "$hasdb" != "True" ]]; then
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/databases" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"name":"e2e","engine":"mariadb"}'
  sleep 3
fi
curl -sS "$BASE/api/v1/accounts/$aid/databases" -H "$AUTH" | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; print("databases", [(i.get("name"), i.get("status"), i.get("engine")) for i in items])'
DBNAME="${UNAME}_e2e"
sudo mariadb "$DBNAME" -e "CREATE TABLE IF NOT EXISTS panel_restore(k varchar(32)); DELETE FROM panel_restore; INSERT INTO panel_restore VALUES ('before-backup');"
echo before-backup | sudo tee "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker" >/dev/null

curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/restore-marker.txt","content":"before-backup"}' >/dev/null
bak=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"kind":"full","destination":"local"}')
echo "$bak"
bid=$(echo "$bak" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("backup") or {}).get("id") or "")')
bop=$(echo "$bak" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$bid" ]]
wait_job "$bop" backup
skey=""
if [[ -f /var/lib/panel/secrets/backup-sftp.env ]]; then
  sbak=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"kind":"full","destination":"sftp"}')
  echo "$sbak"
  sbop=$(echo "$sbak" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  skey=$(echo "$sbak" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("backup") or {}).get("id") or "")')
  wait_job "$sbop" backup-sftp
  found=$(sudo find /var/lib/panel/offsite/inbox -type f -name '*.hpm' | head -n 1)
  [[ -n "$found" ]] || { echo "sftp offsite object missing" >&2; sudo find /var/lib/panel/offsite -ls >&2; exit 1; }
  echo "sftp-offsite $found"
  [[ -n "$skey" ]]
fi
sudo mariadb "$DBNAME" -e "DELETE FROM panel_restore;"
sudo rm -f "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/restore-marker.txt","content":"after-backup"}' >/dev/null
rst=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/restores" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"backup_id\":\"$bid\",\"mode\":\"in_place\"}")
echo "$rst"
rop=$(echo "$rst" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$rop" restore
sudo grep -q before-backup /home/$UNAME/public_html/restore-marker.txt
sudo mariadb -N "$DBNAME" -e "SELECT k FROM panel_restore LIMIT 1;" | grep -q before-backup
sudo grep -q before-backup "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
if [[ -n "${skey:-}" ]]; then
  sudo mariadb "$DBNAME" -e "DELETE FROM panel_restore;"
  sudo rm -f "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"path":"/public_html/restore-marker.txt","content":"after-sftp-backup"}' >/dev/null
  srst=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/restores" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"backup_id\":\"$skey\",\"mode\":\"in_place\"}")
  echo "$srst"
  srop=$(echo "$srst" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$srop" restore-sftp
  sudo grep -q before-backup /home/$UNAME/public_html/restore-marker.txt
  sudo mariadb -N "$DBNAME" -e "SELECT k FROM panel_restore LIMIT 1;" | grep -q before-backup
  sudo grep -q before-backup "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
  echo "sftp-restore ok"
fi
s3key=""
if [[ -f /var/lib/panel/secrets/backup-s3.env ]]; then
  s3bak=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/backups" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"kind":"full","destination":"s3"}')
  echo "$s3bak"
  s3op=$(echo "$s3bak" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  s3key=$(echo "$s3bak" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("backup") or {}).get("id") or "")')
  wait_job "$s3op" backup-s3
  found=$(sudo find /var/lib/panel/objects -type f -name '*.hpm' | head -n 1)
  [[ -n "$found" ]] || { echo "s3 object missing" >&2; sudo find /var/lib/panel/objects -ls >&2; exit 1; }
  echo "s3-object $found"
  [[ -n "$s3key" ]]
  sudo mariadb "$DBNAME" -e "DELETE FROM panel_restore;"
  sudo rm -f "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
  curl -sS -X POST "$BASE/api/v1/accounts/$aid/files" -H "$AUTH" -H 'content-type: application/json' \
    -d '{"path":"/public_html/restore-marker.txt","content":"after-s3-backup"}' >/dev/null
  s3rst=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/restores" -H "$AUTH" -H 'content-type: application/json' \
    -d "{\"backup_id\":\"$s3key\",\"mode\":\"in_place\"}")
  echo "$s3rst"
  s3rop=$(echo "$s3rst" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
  wait_job "$s3rop" restore-s3
  sudo grep -q before-backup /home/$UNAME/public_html/restore-marker.txt
  sudo mariadb -N "$DBNAME" -e "SELECT k FROM panel_restore LIMIT 1;" | grep -q before-backup
  sudo grep -q before-backup "/var/vmail/$DOMAIN/info/Maildir/new/restore-marker"
  echo "s3-restore ok"
fi

exp=$(curl -sS "$BASE/api/v1/accounts/$aid/export" -H "$AUTH")
echo "$exp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("export", d.get("account",{}).get("username"))'
echo "$exp" > /tmp/livehost.export.json
mig=$(curl -sS "$BASE/api/v1/accounts" -H "$AUTH")
mid=$(echo "$mig" | python3 -c "import json,sys; items=json.load(sys.stdin).get('items') or []; print(next((i['id'] for i in items if i.get('username')=='e2emig'), ''))")
if [[ -z "$mid" ]]; then
  imported=$(curl -sS -X POST "$BASE/api/v1/accounts/import?username=e2emig&domain=e2emig.test" -H "$AUTH" \
    -H 'content-type: application/json' --data-binary @/tmp/livehost.export.json)
  echo "$imported"
  mid=$(echo "$imported" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
  hop=$(echo "$imported" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("homedir_job",""))')
  [[ -n "$mid" ]]
  wait_job "$hop" migrate-copy
fi
for i in $(seq 1 20); do
  st=$(curl -sS "$BASE/api/v1/accounts/$mid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "e2emig status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
migcode=$(host_fetch e2emig.test /tmp/e2emig.html)
[[ "$migcode" == "200" ]] || { echo "e2emig HTTP $migcode" >&2; exit 1; }

MUSER="md$(date +%s)"
MDOM="${MUSER}.test"
DUSER="dn${MUSER:2:10}"
DDOM="${DUSER}.test"
mcreated=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"username\":\"$MUSER\",\"primary_domain\":\"$MDOM\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$MDOM\",\"owner_password\":\"TenantPass!2026\"}")
maid=$(echo "$mcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
mop=$(echo "$mcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$mop" migdata-provision
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$maid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  [[ "$st" == "active" ]] && break
  sleep 1
done
mdom=$(curl -sS "$BASE/api/v1/accounts/$maid/mail/domains" -H "$AUTH" | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; print(items[0]["id"] if items else "")')
mbox=$(curl -sS -X POST "$BASE/api/v1/accounts/$maid/mail/mailboxes" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"domain_id\":\"$mdom\",\"local_part\":\"info\",\"password\":\"MailboxPass!2026\"}")
mbop=$(echo "$mbox" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$mbop" migdata-mailbox
malias=$(curl -sS -X POST "$BASE/api/v1/accounts/$maid/mail/aliases" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"domain_id\":\"$mdom\",\"address\":\"sales\",\"destination\":\"info@$MDOM\"}")
maliasop=$(echo "$malias" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$maliasop" migdata-alias
mdb=$(curl -sS -X POST "$BASE/api/v1/accounts/$maid/databases" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"name":"app","engine":"mariadb"}')
mdop=$(echo "$mdb" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$mdop" migdata-db
sudo mariadb "${MUSER}_app" -e "CREATE TABLE panel_mig (k varchar(32) primary key); INSERT INTO panel_mig VALUES ('from-src');"
echo from-src | sudo tee "/var/vmail/$MDOM/info/Maildir/new/mig-marker" >/dev/null
curl -sS -X POST "$BASE/api/v1/accounts/$maid/files" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"path":"/public_html/mig-marker.txt","content":"from-src"}' >/dev/null
curl -sS -X POST "$BASE/api/v1/accounts/$maid/cron" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"schedule":"5 * * * *","command":"true","working_directory":"/public_html","enabled":true}' >/dev/null
mmig=$(curl -sS -X POST "$BASE/api/v1/accounts/$maid/migrate" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"username\":\"$DUSER\",\"domain\":\"$DDOM\"}")
echo "$mmig"
did=$(echo "$mmig" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
dop=$(echo "$mmig" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("homedir_job",""))')
[[ -n "$did" ]]
wait_job "$dop" migdata-reconcile
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$did" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "migdest status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
dcode=""
for _ in $(seq 1 20); do
  dcode=$(host_fetch "$DDOM" /tmp/migdest.html)
  [[ "$dcode" == "200" ]] && break
  sleep 0.2
done
[[ "$dcode" == "200" ]] || { echo "migdest HTTP $dcode" >&2; exit 1; }
sudo grep -q from-src "/home/$DUSER/public_html/mig-marker.txt" || { echo "homedir not copied" >&2; exit 1; }
sudo mariadb -N "${DUSER}_app" -e "SELECT k FROM panel_mig LIMIT 1;" | grep -q from-src || { echo "db not copied" >&2; exit 1; }
sudo grep -q from-src "/var/vmail/$DDOM/info/Maildir/new/mig-marker" || { echo "maildir not copied" >&2; exit 1; }
curl -sS "$BASE/api/v1/accounts/$did/mail/aliases" -H "$AUTH" | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; assert any(i.get("address")=="sales" and "info@" in (i.get("destination") or "") for i in items), items'
curl -sS "$BASE/api/v1/accounts/$did/cron" -H "$AUTH" | python3 -c 'import json,sys; items=json.load(sys.stdin).get("items") or []; assert any(i.get("schedule")=="5 * * * *" for i in items), items'
echo "migrate-data ok"
mterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$did/terminate" -H "$AUTH")
mtop=$(echo "$mterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$mtop" migdata-dest-terminate
sterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$maid/terminate" -H "$AUTH")
stop=$(echo "$sterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$stop" migdata-src-terminate

sus=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/suspend" -H "$AUTH")
sop=$(echo "$sus" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$sop" suspend
suscode=""
for _ in $(seq 1 20); do
  suscode=$(host_fetch "$DOMAIN" /tmp/live-sus.html)
  [[ "$suscode" == "503" ]] && break
  sleep 0.2
done
[[ "$suscode" == "503" ]] || { echo "expected HTTP 503 while suspended, got $suscode" >&2; exit 1; }
uns=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/unsuspend" -H "$AUTH")
uop=$(echo "$uns" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$uop" unsuspend
uncode=""
for _ in $(seq 1 20); do
  uncode=$(host_fetch "$DOMAIN" /tmp/live-uns.html)
  [[ "$uncode" == "200" ]] && break
  sleep 0.2
done
[[ "$uncode" == "200" ]] || { echo "expected HTTP 200 after unsuspend, got $uncode" >&2; exit 1; }
pycode=""
for _ in $(seq 1 20); do
  pycode=$(host_fetch python.livehost.test /tmp/py-uns.out)
  [[ "$pycode" == "200" ]] && break
  sleep 0.3
done
[[ "$pycode" == "200" ]] || { echo "python site down after unsuspend: $pycode" >&2; exit 1; }

TUSER="tm$(date +%s)"
TDOM="${TUSER}.test"
created=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"username\":\"$TUSER\",\"primary_domain\":\"$TDOM\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$TDOM\",\"owner_password\":\"TenantPass!2026\"}")
echo "$created"
tid=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
top=$(echo "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
[[ -n "$tid" ]]
wait_job "$top" term-provision
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$tid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "termacc status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
tcode=""
for _ in $(seq 1 20); do
  tcode=$(host_fetch "$TDOM" /tmp/termacc.html)
  [[ "$tcode" == "200" ]] && break
  sleep 0.2
done
[[ "$tcode" == "200" ]] || { echo "termacc HTTP $tcode" >&2; exit 1; }
tdb=$(curl -sS -X POST "$BASE/api/v1/accounts/$tid/databases" -H "$AUTH" -H 'content-type: application/json' \
  -d '{"name":"term","engine":"mariadb"}')
tdop=$(echo "$tdb" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$tdop" term-db
sudo mariadb -e "SHOW DATABASES" | grep -q "${TUSER}_term"
term=$(curl -sS -X POST "$BASE/api/v1/accounts/$tid/terminate" -H "$AUTH")
trop=$(echo "$term" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$trop" terminate
for i in $(seq 1 20); do
  st=$(curl -sS "$BASE/api/v1/accounts/$tid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "termacc after=$st"
  [[ "$st" == "terminated" ]] && break
  sleep 1
done
[[ "$st" == "terminated" ]] || { echo "account not terminated" >&2; exit 1; }
if getent passwd "$TUSER" >/dev/null; then
  echo "linux user $TUSER still exists" >&2
  exit 1
fi
gone=""
for _ in $(seq 1 20); do
  gone=$(host_fetch "$TDOM" /tmp/term-gone.html)
  [[ "$gone" == "404" ]] && break
  sleep 0.2
done
[[ "$gone" == "404" ]] || { echo "terminated host should 404, got $gone" >&2; cat /tmp/term-gone.html >&2; exit 1; }
if grep -q "$TDOM" /tmp/term-gone.html; then
  echo "terminated host still rendered tenant content" >&2
  exit 1
fi
if grep -q "$TDOM" /var/lib/panel/mail/virtual 2>/dev/null; then
  echo "mail map still lists $TDOM" >&2
  exit 1
fi
if sudo mariadb -e "SHOW DATABASES" | grep -q "${TUSER}_term"; then
  echo "mariadb ${TUSER}_term still exists" >&2
  exit 1
fi

BUSER="bw$(date +%s)"
BDOM="${BUSER}.test"
bwpkg=$(curl -sS -X POST "$BASE/api/v1/packages" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"name\":\"BandwidthHold ${BUSER}\",\"disk_bytes\":10737418240,\"bandwidth_bytes_monthly\":200,\"domains\":10,\"subdomains\":20,\"alias_domains\":5,\"databases\":5,\"database_users\":5,\"mailboxes\":5,\"mailbox_storage_bytes\":1073741824,\"ftp_users\":5,\"cron_jobs\":5,\"application_instances\":2,\"backup_retention_days\":7,\"cpu_percent\":100,\"memory_bytes\":536870912,\"process_limit\":50,\"io_weight\":100,\"iops\":100,\"concurrent_web_requests\":50,\"email_daily_limit\":50}")
bwpkgid=$(echo "$bwpkg" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))')
[[ -n "$bwpkgid" ]] || { echo "bandwidth package create failed: $bwpkg" >&2; exit 1; }
bcreated=$(curl -sS -X POST "$BASE/api/v1/accounts" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"username\":\"$BUSER\",\"primary_domain\":\"$BDOM\",\"package_id\":\"$bwpkgid\",\"owner_email\":\"ops@$BDOM\",\"owner_password\":\"TenantPass!2026\"}")
bid=$(echo "$bcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
bop=$(echo "$bcreated" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$bop" bw-provision
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$bid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "bwacc status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
bcode=""
for _ in $(seq 1 20); do
  bcode=$(host_fetch "$BDOM" /tmp/bw-ok.html)
  [[ "$bcode" == "200" ]] && break
  sleep 0.2
done
[[ "$bcode" == "200" ]] || { echo "bwacc HTTP $bcode before hold" >&2; exit 1; }
wid=$(curl -sS "$BASE/api/v1/accounts/$bid/websites" -H "$AUTH" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or []; print(items[0]["id"] if items else "")')
[[ -n "$wid" ]] || { echo "bwacc website missing" >&2; exit 1; }
sudo grep -q "IOWeight=100" "/etc/systemd/system/panel-account-${BUSER}.slice" || { echo "expected IOWeight on ${BUSER} slice" >&2; cat "/etc/systemd/system/panel-account-${BUSER}.slice" >&2; exit 1; }
sudo grep -q "panel_iops=100" "/etc/systemd/system/panel-account-${BUSER}.slice" || { echo "expected iops comment on slice" >&2; exit 1; }
sudo grep -q 'io_weight=100' "/var/lib/panel/cgroup/${BUSER}" || { echo "cgroup spec missing" >&2; cat "/var/lib/panel/cgroup/${BUSER}" >&2; exit 1; }
echo "cgroup-io ok"
sudo grep -q 'limit_conn panel_acct 50' "/etc/nginx/panel-sites/${wid}.conf" || { echo "expected limit_conn 50 on $wid" >&2; cat "/etc/nginx/panel-sites/${wid}.conf" >&2; exit 1; }
sudo grep -q 'limit_conn_zone $panel_account' /etc/nginx/conf.d/panel-conn-limit.conf || { echo "nginx conn zone missing" >&2; exit 1; }
sudo grep -q "$BDOM $BUSER" /etc/nginx/conf.d/panel-conn-limit.conf || { echo "conn map missing $BDOM" >&2; cat /etc/nginx/conf.d/panel-conn-limit.conf >&2; exit 1; }
echo "conn-limit ok"
now=$(date -u +'%d/%b/%Y:%H:%M:%S +0000')
echo "127.0.0.1 - - [${now}] \"GET / HTTP/1.1\" 200 500 \"-\" \"live-e2e\"" | sudo tee "/var/log/nginx/${wid}.access.log" >/dev/null
sudo chmod 644 "/var/log/nginx/${wid}.access.log"
patch=$(curl -sS -X PATCH "$BASE/api/v1/accounts/$bid" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"package_id\":\"$bwpkgid\"}")
pop=$(echo "$patch" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$pop" bw-reconcile
usage=$(curl -sS "$BASE/api/v1/accounts/$bid/usage" -H "$AUTH")
echo "$usage" | python3 -c 'import json,sys; u=json.load(sys.stdin); n=int(u.get("bandwidth_bytes") or 0); assert n>=500, u; print("bandwidth_bytes", n)'
hold=""
for _ in $(seq 1 20); do
  hold=$(host_fetch "$BDOM" /tmp/bw-hold.html)
  [[ "$hold" == "509" ]] && break
  sleep 0.2
done
[[ "$hold" == "509" ]] || { echo "expected HTTP 509 bandwidth hold, got $hold" >&2; cat /tmp/bw-hold.html >&2; exit 1; }
grep -q 'bandwidth limit exceeded' /tmp/bw-hold.html || { echo "509 body missing hold text" >&2; exit 1; }
bterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$bid/terminate" -H "$AUTH")
btop=$(echo "$bterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$btop" bw-terminate

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CUSER="cp$(date +%s)"
CDOM="${CUSER}.test"
CTREE="/var/tmp/panel-imports/${CUSER}"
sudo mkdir -p "$CTREE"
sudo cp -a "$ROOT/testdata/cpanel-acme42/." "$CTREE/"
sudo sed -i "s/acme.test/${CDOM}/g; s/acme42/${CUSER}/g" "$CTREE/userdata/user" "$CTREE/mysql.sql" "$CTREE/dnszones/acme.test.db" || true
sudo mv "$CTREE/dnszones/acme.test.db" "$CTREE/dnszones/${CDOM}.db" 2>/dev/null || true
sudo mv "$CTREE/va/info@acme.test" "$CTREE/va/info@${CDOM}" 2>/dev/null || true
sudo chmod -R a+rX "$CTREE"
cimp=$(curl -sS -X POST "$BASE/api/v1/accounts/import/cpanel" -H "$AUTH" -H 'content-type: application/json' \
  -d "{\"root\":\"$CTREE\",\"username\":\"$CUSER\"}")
echo "$cimp"
cid=$(echo "$cimp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resource_id",""))')
cjid=$(echo "$cimp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("homedir_job",""))')
[[ -n "$cid" ]] || { echo "cpanel import failed: $cimp" >&2; exit 1; }
wait_job "$cjid" cpanel-import
for i in $(seq 1 40); do
  st=$(curl -sS "$BASE/api/v1/accounts/$cid" -H "$AUTH" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  echo "cpanel acc status=$st"
  [[ "$st" == "active" ]] && break
  sleep 1
done
[[ "$st" == "active" ]] || { echo "cpanel account not active" >&2; exit 1; }
blog=""
for _ in $(seq 1 20); do
  blog=$(sudo mariadb -N -e "SELECT option_value FROM ${CUSER}_wp.wp_options WHERE option_name='blogname'" 2>/dev/null || true)
  [[ "$blog" == "Imported Blog" ]] && break
  sleep 0.5
done
[[ "$blog" == "Imported Blog" ]] || { echo "cpanel mysql.sql not replayed: ${blog}" >&2; sudo mariadb -e "SHOW DATABASES" >&2; exit 1; }
widget=$(sudo mariadb -N -e "SELECT name FROM ${CUSER}_store.products WHERE id=1")
[[ "$widget" == "Widget" ]] || { echo "cpanel store dump missing Widget" >&2; exit 1; }
echo "cpanel-mysql-replay ok"
cterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$cid/terminate" -H "$AUTH")
ctop=$(echo "$cterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$ctop" cpanel-terminate

audit=$(curl -sS "$BASE/api/v1/audit-events" -H "$AUTH")
echo "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d
assert isinstance(items, list) and len(items)>0
acts={i.get("action") for i in items if isinstance(i, dict)}
need={"account.export","account.suspend","account.terminate","website.delete","domain.delete","account.import.cpanel"}
missing=need-acts
assert not missing, missing
print("audit", len(items), "actions_ok")'

echo "LIVE_E2E_OK"
