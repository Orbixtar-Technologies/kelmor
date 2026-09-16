#!/usr/bin/env bash
# Prove Kelmor Director / Kelmor Control speak TLS on a live guest.
set -euo pipefail
curl -sk -o /tmp/sp.html -w "https2087:%{http_code}\n" https://127.0.0.1:2087/
grep -q 'id="root"' /tmp/sp.html
grep -q 'Kelmor Director' /tmp/sp.html
curl -sk -o /tmp/ap.html -w "https2083:%{http_code}\n" https://127.0.0.1:2083/
grep -q 'Kelmor Control' /tmp/ap.html
curl -sk -o /dev/null -w "healthz:%{http_code}\n" https://127.0.0.1:2087/healthz | grep -q 200
sudo grep -q 'listen 2087 ssl' /etc/nginx/panel-sites/90-server-portal.conf
sudo test -f /var/lib/panel/certs/panel-portals.crt
HOST="${PANEL_HOSTNAME:-panel.example.net}"
if sudo test -f "/var/lib/panel/certs/${HOST}.crt"; then
  echo "hostname_cert:${HOST}"
  sudo openssl x509 -in "/var/lib/panel/certs/${HOST}.crt" -noout -issuer -subject
  curl -sk --resolve "${HOST}:2087:127.0.0.1" -o /dev/null -w "sni2087:%{http_code}\n" "https://${HOST}:2087/"
fi
echo GUEST_PORTAL_HTTPS_OK
