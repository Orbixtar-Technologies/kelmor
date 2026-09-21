package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func nginxAccessLogPath(websiteID string) string {
	return "/var/log/nginx/" + websiteID + ".access.log"
}

func nginxErrorLogPath(websiteID string) string {
	return "/var/log/nginx/" + websiteID + ".error.log"
}

func accountHomeLogPath(home, name string) string {
	return filepath.ToSlash(filepath.Join(home, "logs", name))
}

func (a *API) accountsOnIP(r *http.Request, from string) []store.Account {
	from = strings.TrimSpace(from)
	out := make([]store.Account, 0)
	for _, acc := range a.Store.ListAccounts("", "") {
		if !actor(r).CanAccount(acc.ID) {
			continue
		}
		if strings.TrimSpace(acc.IPAddress) != from {
			continue
		}
		out = append(out, acc)
	}
	return out
}

func (a *API) listIPMigration(w http.ResponseWriter, r *http.Request) {
	if !a.requireAny(w, r, rbac.AccountsRead, rbac.AccountsModify) {
		return
	}
	from := strings.TrimSpace(r.URL.Query().Get("from_ip"))
	accounts := a.accountsOnIP(r, from)
	items := make([]map[string]any, 0, len(accounts))
	for _, acc := range accounts {
		items = append(items, map[string]any{
			"id":             acc.ID,
			"username":       acc.Username,
			"primary_domain": acc.PrimaryDomain,
			"ip_address":     acc.IPAddress,
			"home_path":      acc.HomePath,
			"status":         acc.Status,
		})
	}
	note := ""
	if from == "" {
		note = "Accounts with an unset IP stay on the shared address. Enter the dedicated source address to preview a move."
	} else if len(items) == 0 {
		note = "No accounts currently use this source IP. Shared or unset addresses are not migrated."
	}
	writeJSON(w, 200, map[string]any{"items": items, "from_ip": from, "note": note})
}

func (a *API) listMailNotify(w http.ResponseWriter, r *http.Request) {
	if !a.requireAny(w, r, rbac.AccountsRead, rbac.ServerSettingsWrite, rbac.ResellersRead) {
		return
	}
	audience := strings.TrimSpace(r.URL.Query().Get("audience"))
	if audience == "" {
		audience = "owners"
	}
	items := a.mailNotifyRecipients(audience)
	note := ""
	if len(items) == 0 {
		note = "No recipients with an owner email. POST /mail/notify would queue a no-op send list."
	}
	writeJSON(w, 200, map[string]any{
		"items":    items,
		"audience": audience,
		"path":     "POST /mail/notify",
		"note":     note,
	})
}

func (a *API) mailNotifyRecipients(audience string) []map[string]any {
	items := make([]map[string]any, 0)
	switch audience {
	case "resellers":
		for _, reseller := range a.Store.ListResellers() {
			user := a.Store.UserByID(reseller.UserID)
			if user == nil || strings.TrimSpace(user.Email) == "" {
				continue
			}
			items = append(items, map[string]any{
				"kind":     "reseller",
				"id":       reseller.ID,
				"username": user.Username,
				"email":    user.Email,
				"label":    reseller.Name,
			})
		}
	default:
		for _, acc := range a.Store.ListAccounts("", "") {
			owner := a.Store.UserByID(acc.OwnerUserID)
			if owner == nil || strings.TrimSpace(owner.Email) == "" {
				continue
			}
			items = append(items, map[string]any{
				"kind":           "owner",
				"id":             acc.ID,
				"account_id":     acc.ID,
				"username":       acc.Username,
				"email":          owner.Email,
				"primary_domain": acc.PrimaryDomain,
			})
		}
	}
	return items
}

func (a *API) listNginxLogs(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.AccountsRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	items := a.nginxLogItems(r, acc)
	note := ""
	if len(items) == 0 {
		note = "This account has no vhost or home log paths yet."
	}
	writeJSON(w, 200, map[string]any{
		"items":     items,
		"home_path": acc.HomePath,
		"username":  acc.Username,
		"note":      note,
	})
}

func (a *API) nginxLogItems(r *http.Request, acc *store.Account) []map[string]any {
	items := make([]map[string]any, 0)
	add := func(kind, path, websiteID, domain string) {
		present, size := a.managedFileInfo(r, path)
		items = append(items, map[string]any{
			"kind":       kind,
			"path":       path,
			"website_id": websiteID,
			"domain":     domain,
			"present":    present,
			"size_bytes": size,
		})
	}
	if acc.HomePath != "" {
		add("access", accountHomeLogPath(acc.HomePath, "access.log"), "", acc.PrimaryDomain)
		add("error", accountHomeLogPath(acc.HomePath, "error.log"), "", acc.PrimaryDomain)
	}
	for _, site := range a.Store.ListWebsites(acc.ID) {
		domain := acc.PrimaryDomain
		if d := a.Store.GetDomain(site.DomainID); d != nil && d.ASCII != "" {
			domain = d.ASCII
		}
		add("access", nginxAccessLogPath(site.ID), site.ID, domain)
		add("error", nginxErrorLogPath(site.ID), site.ID, domain)
	}
	return items
}

func (a *API) downloadNginxLog(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.AccountsRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	want := strings.TrimSpace(r.URL.Query().Get("path"))
	if want == "" {
		a.fail(w, r, 400, "VALIDATION", "path is required", false)
		return
	}
	var allowed map[string]any
	for _, item := range a.nginxLogItems(r, acc) {
		if item["path"] == want {
			allowed = item
			break
		}
	}
	if allowed == nil {
		a.fail(w, r, 400, "PATH_DENIED", "Path is not an nginx log for this account", false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: mustJSON(map[string]any{"path": want}),
	})
	if err != nil {
		a.fail(w, r, 404, "NOT_FOUND", "Log file is not present on the host", false)
		return
	}
	body, _ := json.Marshal(raw)
	var result struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &result)
	content, err := base64.StdEncoding.DecodeString(result.Message)
	if err != nil {
		content = []byte(result.Message)
	}
	name := filepath.Base(want)
	if name == "." || name == "/" {
		name = "nginx.log"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
	a.audit(r, "account.nginx_log.download", "account", acc.ID, true, nil, map[string]any{"path": want, "bytes": len(content)})
}

func (a *API) managedFileInfo(r *http.Request, path string) (bool, int64) {
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: mustJSON(map[string]any{"path": path}),
	})
	if err != nil {
		return false, 0
	}
	body, _ := json.Marshal(raw)
	var result struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &result)
	content, err := base64.StdEncoding.DecodeString(result.Message)
	if err != nil {
		return true, int64(len(result.Message))
	}
	return true, int64(len(content))
}
