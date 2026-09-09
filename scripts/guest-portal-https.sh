#!/usr/bin/env bash
# Prove Server/Account portals speak TLS on a live guest.
set -euo pipefail
curl -sk -o /tmp/sp.html -w "https8443:%{http_code}\n" https://127.0.0.1:8443/
grep -q 'id="root"' /tmp/sp.html
grep -q 'Server Portal' /tmp/sp.html
curl -sk -o /tmp/ap.html -w "https8444:%{http_code}\n" https://127.0.0.1:8444/
grep -q 'Account Portal' /tmp/ap.html
curl -sk -o /dev/null -w "healthz:%{http_code}\n" https://127.0.0.1:8443/healthz | grep -q 200
sudo grep -q 'listen 8443 ssl' /etc/nginx/panel-sites/90-server-portal.conf
sudo test -f /var/lib/panel/certs/panel-portals.crt
HOST="${PANEL_HOSTNAME:-panel.example.net}"
if sudo test -f "/var/lib/panel/certs/${HOST}.crt"; then
  echo "hostname_cert:${HOST}"
  sudo openssl x509 -in "/var/lib/panel/certs/${HOST}.crt" -noout -issuer -subject
  curl -sk --resolve "${HOST}:8443:127.0.0.1" -o /dev/null -w "sni8443:%{http_code}\n" "https://${HOST}:8443/"
fi
echo GUEST_PORTAL_HTTPS_OK
