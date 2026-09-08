package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
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
	r.Get("/healthz", a.health)
	r.Get("/readyz", a.ready)
	r.Get("/api/v1/version", a.version)
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
			r.Post("/server/reboot", a.notImplemented("server.reboot"))
			r.Get("/jobs", a.listJobs)
			r.Get("/jobs/{jobID}", a.getJob)
			r.Get("/audit-events", a.listAudit)
			r.Get("/packages", a.listPackages)
			r.Post("/packages", a.createPackage)
			r.Get("/resellers", a.listResellers)
			r.Post("/resellers", a.createReseller)
			r.Get("/accounts", a.listAccounts)
			r.Post("/accounts", a.createAccount)
			r.Get("/accounts/{accountID}", a.getAccount)
			r.Patch("/accounts/{accountID}", a.modifyAccount)
			r.Post("/accounts/{accountID}/suspend", a.suspendAccount)
			r.Post("/accounts/{accountID}/unsuspend", a.unsuspendAccount)
			r.Post("/accounts/{accountID}/terminate", a.terminateAccount)
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
				r.Get("/websites", a.listWebsites)
				r.Post("/websites", a.createWebsite)
				r.Get("/applications", a.listApps)
				r.Post("/applications", a.createApp)
				r.Get("/databases", a.listDBs)
				r.Post("/databases", a.createDB)
				r.Get("/dns/zones", a.listZones)
				r.Get("/dns/zones/{zoneID}/records", a.listRecords)
				r.Post("/dns/zones/{zoneID}/records", a.createRecord)
				r.Get("/mail/domains", a.listMailDomains)
				r.Get("/mail/mailboxes", a.listMailboxes)
				r.Post("/mail/mailboxes", a.createMailbox)
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
				r.Get("/ssh-keys", a.listSSH)
				r.Post("/ssh-keys", a.createSSH)
				r.Get("/ftp", a.listFTP)
				r.Post("/ftp", a.createFTP)
				r.Get("/api-tokens", a.listTokens)
				r.Post("/api-tokens", a.createToken)
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
	writeJSON(w, 200, map[string]any{"version": a.Version, "api": "v1"})
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
	return rbac.Actor{
		UserID: u.ID, Username: u.Username, Roles: u.Roles,
		Capabilities:  rbac.Expand(u.Roles, nil),
		AccountIDs:    a.Store.AccountsForUser(u.ID),
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
	if a.Agent != nil {
		root = a.Agent.Root
	}
	writeJSON(w, 200, monitoring.Collect(a.Store, root))
}

func (a *API) serverServices(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerServicesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"services": defaultServices()})
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
	writeJSON(w, 200, map[string]any{"items": a.Store.ListJobs(r.URL.Query().Get("state"), 100)})
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	j := a.Store.GetJob(chi.URLParam(r, "jobID"))
	if j == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Job not found", false)
		return
	}
	writeJSON(w, 200, j)
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
	writeJSON(w, 200, map[string]any{"items": a.Store.ListPackages()})
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
	if p.FeatureSetID == "" {
		if sets := a.Store.ListFeatureSets(); len(sets) > 0 {
			p.FeatureSetID = sets[0].ID
		}
	}
	a.Store.PutPackage(&p)
	a.audit(r, "package.create", "package", p.ID, true, nil, map[string]any{"name": p.Name})
	writeJSON(w, 201, p)
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
	var in store.Reseller
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid reseller", false)
		return
	}
	in.ID = id.New()
	if in.UserID == "" || in.UserID == "pending" {
		in.UserID = actor(r).UserID
	}
	if in.Status == "" {
		in.Status = "active"
	}
	a.Store.PutReseller(&in)
	a.audit(r, "reseller.create", "reseller", in.ID, true, nil, map[string]any{"name": in.Name})
	writeJSON(w, 201, in)
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListAccounts(r.URL.Query().Get("q"), r.URL.Query().Get("status"))})
}

func (a *API) exportAccount(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.require(w, r, rbac.AccountsRead) {
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
	a.audit(r, "account.import", "account", acc.ID, true, nil, map[string]any{"username": acc.Username})
	writeJSON(w, 202, map[string]any{"resource_id": acc.ID, "status": "provisioning", "account": acc})
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
	acc, err := migration.Import(a.Store, raw)
	if err != nil {
		a.fail(w, r, 409, "IMPORT_CONFLICT", err.Error(), false)
		return
	}
	a.Store.AddMember(acc.ID, actor(r).UserID)
	copied := ""
	if exp.Homedir != "" {
		job, _ := a.Store.EnqueueJob(&store.Job{
			Type: "account.copy_homedir", ResourceType: "account", ResourceID: acc.ID,
			Payload: map[string]any{"account_id": acc.ID, "username": acc.Username, "source": exp.Homedir, "dest": acc.HomePath},
			State:   "queued",
		})
		if job != nil {
			copied = job.ID
		}
	}
	a.audit(r, "account.import.cpanel", "account", acc.ID, true, nil, map[string]any{"username": acc.Username, "homedir": exp.Homedir})
	writeJSON(w, 202, map[string]any{"resource_id": acc.ID, "status": "provisioning", "account": acc, "source": "cpanel", "homedir_job": copied})
}

func (a *API) exportAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListAccounts("", ""), "exported_at": time.Now().UTC()})
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
	dom := &store.Domain{ID: id.New(), AccountID: acc.ID, FQDN: ascii, ASCII: ascii, Type: "primary", DocumentRoot: acc.HomePath + "/public_html", DNSManaged: true, Status: "provisioning"}
	a.Store.PutDomain(dom)
	job, _ := a.Store.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "domain_id": dom.ID},
		State:   "queued", Priority: 10, IdempotencyKey: r.Header.Get("Idempotency-Key"),
		ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
	})
	a.audit(r, "account.create", "account", acc.ID, true, nil, map[string]any{"username": acc.Username, "domain": ascii})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "resource_id": acc.ID, "status": "provisioning", "account": acc})
}

func (a *API) modifyAccount(w http.ResponseWriter, r *http.Request) {
	accID := chi.URLParam(r, "accountID")
	if !a.require(w, r, rbac.AccountsModify) {
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
	if v, ok := in["reseller_id"].(string); ok {
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
	if !a.require(w, r, cap) {
		return
	}
	acc := a.Store.GetAccount(chi.URLParam(r, "accountID"))
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
	if !a.require(w, r, rbac.AccountsImpersonate) {
		return
	}
	acc := a.Store.GetAccount(chi.URLParam(r, "accountID"))
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
	if !a.requireAccount(w, r, id, rbac.BillingUsageRead) && !actor(r).Has(rbac.AccountsRead) {
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
	for _, id := range in.IDs {
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
	acc := a.Store.GetAccount(aid)
	d := &store.Domain{ID: id.New(), AccountID: aid, FQDN: ascii, ASCII: ascii, Type: in.Type, DocumentRoot: acc.HomePath + "/" + ascii, DNSManaged: true, Status: "provisioning"}
	a.Store.PutDomain(d)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "domain.provision", ResourceType: "domain", ResourceID: d.ID, Payload: map[string]any{"domain_id": d.ID, "account_id": aid, "runtime": in.Runtime}, State: "queued"})
	a.audit(r, "domain.create", "domain", d.ID, true, nil, map[string]any{"fqdn": ascii, "runtime": in.Runtime})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "resource_id": d.ID, "status": "provisioning", "domain": d})
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
	a.Store.PutWebsite(&in)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "website.provision", ResourceType: "website", ResourceID: in.ID, Payload: map[string]any{"website_id": in.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "website": in})
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
	var in struct {
		Name   string `json:"name"`
		Engine string `json:"engine"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	acc := a.Store.GetAccount(aid)
	if in.Engine == "" {
		in.Engine = "mariadb"
	}
	name := acc.Username + "_" + in.Name
	d := &store.HostedDatabase{ID: id.New(), AccountID: aid, Engine: in.Engine, Name: name, Status: "provisioning"}
	a.Store.PutDB(d)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "database.provision", ResourceType: "database", ResourceID: d.ID, Payload: map[string]any{"database_id": d.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "database": d})
}

func (a *API) listZones(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.DNSRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListZones(aid)})
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
	mb := &store.Mailbox{ID: id.New(), AccountID: aid, DomainID: md.ID, LocalPart: in.LocalPart, QuotaBytes: 1 << 30, PasswordHash: hash, Status: "provisioning"}
	a.Store.PutMailbox(mb)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "mailbox.provision", ResourceType: "mailbox", ResourceID: mb.ID, Payload: map[string]any{"mailbox_id": mb.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "mailbox": mb})
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
	c := &store.Certificate{ID: id.New(), AccountID: aid, Hostname: in.Hostname, Kind: "domain", Status: "requested"}
	a.Store.PutCert(c)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "certificate.provision", ResourceType: "certificate", ResourceID: c.ID, Payload: map[string]any{"certificate_id": c.ID}, State: "queued"})
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "certificate": c})
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
	c.ID = id.New()
	c.AccountID = aid
	a.Store.PutCron(&c)
	job, _ := a.Store.EnqueueJob(&store.Job{Type: "cron.apply", ResourceType: "account", ResourceID: aid, Payload: map[string]any{"account_id": aid}, State: "queued"})
	writeJSON(w, 201, map[string]any{"cron": c, "operation_id": job.ID})
}

func (a *API) listSSH(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Store.ListSSH(aid)})
}

func (a *API) createSSH(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.FilesWrite) {
		return
	}
	var in store.SSHKey
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.ID = id.New()
	in.AccountID = aid
	in.CreatedAt = time.Now().UTC()
	sum := sha256.Sum256([]byte(in.PublicKey))
	in.Fingerprint = hex.EncodeToString(sum[:])
	a.Store.PutSSH(&in)
	acc := a.Store.GetAccount(aid)
	var keys strings.Builder
	for _, k := range a.Store.ListSSH(aid) {
		keys.WriteString(strings.TrimSpace(k.PublicKey))
		keys.WriteByte('\n')
	}
	if acc != nil {
		params, _ := json.Marshal(map[string]any{"username": acc.Username, "body": keys.String()})
		_, _ = a.Agent.Dispatch(r.Context(), operations.Request{Method: "ApplyAuthorizedKeys", Params: params})
	}
	writeJSON(w, 201, in)
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
	var in store.FTPAccount
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.ID = id.New()
	in.AccountID = aid
	in.Status = "active"
	a.Store.PutFTP(&in)
	writeJSON(w, 201, in)
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
	type probe struct{ name, pid, addr string }
	probes := []probe{
		{"nginx", "/run/nginx.pid", "127.0.0.1:80"},
		{"php-fpm", "/run/php/php8.3-fpm.pid", ""},
		{"mariadb", "/run/mysqld/mysqld.pid", "127.0.0.1:3306"},
		{"postgresql", "/var/run/postgresql/16-main.pid", ""},
		{"postfix", "/var/spool/postfix/pid/master.pid", "127.0.0.1:25"},
		{"dovecot", "/run/dovecot/master.pid", "127.0.0.1:993"},
		{"pdns", "/run/pdns.pid", "127.0.0.1:53"},
		{"rspamd", "/run/rspamd/rspamd.pid", "127.0.0.1:11332"},
		{"clamav", "/run/clamav/clamd.pid", "unix:/run/clamav/clamd.ctl"},
		{"sshd", "/run/sshd.pid", "127.0.0.1:22"},
	}
	out := []map[string]any{}
	for _, p := range probes {
		running := false
		if b, err := os.ReadFile(p.pid); err == nil && len(bytesTrim(b)) > 0 {
			running = true
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
		health := "stopped"
		if running {
			health = "healthy"
		}
		out = append(out, map[string]any{"name": p.name, "health": health, "desired_enabled": true, "observed_running": running})
	}
	return out
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
