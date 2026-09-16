package job

import (
	"context"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestExpiredWorkerCannotCompleteAfterSuccessorFinishes(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-fence", Username: "fence42", PrimaryDomain: "fence.test",
		PackageID: st.ListPackages()[0].ID, Status: "provisioning", HomePath: "/home/fence42",
		LinuxUID: 20110, LinuxGID: 20110, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	_, err := st.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID}, State: "queued",
	})
	if err != nil {
		t.Fatal(err)
	}

	stale := st.ClaimJob("worker-a")
	if stale == nil {
		t.Fatal("claim a")
	}
	if err := st.ExpireStaleLeases(stale.LeaseExpires.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	live := New(st, &operations.Host{Root: root}, logging.New("test"), nil, "worker-b")
	live.Drain(context.Background())
	if got := st.GetAccount(acc.ID); got.Status != "active" {
		t.Fatalf("successor status %s", got.Status)
	}
	if got := st.GetJob(stale.ID); got == nil || got.State != "succeeded" || got.Fence == stale.Fence {
		t.Fatalf("successor job: %+v", got)
	}

	stale.State = "succeeded"
	stale.Progress = 100
	if err := st.UpdateJob(stale); err == nil {
		t.Fatal("stale worker completed after successor")
	}
	if got := st.GetJob(stale.ID); got.LockedBy != "" && got.Fence == stale.Fence && got.State != "succeeded" {
		t.Fatalf("stale write landed: %+v", got)
	}
}

func TestOlderRevisionNeverAcknowledgesNewerDesired(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-rev", Username: "rev42", PrimaryDomain: "rev.test",
		PackageID: st.ListPackages()[0].ID, Status: "active", HomePath: "/home/rev42",
		LinuxUID: 20111, LinuxGID: 20111, DesiredRevision: 2, ObservedRevision: 0,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-rev", AccountID: acc.ID, FQDN: "rev.test", ASCII: "rev.test",
		Type: "primary", DocumentRoot: "/home/rev42/public_html", Status: "active",
	})
	root := t.TempDir()
	w := New(st, &operations.Host{Root: root}, logging.New("test"), nil, "tester")
	if err := w.provisionAccount(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		TargetRevision: 1,
		Payload:        map[string]any{"account_id": acc.ID},
	}); err != nil {
		t.Fatal(err)
	}
	got := st.GetAccount(acc.ID)
	if got.ObservedRevision != 1 {
		t.Fatalf("revision 1 acknowledged %d", got.ObservedRevision)
	}
	if got.DesiredRevision != 2 {
		t.Fatalf("desired revision overwritten: %d", got.DesiredRevision)
	}
}

func TestHeartbeatLossCancelsInFlightCommand(t *testing.T) {
	st := store.NewMemory()
	job, err := st.EnqueueJob(&store.Job{
		Type: "php.runtime.ensure", State: "queued",
		Payload: map[string]any{"version": "8.3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed := st.ClaimJob("worker-a")
	if claimed == nil || claimed.ID != job.ID {
		t.Fatal("claim")
	}

	started := make(chan struct{})
	host := &operations.Host{Root: t.TempDir()}
	host.BeforeDispatch = func(ctx context.Context, req operations.Request) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	w := New(st, host, logging.New("test"), nil, "worker-a")
	w.heartbeatEvery = 20 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		w.execute(context.Background(), claimed)
		done <- nil
	}()
	<-started
	deadline := time.Now()
	if claimed.LeaseExpires != nil {
		deadline = claimed.LeaseExpires.Add(time.Second)
	}
	if err := st.ExpireStaleLeases(deadline); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lost lease did not unblock execute")
	}
}
