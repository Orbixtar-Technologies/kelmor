package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/agent/policy"
	openapi "github.com/hosting-panel/panel/api"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/brand"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/limits"
	"github.com/hosting-panel/panel/internal/migration"
	"github.com/hosting-panel/panel/internal/monitoring"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/validate"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

type API struct {
	Store   store.Store
	Log     *logging.Logger
	Agent   *operations.Host
	Version string
	limiter *limiter
}

type ctxActor struct{}

func New(st store.Store, log *logging.Logger, agent *operations.Host) *API {
	return &API{Store: st, Log: log, Agent: agent, Version: "0.1.0", limiter: newLimiter()}
}

func (a *API) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(a.withRequestID)
	r.Use(a.securityHeaders)
	r.Use(a.maxBody)
	r.Use(a.cors)
	r.Get("/", a.productIndex)
	r.Get("/healthz", a.health)
	r.Get("/readyz", a.ready)
	r.Get("/openapi", a.openapiUI)
	r.Get("/openapi.yaml", a.openapi)
	r.Get("/api/v1/version", a.version)
	r.Get("/api/v1/openapi.yaml", a.openapi)
	r.Get("/api/v1/openapi", a.openapiUI)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", a.login)
		r.Post("/auth/logout", a.logout)
		r.Post("/auth/refresh", a.refresh)
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Get("/me", a.me)
			r.Get("/server", a.serverOverview)
			r.Get("/server/monitor", a.serverMonitor)
			r.Get("/server/services", a.serverServices)
			r.Get("/server/processes", a.serverProcesses)
			r.Post("/server/reboot", a.rebootHost)
			r.Post("/server/firewall/apply", a.applyFirewall)
			r.Get("/server/firewall", a.getFirewall)
			r.Get("/jobs", a.listJobs)
			r.Get("/jobs/{jobID}", a.getJob)
			r.Post("/jobs/{jobID}/retry", a.retryJob)
			r.Get("/audit-events", a.listAudit)
			r.Get("/packages", a.listPackages)
			r.Post("/packages", a.createPackage)
			r.Patch("/packages/{packageID}", a.updatePackage)
			r.Delete("/packages/{packageID}", a.deletePackage)
			r.Get("/resellers", a.listResellers)
			r.Post("/resellers", a.createReseller)
			r.Patch("/resellers/{resellerID}", a.updateReseller)
			r.Get("/accounts", a.listAccounts)
			r.Post("/accounts", a.createAccount)
			r.Get("/accounts/{accountID}", a.getAccount)
			r.Patch("/accounts/{accountID}", a.modifyAccount)
			r.Post("/accounts/{accountID}/suspend", a.suspendAccount)
			r.Post("/accounts/{accountID}/unsuspend", a.unsuspendAccount)
			r.Post("/accounts/{accountID}/terminate", a.terminateAccount)
			r.Post("/accounts/{accountID}/migrate", a.migrateAccount)
			r.Post("/accounts/{accountID}/impersonate", a.impersonate)
			r.Get("/accounts/{accountID}/usage", a.accountUsage)
			r.Post("/accounts/bulk/suspend", a.bulkSuspend)
			r.Get("/accounts/export", a.exportAccounts)
			r.Post("/accounts/import", a.importAccount)
			r.Post("/accounts/import/cpanel", a.importCPanel)
			r.Route("/accounts/{accountID}", func(r chi.Router) {
				r.Get("/", a.getAccount)
				r.Patch("/", a.modifyAccount)
				r.Get("/domains", a.listDomains)
				r.Post("/domains", a.createDomain)
				r.Delete("/domains/{domainID}", a.deleteDomain)
				r.Get("/websites", a.listWebsites)
				r.Post("/websites", a.createWebsite)
				r.Delete("/websites/{websiteID}", a.deleteWebsite)
				r.Get("/applications", a.listApps)
				r.Post("/applications", a.createApp)
				r.Delete("/applications/{applicationID}", a.deleteApp)
				r.Post("/wordpress", a.installWordPress)
				r.Get("/databases", a.listDBs)
				r.Post("/databases", a.createDB)
				r.Delete("/databases/{databaseID}", a.deleteDB)
				r.Get("/dns/zones", a.listZones)
				r.Get("/dns/zones/{zoneID}/records", a.listRecords)
				r.Post("/dns/zones/{zoneID}/records", a.createRecord)
				r.Delete("/dns/zones/{zoneID}/records/{recordID}", a.deleteRecord)
				r.Post("/dns/zones/{zoneID}/dnssec", a.setDNSSEC)
				r.Get("/dns/zones/{zoneID}/ds", a.getDSRecords)
				r.Get("/mail/domains", a.listMailDomains)
				r.Patch("/mail/domains/{mailDomainID}", a.patchMailDomain)
				r.Get("/mail/mailboxes", a.listMailboxes)
				r.Post("/mail/mailboxes", a.createMailbox)
				r.Delete("/mail/mailboxes/{mailboxID}", a.deleteMailbox)
				r.Get("/mail/aliases", a.listMailAliases)
				r.Post("/mail/aliases", a.createMailAlias)
				r.Delete("/mail/aliases/{aliasID}", a.deleteMailAlias)
				r.Get("/certificates", a.listCerts)
				r.Post("/certificates", a.requestCert)
				r.Get("/backups", a.listBackups)
				r.Post("/backups", a.createBackup)
				r.Post("/restores", a.restoreBackup)
				r.Get("/export", a.exportAccount)
				r.Get("/files", a.listFiles)
				r.Post("/files", a.writeFile)
				r.Get("/cron", a.listCron)
				r.Post("/cron", a.createCron)
				r.Delete("/cron/{cronID}", a.deleteCron)
				r.Get("/ssh-keys", a.listSSH)
				r.Post("/sftp-password", a.setSFTPPassword)
				r.Post("/ssh-keys", a.createSSH)
				r.Delete("/ssh-keys/{keyID}", a.deleteSSH)
				r.Get("/ftp", a.listFTP)
				r.Post("/ftp", a.createFTP)
				r.Delete("/ftp/{ftpID}", a.deleteFTP)
				r.Get("/api-tokens", a.listTokens)
				r.Post("/api-tokens", a.createToken)
				r.Delete("/api-tokens/{tokenID}", a.deleteToken)
				r.Post("/password", a.rotateAccountPassword)
			})
		})
	})
	return r
}

func (a *API) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = id.New()
		}
		ctx := logging.WithRequestID(r.Context(), rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (a *API) maxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		next.ServeHTTP(w, r)
	})
}

func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok"})
}
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ready"})
}
func (a *API) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"version": a.Version, "api": "v1", "product": brand.Product,
		"director": brand.Director, "control": brand.Control,
	})
}

func (a *API) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(200)
	_, _ = w.Write(openapi.YAML)
}

func (a *API) productIndex(w http.ResponseWriter, r *http.Request) {
	writeBrandHTML(w, brand.Product, `<h1>Kelmor</h1><p>Product family Kelmor. Portals are not compiled into this binary — run <code>npm run dev</code> or <code>make refresh-portals</code>.</p><ul><li>Kelmor Director — provider console on :8443 (installed nginx) or :18443 (Vite)</li><li>Kelmor Control — tenant self-serve on :8444 or :18444</li><li><a href="/openapi">Kelmor Control Plane API</a></li></ul>`)
}

func (a *API) openapiUI(w http.ResponseWriter, r *http.Request) {
	writeBrandHTML(w, brand.APITitle, `<h1>Kelmor Control Plane API</h1><p>OpenAPI 3 for Kelmor Director and Kelmor Control.</p><p><a href="/openapi.yaml">openapi.yaml</a></p>`)
}

func writeBrandHTML(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.WriteHeader(200)
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>%s</title></head><body>%s</body></html>\n", title, body)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !a.limiter.allow("login:"+ip, 8, time.Minute) {
		a.fail(w, r, 429, "RATE_LIMITED", "Too many login attempts", false)
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid request", false)
		return
	}
	u := a.Store.UserByUsername(in.Username)
	ok := u != nil && auth.VerifyPassword(u.PasswordHash, in.Password) && u.Status == "active"
	a.Store.AppendAudit(store.AuditEvent{
		ActorType: "user", Action: "auth.login", RequestID: logging.RequestID(r.Context()),
		Success: ok, SourceIP: ip, UserAgent: r.UserAgent(),
		Metadata: map[string]any{"username": in.Username},
	})
	if !ok {
		a.logAuthFailure(ip, in.Username)
		a.fail(w, r, 401, "INVALID_CREDENTIALS", "Invalid username or password", false)
		return
	}
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		a.fail(w, r, 500, "SESSION_ERROR", "Could not create session", false)
		return
	}
	sess := &store.Session{ID: id.New(), UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(12 * time.Hour), SourceIP: ip, UserAgent: r.UserAgent()}
	a.Store.PutSession(sess)
	http.SetCookie(w, &http.Cookie{Name: "panel_session", Value: plain, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 12 * 3600})
	writeJSON(w, 200, map[string]any{"token": plain, "user": publicUser(u), "expires_at": sess.ExpiresAt})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("panel_session"); err == nil {
		if s := a.Store.SessionByHash(auth.HashToken(c.Value)); s != nil {
			a.Store.RevokeSession(s.ID)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "panel_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	a.login(w, r)
}

func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			if c, err := r.Cookie("panel_session"); err == nil {
				token = c.Value
			}
		}
		if token == "" {
			a.fail(w, r, 401, "UNAUTHENTICATED", "Authentication required", false)
			return
		}
		if strings.HasPrefix(token, "hp_live_") {
			t := a.Store.TokenByHash(auth.HashToken(token))
			if t == nil {
				a.fail(w, r, 401, "INVALID_TOKEN", "Token rejected", false)
				return
			}
			u := a.Store.UserByID(t.UserID)
			if u == nil {
				a.fail(w, r, 401, "INVALID_TOKEN", "Token rejected", false)
				return
			}
			actor := a.actorFromUser(u)
			actor.Capabilities = rbac.Expand(nil, t.Capabilities)
			if t.Scope != "server" {
				actor.IsServerScope = false
				if t.AccountID != "" {
					actor.AccountIDs = []string{t.AccountID}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxActor{}, actor)))
			return
		}
		s := a.Store.SessionByHash(auth.HashToken(token))
		if s == nil {
			a.fail(w, r, 401, "UNAUTHENTICATED", "Session expired", false)
			return
		}
		u := a.Store.UserByID(s.UserID)
		if u == nil {
			a.fail(w, r, 401, "UNAUTHENTICATED", "Unknown user", false)
			return
		}
		actor := a.actorFromUser(u)
		actor.ImpersonatorID = s.ImpersonatorID
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxActor{}, actor)))
	})
}

func (a *API) actorFromUser(u *store.User) rbac.Actor {
	server := false
	for _, r := range u.Roles {
		if r == "root_owner" || r == "server_administrator" || r == "server_operator" || r == "auditor" {
			server = true
		}
	}
	ids := a.Store.AccountsForUser(u.ID)
	rid := ""
	if rs := a.Store.ResellerByUser(u.ID); rs != nil {
		rid = rs.ID
		seen := map[string]bool{}
		for _, id := range ids {
			seen[id] = true
		}
		for _, acc := range a.Store.ListAccounts("", "") {
			if acc.ResellerID == rid && !seen[acc.ID] {
				ids = append(ids, acc.ID)
				seen[acc.ID] = true
			}
		}
	}
	return rbac.Actor{
		UserID: u.ID, Username: u.Username, Roles: u.Roles,
		Capabilities:  rbac.Expand(u.Roles, nil),
		AccountIDs:    ids,
		ResellerID:    rid,
		IsServerScope: server,
	}
}

func actor(r *http.Request) rbac.Actor {
	v, _ := r.Context().Value(ctxActor{}).(rbac.Actor)
	return v
}

func (a *API) require(w http.ResponseWriter, r *http.Request, cap string) bool {
	ac := actor(r)
	if !ac.Has(cap) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability "+cap, false)
		return false
	}
	return true
}

func (a *API) requireAccount(w http.ResponseWriter, r *http.Request, accountID, cap string) bool {
	ac := actor(r)
	if !ac.Has(cap) || !ac.CanAccount(accountID) {
		a.fail(w, r, 403, "FORBIDDEN", "Not authorized for this account", false)
		return false
	}
	return true
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	ac := actor(r)
	u := a.Store.UserByID(ac.UserID)
	writeJSON(w, 200, map[string]any{"user": publicUser(u), "actor": ac})
}

func (a *API) serverOverview(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	info, err := a.Agent.GetSystemInfo()
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"system": info, "stats": a.Store.Stats(), "services": defaultServices()})
}

func (a *API) serverMonitor(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	root := ""
	if a.Agent != nil && a.Agent.Sock == "" {
		root = a.Agent.Root
	}
	writeJSON(w, 200, monitoring.CollectMeasured(a.Store, root, func(acc store.Account) *store.Usage {
		if a.Agent == nil {
			return nil
		}
		params, _ := json.Marshal(map[string]any{"username": acc.Username, "home": acc.HomePath})
		raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "MeasureAccountUsage",
			Params: params,
		})
		if err != nil {
			return nil
		}
		b, _ := json.Marshal(raw)
		var got operations.AccountUsage
		if json.Unmarshal(b, &got) != nil {
			return nil
		}
		return &store.Usage{
			AccountID: acc.ID, CollectedAt: time.Now().UTC(),
			DiskBytes: got.DiskBytes, InodeCount: got.InodeCount,
			MemoryBytes: got.MemoryBytes, ProcessCount: int(got.ProcessCount),
			BandwidthBytes: got.BandwidthBytes,
		}
	}))
}

func (a *API) serverServices(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerServicesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"services": defaultServices()})
}

func (a *API) rebootHost(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Confirm != "REBOOT" {
		a.fail(w, r, 400, "VALIDATION", "confirm must be REBOOT", false)
		return
	}
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "RebootHost"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.reboot", "server", "", true, nil, map[string]any{"confirm": "REBOOT"})
	writeJSON(w, 202, res)
}

func (a *API) applyFirewall(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerFirewallWrite) {
		return
	}
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ApplyFirewall"})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.firewall.apply", "server", "", true, nil, map[string]any{"table": "inet panel"})
	writeJSON(w, 200, res)
}

func (a *API) getFirewall(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerFirewallRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"table": "inet panel", "file": "/etc/panel/nftables-panel.nft"})
}

func (a *API) serverProcesses(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"processes": []map[string]any{
		{"pid": os.Getpid(), "user": "panel", "cmd": "panel-api", "cpu": 0.2, "rss": 48},
	}})
}

func (a *API) listJobs(w http.ResponseWriter, r *http.Request) {
	ac := actor(r)
	if !ac.Has(rbac.ServerRead) && !ac.Has(rbac.AccountsRead) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	jobs := a.Store.ListJobs(r.URL.Query().Get("state"), 100)
	if !ac.IsServerScope {
		visible := make([]store.Job, 0, len(jobs))
		for _, j := range jobs {
			aid := j.ResourceID
			if j.ResourceType != "account" {
				if v, ok := j.Payload["account_id"].(string); ok {
					aid = v
				} else {
					continue
				}
			}
			if ac.CanAccount(aid) {
				visible = append(visible, j)
			}
		}
		jobs = visible
	}
	items := make([]store.Job, 0, len(jobs))
	for i := range jobs {
		items = append(items, publicJob(&jobs[i]))
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func jobAccountID(j *store.Job) string {
	if j == nil {
		return ""
	}
	if j.ResourceType == "account" && j.ResourceID != "" {
		return j.ResourceID
	}
	if v, ok := j.Payload["account_id"].(string); ok && v != "" {
		return v
	}
	return ""
}

func publicJob(j *store.Job) store.Job {
	out := *j
	out.Payload = safeJobPayload(j.Payload)
	return out
}

func safeJobPayload(payload map[string]any) map[string]any {
	raw, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{}
	}
	var copied map[string]any
	if err := json.Unmarshal(raw, &copied); err != nil || copied == nil {
		return map[string]any{}
	}
	scrubSensitiveJobFields(copied)
	return copied
}

func scrubSensitiveJobFields(values map[string]any) {
	for key, value := range values {
		if sensitiveJobField(key) {
			delete(values, key)
			continue
		}
		switch nested := value.(type) {
		case map[string]any:
			scrubSensitiveJobFields(nested)
		case []any:
			for _, item := range nested {
				if object, ok := item.(map[string]any); ok {
					scrubSensitiveJobFields(object)
				}
			}
		}
	}
}

func sensitiveJobField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, marker := range []string{"password", "secret", "token", "credential", "private_key", "hash"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func retryCapability(jobType string) string {
	switch jobType {
	case "account.provision", "account.copy_homedir":
		return rbac.AccountsCreate
	case "account.reconcile":
		return rbac.AccountsModify
	case "domain.provision", "domain.delete":
		return rbac.DomainsWrite
	case "website.provision", "website.delete":
		return rbac.WebsitesWrite
	case "application.deploy", "wordpress.install":
		return rbac.ApplicationsWrite
	case "database.provision", "database.delete":
		return rbac.DatabasesWrite
	case "dns.sync", "dns.dnssec":
		return rbac.DNSWrite
	case "mailbox.provision", "mailbox.delete", "mail.alias", "mail.maps":
		return rbac.MailWrite
	case "certificate.provision":
		return rbac.WebsitesWrite
	case "certificate.portal":
		return rbac.ServerSettingsWrite
	case "backup.create":
		return rbac.BackupsCreate
	case "backup.restore":
		return rbac.BackupsRestore
	case "cron.apply":
		return rbac.CronWrite
	case "ftp.apply":
		return rbac.FilesWrite
	default:
		return ""
	}
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	j := a.Store.GetJob(chi.URLParam(r, "jobID"))
	if j == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Job not found", false)
		return
	}
	ac := actor(r)
	if !ac.Has(rbac.ServerRead) && !ac.Has(rbac.AccountsRead) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	if !ac.IsServerScope {
		aid := jobAccountID(j)
		if aid == "" || !ac.CanAccount(aid) {
			a.fail(w, r, 404, "NOT_FOUND", "Job not found", false)
			return
		}
	}
	writeJSON(w, 200, publicJob(j))
}

func (a *API) retryJob(w http.ResponseWriter, r *http.Request) {
	original := a.Store.GetJob(chi.URLParam(r, "jobID"))
	if original == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Job not found", false)
		return
	}
	ac := actor(r)
	if !ac.Has(rbac.ServerRead) && !ac.Has(rbac.AccountsRead) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	if !ac.IsServerScope {
		accountID := jobAccountID(original)
		if accountID == "" || !ac.CanAccount(accountID) {
			a.fail(w, r, 404, "NOT_FOUND", "Job not found", false)
			return
		}
	}
	if original.State != "failed" {
		a.fail(w, r, 409, "NOT_FAILED", "Only failed jobs can be retried", false)
		return
	}
	capability := retryCapability(original.Type)
	if capability == "" {
		a.fail(w, r, 409, "NOT_RETRYABLE", "Job type cannot be retried", false)
		return
	}
	if !ac.Has(capability) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability "+capability, false)
		return
	}
	payload := safeJobPayload(original.Payload)
	payload["retry_of"] = original.ID
	retry, err := a.Store.EnqueueJob(&store.Job{
		Type: original.Type, ResourceType: original.ResourceType, ResourceID: original.ResourceID,
		Payload: payload, State: "queued", Priority: original.Priority, MaxAttempts: original.MaxAttempts,
		ActorID: ac.UserID, RequestID: logging.RequestID(r.Context()),
	})
	if err != nil {
		a.fail(w, r, 500, "JOB_ERROR", "Could not queue job retry", false)
		return
	}
	a.audit(r, "job.retry", "job", original.ID, true,
		map[string]any{
			"state": original.State, "type": original.Type,
			"actor_id": original.ActorID, "request_id": original.RequestID,
		},
		map[string]any{
			"retry_job_id": retry.ID, "state": retry.State,
			"actor_id": retry.ActorID, "request_id": retry.RequestID,
		})
	writeJSON(w, 202, map[string]any{
		"operation_id": retry.ID, "retried_job_id": original.ID, "status": retry.State,
	})
}

func (a *API) listAudit(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.SecurityAuditRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListAudit(200)})
}

func (a *API) listPackages(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.PackagesRead) {
		return
	}
	items := a.Store.ListPackages()
	ac := actor(r)
	if ac.ResellerID != "" && !ac.IsServerScope {
		visible := make([]store.Package, 0, len(items))
		for _, p := range items {
			if p.ResellerID == "" || p.ResellerID == ac.ResellerID {
				visible = append(visible, p)
			}
		}
		items = visible
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) createPackage(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.PackagesWrite) {
		return
	}
	var p store.Package
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid package", false)
		return
	}
	p.ID = id.New()
	if p.Name == "" {
		a.fail(w, r, 400, "VALIDATION", "Name required", false)
		return
	}
	who := actor(r)
	if who.ResellerID != "" && !who.IsServerScope {
		p.ResellerID = who.ResellerID
	}
	if p.FeatureSetID == "" {
		if sets := a.Store.ListFeatureSets(); len(sets) > 0 {
			p.FeatureSetID = sets[0].ID
		}
	}
	a.Store.PutPackage(&p)
	a.audit(r, "package.create", "package", p.ID, true, nil, map[string]any{"name": p.Name})
	writeJSON(w, 201, p)
}

func (a *API) updatePackage(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.PackagesWrite) {
		return
	}
	packageID := chi.URLParam(r, "packageID")
	current := a.Store.GetPackage(packageID)
	if current == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Package not found", false)
		return
	}
	who := actor(r)
	if !who.IsServerScope && (who.ResellerID == "" || current.ResellerID != who.ResellerID) {
		a.fail(w, r, 403, "FORBIDDEN", "Not authorized for this package", false)
		return
	}
	updated := *current
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid package", false)
		return
	}
	updated.ID = current.ID
	updated.ResellerID = current.ResellerID
	updated.Name = strings.TrimSpace(updated.Name)
	if updated.Name == "" {
		a.fail(w, r, 400, "VALIDATION", "Name required", false)
		return
	}
	a.Store.PutPackage(&updated)
	a.audit(r, "package.update", "package", updated.ID, true,
		map[string]any{"name": current.Name}, map[string]any{"name": updated.Name})
	writeJSON(w, 200, updated)
}

func (a *API) deletePackage(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.PackagesWrite) {
		return
	}
	packageID := chi.URLParam(r, "packageID")
	pkg := a.Store.GetPackage(packageID)
	if pkg == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Package not found", false)
		return
	}
	who := actor(r)
	if !who.IsServerScope && (who.ResellerID == "" || pkg.ResellerID != who.ResellerID) {
		a.fail(w, r, 403, "FORBIDDEN", "Not authorized for this package", false)
		return
	}
	if !a.Store.DeletePackageIfUnused(pkg.ID) {
		a.fail(w, r, 409, "IN_USE", "Package is assigned to an account", false)
		return
	}
	a.audit(r, "package.delete", "package", pkg.ID, true,
		map[string]any{"name": pkg.Name, "reseller_id": pkg.ResellerID}, nil)
	writeJSON(w, 200, map[string]any{"deleted": pkg.ID})
}

func (a *API) listResellers(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ResellersRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListResellers()})
}

func (a *API) createReseller(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ResellersCreate) {
		return
	}
	var in struct {
		store.Reseller
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid reseller", false)
		return
	}
	if in.Name == "" {
		a.fail(w, r, 400, "VALIDATION", "Name required", false)
		return
	}
	in.ID = id.New()
	if in.Username != "" {
		if a.Store.UserByUsername(in.Username) != nil {
			a.fail(w, r, 409, "USERNAME_TAKEN", "Username already exists", false)
			return
		}
		if in.Password == "" {
			a.fail(w, r, 400, "VALIDATION", "Reseller password required", false)
			return
		}
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			a.fail(w, r, 400, "VALIDATION", "Invalid reseller password", false)
			return
		}
		email := in.Email
		if email == "" {
			email = in.Username + "@localhost"
		}
		owner := &store.User{
			ID: id.New(), Username: in.Username, Email: email, PasswordHash: hash,
			DisplayName: in.Name, Status: "active", Roles: []string{"reseller"}, CreatedAt: time.Now().UTC(),
		}
		a.Store.PutUser(owner)
		in.UserID = owner.ID
	}
	if in.UserID == "" || in.UserID == "pending" {
		in.UserID = actor(r).UserID
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if len(in.Nameservers) == 0 {
		in.Nameservers = []string{"ns1.localhost", "ns2.localhost"}
	}
	if in.PrivilegeMask == nil {
		in.PrivilegeMask = []string{}
	}
	rs := in.Reseller
	a.Store.PutReseller(&rs)
	a.audit(r, "reseller.create", "reseller", rs.ID, true, nil, map[string]any{"name": rs.Name})
	writeJSON(w, 201, rs)
}

func (a *API) updateReseller(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ResellersModify) {
		return
	}
	if !actor(r).IsServerScope {
		a.fail(w, r, 403, "FORBIDDEN", "Server scope required", false)
		return
	}
	resellerID := chi.URLParam(r, "resellerID")
	current := a.Store.GetReseller(resellerID)
	if current == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Reseller not found", false)
		return
	}
	updated := *current
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid reseller", false)
		return
	}
	updated.ID = current.ID
	updated.UserID = current.UserID
	updated.Name = strings.TrimSpace(updated.Name)
	if updated.Name == "" {
		a.fail(w, r, 400, "VALIDATION", "Name required", false)
		return
	}
	if updated.PrivilegeMask == nil {
		updated.PrivilegeMask = []string{}
	}
	if updated.Nameservers == nil {
		updated.Nameservers = []string{}
	}
	a.Store.PutReseller(&updated)
	a.audit(r, "reseller.update", "reseller", updated.ID, true,
		map[string]any{
			"name": current.Name, "brand_name": current.BrandName, "privilege_mask": current.PrivilegeMask,
			"nameservers": current.Nameservers, "status": current.Status,
		},
		map[string]any{
			"name": updated.Name, "brand_name": updated.BrandName, "privilege_mask": updated.PrivilegeMask,
			"nameservers": updated.Nameservers, "status": updated.Status,
		})
	writeJSON(w, 200, updated)
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsRead) {
		return
	}
	ac := actor(r)
	items := a.Store.ListAccounts(r.URL.Query().Get("q"), r.URL.Query().Get("status"))
	if !ac.IsServerScope {
		visible := make([]store.Account, 0, len(items))
		for _, acc := range items {
			if ac.CanAccount(acc.ID) {
				visible = append(visible, acc)
			}
		}
		items = visible
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) exportAccount(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.AccountsRead) {
		return
	}
	exp, err := migration.Export(a.Store, aid)
	if err != nil {
		a.fail(w, r, 404, "NOT_FOUND", err.Error(), false)
		return
	}
	a.audit(r, "account.export", "account", aid, true, nil, map[string]any{"username": exp.Account.Username})
	writeJSON(w, 200, exp)
}

func (a *API) migrateAccount(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsCreate) {
		return
	}
	srcID := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, srcID, rbac.AccountsRead) {
		return
	}
	var in struct {
		Username string `json:"username"`
		Domain   string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "invalid migrate request", false)
		return
	}
	if err := validate.Username(in.Username); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	ascii, err := validate.NormalizeDomain(in.Domain)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	exp, err := migration.Export(a.Store, srcID)
	if err != nil {
		a.fail(w, r, 404, "NOT_FOUND", err.Error(), false)
		return
	}
	raw, err := json.Marshal(exp)
	if err != nil {
		a.fail(w, r, 500, "EXPORT", err.Error(), false)
		return
	}
	acc, err := migration.ImportAs(a.Store, raw, in.Username, ascii, actor(r).UserID)
	if err != nil {
		a.fail(w, r, 409, "IMPORT_CONFLICT", err.Error(), false)
		return
	}
	a.Store.AddMember(acc.ID, actor(r).UserID)
	copied := queuedReconcileJob(a.Store, acc.ID)
	a.audit(r, "account.migrate", "account", acc.ID, true, map[string]any{"source": srcID}, map[string]any{"username": acc.Username, "domain": ascii})
	writeJSON(w, 202, map[string]any{"resource_id": acc.ID, "status": "provisioning", "account": acc, "homedir_job": copied, "source_id": srcID})
}

func (a *API) importAccount(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsCreate) {
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "invalid export", false)
		return
	}
	acc, err := migration.ImportAs(a.Store, raw, r.URL.Query().Get("username"), r.URL.Query().Get("domain"), actor(r).UserID)
	if err != nil {
		a.fail(w, r, 409, "IMPORT_CONFLICT", err.Error(), false)
		return
	}
	a.Store.AddMember(acc.ID, actor(r).UserID)
	copied := queuedReconcileJob(a.Store, acc.ID)
	a.audit(r, "account.import", "account", acc.ID, true, nil, map[string]any{"username": acc.Username})
	writeJSON(w, 202, map[string]any{"resource_id": acc.ID, "status": "provisioning", "account": acc, "homedir_job": copied})
}

func (a *API) importCPanel(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsCreate) {
		return
	}
	var in struct {
		Root     string `json:"root"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "invalid cpanel import", false)
		return
	}
	exp, err := migration.FromCPanel(in.Root, in.Username)
	if err != nil {
		a.fail(w, r, 400, "CPANEL_IMPORT", err.Error(), false)
		return
	}
	raw, err := json.Marshal(exp)
	if err != nil {
		a.fail(w, r, 400, "CPANEL_IMPORT", err.Error(), false)
		return
	}
	acc, err := migration.ImportAs(a.Store, raw, "", "", actor(r).UserID)
	if err != nil {
		a.fail(w, r, 409, "IMPORT_CONFLICT", err.Error(), false)
		return
	}
	a.Store.AddMember(acc.ID, actor(r).UserID)
	copied := queuedReconcileJob(a.Store, acc.ID)
	a.audit(r, "account.import.cpanel", "account", acc.ID, true, nil, map[string]any{"username": acc.Username, "homedir": exp.Homedir})
	writeJSON(w, 202, map[string]any{"resource_id": acc.ID, "status": "provisioning", "account": acc, "source": "cpanel", "homedir_job": copied})
}

func (a *API) exportAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsRead) {
		return
	}
	items := a.Store.ListAccounts("", "")
	ac := actor(r)
	if !ac.IsServerScope {
		visible := make([]store.Account, 0, len(items))
		for _, acc := range items {
			if ac.CanAccount(acc.ID) {
				visible = append(visible, acc)
			}
		}
		items = visible
	}
	writeJSON(w, 200, map[string]any{"items": items, "exported_at": time.Now().UTC()})
}

func (a *API) getAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accountID")
	ac := actor(r)
	if !ac.Has(rbac.AccountsRead) && !ac.Has(rbac.DomainsRead) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	if !ac.CanAccount(id) {
		a.fail(w, r, 403, "FORBIDDEN", "Not authorized for this account", false)
		return
	}
	acc := a.Store.GetAccount(id)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	writeJSON(w, 200, acc)
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsCreate) {
		return
	}
	var in struct {
		Username      string `json:"username"`
		PrimaryDomain string `json:"primary_domain"`
		PackageID     string `json:"package_id"`
		ResellerID    string `json:"reseller_id"`
		OwnerEmail    string `json:"owner_email"`
		OwnerPassword string `json:"owner_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid account", false)
		return
	}
	if err := validate.Username(in.Username); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	ascii, err := validate.NormalizeDomain(in.PrimaryDomain)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if a.Store.AccountByUsername(in.Username) != nil {
		a.fail(w, r, 409, "USERNAME_TAKEN", "Username already exists", false)
		return
	}
	if a.Store.DomainTaken(ascii) {
		a.fail(w, r, 409, "DOMAIN_ALREADY_EXISTS", "Domain is already assigned.", false)
		return
	}
	pkg := a.Store.GetPackage(in.PackageID)
	if pkg == nil {
		a.fail(w, r, 400, "VALIDATION", "Unknown package", false)
		return
	}
	who := actor(r)
	if who.ResellerID != "" && !who.IsServerScope {
		in.ResellerID = who.ResellerID
		if pkg.ResellerID != "" && pkg.ResellerID != who.ResellerID {
			a.fail(w, r, 403, "FORBIDDEN", "Package is not available to this reseller", false)
			return
		}
	}
	if in.ResellerID != "" && a.Store.GetReseller(in.ResellerID) == nil {
		a.fail(w, r, 400, "VALIDATION", "Unknown reseller", false)
		return
	}
	hash, err := auth.HashPassword(in.OwnerPassword)
	if err != nil || in.OwnerPassword == "" {
		a.fail(w, r, 400, "VALIDATION", "Owner password required", false)
		return
	}
	owner := &store.User{
		ID: id.New(), Username: in.Username, Email: in.OwnerEmail, PasswordHash: hash,
		DisplayName: in.Username, Status: "active", Roles: []string{"customer_owner"}, CreatedAt: time.Now().UTC(),
	}
	if owner.Email == "" {
		owner.Email = in.Username + "@" + ascii
	}
	a.Store.PutUser(owner)
	uid := a.Store.AllocUID()
	acc := &store.Account{
		ID: id.New(), ResellerID: in.ResellerID, OwnerUserID: owner.ID, Username: in.Username,
		PrimaryDomain: ascii, LinuxUID: uid, LinuxGID: uid, PackageID: pkg.ID,
		Status: "provisioning", HomePath: "/home/" + in.Username, ShellClass: "sftp-only",
		DesiredRevision: 1,
	}
	a.Store.PutAccount(acc)
	a.Store.AddMember(acc.ID, owner.ID)
	if who.ResellerID != "" && who.UserID != owner.ID {
		a.Store.AddMember(acc.ID, who.UserID)
	}
	dom := &store.Domain{ID: id.New(), AccountID: acc.ID, FQDN: ascii, ASCII: ascii, Type: "primary", DocumentRoot: acc.HomePath + "/public_html", DNSManaged: true, Status: "provisioning"}
	a.Store.PutDomain(dom)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "domain_id": dom.ID, "linux_password": in.OwnerPassword},
		State:   "queued", Priority: 10, IdempotencyKey: r.Header.Get("Idempotency-Key"),
		ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
	})
	a.audit(r, "account.create", "account", acc.ID, true, nil, map[string]any{"username": acc.Username, "domain": ascii})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "resource_id": acc.ID, "status": "provisioning", "account": acc})
}

func (a *API) modifyAccount(w http.ResponseWriter, r *http.Request) {
	accID := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, accID, rbac.AccountsModify) {
		return
	}
	acc := a.Store.GetAccount(accID)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	before := *acc
	if v, ok := in["package_id"].(string); ok && v != "" {
		acc.PackageID = v
	}
	who := actor(r)
	if v, ok := in["reseller_id"].(string); ok && who.IsServerScope {
		acc.ResellerID = v
	}
	if v, ok := in["primary_domain"].(string); ok && v != "" {
		ascii, err := validate.NormalizeDomain(v)
		if err != nil {
			a.fail(w, r, 400, "VALIDATION", err.Error(), false)
			return
		}
		acc.PrimaryDomain = ascii
	}
	if v, ok := in["ip_address"].(string); ok {
		acc.IPAddress = v
	}
	if v, ok := in["login_disabled"].(bool); ok {
		acc.LoginDisabled = v
	}
	acc.DesiredRevision++
	a.Store.PutAccount(acc)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID, Payload: map[string]any{"account_id": acc.ID}, State: "queued"})
	a.audit(r, "account.modify", "account", acc.ID, true, map[string]any{"package_id": before.PackageID}, map[string]any{"package_id": acc.PackageID})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "account": acc})
}

func (a *API) rotateAccountPassword(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, accountID, rbac.AccountsModify) {
		return
	}
	acc := a.Store.GetAccount(accountID)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	var in struct {
		Password           string `json:"password"`
		MustChangePassword *bool  `json:"must_change_password"`
		ForceChange        *bool  `json:"force_change"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid password request", false)
		return
	}
	if len(in.Password) < 8 {
		a.fail(w, r, 400, "VALIDATION", "Password must be at least 8 characters", false)
		return
	}
	owner := a.Store.UserByID(acc.OwnerUserID)
	if owner == nil {
		a.fail(w, r, 409, "OWNER_MISSING", "Account owner is missing", false)
		return
	}
	passwordHash, err := auth.HashPassword(in.Password)
	if err != nil {
		a.fail(w, r, 500, "PASSWORD_ERROR", "Could not update password", false)
		return
	}
	mustChange := false
	if in.ForceChange != nil {
		mustChange = *in.ForceChange
	}
	if in.MustChangePassword != nil {
		mustChange = *in.MustChangePassword
	}
	beforeMustChange := owner.MustChangePassword
	owner.PasswordHash = passwordHash
	owner.MustChangePassword = mustChange
	a.Store.PutUser(owner)
	job, err := a.Store.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "linux_password": in.Password},
		State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
	})
	if err != nil {
		a.fail(w, r, 500, "JOB_ERROR", "Could not queue password reconciliation", false)
		return
	}
	a.audit(r, "account.password.rotate", "account", acc.ID, true,
		map[string]any{"must_change_password": beforeMustChange},
		map[string]any{"must_change_password": owner.MustChangePassword, "operation_id": job.ID})
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "account_id": acc.ID,
		"must_change_password": owner.MustChangePassword, "status": job.State,
	})
}

func (a *API) suspendAccount(w http.ResponseWriter, r *http.Request) {
	a.setAccountStatus(w, r, "suspended", "account.suspend", rbac.AccountsSuspend)
}
func (a *API) unsuspendAccount(w http.ResponseWriter, r *http.Request) {
	a.setAccountStatus(w, r, "active", "account.unsuspend", rbac.AccountsSuspend)
}
func (a *API) terminateAccount(w http.ResponseWriter, r *http.Request) {
	a.setAccountStatus(w, r, "terminating", "account.terminate", rbac.AccountsTerminate)
}

func (a *API) setAccountStatus(w http.ResponseWriter, r *http.Request, status, action, cap string) {
	accID := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, accID, cap) {
		return
	}
	acc := a.Store.GetAccount(accID)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	before := acc.Status
	acc.Status = status
	acc.DesiredRevision++
	a.Store.PutAccount(acc)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID, Payload: map[string]any{"account_id": acc.ID, "status": status}, State: "queued"})
	a.audit(r, action, "account", acc.ID, true, map[string]any{"status": before}, map[string]any{"status": status})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "account": acc})
}

func (a *API) impersonate(w http.ResponseWriter, r *http.Request) {
	accID := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, accID, rbac.AccountsImpersonate) {
		return
	}
	acc := a.Store.GetAccount(accID)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Reason == "" {
		a.fail(w, r, 400, "VALIDATION", "Impersonation reason required", false)
		return
	}
	plain, hash, _ := auth.NewOpaqueToken()
	sess := &store.Session{
		ID: id.New(), UserID: acc.OwnerUserID, TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * time.Minute), ImpersonatorID: actor(r).UserID,
		ImpersonationReason: in.Reason, SourceIP: clientIP(r), UserAgent: r.UserAgent(),
	}
	a.Store.PutSession(sess)
	a.audit(r, "account.impersonate", "account", acc.ID, true, nil, map[string]any{"reason": in.Reason, "effective_actor": acc.OwnerUserID})
	writeJSON(w, 200, map[string]any{"token": plain, "account_id": acc.ID, "expires_at": sess.ExpiresAt})
}

func (a *API) accountUsage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accountID")
	ac := actor(r)
	if !ac.CanAccount(id) {
		a.fail(w, r, 403, "FORBIDDEN", "Not authorized for this account", false)
		return
	}
	if !ac.Has(rbac.BillingUsageRead) && !ac.Has(rbac.AccountsRead) {
		a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
		return
	}
	u := a.Store.GetUsage(id)
	if u == nil {
		u = &store.Usage{AccountID: id, CollectedAt: time.Now().UTC()}
	}
	writeJSON(w, 200, u)
}

func (a *API) bulkSuspend(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsSuspend) {
		return
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	ops := []string{}
	who := actor(r)
	for _, id := range in.IDs {
		if !who.CanAccount(id) {
			continue
		}
		if acc := a.Store.GetAccount(id); acc != nil {
			acc.Status = "suspended"
			acc.DesiredRevision++
			a.Store.PutAccount(acc)
			j, _ := a.Store.EnqueueJob(&store.Job{Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID, Payload: map[string]any{"account_id": acc.ID}, State: "queued"})
			ops = append(ops, j.ID)
		}
	}
	writeJSON(w, 202, map[string]any{"operations": ops})
}

func (a *API) listDomains(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DomainsRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListDomains(aid)})
}

func (a *API) createDomain(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DomainsWrite) {
		return
	}
	var in struct {
		FQDN    string `json:"fqdn"`
		Type    string `json:"type"`
		Runtime string `json:"runtime"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	ascii, err := validate.NormalizeDomain(in.FQDN)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if a.Store.DomainTaken(ascii) {
		a.fail(w, r, 409, "DOMAIN_ALREADY_EXISTS", "Domain is already assigned.", false)
		return
	}
	if in.Type == "" {
		in.Type = "addon"
	}
	switch in.Type {
	case "primary", "addon", "subdomain", "alias":
	default:
		a.fail(w, r, 400, "VALIDATION", "type must be primary, addon, subdomain, or alias", false)
		return
	}
	if err := a.enforceDomainLimit(aid, in.Type); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	acc := a.Store.GetAccount(aid)
	doc := acc.HomePath + "/" + ascii
	if in.Type == "alias" {
		doc = acc.HomePath + "/public_html"
	}
	d := &store.Domain{ID: id.New(), AccountID: aid, FQDN: ascii, ASCII: ascii, Type: in.Type, DocumentRoot: doc, DNSManaged: true, Status: "provisioning"}
	a.Store.PutDomain(d)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "domain.provision", ResourceType: "domain", ResourceID: d.ID, Payload: map[string]any{"domain_id": d.ID, "account_id": aid, "runtime": in.Runtime}, State: "queued"})
	a.audit(r, "domain.create", "domain", d.ID, true, nil, map[string]any{"fqdn": ascii, "runtime": in.Runtime})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "resource_id": d.ID, "status": "provisioning", "domain": d})
}

func (a *API) deleteDomain(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DomainsWrite) {
		return
	}
	d := a.Store.GetDomain(chi.URLParam(r, "domainID"))
	if d == nil || d.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "domain missing", false)
		return
	}
	if d.Type == "primary" {
		a.fail(w, r, 409, "IN_USE", "primary domain cannot be deleted", false)
		return
	}
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "domain.delete", ResourceType: "domain", ResourceID: d.ID,
		Payload: map[string]any{"domain_id": d.ID, "account_id": aid},
		State:   "queued",
	})
	a.audit(r, "domain.delete", "domain", d.ID, true, map[string]any{"fqdn": d.ASCII, "type": d.Type}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listWebsites(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListWebsites(aid)})
}

func (a *API) createWebsite(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	var in store.Website
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.ID = id.New()
	in.AccountID = aid
	in.DesiredRevision = 1
	if in.Runtime == "" {
		in.Runtime = "php"
	}
	acc := a.Store.GetAccount(aid)
	if in.DocumentRoot == "" && acc != nil {
		in.DocumentRoot = acc.HomePath + "/public_html"
		if d := a.Store.GetDomain(in.DomainID); d != nil && d.DocumentRoot != "" {
			in.DocumentRoot = d.DocumentRoot
		}
	}
	if d := a.Store.GetDomain(in.DomainID); d != nil && d.AccountID == aid {
		for _, existing := range a.Store.ListWebsites(aid) {
			if existing.DomainID != in.DomainID {
				continue
			}
			if in.Runtime != "" {
				existing.Runtime = in.Runtime
			}
			if in.DocumentRoot != "" {
				existing.DocumentRoot = in.DocumentRoot
			}
			existing.DesiredRevision++
			a.Store.PutWebsite(&existing)
			job, _ := a.Store.EnqueueJob(&store.Job{Type: "website.provision", ResourceType: "website", ResourceID: existing.ID, Payload: map[string]any{"website_id": existing.ID}, State: "queued"})
			writeJSON(w, 202, map[string]any{"operation_id": job.ID, "website": existing})
			return
		}
	}
	if err := a.enforceCountLimit(aid, "websites", len(a.Store.ListWebsites(aid)), func(p *store.Package) int {
		return p.Domains + p.Subdomains + p.AliasDomains
	}); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	a.Store.PutWebsite(&in)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "website.provision", ResourceType: "website", ResourceID: in.ID, Payload: map[string]any{"website_id": in.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "website": in})
}

func (a *API) deleteWebsite(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	site := a.Store.GetWebsite(chi.URLParam(r, "websiteID"))
	if site == nil || site.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "website missing", false)
		return
	}
	if d := a.Store.GetDomain(site.DomainID); d != nil && d.Type == "primary" {
		a.fail(w, r, 409, "IN_USE", "primary domain website cannot be deleted", false)
		return
	}
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "website.delete", ResourceType: "website", ResourceID: site.ID,
		Payload: map[string]any{"website_id": site.ID},
		State:   "queued",
	})
	a.audit(r, "website.delete", "website", site.ID, true, map[string]any{"domain_id": site.DomainID}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listApps(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListApps(aid)})
}

func (a *API) createApp(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsWrite) {
		return
	}
	if err := a.enforceCountLimit(aid, "applications", len(a.Store.ListApps(aid)), func(p *store.Package) int { return p.ApplicationInstances }); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	var in store.Application
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.ID = id.New()
	in.AccountID = aid
	if in.Status == "" {
		in.Status = "provisioning"
	}
	a.Store.PutApp(&in)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "application.deploy", ResourceType: "application", ResourceID: in.ID, Payload: map[string]any{"application_id": in.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "application": in})
}

func (a *API) deleteApp(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsWrite) {
		return
	}
	app := a.Store.GetApp(chi.URLParam(r, "applicationID"))
	if app == nil || app.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "application missing", false)
		return
	}
	a.Store.DeleteApp(app.ID)
	a.audit(r, "application.delete", "application", app.ID, true, map[string]any{"runtime": app.Runtime}, nil)
	writeJSON(w, 200, map[string]any{"deleted": app.ID})
}

func (a *API) installWordPress(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.ApplicationsWrite) {
		return
	}
	if err := a.enforceCountLimit(aid, "applications", len(a.Store.ListApps(aid)), func(p *store.Package) int { return p.ApplicationInstances }); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	var in struct {
		WebsiteID     string `json:"website_id"`
		Title         string `json:"title"`
		AdminUser     string `json:"admin_user"`
		AdminPassword string `json:"admin_password"`
		AdminEmail    string `json:"admin_email"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := validate.Username(in.AdminUser); err != nil {
		a.fail(w, r, 400, "VALIDATION", "admin_user: "+err.Error(), false)
		return
	}
	if len(in.AdminPassword) < 8 {
		a.fail(w, r, 400, "VALIDATION", "admin password must be at least 8 characters", false)
		return
	}
	if in.AdminEmail == "" || !strings.Contains(in.AdminEmail, "@") {
		a.fail(w, r, 400, "VALIDATION", "admin_email required", false)
		return
	}
	if in.Title == "" {
		in.Title = "WordPress"
	}
	site := a.Store.GetWebsite(in.WebsiteID)
	if site == nil || site.AccountID != aid {
		a.fail(w, r, 400, "VALIDATION", "website_id required", false)
		return
	}
	hostname := ""
	if d := a.Store.GetDomain(site.DomainID); d != nil {
		hostname = d.ASCII
	}
	app := &store.Application{
		ID: id.New(), WebsiteID: site.ID, AccountID: aid,
		Runtime: "wordpress", WorkingDirectory: site.DocumentRoot, Status: "provisioning",
	}
	a.Store.PutApp(app)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "wordpress.install", ResourceType: "application", ResourceID: app.ID,
		Payload: map[string]any{
			"application_id": app.ID, "title": in.Title, "hostname": hostname,
			"admin_user": in.AdminUser, "admin_password": in.AdminPassword,
			"admin_email": in.AdminEmail,
		},
		State: "queued",
	})
	a.audit(r, "wordpress.install", "application", app.ID, true, nil, map[string]any{
		"website_id": site.ID, "title": in.Title,
	})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "application": app})
}

func (a *API) listDBs(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DatabasesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListDBs(aid), "users": a.Store.ListDBUsers(aid)})
}

func (a *API) createDB(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DatabasesWrite) {
		return
	}
	if err := a.enforceCountLimit(aid, "databases", len(a.Store.ListDBs(aid)), func(p *store.Package) int { return p.Databases }); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	var in struct {
		Name   string `json:"name"`
		Engine string `json:"engine"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	acc := a.Store.GetAccount(aid)
	if in.Engine == "" {
		in.Engine = "mariadb"
	}
	switch in.Engine {
	case "mariadb", "mysql", "postgres":
	default:
		a.fail(w, r, 400, "VALIDATION", "engine must be mariadb, mysql, or postgres", false)
		return
	}
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	if err := validate.LocalPart(in.Name); err != nil || strings.Contains(in.Name, ".") {
		a.fail(w, r, 400, "VALIDATION", "database name must be a simple identifier", false)
		return
	}
	name := acc.Username + "_" + in.Name
	d := &store.HostedDatabase{ID: id.New(), AccountID: aid, Engine: in.Engine, Name: name, Status: "provisioning"}
	if len(a.Store.ListDBUsers(aid)) == 0 {
		if err := a.enforceCountLimit(aid, "database_users", 0, func(p *store.Package) int { return p.DatabaseUsers }); err != nil {
			a.rejectLimit(w, r, err)
			return
		}
	}
	a.Store.PutDB(d)
	if len(a.Store.ListDBUsers(aid)) == 0 {
		a.Store.PutDBUser(&store.DatabaseUser{ID: id.New(), AccountID: aid, Username: acc.Username + "_u", Engine: in.Engine})
	}
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "database.provision", ResourceType: "database", ResourceID: d.ID, Payload: map[string]any{"database_id": d.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "database": d})
}

func (a *API) deleteDB(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DatabasesWrite) {
		return
	}
	d := a.Store.GetDB(chi.URLParam(r, "databaseID"))
	if d == nil || d.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "database missing", false)
		return
	}
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "database.delete", ResourceType: "database", ResourceID: d.ID,
		Payload: map[string]any{"database_id": d.ID},
		State:   "queued",
	})
	a.audit(r, "database.delete", "database", d.ID, true, map[string]any{"name": d.Name, "engine": d.Engine}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listZones(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSRead) {
		return
	}
	items := a.Store.ListZones(aid)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) listRecords(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListRecords(chi.URLParam(r, "zoneID"))})
}

func (a *API) createRecord(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSWrite) {
		return
	}
	var rec store.DNSRecord
	_ = json.NewDecoder(r.Body).Decode(&rec)
	if err := validateDNS(rec); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	rec.ID = id.New()
	rec.ZoneID = chi.URLParam(r, "zoneID")
	a.Store.PutRecord(&rec)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "dns.sync", ResourceType: "dns_zone", ResourceID: rec.ZoneID, Payload: map[string]any{"zone_id": rec.ZoneID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "record": rec})
}

func (a *API) deleteRecord(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSWrite) {
		return
	}
	zid := chi.URLParam(r, "zoneID")
	z := a.Store.GetZone(zid)
	if z == nil || z.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "zone missing", false)
		return
	}
	rid := chi.URLParam(r, "recordID")
	found := false
	for _, rec := range a.Store.ListRecords(zid) {
		if rec.ID == rid {
			found = true
			break
		}
	}
	if !found {
		a.fail(w, r, 404, "NOT_FOUND", "record missing", false)
		return
	}
	a.Store.DeleteRecord(rid)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "dns.sync", ResourceType: "dns_zone", ResourceID: zid, Payload: map[string]any{"zone_id": zid}, State: "queued"})
	a.audit(r, "dns.record.delete", "dns_record", rid, true, nil, map[string]any{"zone_id": zid})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "deleted": rid})
}

func (a *API) setDNSSEC(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSWrite) {
		return
	}
	z := a.Store.GetZone(chi.URLParam(r, "zoneID"))
	if z == nil || z.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "zone missing", false)
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Enabled == nil {
		a.fail(w, r, 400, "VALIDATION", "enabled is required", false)
		return
	}
	z.DNSSECEnabled = *in.Enabled
	z.DesiredRevision++
	a.Store.PutZone(z)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "dns.dnssec", ResourceType: "dns_zone", ResourceID: z.ID,
		Payload: map[string]any{"zone_id": z.ID}, State: "queued",
	})
	a.audit(r, "dns.dnssec", "dns_zone", z.ID, true, nil, map[string]any{"enabled": z.DNSSECEnabled})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "zone": z})
}

func (a *API) getDSRecords(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSRead) {
		return
	}
	z := a.Store.GetZone(chi.URLParam(r, "zoneID"))
	if z == nil || z.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "zone missing", false)
		return
	}
	items := []operations.DSRecord{}
	if a.Agent != nil {
		params, _ := json.Marshal(map[string]any{"name": z.Name})
		raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "GetDSRecords",
			Params: params,
		})
		if err == nil {
			b, _ := json.Marshal(raw)
			var st operations.DNSSECState
			if json.Unmarshal(b, &st) == nil {
				items = st.DS
			}
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "dnssec_enabled": z.DNSSECEnabled})
}

func (a *API) listMailDomains(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailRead) {
		return
	}
	items := []map[string]any{}
	for _, md := range a.Store.ListMailDomains(aid) {
		name := md.ID
		if d := a.Store.GetDomain(md.DomainID); d != nil {
			name = d.ASCII
		}
		items = append(items, map[string]any{
			"id": md.ID, "domain_id": md.DomainID, "ascii_fqdn": name,
			"catchall_policy": md.CatchallPolicy, "status": md.Status,
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) patchMailDomain(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	mdID := chi.URLParam(r, "mailDomainID")
	var md *store.MailDomain
	for _, item := range a.Store.ListMailDomains(aid) {
		if item.ID == mdID {
			cp := item
			md = &cp
			break
		}
	}
	if md == nil {
		a.fail(w, r, 404, "NOT_FOUND", "mail domain missing", false)
		return
	}
	var in struct {
		CatchallPolicy string `json:"catchall_policy"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	policy := strings.TrimSpace(strings.ToLower(in.CatchallPolicy))
	switch policy {
	case "reject", "discard":
	default:
		if err := validate.LocalPart(policy); err != nil {
			a.fail(w, r, 400, "VALIDATION", "catchall_policy must be reject, discard, or a mailbox local part", false)
			return
		}
	}
	before := md.CatchallPolicy
	md.CatchallPolicy = policy
	a.Store.PutMailDomain(md)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mail.maps", ResourceType: "mail_domain", ResourceID: md.ID,
		Payload: map[string]any{"account_id": aid}, State: "queued",
	})
	a.audit(r, "mail.catchall", "mail_domain", md.ID, true, map[string]any{"catchall_policy": before}, map[string]any{"catchall_policy": policy})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "mail_domain": md})
}

func (a *API) listMailboxes(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListMailboxes(aid)})
}

func (a *API) createMailbox(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	var in struct {
		DomainID  string `json:"domain_id"`
		LocalPart string `json:"local_part"`
		Password  string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := validate.LocalPart(in.LocalPart); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	md := resolveMailDomain(a.Store, aid, in.DomainID)
	if md == nil {
		a.fail(w, r, 400, "VALIDATION", "Mail domain is not provisioned yet", false)
		return
	}
	if in.Password == "" {
		a.fail(w, r, 400, "VALIDATION", "Mailbox password required", false)
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
	mb := &store.Mailbox{ID: id.New(), AccountID: aid, DomainID: md.ID, LocalPart: in.LocalPart, QuotaBytes: quota, PasswordHash: hash, Status: "provisioning"}
	updating := false
	for _, existing := range a.Store.ListMailboxes(aid) {
		if existing.DomainID == md.ID && existing.LocalPart == in.LocalPart {
			existing.PasswordHash = hash
			existing.Status = "provisioning"
			mb = &existing
			updating = true
			break
		}
	}
	if !updating {
		if err := a.enforceCountLimit(aid, "mailboxes", len(a.Store.ListMailboxes(aid)), func(p *store.Package) int { return p.Mailboxes }); err != nil {
			a.rejectLimit(w, r, err)
			return
		}
	}
	a.Store.PutMailbox(mb)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "mailbox.provision", ResourceType: "mailbox", ResourceID: mb.ID, Payload: map[string]any{"mailbox_id": mb.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "mailbox": mb})
}

func (a *API) deleteMailbox(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	mb := a.Store.GetMailbox(chi.URLParam(r, "mailboxID"))
	if mb == nil || mb.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "mailbox missing", false)
		return
	}
	for _, al := range a.Store.ListMailAliases(aid) {
		if al.Destination == mb.LocalPart || strings.HasPrefix(al.Destination, mb.LocalPart+"@") {
			a.fail(w, r, 409, "IN_USE", "mailbox is an alias destination", false)
			return
		}
	}
	a.Store.DeleteMailbox(mb.ID)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mailbox.delete", ResourceType: "mailbox", ResourceID: mb.ID,
		Payload: map[string]any{"account_id": aid, "mailbox_id": mb.ID},
		State:   "queued",
	})
	a.audit(r, "mailbox.delete", "mailbox", mb.ID, true, map[string]any{"local_part": mb.LocalPart}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listMailAliases(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListMailAliases(aid)})
}

func (a *API) createMailAlias(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	var in struct {
		DomainID    string `json:"domain_id"`
		Address     string `json:"address"`
		Destination string `json:"destination"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := validate.LocalPart(in.Address); err != nil {
		a.fail(w, r, 400, "VALIDATION", "address: "+err.Error(), false)
		return
	}
	dest, err := normalizeAliasDestination(in.Destination)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	md := resolveMailDomain(a.Store, aid, in.DomainID)
	if md == nil {
		a.fail(w, r, 400, "VALIDATION", "Mail domain is not provisioned yet", false)
		return
	}
	if dest == in.Address {
		a.fail(w, r, 400, "VALIDATION", "alias cannot point at itself", false)
		return
	}
	for _, existing := range a.Store.ListMailAliases(aid) {
		if existing.DomainID == md.ID && existing.Address == in.Address {
			a.fail(w, r, 409, "CONFLICT", "alias already exists", false)
			return
		}
	}
	al := &store.MailAlias{ID: id.New(), AccountID: aid, DomainID: md.ID, Address: in.Address, Destination: dest}
	a.Store.PutMailAlias(al)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID},
		State:   "queued",
	})
	a.audit(r, "mail.alias.create", "mail_alias", al.ID, true, nil, map[string]any{"address": al.Address, "destination": al.Destination})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "alias": al})
}

func (a *API) deleteMailAlias(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.MailWrite) {
		return
	}
	al := a.Store.GetMailAlias(chi.URLParam(r, "aliasID"))
	if al == nil || al.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "alias missing", false)
		return
	}
	a.Store.DeleteMailAlias(al.ID)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "mail.alias", ResourceType: "mail_alias", ResourceID: al.ID,
		Payload: map[string]any{"account_id": aid, "alias_id": al.ID},
		State:   "queued",
	})
	a.audit(r, "mail.alias.delete", "mail_alias", al.ID, true, map[string]any{"address": al.Address}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func normalizeAliasDestination(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" || strings.ContainsAny(s, ";|&$`\n\r ") {
		return "", fmt.Errorf("invalid destination")
	}
	if !strings.Contains(s, "@") {
		if err := validate.LocalPart(s); err != nil {
			return "", err
		}
		return s, nil
	}
	local, host, ok := strings.Cut(s, "@")
	if !ok || strings.Count(s, "@") != 1 {
		return "", fmt.Errorf("invalid destination")
	}
	if err := validate.LocalPart(local); err != nil {
		return "", err
	}
	if _, err := validate.NormalizeDomain(host); err != nil {
		return "", err
	}
	return local + "@" + host, nil
}

func (a *API) listCerts(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListCerts(aid)})
}

func (a *API) requestCert(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.WebsitesWrite) {
		return
	}
	var in struct {
		Hostname string `json:"hostname"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	host, err := validate.NormalizeDomain(in.Hostname)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	owned := false
	for _, d := range a.Store.ListDomains(aid) {
		if d.ASCII == host {
			owned = true
			break
		}
	}
	if !owned {
		a.fail(w, r, 400, "VALIDATION", "hostname must be a domain on this account", false)
		return
	}
	in.Hostname = host
	var c *store.Certificate
	for _, existing := range a.Store.ListCerts(aid) {
		if existing.Hostname != host {
			continue
		}
		cp := existing
		c = &cp
		break
	}
	if job := inflightCertJob(a.Store, c); job != nil {
		writeJSON(w, 202, map[string]any{"operation_id": job.ID, "certificate": c})
		return
	}
	if c != nil && certStillFresh(c) {
		writeJSON(w, 200, map[string]any{"certificate": c})
		return
	}
	if c == nil {
		c = &store.Certificate{ID: id.New(), AccountID: aid, Hostname: host, Kind: "domain", Status: "requested"}
	} else {
		c.Status = "requested"
	}
	a.Store.PutCert(c)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "certificate.provision", ResourceType: "certificate", ResourceID: c.ID,
		Payload: map[string]any{"certificate_id": c.ID}, State: "queued",
	})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "certificate": c})
}

func queuedReconcileJob(st store.Store, accountID string) string {
	for _, j := range st.ListJobs("queued", 100) {
		if j.ResourceID == accountID && (j.Type == "account.reconcile" || j.Type == "account.provision") {
			return j.ID
		}
	}
	return ""
}

func inflightCertJob(st store.Store, c *store.Certificate) *store.Job {
	if c == nil {
		return nil
	}
	for _, state := range []string{"queued", "running"} {
		for _, j := range st.ListJobs(state, 200) {
			if j.Type != "certificate.provision" {
				continue
			}
			if j.ResourceID == c.ID || strPayload(j.Payload, "certificate_id") == c.ID {
				cp := j
				return &cp
			}
		}
	}
	return nil
}

func certStillFresh(c *store.Certificate) bool {
	if c == nil {
		return false
	}
	switch c.Status {
	case "issued", "active":
	default:
		return false
	}
	if c.NotAfter == nil {
		return true
	}
	return c.NotAfter.After(time.Now().Add(30 * 24 * time.Hour))
}

func strPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	s, _ := payload[key].(string)
	return s
}

func (a *API) listBackups(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.BackupsRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListBackups(aid)})
}

func (a *API) createBackup(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.BackupsCreate) {
		return
	}
	var in struct {
		Kind        string `json:"kind"`
		Destination string `json:"destination"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Kind == "" {
		in.Kind = "full"
	}
	if in.Destination == "" {
		in.Destination = "local"
	}
	switch in.Destination {
	case "local", "sftp", "s3":
	default:
		a.fail(w, r, 400, "VALIDATION", "destination must be local, sftp, or s3", false)
		return
	}
	b := &store.BackupRun{ID: id.New(), AccountID: aid, Kind: in.Kind, State: "queued", Destination: in.Destination, CreatedAt: time.Now().UTC()}
	a.Store.PutBackup(b)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "backup.create", ResourceType: "backup", ResourceID: b.ID, Payload: map[string]any{"backup_id": b.ID, "account_id": aid}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "backup": b})
}

func (a *API) restoreBackup(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.BackupsRestore) {
		return
	}
	var in struct {
		BackupID string `json:"backup_id"`
		Mode     string `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "backup.restore", ResourceType: "backup", ResourceID: in.BackupID, Payload: map[string]any{"backup_id": in.BackupID, "account_id": aid, "mode": in.Mode}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listFiles(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	acc := a.Store.GetAccount(aid)
	rel := r.URL.Query().Get("path")
	if rel == "" {
		rel = "/"
	}
	abs := filepath.Join(acc.HomePath, strings.TrimPrefix(rel, "/"))
	clean, err := policy.WithinAccount(acc.Username, filepath.Clean(abs))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	params, _ := json.Marshal(map[string]any{"path": clean})
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListDirectory",
		Params: params,
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"path": rel, "items": []any{}, "note": "directory not provisioned yet"})
		return
	}
	b, _ := json.Marshal(raw)
	var listing struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(b, &listing)
	if listing.Items == nil {
		listing.Items = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"path": rel, "items": listing.Items})
}

func (a *API) writeFile(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	acc := a.Store.GetAccount(aid)
	abs := filepath.Join(acc.HomePath, strings.TrimPrefix(in.Path, "/"))
	clean, err := policy.WithinAccount(acc.Username, filepath.Clean(abs))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	if err := a.enforceDiskQuota(r.Context(), acc, int64(len(in.Content))); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	mode := uint32(0o640)
	if strings.Contains(clean, "/public_html/") || strings.HasSuffix(clean, "/public_html") {
		mode = 0o644
	}
	_, err = a.Agent.ApplyFile(clean, []byte(in.Content), mode)
	if err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) listCron(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.CronRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListCrons(aid)})
}

func (a *API) createCron(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.CronWrite) {
		return
	}
	var c store.CronJob
	_ = json.NewDecoder(r.Body).Decode(&c)
	if err := validate.CronSchedule(c.Schedule); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if err := validate.CronCommand(c.Command); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if err := a.enforceCountLimit(aid, "cron_jobs", len(a.Store.ListCrons(aid)), func(p *store.Package) int { return p.CronJobs }); err != nil {
		a.rejectLimit(w, r, err)
		return
	}
	if acc := a.Store.GetAccount(aid); acc != nil && c.WorkingDirectory == "" {
		c.WorkingDirectory = acc.HomePath
	}
	c.ID = id.New()
	c.AccountID = aid
	c.Enabled = true
	a.Store.PutCron(&c)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "cron.apply", ResourceType: "account", ResourceID: aid, Payload: map[string]any{"account_id": aid}, State: "queued"})
	writeJSON(w, 201, map[string]any{"cron": c, "operation_id": job.ID})
}

func (a *API) deleteCron(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.CronWrite) {
		return
	}
	c := a.Store.GetCron(chi.URLParam(r, "cronID"))
	if c == nil || c.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "cron job missing", false)
		return
	}
	a.Store.DeleteCron(c.ID)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "cron.apply", ResourceType: "account", ResourceID: aid, Payload: map[string]any{"account_id": aid}, State: "queued"})
	a.audit(r, "cron.delete", "cron_job", c.ID, true, map[string]any{"schedule": c.Schedule, "command": c.Command}, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "deleted": c.ID})
}

func (a *API) listSSH(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListSSH(aid)})
}

func (a *API) setSFTPPassword(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Password) < 8 {
		a.fail(w, r, 400, "VALIDATION", "SFTP password required", false)
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	params, _ := json.Marshal(map[string]any{"username": acc.Username, "password": in.Password})
	_, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "SetLinuxPassword",
		Params: params,
	})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "account.sftp_password", "account", aid, true, nil, map[string]any{"username": acc.Username})
	writeJSON(w, 200, map[string]any{"ok": true, "username": acc.Username})
}

func (a *API) createSSH(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	var in struct {
		Label     string `json:"label"`
		Comment   string `json:"comment"`
		PublicKey string `json:"public_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	key := strings.TrimSpace(in.PublicKey)
	if key == "" || strings.ContainsAny(key, "\n\r") {
		a.fail(w, r, 400, "VALIDATION", "one-line SSH public key required", false)
		return
	}
	if !strings.HasPrefix(key, "ssh-rsa ") && !strings.HasPrefix(key, "ssh-ed25519 ") && !strings.HasPrefix(key, "ecdsa-sha2-nistp256 ") {
		a.fail(w, r, 400, "VALIDATION", "public key must be ssh-rsa, ssh-ed25519, or ecdsa-sha2-nistp256", false)
		return
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = strings.TrimSpace(in.Comment)
	}
	rec := store.SSHKey{
		ID: id.New(), AccountID: aid, Label: label, PublicKey: key,
		CreatedAt: time.Now().UTC(),
	}
	sum := sha256.Sum256([]byte(rec.PublicKey))
	rec.Fingerprint = hex.EncodeToString(sum[:])
	a.Store.PutSSH(&rec)
	a.applyAuthorizedKeys(r.Context(), aid)
	writeJSON(w, 201, rec)
}

func (a *API) deleteSSH(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	k := a.Store.GetSSH(chi.URLParam(r, "keyID"))
	if k == nil || k.AccountID != aid {
		a.fail(w, r, 404, "NOT_FOUND", "ssh key missing", false)
		return
	}
	a.Store.DeleteSSH(k.ID)
	a.applyAuthorizedKeys(r.Context(), aid)
	a.audit(r, "ssh_key.delete", "ssh_key", k.ID, true, map[string]any{"fingerprint": k.Fingerprint}, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) applyAuthorizedKeys(ctx context.Context, accountID string) {
	acc := a.Store.GetAccount(accountID)
	if acc == nil {
		return
	}
	var keys strings.Builder
	for _, k := range a.Store.ListSSH(accountID) {
		keys.WriteString(strings.TrimSpace(k.PublicKey))
		keys.WriteByte('\n')
	}
	params, _ := json.Marshal(map[string]any{"username": acc.Username, "body": keys.String()})
	_, _ = a.Agent.Dispatch(ctx, operations.Request{Method: "ApplyAuthorizedKeys", Params: params})
}

func (a *API) listFTP(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListFTP(aid)})
}

func (a *API) createFTP(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		HomePath string `json:"home_path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := validate.Username(in.Username); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if len(in.Password) < 8 {
		a.fail(w, r, 400, "VALIDATION", "FTP password must be at least 8 characters", false)
		return
	}
	home := strings.TrimSpace(in.HomePath)
	if home == "" {
		home = filepath.Join(acc.HomePath, "public_html")
	} else if !filepath.IsAbs(home) {
		home = filepath.Join(acc.HomePath, strings.TrimPrefix(home, "/"))
	}
	clean, err := policy.WithinAccount(acc.Username, filepath.Clean(home))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	if other := a.Store.AccountByUsername(in.Username); other != nil && other.ID != aid {
		a.fail(w, r, 409, "USERNAME_TAKEN", "FTP username conflicts with a hosting account", false)
		return
	}
	if u := a.Store.UserByUsername(in.Username); u != nil && u.ID != acc.OwnerUserID {
		a.fail(w, r, 409, "USERNAME_TAKEN", "FTP username conflicts with a portal login", false)
		return
	}
	hash, err := auth.SHA512Crypt(in.Password)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", "Could not hash FTP password", false)
		return
	}
	ftp := &store.FTPAccount{
		ID: id.New(), AccountID: aid, Username: in.Username, HomePath: clean,
		PasswordHash: hash, Status: "active",
	}
	updating := false
	for _, existing := range a.Store.ListFTP(aid) {
		if existing.Username == in.Username {
			existing.HomePath = clean
			existing.PasswordHash = hash
			existing.Status = "active"
			ftp = &existing
			updating = true
			break
		}
	}
	if a.Store.FTPUsernameTaken(in.Username, ftp.ID) {
		a.fail(w, r, 409, "USERNAME_TAKEN", "FTP username is already in use", false)
		return
	}
	if !updating {
		if err := a.enforceCountLimit(aid, "ftp_users", len(a.Store.ListFTP(aid)), func(p *store.Package) int { return p.FTPUsers }); err != nil {
			a.rejectLimit(w, r, err)
			return
		}
	}
	a.Store.PutFTP(ftp)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "ftp.apply", ResourceType: "account", ResourceID: aid,
		Payload: map[string]any{"account_id": aid}, State: "queued",
	})
	a.audit(r, "ftp.create", "ftp_account", ftp.ID, true, nil, map[string]any{"username": ftp.Username})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "ftp": ftp})
}

func (a *API) deleteFTP(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	fid := chi.URLParam(r, "ftpID")
	found := false
	for _, f := range a.Store.ListFTP(aid) {
		if f.ID == fid {
			found = true
			break
		}
	}
	if !found {
		a.fail(w, r, 404, "NOT_FOUND", "FTP account missing", false)
		return
	}
	a.Store.DeleteFTP(fid)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "ftp.apply", ResourceType: "account", ResourceID: aid,
		Payload: map[string]any{"account_id": aid}, State: "queued",
	})
	a.audit(r, "ftp.delete", "ftp_account", fid, true, nil, nil)
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "deleted": fid})
}

func (a *API) listTokens(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.APITokensRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListTokens(actor(r).UserID)})
}

func (a *API) createToken(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.APITokensWrite) {
		return
	}
	var in struct {
		Name         string   `json:"name"`
		Scope        string   `json:"scope"`
		AccountID    string   `json:"account_id"`
		Capabilities []string `json:"capabilities"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	plain := "hp_live_" + hex.EncodeToString(raw)
	t := &store.APIToken{
		ID: id.New(), UserID: actor(r).UserID, Name: in.Name, Prefix: auth.TokenPrefix(plain),
		TokenHash: auth.HashToken(plain), Scope: in.Scope, AccountID: in.AccountID, Capabilities: in.Capabilities,
	}
	if t.Scope == "" {
		t.Scope = "account"
	}
	a.Store.PutToken(t)
	a.audit(r, "api_token.create", "api_token", t.ID, true, nil, map[string]any{"prefix": t.Prefix, "scope": t.Scope})
	writeJSON(w, 201, map[string]any{"token": plain, "prefix": t.Prefix, "id": t.ID})
}

func (a *API) deleteToken(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.APITokensWrite) {
		return
	}
	t := a.Store.GetToken(chi.URLParam(r, "tokenID"))
	aid := chi.URLParam(r, "accountID")
	if t == nil || t.UserID != actor(r).UserID || (t.AccountID != "" && t.AccountID != aid) {
		a.fail(w, r, 404, "NOT_FOUND", "api token missing", false)
		return
	}
	a.Store.DeleteToken(t.ID)
	a.audit(r, "api_token.delete", "api_token", t.ID, true, map[string]any{"prefix": t.Prefix}, nil)
	writeJSON(w, 200, map[string]any{"deleted": t.ID})
}

func (a *API) notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.fail(w, r, 501, "NOT_IMPLEMENTED", name+" requires privileged confirmation on a production host", false)
	}
}

func (a *API) audit(r *http.Request, action, rtype, rid string, ok bool, before, after map[string]any) {
	ac := actor(r)
	a.Store.AppendAudit(store.AuditEvent{
		ActorType: "user", ActorID: ac.UserID, EffectiveActor: firstNonEmpty(ac.ImpersonatorID, ac.UserID),
		Action: action, ResourceType: rtype, ResourceID: rid, RequestID: logging.RequestID(r.Context()),
		Success: ok, SourceIP: clientIP(r), UserAgent: r.UserAgent(), Before: before, After: after,
	})
}

func (a *API) logAuthFailure(ip, username string) {
	if ip == "" {
		ip = "0.0.0.0"
	}
	rec, _ := json.Marshal(map[string]any{
		"event": "auth.login", "success": false, "source_ip": ip, "username": username,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
	base := os.Getenv("PANEL_STATE_DIR")
	if base == "" {
		base = "/var/lib/panel"
	}
	if a.Agent != nil && a.Agent.Sock == "" && a.Agent.Root != "" {
		base = filepath.Join(a.Agent.Root, "var/lib/panel")
	}
	dir := filepath.Join(base, "logs")
	_ = os.MkdirAll(dir, 0o750)
	f, err := os.OpenFile(filepath.Join(dir, "api.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	_, _ = f.Write(append(rec, '\n'))
	_ = f.Close()
}

func (a *API) accountPackage(accountID string) *store.Package {
	acc := a.Store.GetAccount(accountID)
	if acc == nil || acc.PackageID == "" {
		return nil
	}
	return a.Store.GetPackage(acc.PackageID)
}

func (a *API) enforceDomainLimit(accountID, typ string) error {
	pkg := a.accountPackage(accountID)
	kind := limits.DomainKind(typ)
	return limits.Enforce(limits.CountDomains(a.Store, accountID, kind), limits.DomainLimit(pkg, typ), kind)
}

func (a *API) enforceCountLimit(accountID, kind string, used int, limitFn func(*store.Package) int) error {
	pkg := a.accountPackage(accountID)
	limit := 0
	if pkg != nil {
		limit = limitFn(pkg)
	}
	return limits.Enforce(used, limit, kind)
}

func (a *API) enforceDiskQuota(ctx context.Context, acc *store.Account, incoming int64) error {
	if acc == nil {
		return nil
	}
	pkg := a.accountPackage(acc.ID)
	if pkg == nil || pkg.DiskBytes <= 0 || a.Agent == nil {
		return nil
	}
	params, _ := json.Marshal(map[string]any{"username": acc.Username, "home": acc.HomePath})
	raw, err := a.Agent.Dispatch(ctx, operations.Request{Method: "MeasureAccountUsage", Params: params})
	if err != nil {
		return limits.Check{Kind: "disk_bytes", Limit: pkg.DiskBytes, Used: pkg.DiskBytes}
	}
	b, _ := json.Marshal(raw)
	var u operations.AccountUsage
	_ = json.Unmarshal(b, &u)
	return limits.DiskWouldExceed(u.DiskBytes, incoming, pkg.DiskBytes)
}

func (a *API) rejectLimit(w http.ResponseWriter, r *http.Request, err error) {
	var c limits.Check
	if errors.As(err, &c) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{
			"code": "PACKAGE_LIMIT", "message": c.Error(),
			"request_id": logging.RequestID(r.Context()),
			"details":    map[string]any{"kind": c.Kind, "used": c.Used, "limit": c.Limit},
		}})
		return
	}
	a.fail(w, r, http.StatusForbidden, "PACKAGE_LIMIT", err.Error(), false)
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, status int, code, msg string, _ bool) {
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": msg, "request_id": logging.RequestID(r.Context()), "details": map[string]any{},
	}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func publicUser(u *store.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{"id": u.ID, "username": u.Username, "email": u.Email, "display_name": u.DisplayName, "roles": u.Roles, "must_change_password": u.MustChangePassword, "totp_enabled": u.TOTPEnabled}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func defaultServices() []map[string]any {
	type probe struct{ name, pid, addr, comm string }
	probes := []probe{
		{"nginx", "/run/nginx.pid", "127.0.0.1:80", "nginx"},
		{"php-fpm", "/run/php/php8.3-fpm.pid", "", "php-fpm8.3"},
		{"mariadb", "/run/mysqld/mysqld.pid", "127.0.0.1:3306", "mysqld"},
		{"postgresql", "/var/run/postgresql/16-main.pid", "", "postgres"},
		{"postfix", "/var/spool/postfix/pid/master.pid", "127.0.0.1:25", "master"},
		{"dovecot", "/run/dovecot/master.pid", "127.0.0.1:993", "dovecot"},
		{"pdns", "/run/pdns.pid", "127.0.0.1:53", "pdns_server"},
		{"rspamd", "/run/rspamd/rspamd.pid", "127.0.0.1:11332", "rspamd"},
		{"clamav", "/run/clamav/clamd.pid", "unix:/run/clamav/clamd.ctl", "clamd"},
		{"sshd", "/run/sshd.pid", "127.0.0.1:22", "sshd"},
		{"vsftpd", "/run/vsftpd.pid", "127.0.0.1:21", "vsftpd"},
		{"panel-api", "", "127.0.0.1:18080", "panel-api"},
		{"panel-worker", "", "", "panel-worker"},
		{"panel-agent", "", "unix:/run/panel/agent.sock", "panel-agent"},
	}
	out := []map[string]any{}
	for _, p := range probes {
		running := false
		if p.pid != "" {
			for _, pidPath := range []string{p.pid, strings.TrimSuffix(p.pid, ".pid") + "/pdns.pid"} {
				if b, err := os.ReadFile(pidPath); err == nil && len(bytesTrim(b)) > 0 {
					running = true
					break
				}
			}
		}
		if !running && p.addr != "" {
			network, addr := "tcp", p.addr
			if strings.HasPrefix(p.addr, "unix:") {
				network, addr = "unix", strings.TrimPrefix(p.addr, "unix:")
			}
			c, err := net.DialTimeout(network, addr, 150*time.Millisecond)
			if err == nil {
				_ = c.Close()
				running = true
			}
		}
		if !running && p.comm != "" && commRunning(p.comm) {
			running = true
		}
		health := "stopped"
		if running {
			health = "healthy"
		}
		out = append(out, map[string]any{"name": p.name, "health": health, "desired_enabled": true, "observed_running": running})
	}
	return out
}

func commRunning(name string) bool {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range ents {
		if len(e.Name()) == 0 || e.Name()[0] < '0' || e.Name()[0] > '9' {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(b)) == name {
			return true
		}
	}
	return false
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func validateDNS(r store.DNSRecord) error {
	switch r.Type {
	case "A":
		return validate.IPv4(r.Content)
	case "AAAA":
		return validate.IPv6(r.Content)
	case "CNAME", "NS", "MX":
		if r.Content == "" {
			return fmt.Errorf("content required")
		}
		return nil
	case "TXT", "CAA", "SRV", "PTR":
		if r.Content == "" {
			return fmt.Errorf("content required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported record type")
	}
}

func resolveMailDomain(st store.Store, accountID, inID string) *store.MailDomain {
	list := st.ListMailDomains(accountID)
	for i := range list {
		if list[i].ID == inID || list[i].DomainID == inID {
			return &list[i]
		}
	}
	if inID == "" && len(list) == 1 {
		return &list[0]
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type limiter struct {
	mu sync.Mutex
	n  map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{n: map[string][]time.Time{}} }

func (l *limiter) allow(key string, n int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	hits := l.n[key]
	fresh := hits[:0]
	for _, t := range hits {
		if now.Sub(t) < window {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) >= n {
		l.n[key] = fresh
		return false
	}
	l.n[key] = append(fresh, now)
	return true
}
