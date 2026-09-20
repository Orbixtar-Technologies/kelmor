package httpserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestFTPServerSettingsQueueHostApply(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"ftp_server": map[string]string{
				"pasv_min": "41000",
				"pasv_max": "41100",
				"banner":   "Kelmor FTP ready.",
			},
		},
	})
	if saved["operation_id"] == nil {
		t.Fatalf("ftp server settings must queue host apply: %v", saved)
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
	if !payloadHasKey(job.Payload["keys"], "ftp_server") {
		t.Fatalf("payload keys must name ftp_server: %+v", job.Payload)
	}

	again := get(t, srv.URL+"/api/v1/server/settings", token)
	row, _ := again["values"].(map[string]any)["ftp_server"].(map[string]any)
	if row["banner"] != "Kelmor FTP ready." || row["pasv_min"] != "41000" || row["pasv_max"] != "41100" {
		t.Fatalf("ftp settings persist: %v", again)
	}
}

func payloadHasKey(raw any, want string) bool {
	switch keys := raw.(type) {
	case []string:
		for _, key := range keys {
			if key == want {
				return true
			}
		}
	case []any:
		for _, key := range keys {
			if key == want {
				return true
			}
		}
	}
	return false
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
	if published["operation_id"] == nil {
		t.Fatalf("publish must queue a host job: %v", published)
	}
	publishJob := st.GetJob(published["operation_id"].(string))
	if publishJob == nil || publishJob.Type != "cluster.snapshot.publish" {
		t.Fatalf("publish job: %+v", publishJob)
	}
	if publishJob.ResourceID != "" {
		t.Fatalf("resource_id must stay empty for host jobs: %q", publishJob.ResourceID)
	}
	snap := get(t, srv.URL+"/api/v1/server/cluster/snapshot", token)
	if snap["packages"] == nil && snap["feature_sets"] == nil {
		t.Fatalf("snapshot: %v", snap)
	}
	caps, _ := snap["capabilities"].(map[string]any)
	if caps["peer_membership"] != true || caps["snapshot_publish"] != true || caps["peer_health_probe"] != true {
		t.Fatalf("single-node capabilities: %v", caps)
	}
	if caps["live_multi_node"] != true {
		t.Fatalf("saved peers must enable multi-node apply: %v", caps)
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

func TestConfigurationClusterImportAndPeerProbe(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(peer.Close)

	srv, st, token := directorFixture(t)
	imported := post(t, srv.URL+"/api/v1/server/cluster/snapshot/import", token, map[string]any{
		"snapshot": map[string]any{
			"note": "imported fixture", "packages": []any{map[string]any{"name": "Imported"}},
		},
	})
	if imported["operation_id"] == nil {
		t.Fatalf("import must queue a host job: %v", imported)
	}
	job := st.GetJob(imported["operation_id"].(string))
	if job == nil || job.Type != "cluster.snapshot.import" {
		t.Fatalf("import job: %+v", job)
	}

	probed := post(t, srv.URL+"/api/v1/server/cluster/probe", token, map[string]any{
		"url": peer.URL,
	})
	if probed["operation_id"] == nil {
		t.Fatalf("probe must queue a host job: %v", probed)
	}
	probeJob := st.GetJob(probed["operation_id"].(string))
	if probeJob == nil || probeJob.Type != "cluster.peer.probe" {
		t.Fatalf("probe job: %+v", probeJob)
	}
	items, _ := probed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("probe items: %v", probed)
	}
	row, _ := items[0].(map[string]any)
	if row["ok"] != true || row["url"] != peer.URL {
		t.Fatalf("probe row: %v", row)
	}

	code, _ := postStatus(t, srv.URL+"/api/v1/server/cluster/probe", token, map[string]any{
		"url": "https://evil.test; rm -rf /",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("hostile probe %d", code)
	}
}

func TestConfigurationClusterApplyRequiresLinkedNodes(t *testing.T) {
	srv, _, token := directorFixture(t)
	snap := get(t, srv.URL+"/api/v1/server/cluster/snapshot", token)
	caps, _ := snap["capabilities"].(map[string]any)
	if caps["live_multi_node"] == true {
		t.Fatalf("no peers must keep multi-node unavailable: %v", caps)
	}
	code, _ := postStatus(t, srv.URL+"/api/v1/server/cluster/apply", token, map[string]any{})
	if code != http.StatusBadRequest {
		t.Fatalf("apply without peers %d", code)
	}
}

func TestConfigurationClusterApplyPushesToLinkedNodes(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/server/cluster/snapshot/import" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(peer.Close)

	srv, st, token := directorFixture(t)
	doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"configuration_cluster": map[string]string{"peers": peer.URL},
		},
	})
	applied := post(t, srv.URL+"/api/v1/server/cluster/apply", token, map[string]any{
		"token": "peer-token",
	})
	if applied["operation_id"] == nil {
		t.Fatalf("apply: %v", applied)
	}
	job := st.GetJob(applied["operation_id"].(string))
	if job == nil || job.Type != "cluster.snapshot.apply" {
		t.Fatalf("apply job: %+v", job)
	}
	if job.ResourceID != "" || job.Payload["target"] != "linked-nodes" {
		t.Fatalf("job contract: %+v", job)
	}
	items, _ := applied["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("apply items: %v", applied)
	}
}

func TestRemoteAccessKeyIsHostAppliedAndHonored(t *testing.T) {
	srv, st, token := directorFixture(t)
	issued := post(t, srv.URL+"/api/v1/server/remote-access-key", token, map[string]any{})
	key, _ := issued["key"].(string)
	if !strings.HasPrefix(key, "hp_remote_") || issued["operation_id"] == nil {
		t.Fatalf("issue: %v", issued)
	}
	job := st.GetJob(issued["operation_id"].(string))
	if job == nil || job.Type != "host.remote_access.apply" {
		t.Fatalf("apply job: %+v", job)
	}
	listed := get(t, srv.URL+"/api/v1/server/remote-access-key", token)
	if listed["applied"] != true || listed["key"] != nil {
		t.Fatalf("status must hide the secret: %v", listed)
	}
	if listed["host_path"] != "/etc/panel/remote-access-key" {
		t.Fatalf("host path: %v", listed)
	}

	me := get(t, srv.URL+"/api/v1/me", key)
	actor, _ := me["actor"].(map[string]any)
	caps, _ := actor["capabilities"].(map[string]any)
	if caps[rbac.ServerRead] != true || actor["is_server_scope"] != true {
		t.Fatalf("remote key actor: %v", me)
	}

	revoked := doJSON(t, http.MethodDelete, srv.URL+"/api/v1/server/remote-access-key", token, nil)
	if revoked["operation_id"] == nil {
		t.Fatalf("revoke: %v", revoked)
	}
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/me", key, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked key still works: %d", status)
	}
}

func TestRemoteAccessKeyRequiresServerTokenCapability(t *testing.T) {
	srv, st, _ := directorFixture(t)
	putUpdateTestUser(t, st, "key-auditor", "auditor")
	token := loginUpdateTestUser(t, srv.URL, "key-auditor", "UpdateTestPass!2026")
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/remote-access-key", token, nil); status != http.StatusForbidden {
		t.Fatalf("auditor must not read remote key: %d", status)
	}
	if status, _ := postStatus(t, srv.URL+"/api/v1/server/remote-access-key", token, map[string]any{}); status != http.StatusForbidden {
		t.Fatalf("auditor must not issue remote key")
	}
}
