package store

import "testing"

func TestDeletePackageIfUnusedIsAtomicWithAccountAssignment(t *testing.T) {
	data := NewMemory()
	assigned := &Package{ID: "assigned", Name: "Assigned"}
	unassigned := &Package{ID: "unassigned", Name: "Unassigned"}
	data.PutPackage(assigned)
	data.PutPackage(unassigned)
	data.PutAccount(&Account{ID: "account", PackageID: assigned.ID})

	if data.DeletePackageIfUnused(assigned.ID) {
		t.Fatal("assigned package was deleted")
	}
	if data.GetPackage(assigned.ID) == nil {
		t.Fatal("assigned package is missing")
	}
	if !data.DeletePackageIfUnused(unassigned.ID) {
		t.Fatal("unassigned package was not deleted")
	}
	if data.GetPackage(unassigned.ID) != nil {
		t.Fatal("unassigned package remains")
	}
}

func TestJobRetryableIsNotPersisted(t *testing.T) {
	data := NewMemory()
	retryable := true
	job, err := data.EnqueueJob(&Job{Type: "website.provision", Retryable: &retryable})
	if err != nil {
		t.Fatal(err)
	}
	if job.Retryable != nil {
		t.Fatalf("enqueue returned persisted retry eligibility: %v", *job.Retryable)
	}
	stored := data.GetJob(job.ID)
	if stored.Retryable != nil {
		t.Fatalf("stored retry eligibility: %v", *stored.Retryable)
	}
	stored.Retryable = &retryable
	data.UpdateJob(stored)
	if updated := data.GetJob(job.ID); updated.Retryable != nil {
		t.Fatalf("updated retry eligibility: %v", *updated.Retryable)
	}
}
