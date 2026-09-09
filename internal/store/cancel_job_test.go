package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/id"
)

func TestMemoryCancelJobIsAtomicAndPreservesHistory(t *testing.T) {
	data := NewMemory()
	started := time.Now().Add(-time.Minute).UTC()
	heartbeat := time.Now().Add(-time.Second).UTC()
	job, err := data.EnqueueJob(&Job{
		Type: "account.reconcile", State: "failed", Attempts: 3, LastError: "agent failed",
		LockedBy: "worker-1", StartedAt: &started, HeartbeatAt: &heartbeat,
		Payload: map[string]any{"account_id": id.New()},
	})
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		job *Job
		err error
	}
	results := make(chan result, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			ready.Done()
			<-start
			cancelled, cancelErr := data.CancelJob(job.ID, id.New(), id.New())
			results <- result{job: cancelled, err: cancelErr}
		}()
	}
	ready.Wait()
	close(start)

	var successes, conflicts int
	for i := 0; i < 2; i++ {
		out := <-results
		switch {
		case out.err == nil:
			successes++
			if out.job.State != "cancelled" || out.job.FinishedAt == nil {
				t.Fatalf("cancel result: %+v", out.job)
			}
		case errors.Is(out.err, ErrJobStateConflict):
			conflicts++
		default:
			t.Fatalf("unexpected cancel error: %v", out.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	stored := data.GetJob(job.ID)
	if stored.Attempts != 3 || stored.LastError != "agent failed" || stored.StartedAt == nil {
		t.Fatalf("history was not preserved: %+v", stored)
	}
	if stored.LockedBy != "" || stored.HeartbeatAt != nil {
		t.Fatalf("lock was not cleared: %+v", stored)
	}

	if _, err := data.CancelJob(id.New(), id.New(), id.New()); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("missing job error = %v", err)
	}
}

func TestPostgresCancelJobUsesConditionalUpdate(t *testing.T) {
	dsn, explicitlyConfigured := os.LookupEnv("PANEL_DATABASE_URL")
	if !explicitlyConfigured {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	data, err := OpenPostgres(context.Background(), dsn)
	if err != nil {
		if explicitlyConfigured {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	t.Cleanup(data.Close)

	job, err := data.EnqueueJob(&Job{
		Type: "account.reconcile", State: "failed", Attempts: 4,
		Payload: map[string]any{"account_id": id.New(), "safe": "retained"},
	})
	if err != nil {
		t.Fatal(err)
	}
	job.LastError = "agent failed"
	started := time.Now().Add(-time.Minute).UTC()
	job.StartedAt = &started
	job.LockedBy = "worker-1"
	job.HeartbeatAt = &started
	data.UpdateJob(job)
	before := data.GetJob(job.ID)

	actorID, requestID := id.New(), id.New()
	cancelled, err := data.CancelJob(job.ID, actorID, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != "cancelled" || cancelled.FinishedAt == nil ||
		cancelled.LockedBy != "" || cancelled.HeartbeatAt != nil {
		t.Fatalf("cancelled job: %+v", cancelled)
	}
	if cancelled.Attempts != before.Attempts || cancelled.LastError != before.LastError ||
		cancelled.StartedAt == nil || cancelled.Payload["safe"] != "retained" {
		t.Fatalf("history changed:\nbefore: %+v\nafter:  %+v", before, cancelled)
	}
	if cancelled.ActorID != actorID || cancelled.RequestID != requestID {
		t.Fatalf("cancellation attribution: %+v", cancelled)
	}
	if _, err := data.CancelJob(job.ID, actorID, requestID); !errors.Is(err, ErrJobStateConflict) {
		t.Fatalf("second cancellation error = %v", err)
	}
	if _, err := data.CancelJob(id.New(), actorID, requestID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("missing cancellation error = %v", err)
	}
}
