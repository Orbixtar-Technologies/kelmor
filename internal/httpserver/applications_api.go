package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) createApp(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsWrite) {
		return
	}
	var req struct {
		WebsiteID        string `json:"website_id"`
		DomainID         string `json:"domain_id"`
		Runtime          string `json:"runtime"`
		RuntimeVersion   string `json:"runtime_version"`
		WorkingDirectory string `json:"working_directory"`
		StartCommand     string `json:"start_command"`
		GitURL           string `json:"git_url"`
		GitBranch        string `json:"git_branch"`
		GitAuth          string `json:"git_auth"`
		AutoDeploy       bool   `json:"auto_deploy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid application", false)
		return
	}
	account := a.Store.GetAccount(aid)
	if account == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	websiteID := req.WebsiteID
	if websiteID == "" {
		if req.DomainID == "" {
			a.fail(w, r, 400, "VALIDATION", "website_id or domain_id required", false)
			return
		}
		runtime := req.Runtime
		if runtime == "" {
			runtime = "node"
		}
		if runtime != "node" && runtime != "python" {
			a.fail(w, r, 400, "VALIDATION", "runtime must be node or python", false)
			return
		}
		d := a.Store.GetDomain(req.DomainID)
		if d == nil || d.AccountID != aid {
			a.fail(w, r, 400, "VALIDATION", "domain_id must belong to the account", false)
			return
		}
		var site *store.Website
		for _, existing := range a.Store.ListWebsites(aid) {
			if existing.DomainID == d.ID {
				cp := existing
				site = &cp
				break
			}
		}
		if site == nil {
			docRoot := d.DocumentRoot
			if docRoot == "" {
				docRoot = "/home/" + account.Username + "/public_html"
			}
			site = &store.Website{
				ID: id.New(), AccountID: aid, DomainID: d.ID,
				Runtime: runtime, DocumentRoot: docRoot,
				Enabled: true, DesiredRevision: 1,
			}
			if _, err := a.Store.UpsertWebsiteWithJob(site, &store.Job{
				Type: "website.provision", ResourceType: "website", ResourceID: site.ID,
				Payload: map[string]any{"website_id": site.ID, "account_id": aid, "target_revision": site.DesiredRevision},
				State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
			}, a.auditEvent(r, aid, "website.create", "website", site.ID, nil, map[string]any{"runtime": runtime, "source": "deploy-apps"})); err != nil {
				a.fail(w, r, 500, "WEBSITE_CREATE_ERROR", "Could not create website for application", true)
				return
			}
		} else if site.Runtime != runtime {
			site.Runtime = runtime
			site.DesiredRevision++
			_, _ = a.Store.UpsertWebsiteWithJob(site, &store.Job{
				Type: "website.provision", ResourceType: "website", ResourceID: site.ID,
				Payload: map[string]any{"website_id": site.ID, "account_id": aid, "target_revision": site.DesiredRevision, "runtime": runtime},
				State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
			}, a.auditEvent(r, aid, "website.update", "website", site.ID, nil, map[string]any{"runtime": runtime}))
		}
		websiteID = site.ID
	}
	in := store.Application{
		ID: id.New(), AccountID: aid, WebsiteID: websiteID,
		Runtime: req.Runtime, RuntimeVersion: req.RuntimeVersion,
		WorkingDirectory: req.WorkingDirectory, StartCommand: req.StartCommand,
		GitURL: req.GitURL, GitBranch: req.GitBranch, GitAuthToken: req.GitAuth,
		AutoDeploy: req.AutoDeploy,
	}
	website := a.Store.GetWebsite(in.WebsiteID)
	if website == nil || website.AccountID != aid {
		a.fail(w, r, 400, "VALIDATION", "website_id must belong to the account", false)
		return
	}
	if in.Runtime == "" {
		in.Runtime = website.Runtime
	}
	if in.Runtime != "node" && in.Runtime != "python" {
		a.fail(w, r, 400, "VALIDATION", "runtime must be node or python", false)
		return
	}
	if !website.Enabled {
		a.fail(w, r, 400, "VALIDATION", "select an enabled website", false)
		return
	}
	if website.Runtime == "" {
		website.Runtime = in.Runtime
		a.Store.PutWebsite(website)
	} else if website.Runtime != in.Runtime {
		a.fail(w, r, 400, "VALIDATION", "select an enabled website with the matching runtime", false)
		return
	}
	for _, existing := range a.Store.ListApps(aid) {
		if existing.WebsiteID == in.WebsiteID {
			a.fail(w, r, 409, "IN_USE", "website already has an application; retry its deployment job instead", false)
			return
		}
	}
	if containsControlCharacters(in.Runtime) || containsControlCharacters(in.RuntimeVersion) ||
		containsControlCharacters(in.WorkingDirectory) || containsControlCharacters(in.StartCommand) ||
		containsControlCharacters(in.GitURL) || containsControlCharacters(in.GitBranch) {
		a.fail(w, r, 400, "VALIDATION", "application fields cannot contain control characters", false)
		return
	}
	if in.GitURL != "" {
		if !strings.HasPrefix(in.GitURL, "https://") && !strings.HasPrefix(in.GitURL, "git@") {
			a.fail(w, r, 400, "VALIDATION", "git_url must use https:// or git@ scheme", false)
			return
		}
	}
	if in.GitURL != "" && in.GitBranch == "" {
		in.GitBranch = "main"
	}
	if in.WorkingDirectory == "" {
		if in.GitURL != "" {
			in.WorkingDirectory = "/home/" + account.Username + "/apps/" + website.ID
		} else {
			in.WorkingDirectory = website.DocumentRoot
			if in.WorkingDirectory == "" {
				in.WorkingDirectory = "/home/" + account.Username + "/apps/" + in.ID
			}
		}
	}
	workDir, err := policy.WithinAccount(account.Username, in.WorkingDirectory)
	if err != nil || workDir != in.WorkingDirectory {
		a.fail(w, r, 400, "VALIDATION", "working_directory must be canonical and within the account", false)
		return
	}
	if in.AutoDeploy {
		token, tokenErr := generateWebhookToken()
		if tokenErr != nil {
			a.fail(w, r, 500, "TOKEN_ERROR", "Could not generate webhook token", false)
			return
		}
		in.DeployWebhookToken = token
	}
	if err := a.enforceCountLimit(aid, "applications", len(a.Store.ListApps(aid)), func(p *store.Package) int { return p.ApplicationInstances }); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	in.Status = "provisioning"
	jobType := "application.deploy"
	if in.GitURL != "" {
		jobType = "application.git_deploy"
	}
	job, err := a.Store.UpsertApplicationWithJob(&in, &store.Job{
		Type: jobType, ResourceType: "application", ResourceID: in.ID,
		Payload: map[string]any{"application_id": in.ID, "account_id": aid},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}, a.auditEvent(r, aid, "application.create", "application", in.ID, nil, map[string]any{"runtime": in.Runtime, "git_url": in.GitURL}))
	if err != nil {
		a.fail(w, r, 500, "APPLICATION_CREATE_ERROR", "Could not persist application deployment", true)
		return
	}
	out := map[string]any{
		"id": in.ID, "account_id": in.AccountID, "website_id": in.WebsiteID,
		"runtime": in.Runtime, "runtime_version": in.RuntimeVersion,
		"working_directory": in.WorkingDirectory, "start_command": in.StartCommand,
		"git_url": in.GitURL, "git_branch": in.GitBranch,
		"auto_deploy": in.AutoDeploy, "status": in.Status,
	}
	if in.AutoDeploy && in.DeployWebhookToken != "" {
		out["webhook_url"] = "/api/v1/hooks/application-deploy/" + in.DeployWebhookToken
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "application": out})
}

func (a *API) redeployApp(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsWrite) {
		return
	}
	app := a.Store.GetApp(chi.URLParam(r, "applicationID"))
	if app == nil || app.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "application missing", false)
		return
	}
	jobType := "application.deploy"
	if app.GitURL != "" {
		jobType = "application.git_deploy"
	}
	app.Status = "provisioning"
	a.Store.PutApp(app)
	job, err := a.Store.EnqueueJob(&store.Job{
		Type: jobType, ResourceType: "application", ResourceID: app.ID,
		Payload: map[string]any{"application_id": app.ID, "account_id": aid},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
	})
	if err != nil {
		a.fail(w, r, 500, "REDEPLOY_ERROR", "Could not queue redeploy", true)
		return
	}
	a.audit(r, "application.redeploy", "application", app.ID, true, nil, map[string]any{"job_type": jobType})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) webhookDeploy(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" || len(token) < 32 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	app := a.appByWebhookToken(token)
	if app == nil || !app.AutoDeploy {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	jobType := "application.deploy"
	if app.GitURL != "" {
		jobType = "application.git_deploy"
	}
	app.Status = "provisioning"
	a.Store.PutApp(app)
	job, err := a.Store.EnqueueJob(&store.Job{
		Type: jobType, ResourceType: "application", ResourceID: app.ID,
		Payload: map[string]any{"application_id": app.ID, "account_id": app.AccountID, "source": "webhook"},
		State:   "queued",
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, 202, map[string]any{"queued": true, "operation_id": job.ID})
}

func (a *API) appByWebhookToken(token string) *store.Application {
	want := []byte(token)
	for _, app := range a.Store.ListApps("") {
		if app.DeployWebhookToken == "" {
			continue
		}
		stored := []byte(app.DeployWebhookToken)
		if len(stored) != len(want) {
			continue
		}
		diff := byte(0)
		for i := range stored {
			diff |= stored[i] ^ want[i]
		}
		if diff == 0 {
			cp := app
			return &cp
		}
	}
	return nil
}

func generateWebhookToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
