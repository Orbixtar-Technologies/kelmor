package job

import (
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
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
