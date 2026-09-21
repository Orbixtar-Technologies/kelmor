package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/validate"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) installAccountCertificate(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	var in struct {
		Hostname string `json:"hostname"`
		CertPEM  string `json:"cert_pem"`
		KeyPEM   string `json:"key_pem"`
		CAPEM    string `json:"ca_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid certificate payload", false)
		return
	}
	host, err := validate.NormalizeDomain(in.Hostname)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if !accountOwnsHostname(a.Store, aid, host) {
		a.fail(w, r, 400, "VALIDATION", "hostname must be a domain on this account", false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "InstallAccountCertificate",
		Params: mustJSON(map[string]any{
			"hostname": host, "cert_pem": in.CertPEM, "key_pem": in.KeyPEM, "ca_pem": in.CAPEM,
		}),
	})
	if err != nil {
		a.fail(w, r, 400, "CERTIFICATE_INSTALL_ERROR", err.Error(), false)
		return
	}
	cert := upsertCustomCertificate(a.Store, aid, host, raw)
	a.applyAccountHostnameWebsites(r, aid, host)
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "host.ssl.account.apply", ResourceType: "server",
		Payload: map[string]any{
			"account_id": aid, "hostname": host, "target": host,
		},
	}, a.auditEvent(r, aid, "certificate.install", "certificate", cert.ID, nil, map[string]any{
		"hostname": host, "kind": "custom",
	}))
	if err != nil {
		a.fail(w, r, 500, "CERTIFICATE_INSTALL_ERROR", "Could not queue certificate apply", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "certificate": cert, "result": raw})
}

func accountOwnsHostname(st store.Store, accountID, hostname string) bool {
	for _, domain := range st.ListDomains(accountID) {
		if domain.ASCII == hostname {
			return true
		}
	}
	return false
}

func upsertCustomCertificate(st store.Store, accountID, hostname string, raw any) *store.Certificate {
	var existing *store.Certificate
	for _, cert := range st.ListCerts(accountID) {
		if cert.Hostname != hostname {
			continue
		}
		cp := cert
		existing = &cp
		break
	}
	if existing == nil {
		existing = &store.Certificate{
			ID: id.New(), AccountID: accountID, Hostname: hostname,
		}
	}
	existing.Kind = "custom"
	existing.Status = "active"
	if payload, ok := raw.(map[string]any); ok {
		if issuer, _ := payload["issuer"].(string); issuer != "" {
			existing.Issuer = issuer
		}
		if stamp, _ := payload["not_after"].(string); stamp != "" {
			if parsed, err := time.Parse(time.RFC3339, stamp); err == nil {
				existing.NotAfter = &parsed
			}
		}
	}
	st.PutCert(existing)
	return existing
}

func (a *API) applyAccountHostnameWebsites(r *http.Request, accountID, hostname string) {
	for _, site := range a.Store.ListWebsites(accountID) {
		domain := a.Store.GetDomain(site.DomainID)
		if domain == nil || domain.ASCII != hostname {
			continue
		}
		cp := site
		a.applyStoredWebsite(r, &cp)
	}
}
