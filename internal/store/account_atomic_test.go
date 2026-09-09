package store

import (
	"reflect"
	"testing"
)

func TestMemoryCreateAccountWithJobCommitsAllState(t *testing.T) {
	data := NewMemory()
	data.PutPackage(&Package{ID: "package-1", Name: "Starter"})
	resellerOwner := &User{ID: "reseller-owner", Username: "reseller"}
	data.PutUser(resellerOwner)
	owner := &User{
		ID: "owner-1", Username: "customer", Email: "customer@example.test",
		Roles: []string{"customer_owner"},
	}
	account := &Account{
		ID: "account-1", OwnerUserID: owner.ID, Username: owner.Username,
		PrimaryDomain: "example.test", PackageID: "package-1", Status: "provisioning",
	}
	domain := &Domain{
		ID: "domain-1", AccountID: account.ID, FQDN: "example.test",
		ASCII: "example.test", Type: "primary", Status: "provisioning",
	}

	job, err := data.CreateAccountWithJob(owner, account, domain, []string{owner.ID, resellerOwner.ID}, &Job{
		Type: "account.provision", ResourceType: "account", ResourceID: account.ID,
		Payload: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.ID == "" || job.State != "queued" || job.MaxAttempts != 5 ||
		job.CreatedAt.IsZero() || job.RunAfter.IsZero() || job.Logs == nil {
		t.Fatalf("job was not normalized: %+v", job)
	}
	if got := data.UserByID(owner.ID); !reflect.DeepEqual(got, owner) {
		t.Fatalf("owner mismatch: %+v", got)
	}
	if got := data.GetAccount(account.ID); !reflect.DeepEqual(got, account) {
		t.Fatalf("account mismatch: %+v", got)
	}
	if got := data.GetDomain(domain.ID); !reflect.DeepEqual(got, domain) {
		t.Fatalf("domain mismatch: %+v", got)
	}
	if got := data.GetJob(job.ID); !reflect.DeepEqual(got, job) {
		t.Fatalf("job mismatch: %+v", got)
	}
	if got := data.AccountsForUser(owner.ID); !reflect.DeepEqual(got, []string{account.ID}) {
		t.Fatalf("owner memberships: %v", got)
	}
	if got := data.AccountsForUser(resellerOwner.ID); !reflect.DeepEqual(got, []string{account.ID}) {
		t.Fatalf("reseller memberships: %v", got)
	}
}

func TestMemoryCreateAccountWithJobRollsBackOnDuplicateJob(t *testing.T) {
	data := NewMemory()
	data.PutPackage(&Package{ID: "package-1"})
	existing, err := data.EnqueueJob(&Job{ID: "job-1", Type: "existing"})
	if err != nil {
		t.Fatal(err)
	}
	owner := &User{ID: "owner-1", Username: "customer", Email: "customer@example.test"}
	account := &Account{
		ID: "account-1", OwnerUserID: owner.ID, Username: owner.Username,
		PrimaryDomain: "example.test", PackageID: "package-1",
	}
	domain := &Domain{ID: "domain-1", AccountID: account.ID, ASCII: "example.test"}

	if job, err := data.CreateAccountWithJob(owner, account, domain, []string{owner.ID}, &Job{
		ID: "job-1", Type: "account.provision", ResourceID: account.ID,
	}); err == nil || job != nil {
		t.Fatalf("duplicate job result: %+v, %v", job, err)
	}
	if data.UserByID(owner.ID) != nil || data.GetAccount(account.ID) != nil || data.GetDomain(domain.ID) != nil {
		t.Fatal("failed create persisted owner, account, or domain")
	}
	if got := data.AccountsForUser(owner.ID); len(got) != 0 {
		t.Fatalf("failed create persisted memberships: %v", got)
	}
	if got := data.GetJob(existing.ID); !reflect.DeepEqual(got, existing) {
		t.Fatalf("existing job changed: %+v", got)
	}
}

func TestMemoryCreateAccountWithJobRejectsMissingMember(t *testing.T) {
	data := NewMemory()
	data.PutPackage(&Package{ID: "package-1"})
	owner := &User{ID: "owner-1", Username: "customer", Email: "customer@example.test"}
	account := &Account{
		ID: "account-1", OwnerUserID: owner.ID, Username: owner.Username,
		PrimaryDomain: "example.test", PackageID: "package-1",
	}
	domain := &Domain{ID: "domain-1", AccountID: account.ID, ASCII: "example.test"}

	if _, err := data.CreateAccountWithJob(owner, account, domain, []string{owner.ID, "missing-user"}, &Job{
		ID: "job-1", Type: "account.provision", ResourceID: account.ID,
	}); err == nil {
		t.Fatal("missing member unexpectedly committed")
	}
	if data.UserByID(owner.ID) != nil || data.GetAccount(account.ID) != nil || data.GetDomain(domain.ID) != nil ||
		data.GetJob("job-1") != nil {
		t.Fatal("invalid reference persisted partial create state")
	}
}

func TestMemoryUpdateAccountWithJobCommitsBoth(t *testing.T) {
	data := NewMemory()
	data.PutPackage(&Package{ID: "package-1"})
	data.PutAccount(&Account{
		ID: "account-1", Username: "customer", PrimaryDomain: "old.test",
		PackageID: "package-1", Status: "active", DesiredRevision: 1,
	})
	updated := data.GetAccount("account-1")
	updated.PrimaryDomain = "new.test"
	updated.DesiredRevision++

	job, err := data.UpdateAccountWithJob(updated, &Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: updated.ID,
		Payload: map[string]any{"account_id": updated.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := data.GetAccount(updated.ID); !reflect.DeepEqual(got, updated) {
		t.Fatalf("account mismatch: %+v", got)
	}
	if job == nil || job.ID == "" || job.State != "queued" || data.GetJob(job.ID) == nil {
		t.Fatalf("job was not committed and normalized: %+v", job)
	}
}

func TestMemoryUpdateAccountWithJobRollsBackOnDuplicateJob(t *testing.T) {
	data := NewMemory()
	data.PutPackage(&Package{ID: "package-1"})
	original := &Account{
		ID: "account-1", Username: "customer", PrimaryDomain: "old.test",
		PackageID: "package-1", Status: "active", DesiredRevision: 1,
	}
	data.PutAccount(original)
	if _, err := data.EnqueueJob(&Job{ID: "job-1", Type: "existing"}); err != nil {
		t.Fatal(err)
	}
	updated := data.GetAccount(original.ID)
	updated.Status = "suspended"
	updated.DesiredRevision++

	if job, err := data.UpdateAccountWithJob(updated, &Job{
		ID: "job-1", Type: "account.reconcile", ResourceID: updated.ID,
	}); err == nil || job != nil {
		t.Fatalf("duplicate job result: %+v, %v", job, err)
	}
	if got := data.GetAccount(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatalf("failed update changed account: %+v", got)
	}
}
