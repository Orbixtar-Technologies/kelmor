#!/usr/bin/env bash
# Post-reboot guest checks. Must not repair units — a failed unit is a failed proof.
set -euo pipefail

API="${PANEL_API_ADDR:-127.0.0.1:18080}"
BASE="http://$API"
USER="${PANEL_ADMIN_USER:-admin}"
PASS="${PANEL_ADMIN_PASSWORD:-ChangeMeOnce!2026}"
UNAME="${PANEL_SMOKE_USER:-freshhost}"
DOMAIN="${PANEL_SMOKE_DOMAIN:-freshhost.test}"
TENANT_PASS="${PANEL_TENANT_PASSWORD:-TenantPass!2026}"

need_active() {
  local u
  for u in "$@"; do
    if ! systemctl is-active --quiet "$u"; then
      echo "unit not active after reboot: $u" >&2
      systemctl status "$u" --no-pager -l >&2 || true
      exit 1
    fi
    echo "unit-ok $u"
  done
}

need_active panel-agent panel-api panel-worker nginx php8.3-fpm postgresql \
  postfix dovecot pdns mariadb

api_user=$(ps -o user= -C panel-api 2>/dev/null | awk 'NR==1{print $1}')
[[ -n "$api_user" && "$api_user" != "root" ]] || { echo "privilege zone A violated: panel-api user='$api_user'" >&2; exit 1; }
sudo test -S /run/panel/agent.sock || { echo "missing agent socket after reboot" >&2; exit 1; }
echo "privilege-ok api_user=$api_user"

login=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
token=$(echo "$login" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))')
[[ -n "$token" ]] || { echo "login failed after reboot: $login" >&2; exit 1; }
echo "login-ok"

curl -sk --max-time 8 -o /tmp/director.html -w '%{http_code}' https://127.0.0.1:8443/ \
  | grep -qE '200|301|302' || { echo "Director :8443 down after reboot" >&2; exit 1; }
grep -qi 'Kelmor' /tmp/director.html || { echo "Director HTML missing Kelmor after reboot" >&2; exit 1; }
echo "director-ok"

curl -sk --max-time 8 -o /tmp/control.html -w '%{http_code}' https://127.0.0.1:8444/ \
  | grep -qE '200|301|302' || { echo "Control :8444 down after reboot" >&2; exit 1; }
grep -qi 'Kelmor Control' /tmp/control.html || { echo "Control HTML missing brand after reboot" >&2; exit 1; }
echo "control-html-ok"

getent passwd "$UNAME" >/dev/null || { echo "tenant $UNAME missing after reboot" >&2; exit 1; }

code=$(curl -sS -o /tmp/reboot-site.html -w '%{http_code}' -H "Host: $DOMAIN" "http://127.0.0.1/" || true)
if [[ "$code" == "301" || "$code" == "302" ]]; then
  code=$(curl -sk -o /tmp/reboot-site.html -w '%{http_code}' --resolve "$DOMAIN:443:127.0.0.1" "https://$DOMAIN/" || true)
fi
[[ "$code" == "200" ]] || { echo "tenant HTTP $code after reboot" >&2; exit 1; }
echo "http-ok"

php=$(curl -sk --resolve "$DOMAIN:443:127.0.0.1" "https://$DOMAIN/index.php" || curl -sS -H "Host: $DOMAIN" "http://127.0.0.1/index.php" || true)
echo "php $php"
echo "$php" | grep -q '^php ' || { echo "PHP did not execute after reboot: $php" >&2; exit 1; }
sudo test -S "/run/php/panel-${UNAME}.sock" || { echo "php-fpm socket missing after reboot" >&2; exit 1; }
echo "php-ok"

a=$(dig +short @"127.0.0.1" "$DOMAIN" A | tail -n1 || true)
echo "dns-a $a"
[[ -n "$a" ]] || { echo "PowerDNS empty after reboot" >&2; exit 1; }
echo "dns-ok"

dbn="${UNAME}_db"
sudo mariadb -N -e "SHOW DATABASES" | grep -qx "$dbn" || { echo "MariaDB $dbn missing after reboot" >&2; exit 1; }
echo "mariadb-ok"

if ! command -v sshpass >/dev/null; then
  echo "sshpass missing; SFTP check skipped would be a lie" >&2
  exit 1
fi
printf 'ls\n' | sshpass -p "$TENANT_PASS" sftp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=password -o PubkeyAuthentication=no \
  -o NumberOfPasswordPrompts=1 "${UNAME}@127.0.0.1" >/tmp/sftp-reboot.txt 2>/tmp/sftp-reboot.err
grep -Eq 'public_html|index.html|index.php' /tmp/sftp-reboot.txt || {
  echo "sftp failed after reboot" >&2
  cat /tmp/sftp-reboot.txt /tmp/sftp-reboot.err >&2
  exit 1
}
echo "sftp-ok"

if ! systemctl is-active --quiet dovecot; then
  echo "dovecot unit not active after reboot" >&2
  exit 1
fi
sudo doveadm auth test "info@${DOMAIN}" "$TENANT_PASS" 2>&1 | grep -q succeeded || {
  echo "mailbox auth failed after reboot" >&2
  exit 1
}
echo "mailbox-ok"

if [[ -f /var/lib/panel/acme.directory ]] && grep -qiE 'pebble|:14000' /var/lib/panel/acme.directory; then
  if ! systemctl is-active --quiet pebble; then
    echo "pebble unit not active after reboot (lab ACME)" >&2
    systemctl status pebble --no-pager -l >&2 || true
    exit 1
  fi
  echo "pebble-unit-ok"
fi

sudo test -f "/var/lib/panel/certs/${DOMAIN}.crt" || { echo "customer cert missing after reboot" >&2; exit 1; }
https_code=$(curl -sk -o /tmp/reboot-https.html -w '%{http_code}' --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/" || true)
[[ "$https_code" == "200" ]] || { echo "customer HTTPS $https_code after reboot" >&2; exit 1; }
echo "customer-tls-ok"

echo "KELMOR_REBOOT_HEALTH_OK"
