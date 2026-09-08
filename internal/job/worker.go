package job

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/acme"
	"github.com/hosting-panel/panel/internal/backup"
	"github.com/hosting-panel/panel/internal/dns"
	"github.com/hosting-panel/panel/internal/mail"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

type Worker struct {
	Store store.Store
	Agent *operations.Host
	Log   *logging.Logger
	Box   *secret.Box
	Name  string
	stop  chan struct{}
}

func New(st store.Store, agent *operations.Host, log *logging.Logger, box *secret.Box, name string) *Worker {
	return &Worker{Store: st, Agent: agent, Log: log, Box: box, Name: name, stop: make(chan struct{})}
}

func (w *Worker) Drain(ctx context.Context) {
	for {
		j := w.Store.ClaimJob(w.Name)
		if j == nil {
			return
		}
		w.execute(ctx, j)
	}
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
		if j.Attempts >= j.MaxAttempts || gone(err) {
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
	case "cron.apply":
		return w.applyCron(j)
	case "account.copy_homedir":
		return w.copyHomedir(j)
	default:
		return fmt.Errorf("unknown job type %s", j.Type)
	}
}

func (w *Worker) jobAccount(j *store.Job) *store.Account {
	if acc := w.Store.GetAccount(str(j.Payload["account_id"])); acc != nil {
		return acc
	}
	if j.ResourceType == "account" {
		return w.Store.GetAccount(j.ResourceID)
	}
	return nil
}

func (w *Worker) provisionAccount(j *store.Job) error {
	acc := w.jobAccount(j)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	if acc.Status == "terminating" {
		return w.retireAccount(acc, j)
	}
	pkg := w.Store.GetPackage(acc.PackageID)
	if pkg == nil {
		pkg = &store.Package{CPUPercent: 100, MemoryBytes: 512 << 20, DiskBytes: 1 << 30, ProcessLimit: 100}
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateLinuxUser",
		Params: mustJSON(map[string]any{"username": acc.Username, "uid": acc.LinuxUID, "gid": acc.LinuxGID, "home": acc.HomePath, "shell": "/usr/sbin/nologin"}),
	})
	if err != nil {
		acc.Status = "failed"
		w.Store.PutAccount(acc)
		return err
	}
	if pw := str(j.Payload["linux_password"]); pw != "" {
		_, err = w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "SetLinuxPassword",
			Params: mustJSON(map[string]any{"username": acc.Username, "password": pw}),
		})
		delete(j.Payload, "linux_password")
		w.Store.UpdateJob(j)
		if err != nil {
			return err
		}
	}
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplySystemdSlice",
		Params: mustJSON(map[string]any{"username": acc.Username, "cpu_percent": pkg.CPUPercent, "memory_bytes": pkg.MemoryBytes}),
	})
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetFilesystemQuota",
		Params: mustJSON(map[string]any{"username": acc.Username, "bytes": pkg.DiskBytes}),
	})
	pubIP := publicIPv4()
	for _, d := range w.Store.ListDomains(acc.ID) {
		_ = w.ensureDomainStack(&d, acc, pubIP, "")
	}
	switch acc.Status {
	case "suspended":
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "LockLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "FreezeAccount", Params: mustJSON(map[string]any{"username": acc.Username, "freeze": true})})
	default:
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "UnlockLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "FreezeAccount", Params: mustJSON(map[string]any{"username": acc.Username, "freeze": false})})
		for _, site := range w.Store.ListWebsites(acc.ID) {
			s := site
			_ = w.applySiteRuntime(&s, acc)
		}
		acc.Status = "active"
	}
	acc.ObservedRevision = acc.DesiredRevision
	w.Store.PutAccount(acc)
	w.recordUsage(acc)
	j.Progress = 90
	w.Store.UpdateJob(j)
	return nil
}

func (w *Worker) retireAccount(acc *store.Account, j *store.Job) error {
	for _, db := range w.Store.ListDBs(acc.ID) {
		d := db
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "DropHostedDatabase",
			Params: mustJSON(map[string]any{
				"engine": d.Engine, "name": d.Name, "username": acc.Username + "_u",
			}),
		})
		if err != nil {
			return err
		}
		d.Status = "terminated"
		w.Store.PutDB(&d)
	}
	var ids []string
	for _, site := range w.Store.ListWebsites(acc.ID) {
		ids = append(ids, site.ID)
	}
	var domains []string
	for _, d := range w.Store.ListDomains(acc.ID) {
		domains = append(domains, d.ASCII)
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "RetireAccount",
		Params: mustJSON(map[string]any{
			"username":    acc.Username,
			"website_ids": ids,
			"domains":     domains,
		}),
	})
	if err != nil {
		return err
	}
	acc.Status = "terminated"
	acc.ObservedRevision = acc.DesiredRevision
	w.Store.PutAccount(acc)
	_ = w.applyMailStack(acc.ID)
	j.Progress = 90
	w.Store.UpdateJob(j)
	return nil
}

func (w *Worker) ensureDomainStack(d *store.Domain, acc *store.Account, pubIP, runtime string) error {
	if pubIP == "" {
		pubIP = publicIPv4()
	}
	d.Status = "active"
	w.Store.PutDomain(d)
	if w.Store.ZoneByDomain(d.ID) == nil {
		z := &store.DNSZone{ID: store.NewID(), AccountID: acc.ID, DomainID: d.ID, Name: d.ASCII, Provider: "powerdns", DesiredRevision: 1, ObservedRevision: 1}
		w.Store.PutZone(z)
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "A", Content: pubIP, TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "ns1", Type: "A", Content: pubIP, TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "mail", Type: "A", Content: pubIP, TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "MX", Content: "mail." + d.ASCII, TTL: 3600, Priority: intPtr(10)})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "@", Type: "TXT", Content: "v=spf1 a mx ip4:" + pubIP + " ~all", TTL: 3600})
		w.Store.PutRecord(&store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: "_dmarc", Type: "TXT", Content: "v=DMARC1; p=none", TTL: 3600})
	}
	if len(w.Store.ListCerts(acc.ID)) == 0 {
		exp := time.Now().Add(90 * 24 * time.Hour)
		c := &store.Certificate{ID: store.NewID(), AccountID: acc.ID, Hostname: d.ASCII, Kind: "domain", Status: "active", NotAfter: &exp, Issuer: "panel-dev"}
		w.Store.PutCert(c)
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "IssueDevCertificate",
			Params: mustJSON(map[string]any{"hostname": d.ASCII, "days": 90}),
		})
	}
	site := findSite(w.Store, d.ID)
	if runtime != "" && site != nil {
		site.Runtime = runtime
	}
	if site == nil {
		if runtime == "" {
			runtime = "php"
		}
		ver := ""
		if runtime == "php" {
			ver = "8.3"
		}
		site = &store.Website{ID: store.NewID(), AccountID: acc.ID, DomainID: d.ID, Runtime: runtime, RuntimeVersion: ver, DocumentRoot: d.DocumentRoot, HTTPSRedirect: false, Enabled: acc.Status != "suspended", DesiredRevision: 1}
		w.Store.PutWebsite(site)
	}
	enabled := acc.Status != "suspended" && acc.Status != "terminating"
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyWebsite",
		Params: mustJSON(map[string]any{"website_id": site.ID, "account": acc.Username, "domain": d.ASCII, "document_root": site.DocumentRoot, "runtime": site.Runtime, "https_redirect": site.HTTPSRedirect, "enabled": enabled}),
	})
	if err != nil {
		return err
	}
	if err := w.applySiteRuntime(site, acc); err != nil {
		return err
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
		if len(w.Store.ListMailboxes(acc.ID)) == 0 {
			w.Store.PutMailbox(&store.Mailbox{
				ID: store.NewID(), AccountID: acc.ID, DomainID: md.ID,
				LocalPart: "postmaster", QuotaBytes: 1 << 30, PasswordHash: "!", Status: "active",
			})
		}
	}
	_ = w.applyMailStack(acc.ID)
	if z := w.Store.ZoneByDomain(d.ID); z != nil {
		_ = w.writeZone(z)
	}
	return nil
}

func (w *Worker) provisionDomain(j *store.Job) error {
	d := w.Store.GetDomain(str(j.Payload["domain_id"]))
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if d == nil || acc == nil {
		return fmt.Errorf("missing domain or account")
	}
	return w.ensureDomainStack(d, acc, publicIPv4(), str(j.Payload["runtime"]))
}

func (w *Worker) provisionWebsite(j *store.Job) error {
	site := w.Store.GetWebsite(str(j.Payload["website_id"]))
	if site == nil {
		return fmt.Errorf("website missing")
	}
	d := w.Store.GetDomain(site.DomainID)
	acc := w.Store.GetAccount(site.AccountID)
	if d == nil || acc == nil {
		return fmt.Errorf("website missing domain or account")
	}
	account := acc.Username
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyWebsite",
		Params: mustJSON(map[string]any{"website_id": site.ID, "account": account, "domain": d.ASCII, "document_root": site.DocumentRoot, "runtime": site.Runtime, "https_redirect": site.HTTPSRedirect, "enabled": acc.Status != "suspended"}),
	})
	if err != nil {
		return err
	}
	site.ObservedRevision = site.DesiredRevision
	w.Store.PutWebsite(site)
	return w.applySiteRuntime(site, acc)
}

func (w *Worker) applySiteRuntime(site *store.Website, acc *store.Account) error {
	switch site.Runtime {
	case "php":
		ver := site.RuntimeVersion
		if ver == "" || ver == "8.5" {
			ver = "8.3"
		}
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "ApplyPhpPool",
			Params: mustJSON(map[string]any{"account": acc.Username, "version": ver, "max_children": 8}),
		})
		return err
	case "node", "python":
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "ApplyAppUnit",
			Params: mustJSON(map[string]any{
				"website_id": site.ID, "account": acc.Username, "runtime": site.Runtime,
				"working_directory": "/home/" + acc.Username + "/apps/" + site.ID,
			}),
		})
		return err
	default:
		return nil
	}
}

func (w *Worker) deployApp(j *store.Job) error {
	app := w.Store.GetApp(str(j.Payload["application_id"]))
	if app == nil {
		return fmt.Errorf("application missing")
	}
	acc := w.Store.GetAccount(app.AccountID)
	account := ""
	if acc != nil {
		account = acc.Username
	}
	wid := app.WebsiteID
	if wid == "" {
		wid = app.ID
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyAppUnit",
		Params: mustJSON(map[string]any{
			"website_id": wid, "account": account, "runtime": app.Runtime,
			"working_directory": app.WorkingDirectory, "start_command": app.StartCommand,
		}),
	})
	if err != nil {
		return err
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
	dbUser := acc.Username + "_u"
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{"engine": d.Engine, "name": d.Name, "username": dbUser, "password": pw}),
	})
	if err != nil {
		return err
	}
	note := fmt.Sprintf("engine=%s\nname=%s\nusername=%s\npassword=%s\nhost=127.0.0.1\n", d.Engine, d.Name, dbUser, pw)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database."+d.Engine+"."+d.Name, []byte(note), 0o600)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database."+d.Engine, []byte(note), 0o600)
	d.Status = "active"
	w.Store.PutDB(d)
	return nil
}

func (w *Worker) syncDNS(j *store.Job) error {
	z := w.Store.GetZone(str(j.Payload["zone_id"]))
	if z == nil {
		return nil
	}
	if err := w.writeZone(z); err != nil {
		return err
	}
	p := &dns.PowerDNS{BaseURL: os.Getenv("PANEL_PDNS_URL"), APIKey: os.Getenv("PANEL_PDNS_API_KEY")}
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

func (w *Worker) writeZone(z *store.DNSZone) error {
	body := dns.ZoneFile(*z, w.Store.ListRecords(z.ID), z.DesiredRevision)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyDNSZone",
		Params: mustJSON(map[string]any{"name": z.Name, "body": body}),
	})
	return err
}

func (w *Worker) applyMailStack(accountID string) error {
	acc := w.Store.GetAccount(accountID)
	if acc == nil || (acc.Status != "terminating" && acc.Status != "terminated") {
		local := mail.Recipients(w.Store, accountID)
		for _, r := range local {
			uid, gid := 20000, 20000
			if acc != nil {
				uid, gid = acc.LinuxUID, acc.LinuxGID
			}
			if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "CreateMailboxHome",
				Params: mustJSON(map[string]any{"domain": r.Domain, "local_part": r.LocalPart, "uid": uid, "gid": gid}),
			}); err != nil {
				return err
			}
		}
	}
	recs := mail.RecipientsForHost(w.Store)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyMailMaps",
		Params: mustJSON(map[string]any{
			"virtual": mail.Virtual(recs),
			"domains": mail.Domains(recs),
			"passwd":  mail.PasswdFile(recs),
			"uids":    mail.UIDMap(recs),
			"gids":    mail.GIDMap(recs),
		}),
	})
	return err
}

func (w *Worker) provisionMailbox(j *store.Job) error {
	mb := w.Store.GetMailbox(str(j.Payload["mailbox_id"]))
	if mb == nil {
		return fmt.Errorf("mailbox missing")
	}
	if err := w.applyMailStack(mb.AccountID); err != nil {
		return err
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
	contact := "admin@localhost"
	if acc := w.Store.GetAccount(c.AccountID); acc != nil {
		if owner := w.Store.UserByID(acc.OwnerUserID); owner != nil && owner.Email != "" {
			contact = owner.Email
		}
	}
	if err := acme.Issue(context.Background(), w.Agent, c.Hostname, contact, acme.Directory()); err != nil {
		return err
	}
	exp := time.Now().Add(90 * 24 * time.Hour)
	c.Status = "active"
	c.NotAfter = &exp
	c.Issuer = acme.IssuerName(acme.Directory())
	w.Store.PutCert(c)
	// Nginx only picks up a newly written certificate after ApplyWebsite.
	if acc := w.Store.GetAccount(c.AccountID); acc != nil {
		for _, site := range w.Store.ListWebsites(acc.ID) {
			d := w.Store.GetDomain(site.DomainID)
			if d == nil || d.ASCII != c.Hostname {
				continue
			}
			_, err := w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "ApplyWebsite",
				Params: mustJSON(map[string]any{
					"website_id": site.ID, "account": acc.Username, "domain": d.ASCII,
					"document_root": site.DocumentRoot, "runtime": site.Runtime,
					"https_redirect": site.HTTPSRedirect, "enabled": acc.Status != "suspended",
				}),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Worker) applyCron(j *store.Job) error {
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	var body strings.Builder
	body.WriteString("# panel crontab — generated, do not edit\n")
	for _, c := range w.Store.ListCrons(acc.ID) {
		if !c.Enabled {
			continue
		}
		fmt.Fprintf(&body, "%s %s\n", c.Schedule, c.Command)
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyAccountCron",
		Params: mustJSON(map[string]any{"username": acc.Username, "body": body.String()}),
	})
	return err
}

func (w *Worker) copyHomedir(j *store.Job) error {
	acc := w.jobAccount(j)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	src := str(j.Payload["source"])
	dest := str(j.Payload["dest"])
	user := str(j.Payload["username"])
	if user == "" {
		user = acc.Username
	}
	if dest == "" {
		dest = acc.HomePath
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CopyHomedir",
		Params: mustJSON(map[string]any{"username": user, "source": src, "dest": dest}),
	})
	return err
}

func (w *Worker) createBackup(j *store.Job) error {
	b := w.Store.GetBackup(str(j.Payload["backup_id"]))
	if b == nil {
		return fmt.Errorf("backup missing")
	}
	acc := w.Store.GetAccount(b.AccountID)
	if acc == nil || w.Box == nil {
		return fmt.Errorf("backup prerequisites missing")
	}
	home := acc.HomePath
	localRoot := "/var/lib/panel/backups"
	if w.Agent != nil && w.Agent.Sock == "" && w.Agent.Root != "" {
		home = filepath.Join(w.Agent.Root, strings.TrimPrefix(acc.HomePath, "/"))
		localRoot = filepath.Join(w.Agent.Root, "var/lib/panel/backups")
	}
	if w.Agent != nil && w.Agent.Sock != "" {
		staging := "/var/lib/panel/backups/staging/" + b.ID + ".tar.gz"
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "PackDirectory",
			Params: mustJSON(map[string]any{"source": acc.HomePath, "dest": staging}),
		}); err != nil {
			return err
		}
		raw, err := os.ReadFile(staging)
		if err != nil {
			return err
		}
		repo, err := backup.Open(b.Destination, localRoot)
		if err != nil {
			return err
		}
		man, key, err := backup.BuildArchive(context.Background(), w.Box, repo, acc, w.Store.ListDBs(acc.ID), w.Store.ListMailboxes(acc.ID), raw)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		b.State = "succeeded"
		b.FinishedAt = &now
		b.Checksum = man.Checksums["files.tar.gz"]
		b.Manifest = map[string]any{"format_version": man.FormatVersion, "key": key, "account_id": man.AccountID, "checksums": man.Checksums}
		w.Store.PutBackup(b)
		return nil
	}
	repo, err := backup.Open(b.Destination, localRoot)
	if err != nil {
		return err
	}
	man, key, err := backup.Build(context.Background(), w.Box, repo, acc, w.Store.ListDBs(acc.ID), w.Store.ListMailboxes(acc.ID), home)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	b.State = "succeeded"
	b.FinishedAt = &now
	b.Checksum = man.Checksums["files.tar.gz"]
	b.Manifest = map[string]any{"format_version": man.FormatVersion, "key": key, "account_id": man.AccountID, "checksums": man.Checksums}
	w.Store.PutBackup(b)
	return nil
}

func (w *Worker) restoreBackup(j *store.Job) error {
	b := w.Store.GetBackup(str(j.Payload["backup_id"]))
	if b == nil {
		return fmt.Errorf("backup missing")
	}
	acc := w.Store.GetAccount(str(j.Payload["account_id"]))
	if acc == nil || w.Box == nil {
		return fmt.Errorf("restore prerequisites missing")
	}
	key, _ := b.Manifest["key"].(string)
	if key == "" {
		return fmt.Errorf("backup object key missing")
	}
	home := acc.HomePath
	repoRoot := "/var/lib/panel/backups"
	if w.Agent != nil && w.Agent.Sock == "" && w.Agent.Root != "" {
		home = filepath.Join(w.Agent.Root, strings.TrimPrefix(acc.HomePath, "/"))
		repoRoot = filepath.Join(w.Agent.Root, "var/lib/panel/backups")
	}
	dest := b.Destination
	if dest == "" {
		dest = "local"
	}
	repo, err := backup.Open(dest, repoRoot)
	if err != nil {
		return err
	}
	if w.Agent != nil && w.Agent.Sock != "" {
		man, raw, err := backup.OpenArchive(context.Background(), w.Box, repo, key)
		if err != nil {
			return err
		}
		if err := backup.Preflight(man, acc); err != nil {
			return err
		}
		staging := "/var/lib/panel/backups/staging/restore-" + b.ID + ".tar.gz"
		if _, err := w.Agent.ApplyFile(staging, raw, 0o640); err != nil {
			return err
		}
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "UnpackDirectory",
			Params: mustJSON(map[string]any{"archive": staging, "dest": acc.HomePath}),
		}); err != nil {
			return err
		}
		acc.Status = "active"
		acc.DesiredRevision++
		acc.ObservedRevision = acc.DesiredRevision
		w.Store.PutAccount(acc)
		return nil
	}
	man, err := backup.Restore(context.Background(), w.Box, repo, key, home)
	if err != nil {
		return err
	}
	if err := backup.Preflight(man, acc); err != nil {
		return err
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

func (w *Worker) recordUsage(acc *store.Account) {
	if acc == nil {
		return
	}
	now := time.Now().UTC()
	u := &store.Usage{AccountID: acc.ID, CollectedAt: now}
	if w.Agent != nil {
		raw, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "MeasureAccountUsage",
			Params: mustJSON(map[string]any{"username": acc.Username, "home": acc.HomePath}),
		})
		if err == nil {
			b, _ := json.Marshal(raw)
			var got operations.AccountUsage
			if json.Unmarshal(b, &got) == nil {
				u.DiskBytes = got.DiskBytes
				u.InodeCount = got.InodeCount
				u.ProcessCount = int(got.ProcessCount)
				u.MemoryBytes = got.MemoryBytes
				w.Store.PutUsage(u)
				_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
					Method: "EnforceAccountDisk",
					Params: mustJSON(map[string]any{"username": acc.Username, "home": acc.HomePath}),
				})
				return
			}
		}
	}
	home := acc.HomePath
	if w.Agent != nil && w.Agent.Root != "" {
		home = filepath.Join(w.Agent.Root, strings.TrimPrefix(acc.HomePath, "/"))
	}
	_ = filepath.Walk(home, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		u.InodeCount++
		if info.Mode().IsRegular() {
			u.DiskBytes += info.Size()
		}
		return nil
	})
	w.Store.PutUsage(u)
}

func publicIPv4() string {
	if v := os.Getenv("PANEL_PUBLIC_IPV4"); v != "" {
		return v
	}
	return "127.0.0.1"
}

func gone(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "missing")
}

func findSite(st store.Store, domainID string) *store.Website {
	var fallback *store.Website
	for _, w := range st.ListWebsites("") {
		if w.DomainID != domainID {
			continue
		}
		cp := w
		if w.Runtime == "node" || w.Runtime == "python" {
			return &cp
		}
		if fallback == nil {
			fallback = &cp
		}
	}
	return fallback
}
