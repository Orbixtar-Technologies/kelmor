package job

import (
	"os"
	"testing"

	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestQueueCertificateProvisionEnqueuesJob(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "https://acme-v02.api.letsencrypt.org/directory")
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "u", Status: "active"})
	w := New(st, nil, logging.New("test"), nil, "w1")
	if err := w.queueCertificateProvision(st.GetAccount("a"), "example.com"); err != nil {
		t.Fatal(err)
	}
	jobs := st.ListJobs("queued", 20)
	found := false
	for _, j := range jobs {
		if j.Type == "certificate.provision" {
			found = true
		}
	}
	if !found {
		t.Fatal("certificate job not queued")
	}
	cert := st.ListCerts("a")[0]
	if cert.Hostname != "example.com" || cert.Status != "requested" {
		t.Fatalf("cert: %+v", cert)
	}
}

func TestEnsureCertificateQueuesOnLiveACME(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "https://acme-v02.api.letsencrypt.org/directory")
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "u", Status: "active"})
	w := New(st, nil, logging.New("test"), nil, "w1")
	if err := w.ensureCertificate(st.GetAccount("a"), "example.com"); err != nil {
		t.Fatal(err)
	}
	if len(st.ListJobs("queued", 20)) == 0 {
		t.Fatal("expected queued cert job")
	}
}

func TestScanPendingCertificatesRetriesFailed(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "https://acme-v02.api.letsencrypt.org/directory")
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "u", Status: "active"})
	st.PutCert(&store.Certificate{ID: "c1", AccountID: "a", Hostname: "retry.test", Kind: "domain", Status: "failed"})
	w := New(st, nil, logging.New("test"), nil, "w1")
	w.scanPendingCertificates()
	found := false
	for _, j := range st.ListJobs("queued", 20) {
		if j.Type == "certificate.provision" && j.ResourceID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatal("failed cert not re-queued")
	}
	got := st.GetCert("c1")
	if got == nil || got.Status != "requested" {
		t.Fatalf("status reset: %+v", got)
	}
	_ = os.Unsetenv("PANEL_ACME_DIRECTORY")
}
