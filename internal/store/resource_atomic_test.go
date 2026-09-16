package store

import "testing"

func TestMemoryCreateDomainWithJobRollsBackOnJobConflict(t *testing.T) {
	data := NewMemory()
	data.PutAccount(&Account{ID: "account-1"})
	if _, err := data.EnqueueJob(&Job{ID: "job-conflict", Type: "existing"}); err != nil {
		t.Fatal(err)
	}
	domain := &Domain{
		ID: "domain-1", AccountID: "account-1", FQDN: "example.test",
		ASCII: "example.test", Type: "addon", Status: "provisioning",
	}
	audit := AuditEvent{
		ID: "audit-1", Action: "domain.create", ResourceType: "domain",
		ResourceID: domain.ID, AccountID: domain.AccountID,
	}

	if job, err := data.CreateDomainWithJob(domain, &Job{
		ID: "job-conflict", Type: "domain.provision", ResourceType: "domain",
		ResourceID: domain.ID,
	}, audit); err == nil || job != nil {
		t.Fatalf("conflicting mutation returned %+v, %v", job, err)
	}
	if data.GetDomain(domain.ID) != nil {
		t.Fatal("failed job insert left domain behind")
	}
	if events := data.ListAudit(100); len(events) != 0 {
		t.Fatalf("failed mutation left audit events behind: %+v", events)
	}
}

func TestMemoryUpsertWebsiteWithJobCommitsRevisionJobAndAudit(t *testing.T) {
	data := NewMemory()
	data.PutAccount(&Account{ID: "account-1"})
	data.PutDomain(&Domain{ID: "domain-1", AccountID: "account-1", ASCII: "example.test"})
	website := &Website{
		ID: "website-1", AccountID: "account-1", DomainID: "domain-1",
		Runtime: "php", DocumentRoot: "/home/customer/public_html",
		DesiredRevision: 1,
	}
	job, err := data.UpsertWebsiteWithJob(website, &Job{
		ID: "job-1", Type: "website.provision", ResourceType: "website",
		ResourceID: website.ID,
	}, AuditEvent{
		ID: "audit-1", Action: "website.create", ResourceType: "website",
		ResourceID: website.ID, AccountID: website.AccountID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || data.GetJob(job.ID) == nil {
		t.Fatal("job was not committed")
	}
	if got := data.GetWebsite(website.ID); got == nil || got.DesiredRevision != 1 {
		t.Fatalf("website was not committed: %+v", got)
	}
	if events := data.ListAudit(100); len(events) != 1 || events[0].ResourceID != website.ID {
		t.Fatalf("audit was not committed: %+v", events)
	}
}
