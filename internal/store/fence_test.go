package store

import (
	"errors"
	"testing"
	"time"
)

func TestClaimJobReturnsMonotonicFenceAndLease(t *testing.T) {
	data := NewMemory()
	job, err := data.EnqueueJob(&Job{
		Type: "account.reconcile", State: "queued",
		ResourceType: "account", ResourceID: "acc-fence",
		Payload: map[string]any{"account_id": "acc-fence"},
	})
	if err != nil {
		t.Fatal(err)
	}

	first := data.ClaimJob("worker-a")
	if first == nil || first.ID != job.ID {
		t.Fatalf("first claim: %+v", first)
	}
	if first.Fence < 1 || first.LockedBy != "worker-a" || first.LeaseExpires == nil {
		t.Fatalf("first claim missing fence/lease: %+v", first)
	}
	if first.OperationID == "" {
		t.Fatal("claim must return an operation identity")
	}

	if err := data.ExpireStaleLeases(first.LeaseExpires.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	expired := data.GetJob(job.ID)
	if expired == nil || expired.State != "retrying" || expired.LockedBy != "" {
		t.Fatalf("expired job: %+v", expired)
	}

	second := data.ClaimJob("worker-b")
	if second == nil || second.Fence <= first.Fence {
		t.Fatalf("second fence %#v must exceed %#v", second, first.Fence)
	}
	if second.LockedBy != "worker-b" || second.OperationID == first.OperationID {
		t.Fatalf("second claim reused identity: %+v", second)
	}
}

func TestStaleFenceCannotHeartbeatOrComplete(t *testing.T) {
	data := NewMemory()
	_, err := data.EnqueueJob(&Job{
		Type: "account.reconcile", State: "queued",
		ResourceType: "account", ResourceID: "acc-stale",
		Payload: map[string]any{"account_id": "acc-stale"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ownerA := data.ClaimJob("worker-a")
	if ownerA == nil {
		t.Fatal("claim a")
	}
	if err := data.ExpireStaleLeases(ownerA.LeaseExpires.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	ownerB := data.ClaimJob("worker-b")
	if ownerB == nil {
		t.Fatal("claim b")
	}

	if err := data.HeartbeatJob(ownerA.ID, ownerA.LockedBy, ownerA.Fence); !errors.Is(err, ErrStaleFence) {
		t.Fatalf("stale heartbeat: %v", err)
	}
	ownerA.State = "succeeded"
	ownerA.Progress = 100
	if err := data.UpdateJob(ownerA); !errors.Is(err, ErrStaleFence) {
		t.Fatalf("stale complete: %v", err)
	}
	stored := data.GetJob(ownerA.ID)
	if stored.State != "running" || stored.LockedBy != "worker-b" || stored.Fence != ownerB.Fence {
		t.Fatalf("stale complete mutated job: %+v", stored)
	}

	if err := data.HeartbeatJob(ownerB.ID, ownerB.LockedBy, ownerB.Fence); err != nil {
		t.Fatal(err)
	}
	ownerB.State = "succeeded"
	ownerB.Progress = 100
	if err := data.UpdateJob(ownerB); err != nil {
		t.Fatal(err)
	}
	if got := data.GetJob(ownerB.ID); got.State != "succeeded" {
		t.Fatalf("owner complete: %+v", got)
	}
}

func TestCancelRunningJobIsAttemptOnly(t *testing.T) {
	data := NewMemory()
	job, err := data.EnqueueJob(&Job{
		Type: "account.reconcile", State: "queued",
		Payload: map[string]any{"account_id": "acc-cancel"},
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed := data.ClaimJob("worker-a")
	if claimed == nil {
		t.Fatal("claim")
	}
	if err := data.RequestJobCancel(job.ID); err != nil {
		t.Fatal(err)
	}
	got := data.GetJob(job.ID)
	if got.State != "running" || !got.CancelRequested {
		t.Fatalf("running cancel should flag the attempt: %+v", got)
	}
}
