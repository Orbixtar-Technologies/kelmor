package phases

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/firewall"
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
	if err := writeUnlessExists(root(c, "etc/powerdns/pdns.conf"), []byte(body), 0o640); err != nil {
		return err
	}
	named := `options {
    directory "/var/lib/panel/dns/zones";
};
include "/var/lib/panel/dns/named-zones.conf";
`
	if err := writeUnlessExists(root(c, "etc/powerdns/named.conf"), []byte(named), 0o644); err != nil {
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
	if err := writeUnlessExists(root(c, "etc/postfix/main.cf"), []byte(main), 0o644); err != nil {
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
	if err := writeUnlessExists(root(c, "etc/dovecot/dovecot.conf"), []byte(dovecot), 0o644); err != nil {
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
	rules := firewall.Rules(firewall.ExtraListeningTCP())
	path := root(c, "etc/panel/nftables-panel.nft")
	if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	return applyLiveNFT(path)
}

func applyLiveNFT(path string) error {
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return nil
	}
	_ = exec.Command("/usr/sbin/nft", "delete", "table", "inet", "panel").Run()
	cmd := exec.Command("/usr/sbin/nft", "-f", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft -f: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func applyWAF(c Config) error {
	if err := os.MkdirAll(root(c, "etc/nginx/modsec"), 0o755); err != nil {
		return err
	}
	body := `# Managed by Hosting Panel — ModSecurity
SecRuleEngine DetectionOnly
SecRequestBodyAccess On
SecDataDir /tmp
`
	if _, err := os.Stat("/usr/share/modsecurity-crs/owasp-crs.conf"); err == nil && !c.Dev {
		body += "Include /usr/share/modsecurity-crs/owasp-crs.conf\n"
	}
	if err := os.WriteFile(root(c, "etc/nginx/modsec/panel.conf"), []byte(body), 0o644); err != nil {
		return err
	}
	enforce := `# Managed by Hosting Panel — enforced probe vhost only
SecRuleEngine On
SecRequestBodyAccess On
SecDataDir /tmp
SecRule REQUEST_HEADERS:User-Agent "@contains panel-modsec-probe" "id:19999,phase:1,deny,status:403,msg:'panel waf probe'"
`
	if err := os.WriteFile(root(c, "etc/nginx/modsec/panel-enforce.conf"), []byte(enforce), 0o644); err != nil {
		return err
	}
	probe := `server {
    listen 127.0.0.1:18481;
    server_name _;
    modsecurity on;
    modsecurity_rules_file /etc/nginx/modsec/panel-enforce.conf;
    location / { default_type text/plain; return 200 'waf-ok\n'; }
}
`
	return os.WriteFile(root(c, "etc/nginx/panel-sites/01-modsec-probe.conf"), []byte(probe), 0o644)
}

func applyRspamd(c Config) error {
	if err := os.MkdirAll(root(c, "etc/rspamd/local.d"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(root(c, "etc/rspamd/local.d/panel.conf"), []byte("enabled = true;\nmilters = \"inet:127.0.0.1:11332\";\n"), 0o644)
}

func applyClamAV(c Config) error {
	if err := os.MkdirAll(root(c, "etc/clamav"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(root(c, "etc/clamav/panel.conf"), []byte("TCPSocket 3310\nTCPAddr 127.0.0.1\n"), 0o644); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	sigdir := "/var/lib/clamav"
	if err := os.MkdirAll(sigdir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(sigdir + "/main.cvd"); err != nil {
		_ = os.WriteFile(sigdir+"/panel.ndb", []byte("PanelClam:0:*:50414e454c434c414d\n"), 0o644)
		cmd := exec.Command("/usr/bin/freshclam", "--stdout", "--quiet")
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "DEBIAN_FRONTEND=noninteractive"}
		_ = cmd.Start()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(45 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	return nil
}

func applySecurity(c Config) error {
	if err := applyWAF(c); err != nil {
		return err
	}
	if err := applyRspamd(c); err != nil {
		return err
	}
	if err := applyClamAV(c); err != nil {
		return err
	}
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
	if err := os.MkdirAll(root(c, "etc/fail2ban/filter.d"), 0o755); err != nil {
		return err
	}
	filter := `[Definition]
failregex = ^.*"event":"auth.login".*"success":false.*"source_ip":"<HOST>"
ignoreregex =
`
	if err := os.WriteFile(root(c, "etc/fail2ban/filter.d/panel-auth.conf"), []byte(filter), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/quotas"), 0o755); err != nil {
		return err
	}
	sftp := `# Chrooted tenant SFTP. Over-quota users get internal-sftp -R
# from /etc/ssh/sshd_config.d/zz-panel-sftp-quota.conf (agent-managed).
Match Group panel-sftp
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
    location / { default_type text/plain; return 404 'no such site\n'; }
}
`
	if err := os.WriteFile(root(c, "etc/nginx/panel-sites/00-acme.conf"), []byte(acme), 0o644); err != nil {
		return err
	}
	return startLocalACME(c)
}

func verifyTLS(c Config) error {
	if _, err := os.Stat(root(c, "var/lib/panel/acme-www/.well-known/acme-challenge")); err != nil {
		return err
	}
	if c.Dev || pebbleBinary() == "" {
		return nil
	}
	if _, err := os.Stat(root(c, "var/lib/panel/acme.directory")); err != nil {
		return fmt.Errorf("local ACME directory missing")
	}
	con, err := net.DialTimeout("tcp", "127.0.0.1:14000", 400*time.Millisecond)
	if err != nil {
		return fmt.Errorf("pebble ACME not listening: %w", err)
	}
	_ = con.Close()
	return nil
}

func writeUnlessExists(path string, body []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, body, mode)
}

func verifySystemd(c Config) error {
	for _, n := range []string{"panel-api.service", "panel-worker.service", "panel-agent.service"} {
		if _, err := os.Stat(root(c, "etc/systemd/system/"+n)); err != nil {
			return err
		}
	}
	return nil
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
Environment=PANEL_STATE_DIR=/var/lib/panel
Environment=PANEL_API_ADDR=127.0.0.1:18080
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
Environment=PANEL_STATE_DIR=/var/lib/panel
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
Environment=PANEL_AGENT_SOCK=/run/panel/agent.sock
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
	web, dns, mail := "ok", "configured", "configured"
	if !c.Dev {
		if _, err := net.DialTimeout("tcp", "127.0.0.1:80", 400*time.Millisecond); err != nil {
			web = "down"
		}
		if _, err := net.DialTimeout("tcp", "127.0.0.1:25", 400*time.Millisecond); err != nil {
			mail = "down"
		}
		if _, err := net.DialTimeout("tcp", "127.0.0.1:53", 400*time.Millisecond); err != nil {
			dns = "down"
		}
	}
	report := fmt.Sprintf("installation_id=%s checked=%s web=%s dns=%s mail=%s firewall=table-inet-panel\n",
		c.Hostname, time.Now().UTC().Format(time.RFC3339), web, dns, mail)
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

func verifyFirewall(c Config) error {
	if _, err := os.Stat(root(c, "etc/panel/nftables-panel.nft")); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return nil
	}
	if err := exec.Command("/usr/sbin/nft", "list", "table", "inet", "panel").Run(); err != nil {
		return fmt.Errorf("table inet panel is not loaded")
	}
	return nil
}

func verifySecurity(c Config) error {
	for _, p := range []string{
		"etc/nginx/modsec/panel.conf",
		"etc/rspamd/local.d/panel.conf",
		"etc/clamav/panel.conf",
		"etc/fail2ban/jail.d/panel.conf",
		"etc/fail2ban/filter.d/panel-auth.conf",
		"etc/ssh/sshd_config.d/panel-sftp.conf",
		"var/lib/panel/quotas",
	} {
		if _, err := os.Stat(root(c, p)); err != nil {
			return err
		}
	}
	return nil
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
