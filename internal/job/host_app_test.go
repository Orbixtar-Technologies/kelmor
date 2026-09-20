package job

import (
	"context"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestHostAppControlJobDispatchesAgent(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	job, err := st.EnqueueJob(&store.Job{
		Type: "host.app.control", ResourceType: "server",
		Payload: map[string]any{"target": "rspamd", "action": "restart"},
		State:   "queued",
	})
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: t.TempDir()}, logging.New("test"), nil, "tester")
	w.Drain(context.Background())
	got := st.GetJob(job.ID)
	if got == nil || got.State != "succeeded" {
		t.Fatalf("host.app.control: %+v", got)
	}
}
