package httpserver

import (
	"net/http"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
)

func TestModifyAccountRejectsInvalidIP(t *testing.T) {
	srv, _, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "ipuser", "ip.example.test", pkg)
	if status, body := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid, token, map[string]any{
		"ip_address": "not-an-ip",
	}); status != http.StatusBadRequest {
		t.Fatalf("invalid IP must be refused: %d %v", status, body)
	}
	updated := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid, token, map[string]any{
		"ip_address": "203.0.113.10",
	})
	account := updated["account"].(map[string]any)
	if account["ip_address"] != "203.0.113.10" {
		t.Fatalf("dedicated IPv4: %v", updated)
	}
}

func TestRestoreAcceptsPathAndLatestBackup(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "restuser", "restore.example.test", pkg)
	st.PutBackup(&store.BackupRun{
		ID: id.New(), AccountID: aid, Kind: "full", State: "succeeded",
		Destination: "local", CreatedAt: time.Now().UTC(),
		Manifest: map[string]any{"key": "hpm1/" + aid + "/latest"},
	})
	queued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/restores", token, map[string]any{
		"path": "public_html",
	})
	if queued["operation_id"] == nil || queued["operation_id"] == "" {
		t.Fatalf("restore job missing: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Payload["path"] != "public_html" {
		t.Fatalf("path not queued: %+v", job)
	}
}

func TestClearBandwidthHoldsSkipsOtherSuspended(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	heldID := seedAccount(t, srv, token, "helduser", "held.example.test", pkg)
	otherID := seedAccount(t, srv, token, "otheruser", "other.example.test", pkg)
	held := st.GetAccount(heldID)
	other := st.GetAccount(otherID)
	held.Status = "active"
	other.Status = "suspended"
	st.PutAccount(held)
	st.PutAccount(other)
	st.PutUsage(&store.Usage{AccountID: heldID, BandwidthHold: true, BandwidthBytes: 1 << 40, CollectedAt: time.Now().UTC()})
	cleared := doJSON(t, http.MethodPost, srv.URL+"/api/v1/accounts/bulk/clear-bandwidth-hold", token, map[string]any{})
	ops, _ := cleared["operations"].([]any)
	if len(ops) != 1 {
		t.Fatalf("expected one hold clear, got %v", cleared)
	}
	if st.GetAccount(otherID).Status != "suspended" {
		t.Fatal("non-bandwidth suspended account must stay suspended")
	}
	usage := st.GetUsage(heldID)
	if usage == nil || usage.BandwidthHold {
		t.Fatalf("hold not cleared: %+v", usage)
	}
}

func TestChromeSettingsSkipHostJob(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{"theme": map[string]string{"density": "compact"}},
	})
	if saved["operation_id"] != nil {
		t.Fatalf("chrome save must not queue host apply: %v", saved)
	}
	for _, job := range st.ListJobs("", 50) {
		if job.Type == "host.config.apply" {
			t.Fatalf("unexpected host apply: %+v", job)
		}
	}
}
