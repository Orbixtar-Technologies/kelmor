package store

import (
	"testing"
	"time"
)

func TestAccountAndZonePagesStayBounded(t *testing.T) {
	st := NewMemory()
	for i, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		st.PutAccount(&Account{ID: "a" + name, Username: name, Status: "active", PrimaryDomain: name + ".test"})
		st.PutUsage(&Usage{AccountID: "a" + name, DiskBytes: int64(i + 1), CollectedAt: time.Now().UTC()})
		st.PutZone(&DNSZone{ID: "z" + name, AccountID: "a" + name, Name: name + ".test"})
		st.PutRecord(&DNSRecord{ID: "r" + name, ZoneID: "z" + name, Name: "www", Type: "A", Content: "203.0.113.10"})
	}
	first := st.ListAccountsPage("", "", "", 2)
	if len(first.Items) != 2 || !first.HasMore || first.NextCursor != "bravo" {
		t.Fatalf("first page: %+v", first)
	}
	if first.Usage["aalpha"].DiskBytes != 1 || first.Usage["abravo"].DiskBytes != 2 {
		t.Fatalf("joined usage: %+v", first.Usage)
	}
	second := st.ListAccountsPage("", "", first.NextCursor, 2)
	if len(second.Items) != 2 || second.HasMore || second.Items[0].Username != "charlie" {
		t.Fatalf("second page: %+v", second)
	}

	zones := st.ListZonesPage("", "", 2)
	if len(zones.Items) != 2 || !zones.HasMore || zones.Items[0].RecordCount != 1 {
		t.Fatalf("zones: %+v", zones)
	}
	next := st.ListZonesPage("", zones.NextCursor, 2)
	if len(next.Items) != 2 || next.HasMore {
		t.Fatalf("zone page 2: %+v", next)
	}
}

func TestAuditCursorPaginationIsStable(t *testing.T) {
	st := NewMemory()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		st.AppendAudit(AuditEvent{
			ID: "e" + string(rune('a'+i)), Action: "account.inspect", Success: true,
			OccurredAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	st.AppendAudit(AuditEvent{ID: "late", Action: "account.create", Success: true, OccurredAt: now.Add(10 * time.Second)})
	page := st.QueryAudit(AuditFilter{Limit: 2})
	if len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("page: %+v", page)
	}
	next := st.QueryAudit(AuditFilter{Limit: 2, Cursor: page.NextCursor})
	seen := map[string]bool{}
	for _, event := range append(page.Items, next.Items...) {
		if seen[event.ID] {
			t.Fatalf("duplicated %s", event.ID)
		}
		seen[event.ID] = true
	}
	if len(next.Items) != 2 {
		t.Fatalf("next: %+v", next)
	}
}

func TestListDueCertificatesIsBounded(t *testing.T) {
	st := NewMemory()
	soon := time.Now().Add(2 * 24 * time.Hour)
	far := time.Now().Add(80 * 24 * time.Hour)
	st.PutCert(&Certificate{ID: "due-1", Status: "active", Hostname: "a.test", NotAfter: &soon})
	st.PutCert(&Certificate{ID: "due-2", Status: "active", Hostname: "b.test", NotAfter: &soon})
	st.PutCert(&Certificate{ID: "later", Status: "active", Hostname: "c.test", NotAfter: &far})
	got := st.ListDueCertificates(time.Now().Add(30*24*time.Hour), 1)
	if len(got) != 1 {
		t.Fatalf("bounded due certs: %+v", got)
	}
}
