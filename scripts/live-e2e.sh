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
  for _ in $(seq 1 40); do
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
    sleep 0.5
  done
  echo "$label timeout state=$st" >&2
  return 1
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
code=$(curl -sS -o /tmp/live-site.html -w '%{http_code}' -H "Host: $DOMAIN" http://127.0.0.1/)
echo "http $code"
php=$(curl -sS -H "Host: $DOMAIN" http://127.0.0.1/index.php || true)
echo "php $php"
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
  acode=$(curl -sS -o /tmp/alias-ok.html -w '%{http_code}' -H "Host: $ADOM" http://127.0.0.1/)
  [[ "$acode" == "200" ]] && break
  sleep 0.2
done
[[ "$acode" == "200" ]] || { echo "alias $ADOM HTTP $acode" >&2; exit 1; }
sudo grep -q "$ADOM" /etc/nginx/panel-sites/*.conf || { echo "alias missing from nginx server_name" >&2; exit 1; }
echo "alias-ok $ADOM"
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
grep -n "$DOMAIN" /var/lib/panel/mail/virtual || true
grep -n "info@$DOMAIN" /var/lib/panel/mail/passwd || true

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
pycode=$(curl -sS -o /tmp/py.out -w '%{http_code}' -H 'Host: python.livehost.test' http://127.0.0.1/ || true)
echo "python_http $pycode"
head -c 80 /tmp/py.out; echo
if [[ "$pycode" != "200" ]]; then
  curl -sS -X PATCH "$BASE/api/v1/accounts/$aid" -H "$AUTH" -H 'content-type: application/json' -d '{}' >/dev/null
  sleep 3
  pycode=$(curl -sS -o /tmp/py.out -w '%{http_code}' -H 'Host: python.livehost.test' http://127.0.0.1/ || true)
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
migcode=$(curl -sS -o /tmp/e2emig.html -w '%{http_code}' -H 'Host: e2emig.test' http://127.0.0.1/)
[[ "$migcode" == "200" ]] || { echo "e2emig HTTP $migcode" >&2; exit 1; }

sus=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/suspend" -H "$AUTH")
sop=$(echo "$sus" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$sop" suspend
suscode=""
for _ in $(seq 1 20); do
  suscode=$(curl -sS -o /tmp/live-sus.html -w '%{http_code}' -H "Host: $DOMAIN" http://127.0.0.1/)
  [[ "$suscode" == "503" ]] && break
  sleep 0.2
done
[[ "$suscode" == "503" ]] || { echo "expected HTTP 503 while suspended, got $suscode" >&2; exit 1; }
uns=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/unsuspend" -H "$AUTH")
uop=$(echo "$uns" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$uop" unsuspend
uncode=""
for _ in $(seq 1 20); do
  uncode=$(curl -sS -o /tmp/live-uns.html -w '%{http_code}' -H "Host: $DOMAIN" http://127.0.0.1/)
  [[ "$uncode" == "200" ]] && break
  sleep 0.2
done
[[ "$uncode" == "200" ]] || { echo "expected HTTP 200 after unsuspend, got $uncode" >&2; exit 1; }
pycode=""
for _ in $(seq 1 20); do
  pycode=$(curl -sS -o /tmp/py-uns.out -w '%{http_code}' -H 'Host: python.livehost.test' http://127.0.0.1/ || true)
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
  tcode=$(curl -sS -o /tmp/termacc.html -w '%{http_code}' -H "Host: $TDOM" http://127.0.0.1/)
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
  gone=$(curl -sS -o /tmp/term-gone.html -w '%{http_code}' -H "Host: $TDOM" http://127.0.0.1/)
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
  bcode=$(curl -sS -o /tmp/bw-ok.html -w '%{http_code}' -H "Host: $BDOM" http://127.0.0.1/)
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
  hold=$(curl -sS -o /tmp/bw-hold.html -w '%{http_code}' -H "Host: $BDOM" http://127.0.0.1/)
  [[ "$hold" == "509" ]] && break
  sleep 0.2
done
[[ "$hold" == "509" ]] || { echo "expected HTTP 509 bandwidth hold, got $hold" >&2; cat /tmp/bw-hold.html >&2; exit 1; }
grep -q 'bandwidth limit exceeded' /tmp/bw-hold.html || { echo "509 body missing hold text" >&2; exit 1; }
bterm=$(curl -sS -X POST "$BASE/api/v1/accounts/$bid/terminate" -H "$AUTH")
btop=$(echo "$bterm" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
wait_job "$btop" bw-terminate

audit=$(curl -sS "$BASE/api/v1/audit-events" -H "$AUTH")
echo "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or d
assert isinstance(items, list) and len(items)>0
acts={i.get("action") for i in items if isinstance(i, dict)}
need={"account.export","account.suspend","account.terminate"}
missing=need-acts
assert not missing, missing
print("audit", len(items), "actions_ok")'

echo "LIVE_E2E_OK"
