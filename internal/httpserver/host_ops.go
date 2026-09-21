package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
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
	writeJSON(w, 200, raw)
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
	apps := decodeHostApps(raw)
	ac := actor(r)
	for i := range apps {
		if len(apps[i].Actions) == 0 && apps[i].Kind == "plugin" {
			apps[i].Actions = []operations.HostAppAction{{
				ID: "unavailable", Label: "No action", Kind: "disabled", Available: false,
				Reason: "No host-backed action is available for this app yet.",
			}}
		}
		for j := range apps[i].Actions {
			action := &apps[i].Actions[j]
			if !action.Available {
				continue
			}
			need := hostAppActionCapability(action.ID)
			if need != "" && !ac.Has(need) {
				action.Available = false
				action.Reason = "Requires " + need
			}
		}
	}
	writeJSON(w, 200, map[string]any{"items": apps})
}

func (a *API) controlHostApp(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(chi.URLParam(r, "appID"))
	var in struct {
		Action string `json:"action"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if !knownHostAppID(appID) {
		a.fail(w, r, 404, "NOT_FOUND", "unknown host app", false)
		return
	}
	if !hostAppAllowsAction(appID, action) {
		a.fail(w, r, 400, "VALIDATION", "action must be enable, disable, restart, start, stop, reload, or install", false)
		return
	}
	if !a.require(w, r, hostAppActionCapability(action)) {
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "host.app.control", ResourceType: "server",
		Payload: map[string]any{"target": appID, "action": action},
		State:   "queued",
	}, a.auditEvent(r, "", "server.app."+action, "server", "", nil, map[string]any{
		"app": appID, "action": action, "target": appID,
	}))
	if err != nil {
		a.fail(w, r, 500, "HOST_APP_CONTROL_ERROR", "Could not queue host application action", true)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "status": "queued", "app": appID, "action": action,
	})
}

func decodeHostApps(raw any) []operations.HostApp {
	if apps, ok := raw.([]operations.HostApp); ok {
		return apps
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var apps []operations.HostApp
	if err := json.Unmarshal(b, &apps); err != nil {
		return nil
	}
	return apps
}

func knownHostAppID(id string) bool {
	switch id {
	case "phpmyadmin", "roundcube", "wordpress", "rspamd":
		return true
	default:
		return false
	}
}

func hostAppAllowsAction(id, action string) bool {
	if id != "rspamd" {
		return false
	}
	switch action {
	case "enable", "disable", "restart", "start", "stop", "reload", "install":
		return true
	default:
		return false
	}
}

func hostAppActionCapability(action string) string {
	switch action {
	case "install":
		return rbac.ServerSettingsWrite
	case "enable", "disable", "restart", "start", "stop", "reload":
		return rbac.ServerServicesRestart
	default:
		return ""
	}
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
			Params: mustJSON(map[string]any{"domain": domain, "tools": []string{appID}}),
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
		a.queueWordPressInstall(w, r, in.AccountID, wordpressInstallInput{
			WebsiteID: in.WebsiteID, Title: in.Title, AdminUser: in.AdminUser,
			AdminPassword: in.AdminPass, AdminEmail: in.AdminMail,
		})
	default:
		a.fail(w, r, 404, "NOT_FOUND", "unknown host app", false)
	}
}

func (a *API) listHostRecipes(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListHostRecipes"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw})
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
	if !a.requireServer(w, r, rbac.ServerSettingsWrite) {
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
	if !a.requireServer(w, r, rbac.ServerSettingsWrite) {
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
	if !a.requireAny(w, r, rbac.ServerRead, rbac.WebsitesRead) {
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
	if !supportedPHPVersion(in.Version) {
		a.fail(w, r, 400, "VALIDATION", "unsupported PHP version", false)
		return
	}
	job, err := a.Store.EnqueueJobWithAudit(&store.Job{
		Type: "php.runtime.ensure", ResourceType: "php",
		Payload: map[string]any{"version": in.Version, "target": in.Version},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, "", "server.runtime.ensure", "php", "", nil, map[string]any{"version": in.Version, "target": in.Version}))
	if err != nil {
		a.fail(w, r, 500, "RUNTIME_ENSURE_ERROR", "Could not persist runtime installation", true)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "status": "provisioning", "version": in.Version,
	})
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
	if msg := mailLocalPartConflict(a.Store, aid, md.ID, in.LocalPart); msg != "" {
		a.fail(w, r, 409, "CONFLICT", msg, false)
		return
	}
	if !strings.Contains(dest, ",") {
		dest += ","
	}
	al := &store.MailAlias{ID: id.New(), AccountID: aid, DomainID: md.ID, Address: in.LocalPart, Destination: dest}
	job, err := a.Store.UpsertMailAliasWithJob(al, &store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID, "kind": "list"},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, aid, "mail.list.create", "mail_alias", al.ID, nil, map[string]any{"address": al.Address}))
	if err != nil {
		a.fail(w, r, 500, "MAIL_LIST_CREATE_ERROR", "Could not persist mailing list", true)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "list": mailingListFromAlias(a.Store, al, "provisioning")})
}

func (a *API) updateMailingList(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	al := a.Store.GetMailAlias(chi.URLParam(r, "listID"))
	if al == nil || al.AccountID != aid || !isMailingList(al) {
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
	job, err := a.Store.UpsertMailAliasWithJob(al, &store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID, "kind": "list"},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, aid, "mail.list.update", "mail_alias", al.ID, nil, map[string]any{"members": len(in.Members)}))
	if err != nil {
		a.fail(w, r, 500, "MAIL_LIST_UPDATE_ERROR", "Could not persist mailing list update", true)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "list": mailingListFromAlias(a.Store, al, "provisioning")})
}

func (a *API) deleteMailingList(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	al := a.Store.GetMailAlias(chi.URLParam(r, "listID"))
	if al == nil || al.AccountID != aid || !isMailingList(al) {
		a.fail(w, r, 404, "NOT_FOUND", "mailing list missing", false)
		return
	}
	job, err := a.Store.DeleteMailAliasWithJob(al.ID, aid, &store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, aid, "mail.list.delete", "mail_alias", al.ID, map[string]any{"address": al.Address}, nil))
	if err != nil {
		a.fail(w, r, 500, "MAIL_LIST_DELETE_ERROR", "Could not persist mailing list deletion", true)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func isMailingList(alias *store.MailAlias) bool {
	return alias != nil && strings.Contains(alias.Destination, ",")
}

func mailLocalPartConflict(st store.Store, accountID, domainID, localPart string) string {
	for _, mb := range st.ListMailboxes(accountID) {
		if mb.DomainID == domainID && mb.LocalPart == localPart {
			return "mailbox already exists"
		}
	}
	for _, al := range st.ListMailAliases(accountID) {
		if al.DomainID == domainID && al.Address == localPart {
			if isMailingList(&al) {
				return "list or alias already exists"
			}
			return "alias already exists"
		}
	}
	return ""
}

func mailingListsFor(st store.Store, accountID string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, alias := range st.ListMailAliases(accountID) {
		if !isMailingList(&alias) {
			continue
		}
		out = append(out, mailingListFromAlias(st, &alias, "active"))
	}
	return out
}

func listOwnerLocalPart(localPart string) string {
	return strings.TrimSpace(localPart) + "-owner"
}

func (a *API) resetMailingListPassword(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	al := a.Store.GetMailAlias(chi.URLParam(r, "listID"))
	if al == nil || al.AccountID != aid || !isMailingList(al) {
		a.fail(w, r, 404, "NOT_FOUND", "mailing list missing", false)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(in.Password) == "" {
		a.fail(w, r, 400, "VALIDATION", "Mailbox password required", false)
		return
	}
	md := resolveMailDomain(a.Store, aid, al.DomainID)
	if md == nil {
		a.fail(w, r, 400, "VALIDATION", "Mail domain is not provisioned yet", false)
		return
	}
	ownerLocal := listOwnerLocalPart(al.Address)
	if err := validate.LocalPart(ownerLocal); err != nil {
		a.fail(w, r, 400, "VALIDATION", "owner mailbox: "+err.Error(), false)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", "Could not hash mailbox password", false)
		return
	}
	quota := int64(1 << 30)
	if pkg := a.accountPackage(aid); pkg != nil && pkg.MailboxStorageBytes > 0 {
		quota = pkg.MailboxStorageBytes
	}
	mb := &store.Mailbox{
		ID: id.New(), AccountID: aid, DomainID: md.ID, LocalPart: ownerLocal,
		QuotaBytes: quota, PasswordHash: hash, Status: "provisioning",
	}
	updating := false
	for _, existing := range a.Store.ListMailboxes(aid) {
		if existing.DomainID == md.ID && existing.LocalPart == ownerLocal {
			existing.PasswordHash = hash
			existing.Status = "provisioning"
			cp := existing
			mb = &cp
			updating = true
			break
		}
	}
	if !updating {
		if msg := mailLocalPartConflict(a.Store, aid, md.ID, ownerLocal); msg != "" && msg != "mailbox already exists" {
			a.fail(w, r, 409, "CONFLICT", msg, false)
			return
		}
		if err := a.enforceCountLimit(aid, "mailboxes", len(a.Store.ListMailboxes(aid)), func(p *store.Package) int { return p.Mailboxes }); err != nil {
			a.rejectLimit(w, r, err)
			return
		}
	}
	job, err := a.Store.UpsertMailboxWithJob(mb, &store.Job{
		Type: "mailbox.provision", ResourceType: "mailbox", ResourceID: mb.ID,
		Payload: map[string]any{"mailbox_id": mb.ID, "account_id": aid},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, aid, "mail.list.reset_password", "mailbox", mb.ID, nil, map[string]any{
		"list_id": al.ID, "local_part": mb.LocalPart,
	}))
	if err != nil {
		a.fail(w, r, 500, "MAIL_LIST_PASSWORD_ERROR", "Could not persist list owner mailbox", true)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID,
		"mailbox":      mb,
		"list":         mailingListFromAlias(a.Store, al, "active"),
	})
}

func mailingListFromAlias(st store.Store, alias *store.MailAlias, status string) map[string]any {
	members := make([]string, 0)
	for _, part := range strings.Split(alias.Destination, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			members = append(members, part)
		}
	}
	if status == "" {
		status = "active"
	}
	ownerLocal := listOwnerLocalPart(alias.Address)
	out := map[string]any{
		"id":               alias.ID,
		"account_id":       alias.AccountID,
		"domain_id":        alias.DomainID,
		"local_part":       alias.Address,
		"members":          members,
		"status":           status,
		"owner_local_part": ownerLocal,
	}
	if st != nil {
		for _, mb := range st.ListMailboxes(alias.AccountID) {
			if mb.DomainID == alias.DomainID && mb.LocalPart == ownerLocal {
				out["owner_mailbox_id"] = mb.ID
				out["owner_status"] = mb.Status
				break
			}
		}
	}
	return out
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

func supportedPHPVersion(version string) bool {
	switch version {
	case "8.3", "8.4", "8.5":
		return true
	default:
		return false
	}
}

func (a *API) installedPHPVersions(ctx context.Context) ([]string, error) {
	if a.Agent == nil {
		return nil, fmt.Errorf("agent unavailable")
	}
	raw, err := a.Agent.Dispatch(ctx, operations.Request{Method: "ListPHPRuntimes"})
	if err != nil {
		return nil, err
	}
	return installedPHPVersionsFrom(raw), nil
}

func installedPHPVersionsFrom(raw any) []string {
	var items []any
	switch typed := raw.(type) {
	case []map[string]any:
		for _, item := range typed {
			items = append(items, item)
		}
	case []any:
		items = typed
	default:
		return nil
	}
	var out []string
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(entry["status"]) != "installed" {
			continue
		}
		version := strings.TrimSpace(fmt.Sprint(entry["version"]))
		if !supportedPHPVersion(version) {
			continue
		}
		out = append(out, version)
	}
	return out
}

func defaultInstalledPHP(installed []string) string {
	for _, version := range installed {
		if version == "8.3" {
			return version
		}
	}
	if len(installed) > 0 {
		return installed[0]
	}
	return ""
}

func phpVersionInstalled(installed []string, version string) bool {
	for _, item := range installed {
		if item == version {
			return true
		}
	}
	return false
}

func phpFPMNotInstalledMessage(version string) string {
	if version == "" {
		return "php-fpm is not installed"
	}
	return "php-fpm " + version + " is not installed"
}
