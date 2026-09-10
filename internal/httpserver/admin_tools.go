package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/rbac"
)

func (a *API) databaseCredentials(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DatabasesRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	engine := strings.TrimSpace(r.URL.Query().Get("engine"))
	if engine == "" {
		engine = "mariadb"
	}
	dbName := strings.TrimSpace(r.URL.Query().Get("database"))
	credRel := ".panel-database." + engine
	if dbName != "" {
		credRel = ".panel-database." + engine + "." + dbName
	}
	credPath, err := policy.WithinAccount(acc.Username, filepath.Join(acc.HomePath, credRel))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	params, _ := json.Marshal(map[string]any{"path": credPath})
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: params,
	})
	if err != nil {
		a.fail(w, r, 404, "NOT_FOUND", "database credentials not provisioned yet", false)
		return
	}
	b, _ := json.Marshal(raw)
	var result struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &result)
	content, err := decodeBase64Content(result.Message)
	if err != nil {
		a.fail(w, r, 500, "FILE_ERROR", "could not read credentials", false)
		return
	}
	creds := parseCredentialFile(string(content))
	a.audit(r, "databases.credentials.read", "account", aid, true, nil, map[string]any{"engine": engine, "database": dbName})
	writeJSON(w, 200, map[string]any{"credentials": creds})
}

func (a *API) accountAdminTools(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.AccountsRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	domain := acc.PrimaryDomain
	if d := a.primaryDomainRecord(aid); d != "" {
		domain = d
	}
	params, _ := json.Marshal(map[string]any{"domain": domain})
	if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ApplyAdminTools",
		Params: params,
	}); err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	writeJSON(w, 200, map[string]any{
		"domain": domain,
		"phpmyadmin_url": fmt.Sprintf("%s://phpmyadmin.%s/", scheme, domain),
		"webmail_url":    fmt.Sprintf("%s://webmail.%s/", scheme, domain),
	})
}

func (a *API) primaryDomainRecord(aid string) string {
	for _, d := range a.Store.ListDomains(aid) {
		if d.Type == "primary" || d.ASCII != "" {
			return d.ASCII
		}
	}
	return ""
}

func parseCredentialFile(body string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out
}

func decodeBase64Content(raw string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(raw)
}
