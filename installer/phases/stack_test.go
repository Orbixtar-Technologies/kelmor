package phases

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevInstallWritesHostStack(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.10")
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	cfg := Config{Hostname: "panel.example.net", AdminEmail: "ops@example.net", Dev: true, Channel: "stable"}
	for _, p := range All() {
		if err := p.Check(cfg); err != nil {
			t.Fatalf("check %s: %v", p.Name(), err)
		}
		if err := p.Apply(cfg); err != nil {
			t.Fatalf("apply %s: %v", p.Name(), err)
		}
		if err := p.Verify(cfg); err != nil {
			t.Fatalf("verify %s: %v", p.Name(), err)
		}
	}
	need := []string{
		"var/panel/host/etc/postfix/main.cf",
		"var/panel/host/etc/postfix/master.cf",
		"var/panel/host/etc/dovecot/conf.d/99-panel-sasl.conf",
		"var/panel/host/etc/dovecot/dovecot.conf",
		"var/panel/host/etc/powerdns/pdns.conf",
		"var/panel/host/etc/panel/nftables-panel.nft",
		"var/panel/host/etc/fail2ban/jail.d/panel.conf",
		"var/panel/host/etc/fail2ban/filter.d/panel-auth.conf",
		"var/panel/host/etc/nginx/modsec/panel.conf",
		"var/panel/host/etc/nginx/modsec/panel-enforce.conf",
		"var/panel/host/etc/nginx/panel-sites/01-modsec-probe.conf",
		"var/panel/host/etc/rspamd/local.d/panel.conf",
		"var/panel/host/etc/clamav/panel.conf",
		"var/panel/host/etc/ssh/sshd_config.d/panel-sftp.conf",
		"var/panel/host/etc/ssh/sshd_config.d/panel-backup-sftp.conf",
		"var/panel/host/var/lib/panel/offsite",
		"var/panel/host/var/lib/panel/secrets/backup-sftp.env",
		"var/panel/host/var/lib/panel/objects",
		"var/panel/host/var/lib/panel/secrets/backup-s3.env",
		"var/panel/host/etc/vsftpd.conf",
		"var/panel/host/etc/pam.d/vsftpd",
		"var/panel/host/var/lib/panel/ftp/user_conf",
		"var/panel/host/var/lib/panel/quotas",
		"var/panel/host/etc/nginx/panel-sites/00-acme.conf",
		"var/panel/host/etc/nginx/panel-sites/90-server-portal.conf",
		"var/panel/host/etc/nginx/panel-sites/91-account-portal.conf",
		"var/panel/host/usr/local/panel/share/portals/server/index.html",
		"var/panel/host/usr/local/panel/share/portals/account/index.html",
		"var/panel/host/var/lib/panel/acme-www/.well-known/acme-challenge",
		"var/panel/host/etc/systemd/system/panel-agent.service",
		"var/panel/host/etc/systemd/system/multi-user.target.wants/panel-agent.service",
		"var/panel/host/etc/systemd/system/multi-user.target.wants/panel-worker.service",
		"var/panel/host/etc/systemd/system/panel-smtp-policy.service",
		"var/panel/host/var/lib/panel/health-report.txt",
		"var/panel/host/var/lib/panel/public.env",
	}
	for _, rel := range need {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	unit, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/systemd/system/panel-api.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(unit), "PANEL_STATE_DIR=/var/lib/panel") {
		t.Fatalf("api unit missing state dir: %s", unit)
	}
	worker, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/systemd/system/panel-worker.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(worker), "PANEL_PDNS_URL=http://127.0.0.1:8081") {
		t.Fatalf("worker unit missing PowerDNS URL: %s", worker)
	}
	if !contains(string(worker), "EnvironmentFile=-/var/lib/panel/acme.env") {
		t.Fatalf("worker unit missing ACME env file: %s", worker)
	}
	if !contains(string(worker), "EnvironmentFile=-/var/lib/panel/public.env") {
		t.Fatalf("worker unit missing public IPv4 env file: %s", worker)
	}
	if !contains(string(worker), "EnvironmentFile=-/var/lib/panel/secrets/backup-sftp.env") {
		t.Fatalf("worker unit missing offsite SFTP env file: %s", worker)
	}
	if !contains(string(worker), "EnvironmentFile=-/var/lib/panel/secrets/backup-s3.env") {
		t.Fatalf("worker unit missing S3 backup env file: %s", worker)
	}
	if _, err := os.Stat(filepath.Join(dir, "var/panel/host/etc/systemd/system/panel-object-store.service")); err != nil {
		t.Fatal(err)
	}
	s3env, err := os.ReadFile(filepath.Join(dir, "var/panel/host/var/lib/panel/secrets/backup-s3.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(s3env), "PANEL_S3_ENDPOINT=http://127.0.0.1:19090") {
		t.Fatalf("backup-s3.env: %s", s3env)
	}
	pdns, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/powerdns/pdns.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(pdns), "bind-dnssec-db=/var/lib/panel/dns/bind-dnssec.sqlite3") {
		t.Fatalf("pdns.conf missing DNSSEC db: %s", pdns)
	}
	if !contains(string(pdns), "local-address=127.0.0.1,203.0.113.10") {
		t.Fatalf("pdns.conf missing public listen: %s", pdns)
	}
	pubenv, err := os.ReadFile(filepath.Join(dir, "var/panel/host/var/lib/panel/public.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(pubenv), "PANEL_PUBLIC_IPV4=203.0.113.10") {
		t.Fatalf("public.env: %s", pubenv)
	}
	acme, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/nginx/panel-sites/00-acme.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(acme), "return 404") {
		t.Fatalf("default vhost must 404 unknown hosts: %s", acme)
	}
	maincf, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/postfix/main.cf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(maincf), "virtual_alias_maps = hash:/var/lib/panel/mail/aliases") {
		t.Fatalf("postfix missing alias maps: %s", maincf)
	}
	if !contains(string(maincf), "smtpd_sasl_type = dovecot") {
		t.Fatalf("postfix missing dovecot sasl: %s", maincf)
	}
	mastercf, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/postfix/master.cf"))
	if err != nil {
		t.Fatal(err)
	}
	vsftpd, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/vsftpd.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(vsftpd), "pasv_address=203.0.113.10") {
		t.Fatalf("vsftpd PASV should publish public IPv4: %s", vsftpd)
	}
	if !contains(string(maincf), "smtpd_sender_login_maps = hash:/var/lib/panel/mail/sender-login") {
		t.Fatalf("postfix missing sender-login maps: %s", maincf)
	}
	if !contains(string(mastercf), "panel-submission") || !contains(string(mastercf), "smtpd_sasl_auth_enable=yes") {
		t.Fatalf("master.cf missing authenticated submission: %s", mastercf)
	}
	if !contains(string(mastercf), "reject_sender_login_mismatch") {
		t.Fatalf("master.cf missing sender-login mismatch: %s", mastercf)
	}
	sasl, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/dovecot/conf.d/99-panel-sasl.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(sasl), "/var/spool/postfix/private/auth") {
		t.Fatalf("dovecot sasl socket: %s", sasl)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()))
}

func TestWriteUnlessExistsKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.cf")
	if err := os.WriteFile(p, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeUnlessExists(p, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "keep\n" {
		t.Fatalf("overwrote existing file: %q", b)
	}
	missing := filepath.Join(dir, "fresh.cf")
	if err := writeUnlessExists(missing, []byte("created\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(missing)
	if err != nil || string(b) != "created\n" {
		t.Fatalf("did not create missing file: %q %v", b, err)
	}
}
