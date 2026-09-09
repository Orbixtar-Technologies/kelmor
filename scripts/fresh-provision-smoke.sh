#!/usr/bin/env bash
set -euo pipefail
# Empty-disk guest smoke: login → provision → HTTP/PHP isolation → mail → backup → suspend → audit.

API="${PANEL_API_ADDR:-127.0.0.1:18080}"
BASE="http://$API"
USER="${PANEL_ADMIN_USER:-admin}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
UNAME="${PANEL_SMOKE_USER:-freshhost}"
DOMAIN="${PANEL_SMOKE_DOMAIN:-freshhost.test}"
TENANT_PASS="${PANEL_TENANT_PASSWORD:-TenantPass!2026}"
WAIT_ITERS="${PANEL_JOB_WAIT_ITERS:-180}"
REQUIRE_DIRECTOR="${PANEL_REQUIRE_DIRECTOR:-0}"

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
    -d "{\"username\":\"$UNAME\",\"primary_domain\":\"$DOMAIN\",\"package_id\":\"$pkg\",\"owner_email\":\"ops@$DOMAIN\",\"owner_password\":\"$TENANT_PASS\"}")
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

api_user=$(ps -o user= -C panel-api 2>/dev/null | awk 'NR==1{print $1}')
if [[ -z "$api_user" ]]; then
  api_user=$(ps -o user= -C kelmor-api 2>/dev/null | awk 'NR==1{print $1}')
fi
[[ -n "$api_user" && "$api_user" != "root" ]] || { echo "privilege zone A violated: panel-api user='$api_user'" >&2; exit 1; }
sudo test -S /run/panel/agent.sock || { echo "missing Kelmor Agent socket /run/panel/agent.sock" >&2; exit 1; }
echo "privilege-ok api_user=$api_user"

if curl -sk --max-time 8 -o /tmp/director.html -w '%{http_code}' https://127.0.0.1:8443/ | grep -qE '200|301|302'; then
  grep -qi 'Kelmor' /tmp/director.html || { echo "Director HTML missing Kelmor brand" >&2; cat /tmp/director.html >&2; exit 1; }
  echo "director-ok"
else
  echo "director-https missing"
  if [[ "$REQUIRE_DIRECTOR" == "1" ]]; then
    echo "expected Kelmor Director on :8443" >&2
    exit 1
  fi
fi

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
echo "$php" | grep -q '^php ' || { echo "index.php did not execute PHP: $php" >&2; exit 1; }
pool="/etc/php/8.3/fpm/pool.d/panel-${UNAME}.conf"
sudo test -f "$pool" || { echo "missing php-fpm pool $pool" >&2; exit 1; }
sudo grep -q "user = ${UNAME}" "$pool" || { echo "php-fpm pool not isolated to ${UNAME}" >&2; exit 1; }
sudo grep -q 'open_basedir' "$pool" || { echo "php-fpm pool missing open_basedir" >&2; exit 1; }
sudo test -S "/run/php/panel-${UNAME}.sock" || { echo "php-fpm socket missing for $UNAME" >&2; exit 1; }
systemctl is-active php8.3-fpm >/dev/null || { echo "php8.3-fpm is not active" >&2; systemctl status php8.3-fpm --no-pager >&2 || true; exit 1; }
stat -c '%a %U:%G' "/home/${UNAME}" | grep -Eq '751 root:root' || { echo "home perms not isolated" >&2; exit 1; }
echo "linux-isolation ok"
echo "php-fpm-ok"

sudo test -f "/var/lib/panel/dns/zones/${DOMAIN}.zone" || { echo "missing DNS zone file for $DOMAIN" >&2; exit 1; }
a=$(dig +short @"127.0.0.1" "$DOMAIN" A | tail -n1 || true)
echo "dns-a $a"
[[ -n "$a" ]] || { echo "PowerDNS returned no A for $DOMAIN" >&2; sudo cat "/var/lib/panel/dns/zones/${DOMAIN}.zone" >&2; exit 1; }
echo "dns-ok"

assert_customer_tls() {
  local dir issuer https_code certs cid cop want
  dir=$(sudo cat /var/lib/panel/acme.directory 2>/dev/null | tr -d '[:space:]' || true)
  want=""
  if echo "$dir" | grep -qiE 'pebble|:14000'; then
    want=pebble
    if ! timeout 1 bash -c 'echo >/dev/tcp/127.0.0.1/14000' 2>/dev/null; then
      sudo systemctl start pebble.service >/dev/null 2>&1 || true
      sleep 1
    fi
  elif echo "$dir" | grep -qi staging; then
    want=staging
  elif echo "$dir" | grep -qi letsencrypt.org; then
    want=letsencrypt
  elif [[ -z "$dir" ]]; then
    want=panel-dev
  fi

  certs=$(curl -sS "$BASE/api/v1/accounts/$aid/certificates" -H "$AUTH")
  cid=$(echo "$certs" | python3 -c "import json,sys
items=json.load(sys.stdin).get('items') or []
print(next((i['id'] for i in items if i.get('hostname')=='$DOMAIN' and i.get('status')=='active'), ''))")
  if [[ -z "$cid" ]]; then
    echo "customer-tls requesting $DOMAIN"
    cop=$(curl -sS -X POST "$BASE/api/v1/accounts/$aid/certificates" -H "$AUTH" -H 'content-type: application/json' \
      -d "{\"hostname\":\"$DOMAIN\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("operation_id",""))')
    wait_job "$cop" customer-tls
  fi

  sudo test -f "/var/lib/panel/certs/${DOMAIN}.crt" || { echo "missing customer cert file for $DOMAIN" >&2; exit 1; }
  issuer=$(sudo openssl x509 -in "/var/lib/panel/certs/${DOMAIN}.crt" -noout -issuer -subject || true)
  echo "tls-issuer $issuer"
  echo "tls-directory ${dir:-empty}"
  case "$want" in
    pebble)
      echo "$issuer" | grep -qi pebble || { echo "expected Pebble-issued customer cert, got: $issuer" >&2; exit 1; }
      ;;
    staging)
      echo "$issuer" | grep -qiE 'staging|fake le|let.s encrypt' || { echo "expected Let's Encrypt staging cert, got: $issuer" >&2; exit 1; }
      ;;
    letsencrypt)
      echo "$issuer" | grep -qiE 'let.s encrypt' || { echo "expected Let's Encrypt cert, got: $issuer" >&2; exit 1; }
      ;;
    panel-dev)
      echo "$issuer" | grep -qiE 'kelmor|pebble|let.s encrypt' || { echo "expected panel-dev or ACME cert, got: $issuer" >&2; exit 1; }
      ;;
    *)
      echo "unknown ACME directory $dir; not treating as success" >&2
      exit 1
      ;;
  esac
  https_code=$(curl -sk -o /tmp/live-https.html -w '%{http_code}' --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/" || true)
  echo "https $https_code"
  [[ "$https_code" == "200" ]] || { echo "expected HTTPS 200 for $DOMAIN, got $https_code" >&2; exit 1; }
  echo "customer-tls-ok issuer=$want"
}

assert_customer_tls

dbn="${UNAME}_db"
sudo mariadb -N -e "SHOW DATABASES" | grep -qx "$dbn" || { echo "default MariaDB $dbn missing" >&2; sudo mariadb -e "SHOW DATABASES" >&2; exit 1; }
sudo test -f "/home/${UNAME}/.panel-database.mariadb.${dbn}" || { echo "missing MariaDB credential file" >&2; exit 1; }
echo "mariadb-ok $dbn"

sudo grep -q "info@${DOMAIN}" /var/lib/panel/mail/passwd || { echo "info@$DOMAIN missing from mail passwd" >&2; sudo cat /var/lib/panel/mail/passwd >&2; exit 1; }
if sudo grep "info@${DOMAIN}" /var/lib/panel/mail/passwd | grep -q ':!:'; then
  echo "info@$DOMAIN still has stub hash !" >&2
  exit 1
fi
if command -v doveadm >/dev/null; then
  sudo doveadm reload >/dev/null 2>&1 || true
  imap_ok=0
  for _ in $(seq 1 20); do
    if sudo doveadm auth test "info@${DOMAIN}" "$TENANT_PASS" 2>&1 | grep -q succeeded; then
      imap_ok=1
      break
    fi
    sleep 1
  done
  [[ "$imap_ok" == "1" ]] || { echo "doveadm auth failed for info@$DOMAIN with owner password" >&2; exit 1; }
  echo "mailbox-ok info@$DOMAIN"
else
  echo "doveadm missing; passwd-file hash checked only"
fi

getent group panel-sftp | grep -q "$UNAME" || { echo "$UNAME not in panel-sftp" >&2; getent group panel-sftp >&2; exit 1; }
sudo test -f /etc/ssh/sshd_config.d/panel-sftp.conf || { echo "missing panel-sftp.conf" >&2; exit 1; }
sudo grep -q 'ForceCommand internal-sftp' /etc/ssh/sshd_config.d/panel-sftp.conf || { echo "sftp ForceCommand missing" >&2; exit 1; }
sudo grep -q 'PasswordAuthentication yes' /etc/ssh/sshd_config.d/panel-sftp.conf || { echo "sftp Match missing PasswordAuthentication yes" >&2; exit 1; }
if ! command -v sshpass >/dev/null; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq sshpass
fi
printf 'ls\n' | sshpass -p "$TENANT_PASS" sftp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=password -o PubkeyAuthentication=no \
  -o NumberOfPasswordPrompts=1 "${UNAME}@127.0.0.1" >/tmp/sftp-ls.txt 2>/tmp/sftp-ls.err
if ! grep -Eq 'public_html|index.html|index.php' /tmp/sftp-ls.txt; then
  echo "sftp session failed" >&2
  cat /tmp/sftp-ls.txt /tmp/sftp-ls.err >&2
  exit 1
fi
echo "sftp-ok"

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
