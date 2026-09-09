package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func TestScanCertRenewalsQueuesExpiring(t *testing.T) {
	st := store.NewMemory()
	soon := time.Now().Add(5 * 24 * time.Hour)
	far := time.Now().Add(80 * 24 * time.Hour)
	st.PutCert(&store.Certificate{ID: "c-soon", AccountID: "a", Hostname: "soon.test", Status: "active", NotAfter: &soon})
	st.PutCert(&store.Certificate{ID: "c-far", AccountID: "a", Hostname: "far.test", Status: "active", NotAfter: &far})
	w := New(st, nil, logging.New("test"), nil, "w1")
	w.scanCertRenewals()
	jobs := st.ListJobs("queued", 20)
	found := false
	for _, j := range jobs {
		if j.Type == "certificate.provision" && j.ResourceID == "c-soon" {
			found = true
		}
		if j.ResourceID == "c-far" {
			t.Fatal("far cert renewed")
		}
	}
	if !found {
		t.Fatal("expiring cert not queued")
	}
	got := st.GetCert("c-soon")
	if got == nil || got.Status != "renewing" {
		t.Fatalf("%v", got)
	}
}

func TestEnsureCertificateSkipsFresh(t *testing.T) {
	st := store.NewMemory()
	far := time.Now().Add(80 * 24 * time.Hour)
	st.PutAccount(&store.Account{ID: "a", Username: "u", Status: "active"})
	st.PutCert(&store.Certificate{ID: "c", AccountID: "a", Hostname: "x.test", Kind: "domain", Status: "active", NotAfter: &far})
	w := New(st, nil, logging.New("test"), nil, "w1")
	if err := w.ensureCertificate(st.GetAccount("a"), "x.test"); err != nil {
		t.Fatal(err)
	}
}

func TestScanPortalHostnameCertQueuesExpiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PANEL_STATE_DIR", dir)
	t.Setenv("PANEL_HOSTNAME", "panel.example.net")
	t.Setenv("PANEL_ACME_DIRECTORY", "https://127.0.0.1:14000/dir")
	if err := os.MkdirAll(filepath.Join(dir, "certs"), 0o755); err != nil {
		t.Fatal(err)
	}
	cert, _, err := paneltls.SelfSigned("panel.example.net", time.Now().Add(5*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "certs", "panel.example.net.crt"), cert, 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemory()
	w := New(st, nil, logging.New("test"), nil, "w1")
	w.scanPortalHostnameCert()
	found := false
	for _, j := range st.ListJobs("queued", 20) {
		if j.Type == "certificate.portal" && j.ResourceID == "panel.example.net" {
			found = true
		}
	}
	if !found {
		t.Fatal("expiring portal hostname cert not queued")
	}
}

func TestScanPortalHostnameCertSkipsFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PANEL_STATE_DIR", dir)
	t.Setenv("PANEL_HOSTNAME", "panel.example.net")
	t.Setenv("PANEL_ACME_DIRECTORY", "https://127.0.0.1:14000/dir")
	if err := os.MkdirAll(filepath.Join(dir, "certs"), 0o755); err != nil {
		t.Fatal(err)
	}
	cert, _, err := paneltls.SelfSigned("panel.example.net", time.Now().Add(80*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "certs", "panel.example.net.crt"), cert, 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemory()
	w := New(st, nil, logging.New("test"), nil, "w1")
	w.scanPortalHostnameCert()
	for _, j := range st.ListJobs("queued", 20) {
		if j.Type == "certificate.portal" {
			t.Fatal("fresh portal cert must not queue")
		}
	}
}
