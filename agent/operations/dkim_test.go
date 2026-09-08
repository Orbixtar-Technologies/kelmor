package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDKIMWritesKeyAndTXT(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	rec, err := h.ensureDKIM("Mail.Example.TEST")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Domain != "mail.example.test" || rec.Selector != "default" || !strings.Contains(rec.TXT, "v=DKIM1; k=rsa; p=") {
		t.Fatalf("%+v", rec)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/dkim/mail.example.test/default.key")); err != nil {
		t.Fatal(err)
	}
	again, err := h.ensureDKIM("mail.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if again.TXT != rec.TXT {
		t.Fatal("existing key must be reused")
	}
	if _, err := h.applyDKIMSigning([]string{"mail.example.test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "etc/rspamd/local.d/dkim_signing.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "mail.example.test") || !strings.Contains(string(cfg), "selector = \"default\"") {
		t.Fatal(string(cfg))
	}
}

func TestEnsureDKIMRejectsBadDomain(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.ensureDKIM("../etc/passwd"); err == nil {
		t.Fatal("expected reject")
	}
}
