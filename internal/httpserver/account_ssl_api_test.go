package httpserver

import (
	"net/http"
	"testing"
	"time"

	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func TestInstallAccountCertificateQueuesHostJob(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "shopssl", "shopssl.test", pkg)
	cert, key, err := paneltls.SelfSigned("shopssl.test", time.Now().Add(40*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	queued := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates/install", token, map[string]any{
		"hostname": "shopssl.test",
		"cert_pem": string(cert),
		"key_pem":  string(key),
	})
	if queued["operation_id"] == nil {
		t.Fatalf("install: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "host.ssl.account.apply" {
		t.Fatalf("job: %+v", job)
	}
	if job.ResourceID != "" {
		t.Fatalf("resource_id must stay empty: %q", job.ResourceID)
	}
	if job.Payload["target"] != "shopssl.test" {
		t.Fatalf("payload: %+v", job.Payload)
	}
	if job.Payload["cert_pem"] != nil || job.Payload["key_pem"] != nil || job.Payload["ca_pem"] != nil {
		t.Fatalf("job must not store PEM material: %+v", job.Payload)
	}
	certs := st.ListCerts(aid)
	if len(certs) != 1 || certs[0].Hostname != "shopssl.test" || certs[0].Kind != "custom" || certs[0].Status != "active" {
		t.Fatalf("store certs: %+v", certs)
	}
	listed := get(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", token)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("list after install: %v", listed)
	}
}

func TestInstallAccountCertificateRejectsForeignAndInvalid(t *testing.T) {
	srv, _, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "badssl", "badssl.test", pkg)
	cert, key, err := paneltls.SelfSigned("badssl.test", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates/install", token, map[string]any{
		"hostname": "evil.example", "cert_pem": string(cert), "key_pem": string(key),
	}); code != http.StatusBadRequest {
		t.Fatalf("foreign hostname %d", code)
	}
	if code, _ := postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates/install", token, map[string]any{
		"hostname": "badssl.test", "cert_pem": "x", "key_pem": "y",
	}); code != http.StatusBadRequest {
		t.Fatalf("invalid pem %d", code)
	}
}
