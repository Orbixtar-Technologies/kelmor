package httpserver

import (
	"net/http"
	"testing"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
)

func TestDNSCleanupAndSynchronizeInventory(t *testing.T) {
	srv, st, token := directorFixture(t)
	pid := firstPackageID(t, srv, token)
	liveID := seedAccount(t, srv, token, "shop", "shop.test", pid)
	goneID := seedAccount(t, srv, token, "oldshop", "oldshop.test", pid)

	liveDomain := st.ListDomains(liveID)[0]
	goneDomain := st.ListDomains(goneID)[0]
	liveZone := &store.DNSZone{
		ID: id.New(), AccountID: liveID, DomainID: liveDomain.ID,
		Name: liveDomain.ASCII, Provider: "powerdns", DesiredRevision: 3,
	}
	goneZone := &store.DNSZone{
		ID: id.New(), AccountID: goneID, DomainID: goneDomain.ID,
		Name: goneDomain.ASCII, Provider: "powerdns",
	}
	untiedZone := &store.DNSZone{
		ID: id.New(), AccountID: id.New(), DomainID: id.New(),
		Name: "orphan.test", Provider: "powerdns",
	}
	st.PutZone(liveZone)
	st.PutZone(goneZone)
	st.PutZone(untiedZone)
	gone := st.GetAccount(goneID)
	gone.Status = "terminated"
	st.PutAccount(gone)

	emptyCleanup := get(t, srv.URL+"/api/v1/dns/cleanup", token)
	if emptyCleanup["items"] == nil {
		t.Fatalf("cleanup list missing items: %v", emptyCleanup)
	}

	cleanupItems := namedItems(t, get(t, srv.URL+"/api/v1/dns/cleanup", token))
	if _, ok := cleanupItems["shop.test"]; ok {
		t.Fatalf("live zone must not be leftover: %v", cleanupItems)
	}
	if cleanupItems["oldshop.test"]["reason"] != "terminated_account" {
		t.Fatalf("terminated leftover: %v", cleanupItems["oldshop.test"])
	}
	if cleanupItems["orphan.test"]["reason"] != "untied_zone" {
		t.Fatalf("untied leftover: %v", cleanupItems["orphan.test"])
	}

	syncItems := namedItems(t, get(t, srv.URL+"/api/v1/dns/synchronize", token))
	if _, ok := syncItems["shop.test"]; !ok {
		t.Fatalf("managed list omitted live zone: %v", syncItems)
	}
	if _, ok := syncItems["oldshop.test"]; ok {
		t.Fatalf("terminated zone must not be managed: %v", syncItems)
	}
	if _, ok := syncItems["orphan.test"]; ok {
		t.Fatalf("untied zone must not be managed: %v", syncItems)
	}

	queued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/dns/cleanup", token, map[string]any{
		"zone_ids": []string{goneZone.ID, untiedZone.ID},
	})
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "dns.cleanup" {
		t.Fatalf("cleanup job: %+v", job)
	}
	ids, _ := job.Payload["zone_ids"].([]string)
	if len(ids) != 2 {
		if raw, ok := job.Payload["zone_ids"].([]any); ok {
			ids = make([]string, 0, len(raw))
			for _, value := range raw {
				ids = append(ids, value.(string))
			}
		}
	}
	if len(ids) != 2 {
		t.Fatalf("selected leftover ids must stay on the job: %+v", job.Payload)
	}

	syncQueued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/dns/synchronize", token, map[string]any{
		"zone_ids": []string{liveZone.ID},
	})
	syncJob := st.GetJob(syncQueued["operation_id"].(string))
	if syncJob == nil || syncJob.Type != "dns.synchronize" {
		t.Fatalf("sync job: %+v", syncJob)
	}
}

func TestDNSInventoryEmptyLists(t *testing.T) {
	srv, _, token := directorFixture(t)
	cleanup := get(t, srv.URL+"/api/v1/dns/cleanup", token)
	if items, _ := cleanup["items"].([]any); len(items) != 0 {
		t.Fatalf("fresh host leftover %v", cleanup)
	}
	sync := get(t, srv.URL+"/api/v1/dns/synchronize", token)
	if items, _ := sync["items"].([]any); len(items) != 0 {
		t.Fatalf("fresh host managed %v", sync)
	}
}

func namedItems(t *testing.T, body map[string]any) map[string]map[string]any {
	t.Helper()
	raw, _ := body["items"].([]any)
	out := map[string]map[string]any{}
	for _, item := range raw {
		row, _ := item.(map[string]any)
		name, _ := row["name"].(string)
		if name == "" {
			t.Fatalf("inventory row missing name: %v", item)
		}
		out[name] = row
	}
	return out
}
