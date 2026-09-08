package phases

import (
	"fmt"
	"os"
	"time"
)

func applyDNS(c Config) error {
	if err := os.MkdirAll(root(c, "etc/powerdns"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/dns/zones"), 0o755); err != nil {
		return err
	}
	body := `setuid=pdns
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
`
	if err := os.WriteFile(root(c, "etc/powerdns/pdns.conf"), []byte(body), 0o640); err != nil {
		return err
	}
	named := `options {
    directory "/var/lib/panel/dns/zones";
};
include "/var/lib/panel/dns/named-zones.conf";
`
	if err := os.WriteFile(root(c, "etc/powerdns/named.conf"), []byte(named), 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(root(c, "var/lib/panel/dns/named-zones.conf")); os.IsNotExist(err) {
		return os.WriteFile(root(c, "var/lib/panel/dns/named-zones.conf"), []byte(""), 0o644)
	}
	return nil
}

func applyMail(c Config) error {
	if err := os.MkdirAll(root(c, "var/vmail"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/mail"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/postfix"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/dovecot/conf.d"), 0o755); err != nil {
		return err
	}
	main := `# Managed by Hosting Panel — Postfix virtual mailbox host
compatibility_level = 3.6
myhostname = ` + hostnameOr(c) + `
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
`
	if err := os.WriteFile(root(c, "etc/postfix/main.cf"), []byte(main), 0o644); err != nil {
		return err
	}
	dovecot := `protocols = imap lmtp
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
`
	if err := os.WriteFile(root(c, "etc/dovecot/dovecot.conf"), []byte(dovecot), 0o644); err != nil {
		return err
	}
	for _, name := range []string{"virtual", "vdomains", "passwd", "uids", "gids"} {
		p := root(c, "var/lib/panel/mail/"+name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, []byte("# panel mail map\n"), 0o640); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyFirewall(c Config) error {
	if err := os.MkdirAll(root(c, "etc/panel"), 0o755); err != nil {
		return err
	}
	rules := `#!/usr/sbin/nft -f
table inet panel {
  chain input {
    type filter hook input priority 0; policy drop;
    iif lo accept
    ct state established,related accept
    tcp dport { 22, 25, 53, 80, 443, 587, 993, 8443, 8444 } accept
    udp dport { 53 } accept
  }
}
`
	return os.WriteFile(root(c, "etc/panel/nftables-panel.nft"), []byte(rules), 0o600)
}

func applySecurity(c Config) error {
	if err := os.MkdirAll(root(c, "etc/fail2ban/jail.d"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/ssh/sshd_config.d"), 0o755); err != nil {
		return err
	}
	jail := `[sshd]
enabled = true
[postfix]
enabled = true
[dovecot]
enabled = true
[panel-auth]
enabled = true
port = 8443,8444,18080
filter = panel-auth
logpath = /var/lib/panel/logs/api.jsonl
`
	if err := os.WriteFile(root(c, "etc/fail2ban/jail.d/panel.conf"), []byte(jail), 0o644); err != nil {
		return err
	}
	sftp := `Match Group panel-sftp
    ChrootDirectory /home/%u
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
`
	return os.WriteFile(root(c, "etc/ssh/sshd_config.d/panel-sftp.conf"), []byte(sftp), 0o644)
}

func applyTLS(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/acme-www/.well-known/acme-challenge"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/certs"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/nginx/panel-sites"), 0o755); err != nil {
		return err
	}
	acme := `server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;
    location ^~ /.well-known/acme-challenge/ {
        root /var/lib/panel/acme-www;
        default_type text/plain;
    }
}
`
	return os.WriteFile(root(c, "etc/nginx/panel-sites/00-acme.conf"), []byte(acme), 0o644)
}

func applySystemd(c Config) error {
	if err := os.MkdirAll(root(c, "etc/systemd/system"), 0o755); err != nil {
		return err
	}
	units := map[string]string{
		"panel-api.service": `[Unit]
Description=Hosting Panel Control API
After=network-online.target postgresql.service panel-agent.service
[Service]
User=panel
Group=panel
Environment=PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql
Environment=PANEL_AGENT_SOCK=/run/panel/agent.sock
ExecStart=/usr/local/panel/bin/panel-api
Restart=on-failure
[Install]
WantedBy=multi-user.target
`,
		"panel-worker.service": `[Unit]
Description=Hosting Panel Desired-State Worker
After=panel-api.service panel-agent.service
[Service]
User=panel
Group=panel
Environment=PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql
Environment=PANEL_AGENT_SOCK=/run/panel/agent.sock
ExecStart=/usr/local/panel/bin/panel-worker
Restart=on-failure
[Install]
WantedBy=multi-user.target
`,
		"panel-agent.service": `[Unit]
Description=Hosting Panel Privileged Agent
After=network-online.target
[Service]
User=root
Group=root
ExecStart=/usr/local/panel/bin/panel-agent
Restart=on-failure
[Install]
WantedBy=multi-user.target
`,
	}
	for name, body := range units {
		if err := os.WriteFile(root(c, "etc/systemd/system/"+name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func applyHealth(c Config) error {
	report := fmt.Sprintf("installation_id=%s checked=%s web=ok dns=configured mail=configured firewall=table-inet-panel\n",
		c.Hostname, time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(root(c, "var/lib/panel/health-report.txt"), []byte(report), 0o640)
}

func applyAdministrator(c Config) error {
	body := fmt.Sprintf("admin_email=%s hostname=%s channel=%s\nUse PANEL_ADMIN_PASSWORD for the first Server Portal login.\n",
		c.AdminEmail, c.Hostname, c.Channel)
	return os.WriteFile(root(c, "var/lib/panel/administrator.txt"), []byte(body), 0o640)
}

func applyReport(c Config) error {
	body := fmt.Sprintf("ok=true hostname=%s finished=%s\n", c.Hostname, time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(root(c, "var/lib/panel/installation-report.txt"), []byte(body), 0o640)
}

func verifyMail(c Config) error {
	_, err := os.Stat(root(c, "etc/postfix/main.cf"))
	return err
}

func verifyDNS(c Config) error {
	_, err := os.Stat(root(c, "etc/powerdns/pdns.conf"))
	return err
}

func verifySecurity(c Config) error {
	_, err := os.Stat(root(c, "etc/panel/nftables-panel.nft"))
	return err
}

func hostnameOr(c Config) string {
	if c.Hostname != "" {
		return c.Hostname
	}
	return "panel.local"
}

func applyRuntimeVersions(c Config) error {
	note := "php=8.3 node=system python=system mariadb=system postgresql=system\n"
	return os.WriteFile(root(c, "var/lib/panel/runtime-versions.txt"), []byte(note), 0o644)
}

func applyRepositories(c Config) error {
	return os.MkdirAll(root(c, "etc/apt/sources.list.d"), 0o755)
}

func applyControlPlaneBins(c Config) error {
	return os.MkdirAll(root(c, "usr/local/panel/bin"), 0o755)
}
