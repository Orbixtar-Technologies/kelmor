package httpserver

import (
	"net/http"
	"testing"
)

func TestPostgresSettingsQueueHostApplyWithAuth(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"postgres": map[string]string{"listen": "127.0.0.1", "auth": "scram-sha-256"},
		},
	})
	if saved["operation_id"] == nil {
		t.Fatalf("postgres settings must queue host apply: %v", saved)
	}
	job := st.GetJob(saved["operation_id"].(string))
	if job == nil || job.Type != "host.config.apply" {
		t.Fatalf("job: %+v", job)
	}
	if job.ResourceID != "" {
		t.Fatalf("resource_id must stay empty for host jobs: %q", job.ResourceID)
	}
	if job.Payload["target"] != "host-config" {
		t.Fatalf("logical target must stay in payload: %+v", job.Payload)
	}

	status := get(t, srv.URL+"/api/v1/server/postgres", token)
	if status["installed"] == true {
		t.Fatalf("sandbox must not invent a postgres daemon: %v", status)
	}
	if msg, _ := status["message"].(string); msg == "" {
		t.Fatalf("missing postgres needs an honest message: %v", status)
	}
}

func TestMySQLUpgradeQueuesHostJob(t *testing.T) {
	srv, st, token := directorFixture(t)

	listed := get(t, srv.URL+"/api/v1/server/mysql-upgrade", token)
	if listed["installed"] == true {
		t.Fatalf("sandbox must not invent MariaDB: %v", listed)
	}
	if listed["engine"] != "" && listed["engine"] != nil {
		if listed["installed"] != false {
			t.Fatalf("engine without install flag: %v", listed)
		}
	}

	queued := post(t, srv.URL+"/api/v1/server/mysql-upgrade", token, map[string]string{
		"target": "10.11",
	})
	if queued["operation_id"] == nil || queued["status"] != "provisioning" {
		t.Fatalf("upgrade: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "mysql.upgrade" {
		t.Fatalf("job: %+v", job)
	}
	if job.ResourceID != "" {
		t.Fatalf("resource_id must stay empty for host jobs: %q", job.ResourceID)
	}
	if job.Payload["target"] != "10.11" {
		t.Fatalf("payload: %+v", job.Payload)
	}

	code, _ := postStatus(t, srv.URL+"/api/v1/server/mysql-upgrade", token, map[string]string{
		"target": "8.0; rm -rf /",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("hostile target %d", code)
	}
}

func TestMySQLUpgradeRequiresWriteCapability(t *testing.T) {
	srv, st, _ := directorFixture(t)
	putUpdateTestUser(t, st, "sql-auditor", "auditor")
	token := loginUpdateTestUser(t, srv.URL, "sql-auditor", "UpdateTestPass!2026")
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/mysql-upgrade", token, nil); status != http.StatusOK {
		t.Fatalf("auditor can read upgrade status: %d", status)
	}
	if status, _ := postStatus(t, srv.URL+"/api/v1/server/mysql-upgrade", token, map[string]string{
		"target": "10.11",
	}); status != http.StatusForbidden {
		t.Fatalf("auditor must not queue upgrade")
	}
}
