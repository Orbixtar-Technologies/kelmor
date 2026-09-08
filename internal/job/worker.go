package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/dns"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

type Worker struct {
	Store  store.Store
	Agent  *operations.Host
	Log    *logging.Logger
	Name   string
	stop   chan struct{}
}

func New(st store.Store, agent *operations.Host, log *logging.Logger, name string) *Worker {
	return &Worker{Store: st, Agent: agent, Log: log, Name: name, stop: make(chan struct{})}
}

func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-t.C:
			w.reap()
			if j := w.Store.ClaimJob(w.Name); j != nil {
				w.execute(ctx, j)
			} else {
				w.scanDrift(ctx)
			}
		}
	}
}

func (w *Worker) reap() {
	now := time.Now()
	for _, j := range w.Store.ListJobs("running", 500) {
		if j.HeartbeatAt != nil && now.Sub(*j.HeartbeatAt) > 2*time.Minute {
			j.State = "retrying"
			j.RunAfter = now
			j.LockedBy = ""
			w.Store.UpdateJob(&j)
		}
	}
}

func (w *Worker) scanDrift(ctx context.Context) {
	for _, acc := range w.Store.DriftedAccounts() {
		_, _ = w.Store.EnqueueJob(&store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
			Payload: map[string]any{"account_id": acc.ID}, State: "queued",
			IdempotencyKey: fmt.Sprintf("reconcile:%s:%d", acc.ID, acc.DesiredRevision),
		})
	}
}

func (w *Worker) execute(ctx context.Context, j *store.Job) {
	j.Logs = append(j.Logs, "started "+j.Type)
	j.Progress = 5
	w.Store.UpdateJob(j)
	err := w.handle(ctx, j)
	now := time.Now()
	if err != nil {
		j.LastError = err.Error()
		j.Logs = append(j.Logs, "error: "+err.Error())
		if j.Attempts >= j.MaxAttempts {
			j.State = "failed"
			j.FinishedAt = &now
		} else {
			j.State = "retrying"
			j.RunAfter = now.Add(store.RetryDelay(j.Attempts + 1))
		}
		w.Log.Error(ctx, j.Type+".failed", map[string]any{"job_id": j.ID, "error": err.Error()})
		w.Store.UpdateJob(j)
		return
	}
	j.State = "succeeded"
	j.Progress = 100
	j.FinishedAt = &now
	j.Logs = append(j.Logs, "succeeded")
	w.Log.Info(ctx, j.Type+".success", map[string]any{"job_id": j.ID})
	w.Store.UpdateJob(j)
}

func (w *Worker) handle(ctx context.Context, j *store.Job) error {
	switch j.Type {
	case "account.provision", "account.reconcile":
		return w.provisionAccount(j)
	case "domain.provision":
		return w.provisionDomain(j)
	case "website.provision":
		return w.provisionWebsite(j)
	case "application.deploy":
		return w.deployApp(j)
	case "database.provision":
		return w.provisionDB(j)
	case "dns.sync":
		return w.syncDNS(j)
	case "mailbox.provision":
		return w.provisionMailbox(j)
	case "certificate.provision":
		return w.provisionCert(j)
	case "backup.create":
		return w.createBackup(j)
	case "backup.restore":
		return w.restoreBackup(j)
	default:
		return fmt.Errorf("unknown job type %s", j.Type)
	}
}

func (w *Worker) provisionAccount(j *store.Job) error {
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	pkg := w.Store.GetPackage(acc.PackageID)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateLinuxUser",
		Params: mustJSON(map[string]any{"username": acc.Username, "uid": acc.LinuxUID, "gid": acc.LinuxGID, "home": acc.HomePath, "shell": "/usr/sbin/nologin"}),
	})
	if err != nil {
		acc.Status = "failed"
		w.Store.PutAccount(acc)
		return err
	}
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplySystemdSlice",
		Params: mustJSON(map[string]any{"username": acc.Username, "cpu_percent": pkg.CPUPercent, "memory_bytes": pkg.MemoryBytes}),
	})
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetFilesystemQuota",
		Params: mustJSON(map[string]any{"username": acc.Username, "bytes": pkg.DiskBytes}),
	})
	for _, d := range w.Store.ListDomains(acc.ID) {
		_ = w.ensureDomainStack(&d, acc)
	}
	switch acc.Status {
	case "terminating":
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "DeleteLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
		acc.Status = "terminated"
	case "suspended":
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "LockLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
	default:
		acc.Status = "active"
	}
	acc.ObservedRevision = acc.DesiredRevision
	w.Store.PutAccount(acc)
	j.Progress = 90
	w.Store.UpdateJob(j)
	return nil
}

func (w *Worker) ensureDomainStack(d *store.Domain, acc *store.Account) error {
	d.Status = "active"
	w.Store.PutDomain(d)
	if w.Store.ZoneByDomain(d.ID) == nil {
		z := &store.DNSZone{ID: store.NewID(), AccountID: acc.ID, DomainID: d.ID, Name: d.ASCII, Provider: "powerdns", DesiredRevision: 1, ObservedRevision: 1}
		w.Store.PutZone(z)
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "A", Content: "203.0.113.10", TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "MX", Content: "mail." + d.ASCII, TTL: 3600, Priority: intPtr(10)})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "TXT", Content: "v=spf1 a mx ip4:203.0.113.10 ~all", TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "_dmarc", Type: "TXT", Content: "v=DMARC1; p=none", TTL: 3600})
	}
	site := findSite(w.Store, d.ID)
	if site == nil {
		site = &store.Website{ID: store.NewID(), AccountID: acc.ID, DomainID: d.ID, Runtime: "php", RuntimeVersion: "8.5", DocumentRoot: d.DocumentRoot, HTTPSRedirect: true, Enabled: acc.Status != "suspended", DesiredRevision: 1}
		w.Store.PutWebsite(site)
	}
	_, err := w.Agent.ApplyWebsite(site.ID, d.ASCII, site.DocumentRoot, site.Runtime)
	if err != nil {
		return err
	}
	if site.Runtime == "php" {
		ver := site.RuntimeVersion
		if ver == "" {
			ver = "8.3"
		}
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "ApplyPhpPool",
			Params: mustJSON(map[string]any{"account": acc.Username, "version": ver, "max_children": 8}),
		})
	}
	site.ObservedRevision = site.DesiredRevision
	if acc.Status == "suspended" {
		site.Enabled = false
	} else {
		site.Enabled = true
	}
	w.Store.PutWebsite(site)
	if w.Store.MailDomainByDomain(d.ID) == nil {
		md := &store.MailDomain{ID: store.NewID(), AccountID: acc.ID, DomainID: d.ID, CatchallPolicy: "reject", Status: "active"}
		w.Store.PutMailDomain(md)
	}
	if len(w.Store.ListCerts(acc.ID)) == 0 {
		exp := time.Now().Add(90 * 24 * time.Hour)
		w.Store.PutCert(&store.Certificate{ID: store.NewID(), AccountID: acc.ID, Hostname: d.ASCII, Kind: "domain", Status: "active", NotAfter: &exp, Issuer: "dev-acme"})
	}
	return nil
}

func (w *Worker) provisionDomain(j *store.Job) error {
	d := w.Store.GetDomain(str(j.Payload["domain_id"]))
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if d == nil || acc == nil {
		return fmt.Errorf("missing domain or account")
	}
	return w.ensureDomainStack(d, acc)
}

func (w *Worker) provisionWebsite(j *store.Job) error {
	site := w.Store.GetWebsite(str(j.Payload["website_id"]))
	if site == nil {
		return fmt.Errorf("website missing")
	}
	d := w.Store.GetDomain(site.DomainID)
	_, err := w.Agent.ApplyWebsite(site.ID, d.ASCII, site.DocumentRoot, site.Runtime)
	if err != nil {
		return err
	}
	site.ObservedRevision = site.DesiredRevision
	w.Store.PutWebsite(site)
	return nil
}

func (w *Worker) deployApp(j *store.Job) error {
	app := w.Store.GetApp(str(j.Payload["application_id"]))
	if app == nil {
		return fmt.Errorf("application missing")
	}
	app.Status = "running"
	w.Store.PutApp(app)
	return nil
}

func (w *Worker) provisionDB(j *store.Job) error {
	d := w.Store.GetDB(str(j.Payload["database_id"]))
	if d == nil {
		return fmt.Errorf("database missing")
	}
	acc := w.Store.GetAccount(d.AccountID)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	pw := fmt.Sprintf("db-%s", store.NewID())
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{"engine": d.Engine, "name": d.Name, "username": acc.Username + "_u", "password": pw}),
	})
	if err != nil {
		return err
	}
	d.Status = "active"
	w.Store.PutDB(d)
	return nil
}

func (w *Worker) syncDNS(j *store.Job) error {
	z := w.Store.GetZone(str(j.Payload["zone_id"]))
	if z == nil {
		return nil
	}
	p := &dns.PowerDNS{}
	if err := p.CreateZone(context.Background(), z.Name); err != nil {
		return err
	}
	for _, rec := range w.Store.ListRecords(z.ID) {
		if err := p.UpsertRecord(context.Background(), z.Name, dns.Record{Name: rec.Name, Type: rec.Type, Content: rec.Content, TTL: rec.TTL}); err != nil {
			return err
		}
	}
	z.ObservedRevision = z.DesiredRevision
	w.Store.PutZone(z)
	return nil
}

func (w *Worker) provisionMailbox(j *store.Job) error {
	mb := w.Store.GetMailbox(str(j.Payload["mailbox_id"]))
	if mb == nil {
		return fmt.Errorf("mailbox missing")
	}
	mb.Status = "active"
	w.Store.PutMailbox(mb)
	return nil
}

func (w *Worker) provisionCert(j *store.Job) error {
	c := w.Store.GetCert(str(j.Payload["certificate_id"]))
	if c == nil {
		return fmt.Errorf("certificate missing")
	}
	exp := time.Now().Add(90 * 24 * time.Hour)
	c.Status = "active"
	c.NotAfter = &exp
	c.Issuer = "Let's Encrypt (dev)"
	w.Store.PutCert(c)
	return nil
}

func (w *Worker) createBackup(j *store.Job) error {
	b := w.Store.GetBackup(str(j.Payload["backup_id"]))
	if b == nil {
		return fmt.Errorf("backup missing")
	}
	acc := w.Store.GetAccount(b.AccountID)
	now := time.Now().UTC()
	b.State = "succeeded"
	b.FinishedAt = &now
	b.Checksum = "sha256:dev"
	b.Manifest = map[string]any{
		"format_version": 1,
		"account_id":     acc.ID,
		"created_at":     now.Format(time.RFC3339),
		"panel_version":  "0.1.0",
		"files":          map[string]any{"home": acc.HomePath},
		"databases":      w.Store.ListDBs(acc.ID),
		"mailboxes":      w.Store.ListMailboxes(acc.ID),
		"checksums":      map[string]any{"manifest": "sha256:dev"},
	}
	w.Store.PutBackup(b)
	return nil
}

func (w *Worker) restoreBackup(j *store.Job) error {
	b := w.Store.GetBackup(str(j.Payload["backup_id"]))
	if b == nil {
		return fmt.Errorf("backup missing")
	}
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	acc.Status = "active"
	acc.DesiredRevision++
	acc.ObservedRevision = acc.DesiredRevision
	w.Store.PutAccount(acc)
	return nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func intPtr(v int) *int { return &v }

func findSite(st store.Store, domainID string) *store.Website {
	for _, w := range st.ListWebsites("") {
		if w.DomainID == domainID {
			cp := w
			return &cp
		}
	}
	return nil
}
