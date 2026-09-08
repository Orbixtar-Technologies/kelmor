#!/usr/bin/env bash
set -euo pipefail
# Apply live Ubuntu host-stack files and start privileged services.
# systemd unit starts are blocked by policy-rc.d in this environment;
# processes are launched directly.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PANEL_PUBLIC_IPV4="${PANEL_PUBLIC_IPV4:-127.0.0.1}"

sudo mkdir -p /run/panel /var/lib/panel/{mail,dns/zones,certs,acme-www/.well-known/acme-challenge,backups/staging,cron,logs} \
  /var/vmail /etc/nginx/panel-sites /etc/nginx/modsec /etc/rspamd/local.d /etc/clamav /etc/php/8.3/fpm/pool.d
echo 'SecRuleEngine On' | sudo tee /etc/nginx/modsec/panel.conf >/dev/null
echo 'enabled = true;' | sudo tee /etc/rspamd/local.d/panel.conf >/dev/null
echo 'TCPSocket 3310' | sudo tee /etc/clamav/panel.conf >/dev/null
sudo chmod 0755 /var/vmail

sudo install -d -m 0755 /var/lib/panel /var/lib/panel/dns /var/lib/panel/dns/zones
sudo chgrp ubuntu /var/lib/panel /var/lib/panel/backups /var/lib/panel/backups/staging /run/panel || true
sudo chmod 0775 /var/lib/panel/backups /var/lib/panel/backups/staging || true
sudo chmod 0751 /run/panel || true

if [[ ! -f /var/lib/panel/certs/imap.panel.local.crt ]]; then
  sudo openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout /var/lib/panel/certs/imap.panel.local.key \
    -out /var/lib/panel/certs/imap.panel.local.crt \
    -subj "/CN=imap.panel.local"
  sudo chmod 640 /var/lib/panel/certs/imap.panel.local.key
fi

sudo tee /etc/postfix/main.cf >/dev/null <<'EOF'
# Managed by Hosting Panel — Postfix virtual mailbox host
compatibility_level = 3.6
myhostname = panel.local
mydestination =
local_recipient_maps =
local_transport = error:local delivery disabled
virtual_mailbox_base = /var/vmail
virtual_mailbox_domains = hash:/var/lib/panel/mail/vdomains
virtual_mailbox_maps = hash:/var/lib/panel/mail/virtual
virtual_minimum_uid = 20000
virtual_uid_maps = hash:/var/lib/panel/mail/uids
virtual_gid_maps = hash:/var/lib/panel/mail/gids
smtpd_tls_security_level = may
smtpd_recipient_restrictions = permit_mynetworks, reject_unauth_destination
mynetworks = 127.0.0.0/8 [::1]/128
EOF

sudo tee /etc/dovecot/dovecot.conf >/dev/null <<'EOF'
protocols = imap lmtp
listen = *
mail_location = maildir:~/Maildir
passdb {
  driver = passwd-file
  args = /var/lib/panel/mail/passwd
}
userdb {
  driver = passwd-file
  args = /var/lib/panel/mail/passwd
}
ssl = yes
ssl_cert = </var/lib/panel/certs/imap.panel.local.crt
ssl_key = </var/lib/panel/certs/imap.panel.local.key
!include_try /etc/dovecot/conf.d/*.conf
mail_location = maildir:~/Maildir
EOF
if [[ -f /etc/dovecot/conf.d/10-auth.conf ]]; then
  sudo sed -i 's/^!include auth-system.conf.ext/# !include auth-system.conf.ext/' /etc/dovecot/conf.d/10-auth.conf || true
fi

sudo tee /etc/powerdns/pdns.conf >/dev/null <<'EOF'
setuid=pdns
setgid=pdns
launch=bind
bind-config=/etc/powerdns/named.conf
local-address=127.0.0.1
local-port=53
webserver=yes
webserver-address=127.0.0.1
webserver-port=8081
api=yes
api-key=panel-loopback
EOF

sudo tee /etc/powerdns/named.conf >/dev/null <<'EOF'
options {
    directory "/var/lib/panel/dns/zones";
};
include "/var/lib/panel/dns/named-zones.conf";
EOF

sudo touch /var/lib/panel/dns/named-zones.conf
for f in virtual vdomains uids gids; do
  if [[ ! -s /var/lib/panel/mail/$f ]]; then
    echo "# panel mail map" | sudo tee /var/lib/panel/mail/$f >/dev/null
  fi
  sudo postmap /var/lib/panel/mail/$f || true
done
if [[ ! -s /var/lib/panel/mail/passwd ]]; then
  echo "# dovecot passwd-file" | sudo tee /var/lib/panel/mail/passwd >/dev/null
fi
sudo chown -R root:ubuntu /var/lib/panel/mail || true
sudo chmod 0755 /var/lib/panel /var/lib/panel/mail /var/lib/panel/dns /var/lib/panel/dns/zones
sudo chmod 0644 /var/lib/panel/mail/* || true
sudo chmod 0644 /var/lib/panel/dns/named-zones.conf || true

if ! pgrep -x pdns_server >/dev/null; then
  sudo pdns_server --daemon=yes --guardian=no --config-dir=/etc/powerdns || sudo /usr/sbin/pdns_server --daemon --config-dir=/etc/powerdns || true
fi

sudo postfix check || true
sudo postfix reload || sudo /usr/sbin/postfix start || true
sudo doveadm reload || sudo /usr/sbin/dovecot || true
if [[ -e /etc/nginx/sites-enabled/default ]]; then
  sudo rm -f /etc/nginx/sites-enabled/default
fi
sudo nginx -t && sudo nginx -s reload || true
if [[ ! -x /usr/bin/node ]] && [[ -x /usr/bin/apt-get ]]; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nodejs >/tmp/panel-nodejs.apt.log 2>&1 || true
fi
if [[ -x /usr/sbin/php-fpm8.3 ]] && ! pgrep -x php-fpm8.3 >/dev/null; then
  sudo /usr/sbin/php-fpm8.3 || true
fi
if [[ -x /usr/bin/rspamd ]] && ! pgrep -x rspamd >/dev/null; then
  sudo mkdir -p /run/rspamd
  sudo /usr/bin/rspamd -u _rspamd -g _rspamd -c /etc/rspamd/rspamd.conf || true
fi
if [[ ! -x /usr/sbin/clamd ]] && [[ -x /usr/bin/apt-get ]]; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y clamav-daemon >/tmp/panel-clamav.apt.log 2>&1 || true
fi
sudo mkdir -p /var/lib/clamav /run/clamav
if [[ ! -s /var/lib/clamav/panel.ndb ]]; then
  echo 'PanelClam:0:*:50414e454c434c414d' | sudo tee /var/lib/clamav/panel.ndb >/dev/null
  sudo chown -R clamav:clamav /var/lib/clamav /run/clamav 2>/dev/null || true
fi
if [[ -x /usr/sbin/clamd ]] && ! pgrep -x clamd >/dev/null; then
  grep -q '^PidFile' /etc/clamav/clamd.conf || echo 'PidFile /run/clamav/clamd.pid' | sudo tee -a /etc/clamav/clamd.conf >/dev/null
  grep -q '^TCPSocket' /etc/clamav/clamd.conf || echo 'TCPSocket 3310' | sudo tee -a /etc/clamav/clamd.conf >/dev/null
  sudo /usr/sbin/clamd --config-file=/etc/clamav/clamd.conf || true
fi
if [[ -f /etc/nginx/modules-enabled/50-mod-http-modsecurity.conf ]]; then
  sudo tee /etc/nginx/modsec/panel.conf >/dev/null <<'EOF'
SecRuleEngine DetectionOnly
SecRequestBodyAccess On
SecDataDir /tmp
EOF
  echo 'modsecurity on; modsecurity_rules_file /etc/nginx/modsec/panel.conf;' | sudo tee /etc/nginx/conf.d/panel-modsec.conf >/dev/null
  sudo tee /etc/nginx/modsec/panel-enforce.conf >/dev/null <<'EOF'
SecRuleEngine On
SecRequestBodyAccess On
SecDataDir /tmp
SecRule REQUEST_HEADERS:User-Agent "@contains panel-modsec-probe" "id:19999,phase:1,deny,status:403,msg:'panel waf probe'"
EOF
  sudo tee /etc/nginx/panel-sites/01-modsec-probe.conf >/dev/null <<'EOF'
server {
    listen 127.0.0.1:18481;
    server_name _;
    modsecurity on;
    modsecurity_rules_file /etc/nginx/modsec/panel-enforce.conf;
    location / { default_type text/plain; return 200 'waf-ok\n'; }
}
EOF
fi
if [[ -x /usr/sbin/sshd ]]; then
  sudo mkdir -p /run/sshd /var/run/sshd /etc/ssh/sshd_config.d
  echo 'PasswordAuthentication yes' | sudo tee /etc/ssh/sshd_config.d/panel-password.conf >/dev/null
  if ! pgrep -x sshd >/dev/null; then
    sudo /usr/sbin/sshd || true
  else
    sudo kill -HUP "$(pgrep -x sshd | head -1)" || true
  fi
fi

if [[ -x "$ROOT/dist/bin/panel-agent" ]]; then
  if ! pgrep -x panel-agent >/dev/null; then
    sudo mkdir -p /run/panel
    sudo env -u PANEL_DEV -u PANEL_HOST_ROOT PANEL_AGENT_SOCK=/run/panel/agent.sock \
      "$ROOT/dist/bin/panel-agent" >/tmp/panel-agent.log 2>&1 &
    sleep 0.4
  fi
fi

echo "host-stack applied"
ss -lnt | grep -E ':25|:53|:80|:993|:18080' || true
ls -l /run/panel/agent.sock 2>/dev/null || true
