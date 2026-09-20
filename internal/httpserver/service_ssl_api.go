package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) listServiceCertificates(w http.ResponseWriter, r *http.Request) {
	ac := actor(r)
	if !ac.Has(rbac.WebsitesRead) && !ac.Has(rbac.ServerRead) {
		a.require(w, r, rbac.WebsitesRead)
		return
	}
	hostname := publicPortalHostname(r)
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListServiceCertificates",
		Params: mustJSON(map[string]any{"hostname": hostname}),
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{
			"items":    []any{},
			"hostname": hostname,
			"note":     "Host service certificates are not readable yet. OpenSSL material is not invented.",
		})
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) installServiceCertificate(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Service  string `json:"service"`
		Hostname string `json:"hostname"`
		CertPEM  string `json:"cert_pem"`
		KeyPEM   string `json:"key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid service certificate payload", false)
		return
	}
	if in.Hostname == "" {
		in.Hostname = publicPortalHostname(r)
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "InstallServiceCertificate",
		Params: mustJSON(map[string]any{
			"service": in.Service, "hostname": in.Hostname,
			"cert_pem": in.CertPEM, "key_pem": in.KeyPEM,
		}),
	})
	if err != nil {
		a.fail(w, r, 400, "SERVICE_CERT_ERROR", err.Error(), false)
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "host.ssl.service.apply", ResourceType: "server",
		Payload: map[string]any{
			"service": in.Service, "hostname": in.Hostname, "target": in.Service,
		},
	}, a.auditEvent(r, "", "server.ssl.service.install", "server", "", nil, map[string]any{
		"service": in.Service, "hostname": in.Hostname,
	}))
	if err != nil {
		a.fail(w, r, 500, "SERVICE_CERT_ERROR", "Could not queue service certificate apply", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "result": raw})
}
