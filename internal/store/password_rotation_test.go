package store

import "testing"

func TestMemoryRotatePasswordAndEnqueueMissingUserIsAtomic(t *testing.T) {
	data := NewMemory()
	job := &Job{
		Type:         "account.reconcile",
		ResourceType: "account",
		ResourceID:   "account-1",
		Payload:      map[string]any{"account_id": "account-1", "linux_password": "NewPassword!2026"},
	}

	queued, err := data.RotatePasswordAndEnqueue("missing-user", "new-hash", false, job)

	if err == nil {
		t.Fatal("expected missing user error")
	}
	if queued != nil {
		t.Fatalf("unexpected queued job: %+v", queued)
	}
	if jobs := data.ListJobs("", 10); len(jobs) != 0 {
		t.Fatalf("password rotation inserted a job for a missing user: %+v", jobs)
	}
}
