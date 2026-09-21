package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) listMailQueue(w http.ResponseWriter, r *http.Request) {
	ac := actor(r)
	if !ac.Has(rbac.MailRead) && !ac.Has(rbac.MailWrite) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListMailQueue"})
	if err != nil {
		writeJSON(w, 200, map[string]any{
			"items":   []any{},
			"partial": true,
			"message": "Could not read the live Postfix queue. Showing an empty list.",
		})
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) listAccountRedirects(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesRead) {
		return
	}
	items := make([]map[string]any, 0)
	for _, site := range a.Store.ListWebsites(aid) {
		policy, err := a.readVhostPolicy(r, site.ID)
		if err != nil {
			continue
		}
		for _, redirect := range policy.Redirects {
			items = append(items, redirectItem(site, redirect))
		}
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) createAccountRedirect(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	var in struct {
		WebsiteID string `json:"website_id"`
		Source    string `json:"source"`
		Target    string `json:"target"`
		Status    int    `json:"status"`
		Wildcard  bool   `json:"wildcard"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "VALIDATION", "invalid redirect", false)
		return
	}
	site := a.Store.GetWebsite(in.WebsiteID)
	if site == nil || site.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "website missing", false)
		return
	}
	source := strings.TrimSpace(in.Source)
	target := strings.TrimSpace(in.Target)
	if !strings.HasPrefix(source, "/") || target == "" {
		a.fail(w, r, 400, "VALIDATION", "source must start with / and target is required", false)
		return
	}
	status := in.Status
	if status != 302 {
		status = 301
	}
	policy, err := a.readVhostPolicy(r, site.ID)
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	redirect := operations.VhostRedirect{
		ID: id.New(), Source: source, Target: target, Status: status, Wildcard: in.Wildcard,
	}
	policy.WebsiteID = site.ID
	policy.Redirects = append(policy.Redirects, redirect)
	if err := a.writeVhostPolicy(r, policy); err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.applyStoredWebsite(r, site)
	a.audit(r, "website.redirect.create", "website", site.ID, true, nil, map[string]any{"source": source, "target": target})
	writeJSON(w, 200, redirectItem(*site, redirect))
}

func (a *API) deleteAccountRedirect(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	redirectID := chi.URLParam(r, "redirectID")
	for _, site := range a.Store.ListWebsites(aid) {
		policy, err := a.readVhostPolicy(r, site.ID)
		if err != nil {
			continue
		}
		kept := policy.Redirects[:0]
		found := false
		for _, item := range policy.Redirects {
			if item.ID == redirectID {
				found = true
				continue
			}
			kept = append(kept, item)
		}
		if !found {
			continue
		}
		policy.Redirects = kept
		if err := a.writeVhostPolicy(r, policy); err != nil {
			a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
			return
		}
		a.applyStoredWebsite(r, &site)
		a.audit(r, "website.redirect.delete", "website", site.ID, true, nil, map[string]any{"id": redirectID})
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	a.fail(w, r, 404, "NOT_FOUND", "redirect missing", false)
}

func (a *API) getWebsiteHotlink(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesRead) {
		return
	}
	site := a.accountWebsite(w, r, aid, chi.URLParam(r, "websiteID"))
	if site == nil {
		return
	}
	policy, err := a.readVhostPolicy(r, site.ID)
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, hotlinkItem(site.ID, policy.Hotlink))
}

func (a *API) putWebsiteHotlink(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	site := a.accountWebsite(w, r, aid, chi.URLParam(r, "websiteID"))
	if site == nil {
		return
	}
	var in operations.VhostHotlink
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "VALIDATION", "invalid hotlink policy", false)
		return
	}
	policy, err := a.readVhostPolicy(r, site.ID)
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	policy.WebsiteID = site.ID
	policy.Hotlink = in
	if err := a.writeVhostPolicy(r, policy); err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.applyStoredWebsite(r, site)
	a.audit(r, "website.hotlink.apply", "website", site.ID, true, nil, map[string]any{"enabled": in.Enabled})
	writeJSON(w, 200, hotlinkItem(site.ID, policy.Hotlink))
}

func (a *API) listAccountImages(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	root := acc.HomePath + "/public_html"
	if siteID := r.URL.Query().Get("website_id"); siteID != "" {
		if site := a.Store.GetWebsite(siteID); site != nil && site.AccountID == aid && site.DocumentRoot != "" {
			root = site.DocumentRoot
		}
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListImages",
		Params: mustJSON(map[string]any{"root": root}),
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"items": []any{}, "root": root})
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw, "root": root})
}

func (a *API) listAccountGit(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListGitRepos",
		Params: mustJSON(map[string]any{"home": acc.HomePath}),
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"items": []any{}, "home": acc.HomePath})
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw, "home": acc.HomePath})
}

func (a *API) accountWebsite(w http.ResponseWriter, r *http.Request, accountID, websiteID string) *store.Website {
	site := a.Store.GetWebsite(websiteID)
	if site == nil || site.AccountID != accountID {
		a.fail(w, r, 404, "NOT_FOUND", "website missing", false)
		return nil
	}
	return site
}

func (a *API) readVhostPolicy(r *http.Request, websiteID string) (operations.VhostPolicy, error) {
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadVhostPolicy",
		Params: mustJSON(map[string]any{"website_id": websiteID}),
	})
	if err != nil {
		return operations.VhostPolicy{WebsiteID: websiteID, Redirects: []operations.VhostRedirect{}}, err
	}
	b, _ := json.Marshal(raw)
	var policy operations.VhostPolicy
	if err := json.Unmarshal(b, &policy); err != nil {
		return operations.VhostPolicy{WebsiteID: websiteID, Redirects: []operations.VhostRedirect{}}, nil
	}
	if policy.Redirects == nil {
		policy.Redirects = []operations.VhostRedirect{}
	}
	policy.WebsiteID = websiteID
	return policy, nil
}

func (a *API) writeVhostPolicy(r *http.Request, policy operations.VhostPolicy) error {
	_, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "WriteVhostPolicy",
		Params: mustJSON(policy),
	})
	return err
}

func (a *API) applyStoredWebsite(r *http.Request, site *store.Website) {
	if site == nil {
		return
	}
	acc := a.Store.GetAccount(site.AccountID)
	if acc == nil {
		return
	}
	domain := ""
	if d := a.Store.GetDomain(site.DomainID); d != nil {
		domain = d.ASCII
	}
	_, _ = a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ApplyWebsite",
		Params: mustJSON(map[string]any{
			"website_id": site.ID, "account": acc.Username, "domain": domain,
			"document_root": site.DocumentRoot, "runtime": site.Runtime,
			"php_version": site.RuntimeVersion, "https_redirect": site.HTTPSRedirect,
			"enabled": site.Enabled,
		}),
	})
}

func redirectItem(site store.Website, redirect operations.VhostRedirect) map[string]any {
	return map[string]any{
		"id": redirect.ID, "website_id": site.ID, "source": redirect.Source,
		"target": redirect.Target, "status": redirect.Status, "wildcard": redirect.Wildcard,
	}
}

func hotlinkItem(websiteID string, policy operations.VhostHotlink) map[string]any {
	if policy.Extensions == nil {
		policy.Extensions = []string{}
	}
	if policy.AllowedReferers == nil {
		policy.AllowedReferers = []string{}
	}
	return map[string]any{
		"website_id": websiteID, "enabled": policy.Enabled, "allow_direct": policy.AllowDirect,
		"extensions": policy.Extensions, "allowed_referers": policy.AllowedReferers,
	}
}
