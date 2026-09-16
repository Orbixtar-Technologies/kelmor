package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestDuplicateRestoreIsAccountUnique(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]string{
		"username": "rst42", "primary_domain": "rst.test",
		"package_id": st.ListPackages()[0].ID, "owner_email": "owner@rst.test",
		"owner_password": "TenantPass!2026",
	})
	aid := created["resource_id"].(string)
	st.PutBackup(&store.BackupRun{
		ID: "11111111-1111-7111-8111-111111111111", AccountID: aid,
		Kind: "full", State: "succeeded", Destination: "local",
		Manifest: map[string]any{"key": "rst42/one.hpm", "format_version": 3},
	})
	firstStatus, first := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/restores", token, map[string]string{
		"backup_id": "11111111-1111-7111-8111-111111111111",
	}, nil)
	if firstStatus != http.StatusAccepted {
		t.Fatalf("first restore %d %v", firstStatus, first)
	}
	secondStatus, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/restores", token, map[string]string{
		"backup_id": "11111111-1111-7111-8111-111111111111",
	}, nil)
	if secondStatus != http.StatusConflict {
		t.Fatalf("duplicate restore status %d", secondStatus)
	}
}
