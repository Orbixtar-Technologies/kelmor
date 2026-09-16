package job

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestCollectUsageEnforcesIdleAccount(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "idle1")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "big.bin"), make([]byte, 200), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "var/lib/panel/quotas"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "var/lib/panel/quotas", "idle1"), []byte("100\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemory()
	st.PutPackage(&store.Package{ID: "pkg", DiskBytes: 100, BandwidthBytesMonthly: 50})
	st.PutAccount(&store.Account{
		ID: "acc-idle", Username: "idle1", Status: "active", PackageID: "pkg",
		HomePath: "/home/idle1", DesiredRevision: 4, ObservedRevision: 4,
	})
	w := New(st, &operations.Host{Root: root}, logging.New("test"), nil, "w1")
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(home, "public_html"), 0o750)
	})
	w.collectUsageBatch()
	usage := st.GetUsage("acc-idle")
	if usage == nil || !usage.DiskLimited || usage.EnforcedAt == nil {
		t.Fatalf("idle usage enforcement: %+v", usage)
	}
	if st.GetAccount("acc-idle").DesiredRevision != 4 {
		t.Fatal("usage collection must not require desired-state drift")
	}
}

func TestScanDriftSkipsFreshCertificateScan(t *testing.T) {
	st := store.NewMemory()
	soon := time.Now().Add(2 * 24 * time.Hour)
	st.PutCert(&store.Certificate{ID: "c-soon", AccountID: "a", Hostname: "soon.test", Status: "active", NotAfter: &soon})
	w := New(st, nil, logging.New("test"), nil, "w1")
	w.lastCertScan = time.Now()
	w.scanDrift(context.Background())
	for _, job := range st.ListJobs("queued", 20) {
		if job.Type == "certificate.provision" {
			t.Fatal("fresh cert scan ran before the minute interval")
		}
	}
}
