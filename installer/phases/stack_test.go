package phases

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevInstallWritesHostStack(t *testing.T) {
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
		"var/panel/host/etc/dovecot/dovecot.conf",
		"var/panel/host/etc/powerdns/pdns.conf",
		"var/panel/host/etc/panel/nftables-panel.nft",
		"var/panel/host/etc/fail2ban/jail.d/panel.conf",
		"var/panel/host/etc/nginx/modsec/panel.conf",
		"var/panel/host/etc/nginx/modsec/panel-enforce.conf",
		"var/panel/host/etc/nginx/panel-sites/01-modsec-probe.conf",
		"var/panel/host/etc/rspamd/local.d/panel.conf",
		"var/panel/host/etc/clamav/panel.conf",
		"var/panel/host/etc/ssh/sshd_config.d/panel-sftp.conf",
		"var/panel/host/etc/nginx/panel-sites/00-acme.conf",
		"var/panel/host/etc/systemd/system/panel-agent.service",
		"var/panel/host/var/lib/panel/health-report.txt",
	}
	for _, rel := range need {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
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
