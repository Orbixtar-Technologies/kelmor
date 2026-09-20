package httpserver

import (
	"net/http"
	"testing"
	"time"

	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func TestServiceCertificateInventoryIsHonestWhenEmpty(t *testing.T) {
	srv, _, token := directorFixture(t)
	listed := get(t, srv.URL+"/api/v1/server/ssl/service", token)
	items, _ := listed["items"].([]any)
	if len(items) < 3 {
		t.Fatalf("expected service slots: %v", listed)
	}
	for _, raw := range items {
		row := raw.(map[string]any)
		if row["status"] != "missing" {
			t.Fatalf("empty host must not invent certs: %v", row)
		}
	}
}

func TestServiceCertificateInstallQueuesHostJob(t *testing.T) {
	srv, st, token := directorFixture(t)
	cert, key, err := paneltls.SelfSigned("kelmor.host", time.Now().Add(40*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	queued := post(t, srv.URL+"/api/v1/server/ssl/service", token, map[string]any{
		"service": "director", "hostname": "kelmor.host",
		"cert_pem": string(cert), "key_pem": string(key),
	})
	if queued["operation_id"] == nil {
		t.Fatalf("install: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "host.ssl.service.apply" {
		t.Fatalf("job: %+v", job)
	}
	if job.ResourceID != "" {
		t.Fatalf("resource_id must stay empty: %q", job.ResourceID)
	}
	if job.Payload["target"] != "director" {
		t.Fatalf("payload: %+v", job.Payload)
	}
	if job.Payload["cert_pem"] != nil || job.Payload["key_pem"] != nil {
		t.Fatalf("job must not store PEM material: %+v", job.Payload)
	}
	listed := get(t, srv.URL+"/api/v1/server/ssl/service", token)
	found := false
	for _, raw := range listed["items"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == "director" && row["status"] == "installed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("inventory after install: %v", listed)
	}
}

func TestServiceCertificateInstallRejectsMismatch(t *testing.T) {
	srv, _, token := directorFixture(t)
	cert, _, err := paneltls.SelfSigned("kelmor.host", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := paneltls.SelfSigned("other.host", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	code, _ := postStatus(t, srv.URL+"/api/v1/server/ssl/service", token, map[string]any{
		"service": "mail", "cert_pem": string(cert), "key_pem": string(other),
	})
	if code != http.StatusBadRequest {
		t.Fatalf("mismatch %d", code)
	}
}

func TestServiceCertificateListAcceptsWebsitesRead(t *testing.T) {
	srv, st, _ := directorFixture(t)
	putUpdateTestUser(t, st, "ssl-auditor", "auditor")
	token := loginUpdateTestUser(t, srv.URL, "ssl-auditor", "UpdateTestPass!2026")
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/ssl/service", token, nil); status != http.StatusOK {
		t.Fatalf("auditor can list service certs: %d", status)
	}
	if status, _ := postStatus(t, srv.URL+"/api/v1/server/ssl/service", token, map[string]any{
		"service": "director", "cert_pem": "x", "key_pem": "y",
	}); status != http.StatusForbidden {
		t.Fatalf("auditor must not install")
	}
}
