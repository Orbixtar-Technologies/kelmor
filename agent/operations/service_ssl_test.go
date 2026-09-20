package operations

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func TestListServiceCertificatesReportsMissingSlots(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	raw, err := h.listServiceCertificates("")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := raw.(map[string]any)
	items, _ := payload["items"].([]map[string]any)
	if len(items) < 3 {
		t.Fatalf("expected director/control/mail slots: %#v", raw)
	}
	for _, row := range items {
		if row["status"] != "missing" {
			t.Fatalf("empty host must not invent certs: %#v", row)
		}
		if row["cert_path"] == "" || row["key_path"] == "" {
			t.Fatalf("slot paths: %#v", row)
		}
	}
}

func TestInstallServiceCertificateWritesHostPair(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cert, key, err := paneltls.SelfSigned("kelmor.host", time.Now().Add(30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.installServiceCertificate("director", "kelmor.host", string(cert), string(key)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/certs/panel-portals.crt")); err != nil {
		t.Fatal(err)
	}
	listed, err := h.listServiceCertificates("kelmor.host")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := listed.(map[string]any)
	items, _ := payload["items"].([]map[string]any)
	found := false
	for _, row := range items {
		if row["id"] != "director" {
			continue
		}
		found = true
		if row["status"] != "installed" || row["subject"] != "kelmor.host" {
			t.Fatalf("director row: %#v", row)
		}
	}
	if !found {
		t.Fatal("director slot missing after install")
	}
}

func TestInstallServiceCertificateRejectsMismatchedKey(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cert, _, err := paneltls.SelfSigned("kelmor.host", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := paneltls.SelfSigned("other.host", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.installServiceCertificate("mail", "", string(cert), string(otherKey)); err == nil {
		t.Fatal("expected mismatch rejection")
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/certs/imap.panel.local.crt")); err == nil {
		t.Fatal("failed install must not leave a cert")
	}
}

func TestInstallServiceCertificateRejectsUnknownService(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.installServiceCertificate("root-shell", "", "cert", "key"); err == nil {
		t.Fatal("expected unknown service rejection")
	}
}
