package monitoring

import (
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestCollectCountsFailedJobs(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme42", HomePath: t.TempDir()})
	_, _ = st.EnqueueJob(&store.Job{Type: "x", State: "failed"})
	snap := Collect(st, "")
	if snap.FailedJobs < 1 {
		t.Fatal(snap)
	}
}
