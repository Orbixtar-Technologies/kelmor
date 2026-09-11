package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/validate"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) serverProcesses(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListProcesses"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"processes": raw})
}

func (a *API) signalProcess(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
	if err != nil || pid <= 1 {
		a.fail(w, r, 400, "VALIDATION", "invalid pid", false)
		return
	}
	var in struct {
		Signal string `json:"signal"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Signal == "" {
		in.Signal = "TERM"
	}
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "SignalProcess",
		Params: mustJSON(map[string]any{"pid": pid, "signal": in.Signal}),
	})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.process.signal", "process", strconv.Itoa(pid), true, nil, map[string]any{"signal": in.Signal})
	writeJSON(w, 200, res)
}

func (a *API) listHostApps(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListHostApps"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw})
}

func (a *API) enableHostApp(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	appID := chi.URLParam(r, "appID")
	var in struct {
		AccountID string `json:"account_id"`
		WebsiteID string `json:"website_id"`
		Title     string `json:"title"`
		AdminUser string `json:"admin_user"`
		AdminPass string `json:"admin_password"`
		AdminMail string `json:"admin_email"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	switch appID {
	case "phpmyadmin", "roundcube":
		domain := ""
		if in.AccountID != "" {
			if acc := a.Store.GetAccount(in.AccountID); acc != nil {
				domain = acc.PrimaryDomain
				if d := a.primaryDomainRecord(in.AccountID); d != "" {
					domain = d
				}
			}
		}
		if domain == "" {
			a.fail(w, r, 400, "VALIDATION", "choose an account so Kelmor can publish the tool on its primary domain", false)
			return
		}
		res, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "ApplyAdminTools",
			Params: mustJSON(map[string]any{"domain": domain}),
		})
		if err != nil {
			a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
			return
		}
		a.audit(r, "server.app.enable", "app", appID, true, nil, map[string]any{"domain": domain})
		writeJSON(w, 200, res)
	case "wordpress":
		if in.AccountID == "" || in.WebsiteID == "" {
			a.fail(w, r, 400, "VALIDATION", "account_id and website_id are required", false)
			return
		}
		if !a.requireAccount(w, r, in.AccountID, rbac.ApplicationsWrite) {
			return
		}
		site := a.Store.GetWebsite(in.WebsiteID)
		if site == nil || site.AccountID != in.AccountID {
			a.fail(w, r, 400, "VALIDATION", "website_id required", false)
			return
		}
		hostname := ""
		if d := a.Store.GetDomain(site.DomainID); d != nil {
			hostname = d.ASCII
		}
		app := &store.Application{
			ID: id.New(), WebsiteID: site.ID, AccountID: in.AccountID,
			Runtime: "wordpress", WorkingDirectory: site.DocumentRoot, Status: "provisioning",
		}
		a.Store.PutApp(app)
		job, _ := a.Store.EnqueueJob(&store.Job{
			Type: "wordpress.install", ResourceType: "application", ResourceID: app.ID,
			Payload: map[string]any{
				"application_id": app.ID, "title": in.Title, "hostname": hostname,
				"admin_user": in.AdminUser, "admin_password": in.AdminPass, "admin_email": in.AdminMail,
			},
			State: "queued",
		})
		a.audit(r, "server.app.enable", "app", appID, true, nil, map[string]any{"website_id": site.ID})
		writeJSON(w, 202, map[string]any{"operation_id": job.ID, "application": app})
	default:
		a.fail(w, r, 404, "NOT_FOUND", "unknown host app", false)
	}
}

func (a *API) listHostRecipes(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": []map[string]string{
		{"id": "nginx-test", "label": "Test nginx configuration", "description": "Run nginx -t without reloading."},
		{"id": "nginx-reload", "label": "Reload nginx", "description": "Reload nginx after a successful config test."},
		{"id": "postfix-queue", "label": "Show mail queue", "description": "List deferred and active Postfix queue entries."},
		{"id": "postfix-flush", "label": "Flush mail queue", "description": "Ask Postfix to retry deferred mail."},
		{"id": "postfix-status", "label": "Postfix status", "description": "Show Postfix service status."},
	}})
}

func (a *API) runHostRecipe(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "RunHostRecipe",
		Params: mustJSON(map[string]any{"id": in.ID}),
	})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.console.run", "recipe", in.ID, true, nil, map[string]any{"recipe": in.ID})
	writeJSON(w, 200, res)
}

func (a *API) setRootPassword(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "SetRootPassword",
		Params: mustJSON(map[string]any{"password": in.Password}),
	})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.root_password.set", "server", "", true, nil, map[string]any{"stored": false})
	writeJSON(w, 200, res)
}

func (a *API) setDatabaseRootPassword(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.DatabasesWrite) {
		return
	}
	var in struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "SetMariaDBRootPassword",
		Params: mustJSON(map[string]any{"current": in.Current, "password": in.Password}),
	})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.database_root_password.set", "server", "", true, nil, map[string]any{"stored": false})
	writeJSON(w, 200, res)
}

func (a *API) listPHPRuntimes(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListPHPRuntimes"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw})
}

func (a *API) ensurePHPRuntime(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Version string `json:"version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "EnsurePHPRuntime",
		Params: mustJSON(map[string]any{"version": in.Version}),
	})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.runtime.ensure", "php", in.Version, true, nil, map[string]any{"version": in.Version})
	writeJSON(w, 200, res)
}

func (a *API) listMailingLists(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": mailingListsFor(a.Store, aid)})
}

func (a *API) createMailingList(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	var in struct {
		DomainID  string   `json:"domain_id"`
		LocalPart string   `json:"local_part"`
		Members   []string `json:"members"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := validate.LocalPart(in.LocalPart); err != nil {
		a.fail(w, r, 400, "VALIDATION", "local_part: "+err.Error(), false)
		return
	}
	dest, err := normalizeAliasDestinations(strings.Join(in.Members, ","))
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	md := resolveMailDomain(a.Store, aid, in.DomainID)
	if md == nil {
		a.fail(w, r, 400, "VALIDATION", "Mail domain is not provisioned yet", false)
		return
	}
	for _, existing := range a.Store.ListMailAliases(aid) {
		if existing.DomainID == md.ID && existing.Address == in.LocalPart {
			a.fail(w, r, 409, "CONFLICT", "list or alias already exists", false)
			return
		}
	}
	if !strings.Contains(dest, ",") {
		dest += ","
	}
	al := &store.MailAlias{ID: id.New(), AccountID: aid, DomainID: md.ID, Address: in.LocalPart, Destination: dest}
	a.Store.PutMailAlias(al)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID, "kind": "list"},
		State:   "queued",
	})
	a.audit(r, "mail.list.create", "mail_alias", al.ID, true, nil, map[string]any{"address": al.Address})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "list": mailingListFromAlias(al)})
}

func (a *API) updateMailingList(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	al := a.Store.GetMailAlias(chi.URLParam(r, "listID"))
	if al == nil || al.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "mailing list missing", false)
		return
	}
	var in struct {
		Members []string `json:"members"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	dest, err := normalizeAliasDestinations(strings.Join(in.Members, ","))
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if !strings.Contains(dest, ",") {
		dest += ","
	}
	al.Destination = dest
	a.Store.PutMailAlias(al)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID, "kind": "list"},
		State:   "queued",
	})
	a.audit(r, "mail.list.update", "mail_alias", al.ID, true, nil, map[string]any{"members": len(in.Members)})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "list": mailingListFromAlias(al)})
}

func mailingListsFor(st store.Store, accountID string) []map[string]any {
	var out []map[string]any
	for _, alias := range st.ListMailAliases(accountID) {
		if !strings.Contains(alias.Destination, ",") {
			continue
		}
		out = append(out, mailingListFromAlias(&alias))
	}
	return out
}

func mailingListFromAlias(alias *store.MailAlias) map[string]any {
	var members []string
	for _, part := range strings.Split(alias.Destination, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			members = append(members, part)
		}
	}
	return map[string]any{
		"id":         alias.ID,
		"account_id": alias.AccountID,
		"domain_id":  alias.DomainID,
		"local_part": alias.Address,
		"members":    members,
		"status":     "active",
	}
}

func normalizeAliasDestinations(raw string) (string, error) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		dest, err := normalizeAliasDestination(strings.TrimSpace(part))
		if err != nil {
			return "", err
		}
		if seen[dest] {
			continue
		}
		seen[dest] = true
		out = append(out, dest)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("at least one member is required")
	}
	return strings.Join(out, ","), nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
