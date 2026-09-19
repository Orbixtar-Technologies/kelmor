package httpserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/rbac"
)

func TestLanguageModulesQueueHostInstall(t *testing.T) {
	srv, st, token := directorFixture(t)

	empty := get(t, srv.URL+"/api/v1/server/modules?kind=perl", token)
	items, _ := empty["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected empty modules: %v", empty)
	}

	queued := post(t, srv.URL+"/api/v1/server/modules", token, map[string]string{
		"kind": "perl", "name": "JSON::XS",
	})
	if queued["operation_id"] == nil || queued["status"] != "provisioning" {
		t.Fatalf("install: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "host.module.install" {
		t.Fatalf("job: %+v", job)
	}
	if job.Payload["kind"] != "perl" || job.Payload["name"] != "JSON::XS" {
		t.Fatalf("payload: %+v", job.Payload)
	}
	if job.ResourceID != "" {
		t.Fatalf("resource_id must stay empty for host jobs: %q", job.ResourceID)
	}

	code, body := postStatus(t, srv.URL+"/api/v1/server/modules", token, map[string]string{
		"kind": "perl", "name": "JSON::XS; rm -rf /",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("hostile name %d %v", code, body)
	}
}

func TestLanguageModulesRequireWriteCapability(t *testing.T) {
	srv, st, _ := directorFixture(t)
	putUpdateTestUser(t, st, "mod-auditor", "auditor")
	token := loginUpdateTestUser(t, srv.URL, "mod-auditor", "UpdateTestPass!2026")
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/modules", token, nil); status != http.StatusOK {
		t.Fatalf("auditor can list modules: %d", status)
	}
	if status, _ := postStatus(t, srv.URL+"/api/v1/server/modules", token, map[string]string{
		"kind": "pecl", "name": "redis",
	}); status != http.StatusForbidden {
		t.Fatalf("auditor must not install")
	}
}

func TestServerProfileSettingsQueueHostApply(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"server_profile": map[string]string{"profile": "mail"},
		},
	})
	if saved["operation_id"] == nil {
		t.Fatalf("profile must queue host apply: %v", saved)
	}
	job := st.GetJob(saved["operation_id"].(string))
	if job == nil || job.Type != "host.config.apply" {
		t.Fatalf("job: %+v", job)
	}
}

func TestConfigurationClusterSettingsQueueHostApply(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"configuration_cluster": map[string]string{"peers": "https://peer.example.test:2087"},
		},
	})
	if saved["operation_id"] == nil {
		t.Fatalf("cluster must queue host apply: %v", saved)
	}
	job := st.GetJob(saved["operation_id"].(string))
	if job == nil || job.Type != "host.config.apply" {
		t.Fatalf("job: %+v", job)
	}

	published := post(t, srv.URL+"/api/v1/server/cluster/publish", token, map[string]any{})
	if published["ok"] != true {
		t.Fatalf("publish: %v", published)
	}
	snap := get(t, srv.URL+"/api/v1/server/cluster/snapshot", token)
	if snap["packages"] == nil && snap["feature_sets"] == nil {
		t.Fatalf("snapshot: %v", snap)
	}
}

func TestGrantSupportAccessIssuesOperatorSession(t *testing.T) {
	srv, st, token := directorFixture(t)
	granted := post(t, srv.URL+"/api/v1/server/support-access", token, map[string]any{
		"ticket": "CASE-99", "hours": 2,
	})
	supportToken, _ := granted["token"].(string)
	username, _ := granted["username"].(string)
	if supportToken == "" || username == "" || granted["expires_at"] == nil {
		t.Fatalf("grant: %v", granted)
	}
	user := st.UserByUsername(username)
	if user == nil || !hasRole(user.Roles, "server_operator") {
		t.Fatalf("support user: %+v", user)
	}
	me := get(t, srv.URL+"/api/v1/me", supportToken)
	actor, _ := me["actor"].(map[string]any)
	caps, _ := actor["capabilities"].(map[string]any)
	if caps[rbac.ServerRead] != true || caps[rbac.ServerSettingsWrite] == true {
		t.Fatalf("support caps: %v", caps)
	}

	listed := get(t, srv.URL+"/api/v1/server/support-access", token)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("list: %v", listed)
	}
	grantID := items[0].(map[string]any)["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/server/support-access/"+grantID, token, nil) != http.StatusOK {
		t.Fatal("revoke")
	}
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/me", supportToken, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked session still works: %d", status)
	}
}

func TestDiagnosticsDownloadIsAuthenticatedArchive(t *testing.T) {
	srv, _, token := directorFixture(t)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/server/diagnostics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("diagnostics %d %s", res.StatusCode, body)
	}
	if !strings.Contains(res.Header.Get("Content-Disposition"), "kelmor-diagnostics") {
		t.Fatalf("disposition: %s", res.Header.Get("Content-Disposition"))
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil || len(raw) < 20 {
		t.Fatalf("archive: %d %v", len(raw), err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	names := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names[hdr.Name] = true
	}
	if !names["director/jobs.json"] && !names["director/settings.json"] {
		t.Fatalf("bundle names: %v", names)
	}

	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/diagnostics", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated diagnostics: %d", status)
	}
}

func TestSupportAccessRejectsInvalidTicket(t *testing.T) {
	srv, _, token := directorFixture(t)
	code, _ := postStatus(t, srv.URL+"/api/v1/server/support-access", token, map[string]any{
		"ticket": "bad ticket;rm", "hours": 2,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("ticket %d", code)
	}
}
