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
	"github.com/hosting-panel/panel/internal/netaddr"
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
	w.scanCertRenewals()
	for _, acc := range w.Store.DriftedAccounts() {
		_, _ = w.Store.EnqueueJob(&store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
			Payload: map[string]any{"account_id": acc.ID}, State: "queued",
			IdempotencyKey: fmt.Sprintf("reconcile:%s:%d", acc.ID, acc.DesiredRevision),
		})
	}
}

func (w *Worker) scanCertRenewals() {
	cutoff := time.Now().Add(30 * 24 * time.Hour)
	for _, c := range w.Store.ListCerts("") {
		if c.Status != "active" || c.NotAfter == nil || !c.NotAfter.Before(cutoff) {
			continue
		}
		cp := c
		cp.Status = "renewing"
		w.Store.PutCert(&cp)
		_, _ = w.Store.EnqueueJob(&store.Job{
			Type: "certificate.provision", ResourceType: "certificate", ResourceID: c.ID,
			Payload: map[string]any{"certificate_id": c.ID}, State: "queued",
			IdempotencyKey: fmt.Sprintf("cert-renew:%s:%s", c.ID, c.NotAfter.UTC().Format("20060102")),
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
	case "domain.delete":
		return w.deleteDomain(j)
	case "website.provision":
		return w.provisionWebsite(j)
	case "website.delete":
		return w.deleteWebsite(j)
	case "application.deploy":
		return w.deployApp(j)
	case "wordpress.install":
		return w.installWordPress(j)
	case "database.provision":
		return w.provisionDB(j)
	case "database.delete":
		return w.deleteDB(j)
	case "dns.sync":
		return w.syncDNS(j)
	case "mailbox.provision":
		return w.provisionMailbox(j)
	case "mailbox.delete", "mail.alias":
		return w.applyMailStack(str(j.Payload["account_id"]))
	case "mail.maps":
		return w.applyMailStack(str(j.Payload["account_id"]))
	case "dns.dnssec":
		return w.applyZoneDNSSEC(j)
	case "certificate.provision":
		return w.provisionCert(j)
	case "backup.create":
		return w.createBackup(j)
	case "backup.restore":
		return w.restoreBackup(j)
	case "cron.apply":
		return w.applyCron(j)
	case "ftp.apply":
		return w.syncFTPUsers()
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
		Params: mustJSON(map[string]any{
			"username": acc.Username, "cpu_percent": pkg.CPUPercent, "memory_bytes": pkg.MemoryBytes,
			"tasks_max": pkg.ProcessLimit, "io_weight": pkg.IOWeight, "iops": pkg.IOPS,
		}),
	})
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetFilesystemQuota",
		Params: mustJSON(map[string]any{"username": acc.Username, "bytes": pkg.DiskBytes}),
	})
	pubIP := publicIPv4()
	for _, d := range w.Store.ListDomains(acc.ID) {
		_ = w.ensureDomainStack(&d, acc, pubIP, "")
	}
	if latest := w.Store.GetAccount(acc.ID); latest != nil {
		acc.Status = latest.Status
		acc.DesiredRevision = latest.DesiredRevision
		acc.PackageID = latest.PackageID
	}
	if acc.Status == "terminating" || acc.Status == "terminated" {
		return w.retireAccount(acc, j)
	}
	switch acc.Status {
	case "suspended":
		w.applySuspendedHost(acc)
	default:
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "UnlockLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
		_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "FreezeAccount", Params: mustJSON(map[string]any{"username": acc.Username, "freeze": false})})
		for _, site := range w.Store.ListWebsites(acc.ID) {
			s := site
			_ = w.applySiteRuntime(&s, acc)
		}
		if acc.Status == "provisioning" || acc.Status == "failed" || acc.Status == "" {
			acc.Status = "active"
		}
		w.reapplyAccountWebsites(acc)
	}
	if latest := w.Store.GetAccount(acc.ID); latest != nil {
		switch latest.Status {
		case "suspended":
			acc.Status = "suspended"
			acc.DesiredRevision = latest.DesiredRevision
			acc.PackageID = latest.PackageID
			w.applySuspendedHost(acc)
		case "terminating", "terminated":
			return w.retireAccount(latest, j)
		}
	}
	acc.ObservedRevision = acc.DesiredRevision
	w.Store.PutAccount(acc)
	if err := w.applyMigratedData(j); err != nil {
		return err
	}
	w.recordUsage(acc)
	_ = w.syncFTPUsers()
	_ = w.applyCron(j)
	_ = w.applyMailStack(acc.ID)
	w.syncWordPressDatabase(acc)
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
				"engine": d.Engine, "name": d.Name, "username": acc.Username + "_u", "drop_user": true,
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
	_ = w.syncFTPUsers()
	j.Progress = 90
	w.Store.UpdateJob(j)
	return nil
}

func (w *Worker) ensureDomainStack(d *store.Domain, acc *store.Account, pubIP, runtime string) error {
	if pubIP == "" {
		pubIP = publicIPv4()
	}
	if d.Type == "alias" {
		d.DocumentRoot = acc.HomePath + "/public_html"
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
	} else {
		w.republishPublicAddresses(w.Store.ZoneByDomain(d.ID), pubIP)
	}
	if z := w.Store.ZoneByDomain(d.ID); z != nil {
		if err := w.writeZone(z); err != nil && w.liveACME() {
			return fmt.Errorf("publish zone for ACME: %w", err)
		}
	}
	if d.Type != "alias" {
		if err := w.ensureCertificate(acc, d.ASCII); err != nil {
			return err
		}
	}
	if d.Type == "alias" {
		if primary := w.primaryDomain(acc); primary != nil {
			if site := findSite(w.Store, primary.ID); site != nil {
				if err := w.applyWebsiteDispatch(acc, site, primary); err != nil {
					return err
				}
			}
			if err := w.renewCertificate(acc, primary.ASCII); err != nil {
				return err
			}
		}
	} else {
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
		if err := w.applyWebsiteDispatch(acc, site, d); err != nil {
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
		w.collapseDomainWebsites(acc, d, site)
	}
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

func (w *Worker) deleteDomain(j *store.Job) error {
	d := w.Store.GetDomain(str(j.Payload["domain_id"]))
	if d == nil {
		return nil
	}
	if d.Type == "primary" {
		return fmt.Errorf("primary domain cannot be deleted")
	}
	acc := w.Store.GetAccount(d.AccountID)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	var websiteIDs []string
	for _, site := range w.Store.ListWebsites(acc.ID) {
		if site.DomainID == d.ID {
			websiteIDs = append(websiteIDs, site.ID)
		}
	}
	if w.Agent != nil {
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "RetireDomain",
			Params: mustJSON(map[string]any{
				"account": acc.Username, "domain": d.ASCII, "website_ids": websiteIDs,
			}),
		})
		if err != nil {
			return err
		}
	}
	w.Store.DeleteDomain(d.ID)
	if d.Type == "alias" {
		if primary := w.primaryDomain(acc); primary != nil {
			if site := findSite(w.Store, primary.ID); site != nil {
				_ = w.applyWebsiteDispatch(acc, site, primary)
			}
		}
	}
	_ = w.applyMailStack(acc.ID)
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

func (w *Worker) deleteWebsite(j *store.Job) error {
	site := w.Store.GetWebsite(str(j.Payload["website_id"]))
	if site == nil {
		return nil
	}
	acc := w.Store.GetAccount(site.AccountID)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	if d := w.Store.GetDomain(site.DomainID); d != nil && d.Type == "primary" {
		return fmt.Errorf("primary domain website cannot be deleted")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "RetireWebsite",
		Params: mustJSON(map[string]any{"website_id": site.ID, "account": acc.Username}),
	})
	if err != nil {
		return err
	}
	w.Store.DeleteWebsite(site.ID)
	return nil
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
	if err := w.applyWebsiteDispatch(acc, site, d); err != nil {
		return err
	}
	site.ObservedRevision = site.DesiredRevision
	w.Store.PutWebsite(site)
	w.collapseDomainWebsites(acc, d, site)
	return w.applySiteRuntime(site, acc)
}

func (w *Worker) applySiteRuntime(site *store.Website, acc *store.Account) error {
	switch site.Runtime {
	case "php":
		ver := site.RuntimeVersion
		if ver == "" || ver == "8.5" {
			ver = "8.3"
		}
		children := w.concurrentWebRequests(acc)
		if children < 1 {
			children = 8
		}
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "ApplyPhpPool",
			Params: mustJSON(map[string]any{"account": acc.Username, "version": ver, "max_children": children}),
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

func (w *Worker) installWordPress(j *store.Job) error {
	app := w.Store.GetApp(str(j.Payload["application_id"]))
	if app == nil {
		return fmt.Errorf("application missing")
	}
	acc := w.Store.GetAccount(app.AccountID)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	site := w.Store.GetWebsite(app.WebsiteID)
	if site == nil {
		for _, s := range w.Store.ListWebsites(acc.ID) {
			if s.Runtime == "php" || s.Runtime == "" {
				cp := s
				site = &cp
				break
			}
		}
	}
	if site == nil {
		return fmt.Errorf("website missing")
	}
	doc := site.DocumentRoot
	if doc == "" {
		doc = acc.HomePath + "/public_html"
	}
	dbName := acc.Username + "_wp"
	dbUser, pw, reset := w.hostedDBCredentials(acc, "mariadb")
	if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{
			"engine": "mariadb", "name": dbName, "username": dbUser, "password": pw, "reset_password": reset,
		}),
	}); err != nil {
		return err
	}
	found := false
	for _, d := range w.Store.ListDBs(acc.ID) {
		if d.Name == dbName {
			found = true
			break
		}
	}
	if !found {
		w.Store.PutDB(&store.HostedDatabase{
			ID: store.NewID(), AccountID: acc.ID, Engine: "mariadb", Name: dbName, Status: "active",
		})
	}
	note := fmt.Sprintf("engine=mariadb\nname=%s\nusername=%s\npassword=%s\nhost=127.0.0.1\n", dbName, dbUser, pw)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database.mariadb."+dbName, []byte(note), 0o600)
	host := str(j.Payload["hostname"])
	if host == "" {
		if d := w.Store.GetDomain(site.DomainID); d != nil {
			host = d.ASCII
		}
	}
	title := str(j.Payload["title"])
	if title == "" {
		title = host
	}
	if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "InstallWordPress",
		Params: mustJSON(map[string]any{
			"username":       acc.Username,
			"document_root":  doc,
			"db_name":        dbName,
			"db_user":        dbUser,
			"db_password":    pw,
			"db_host":        "127.0.0.1",
			"site_url":       "http://" + host,
			"title":          title,
			"admin_user":     str(j.Payload["admin_user"]),
			"admin_password": str(j.Payload["admin_password"]),
			"admin_email":    str(j.Payload["admin_email"]),
		}),
	}); err != nil {
		return err
	}
	if site.Runtime != "php" {
		site.Runtime = "php"
		w.Store.PutWebsite(site)
		_ = w.applySiteRuntime(site, acc)
	}
	if d := w.Store.GetDomain(site.DomainID); d != nil {
		_ = w.applyWebsiteDispatch(acc, site, d)
	}
	app.Status = "running"
	app.Runtime = "wordpress"
	app.WorkingDirectory = doc
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
	dbUser, pw, reset := w.hostedDBCredentials(acc, d.Engine)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{
			"engine": d.Engine, "name": d.Name, "username": dbUser, "password": pw, "reset_password": reset,
		}),
	})
	if err != nil {
		return err
	}
	note := fmt.Sprintf("engine=%s\nname=%s\nusername=%s\npassword=%s\nhost=127.0.0.1\n", d.Engine, d.Name, dbUser, pw)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database."+d.Engine+"."+d.Name, []byte(note), 0o600)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database."+d.Engine, []byte(note), 0o600)
	d.Status = "active"
	w.Store.PutDB(d)
	w.syncWordPressDatabase(acc)
	return nil
}

func (w *Worker) deleteDB(j *store.Job) error {
	d := w.Store.GetDB(str(j.Payload["database_id"]))
	if d == nil {
		return nil
	}
	acc := w.Store.GetAccount(d.AccountID)
	if acc == nil {
		return fmt.Errorf("account missing")
	}
	remaining := 0
	for _, other := range w.Store.ListDBs(acc.ID) {
		if other.ID != d.ID && other.Status != "terminated" {
			remaining++
		}
	}
	dropUser := remaining == 0
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "DropHostedDatabase",
		Params: mustJSON(map[string]any{
			"engine": d.Engine, "name": d.Name, "username": acc.Username + "_u", "drop_user": dropUser,
		}),
	})
	if err != nil {
		return err
	}
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "RemoveManagedFile",
		Params: mustJSON(map[string]any{"path": "/home/" + acc.Username + "/.panel-database." + d.Engine + "." + d.Name}),
	})
	w.Store.DeleteDB(d.ID)
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
	if err := w.applyDNSSECState(z); err != nil {
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

func (w *Worker) republishPublicAddresses(z *store.DNSZone, pubIP string) {
	if z == nil || pubIP == "" || pubIP == "127.0.0.1" {
		return
	}
	changed := false
	for _, rec := range w.Store.ListRecords(z.ID) {
		switch rec.Type {
		case "A":
			if rec.Content == "127.0.0.1" || rec.Content == "0.0.0.0" {
				rec.Content = pubIP
				w.Store.PutRecord(&rec)
				changed = true
			}
		case "TXT":
			if strings.Contains(rec.Content, "ip4:127.0.0.1") {
				rec.Content = strings.ReplaceAll(rec.Content, "ip4:127.0.0.1", "ip4:"+pubIP)
				w.Store.PutRecord(&rec)
				changed = true
			}
		}
	}
	if !changed {
		return
	}
	z.DesiredRevision++
	w.Store.PutZone(z)
}

func (w *Worker) writeZone(z *store.DNSZone) error {
	body := dns.ZoneFile(*z, w.Store.ListRecords(z.ID), z.DesiredRevision)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyDNSZone",
		Params: mustJSON(map[string]any{"name": z.Name, "body": body}),
	})
	return err
}

func (w *Worker) applyZoneDNSSEC(j *store.Job) error {
	z := w.Store.GetZone(str(j.Payload["zone_id"]))
	if z == nil {
		return nil
	}
	return w.applyDNSSECState(z)
}

func (w *Worker) applyDNSSECState(z *store.DNSZone) error {
	if w.Agent == nil {
		return nil
	}
	raw, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetDNSSEC",
		Params: mustJSON(map[string]any{"name": z.Name, "enabled": z.DNSSECEnabled}),
	})
	if err != nil {
		return err
	}
	_ = raw
	z.ObservedRevision = z.DesiredRevision
	w.Store.PutZone(z)
	return nil
}

func (w *Worker) ensureCatchallHomes() error {
	if w.Agent == nil {
		return nil
	}
	for _, acc := range w.Store.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, md := range w.Store.ListMailDomains(acc.ID) {
			local := mail.CatchallLocal(md.CatchallPolicy)
			if local == "" {
				continue
			}
			d := w.Store.GetDomain(md.DomainID)
			if d == nil {
				continue
			}
			if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "CreateMailboxHome",
				Params: mustJSON(map[string]any{
					"domain": d.ASCII, "local_part": local,
					"uid": acc.LinuxUID, "gid": acc.LinuxGID,
				}),
			}); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err := w.ensureCatchallHomes(); err != nil {
		return err
	}
	recs := mail.RecipientsForHost(w.Store)
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyMailMaps",
		Params: mustJSON(map[string]any{
			"virtual":      mail.Virtual(recs) + mail.CatchallVirtual(w.Store),
			"domains":      mail.VDomains(w.Store),
			"passwd":       mail.PasswdFile(recs),
			"uids":         mail.UIDMap(recs),
			"gids":         mail.GIDMap(recs),
			"send_limits":  mail.SendLimits(recs),
			"aliases":      mail.AliasMap(w.Store),
			"sender_login": mail.SenderLogin(w.Store, recs),
		}),
	})
	if err != nil {
		return err
	}
	return w.ensureDKIM(recs)
}

func (w *Worker) ensureDKIM(recs []mail.Recipient) error {
	if w.Agent == nil {
		return nil
	}
	seen := map[string]bool{}
	var domains []string
	for _, r := range recs {
		if r.Domain == "" || seen[r.Domain] {
			continue
		}
		seen[r.Domain] = true
		domains = append(domains, r.Domain)
	}
	for _, domain := range domains {
		raw, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "EnsureDKIM",
			Params: mustJSON(map[string]any{"domain": domain}),
		})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(raw)
		var rec operations.DKIMRecord
		if json.Unmarshal(b, &rec) != nil || rec.TXT == "" {
			continue
		}
		w.publishDKIMTXT(domain, rec.TXT)
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyDKIMSigning",
		Params: mustJSON(map[string]any{"domains": domains}),
	})
	return err
}

func (w *Worker) publishDKIMTXT(domain, txt string) {
	for _, acc := range w.Store.ListAccounts("", "") {
		for _, d := range w.Store.ListDomains(acc.ID) {
			if d.ASCII != domain {
				continue
			}
			z := w.Store.ZoneByDomain(d.ID)
			if z == nil {
				return
			}
			updated := false
			for _, rec := range w.Store.ListRecords(z.ID) {
				if rec.Type == "TXT" && rec.Name == "default._domainkey" {
					rec.Content = txt
					w.Store.PutRecord(&rec)
					updated = true
					break
				}
			}
			if !updated {
				w.Store.PutRecord(&store.DNSRecord{
					ID: store.NewID(), ZoneID: z.ID, Name: "default._domainkey",
					Type: "TXT", Content: txt, TTL: 3600,
				})
			}
			z.DesiredRevision++
			w.Store.PutZone(z)
			_ = w.writeZone(z)
			p := &dns.PowerDNS{BaseURL: os.Getenv("PANEL_PDNS_URL"), APIKey: os.Getenv("PANEL_PDNS_API_KEY")}
			_ = p.UpsertRecord(context.Background(), z.Name, dns.Record{
				Name: "default._domainkey", Type: "TXT", Content: txt, TTL: 3600,
			})
			return
		}
	}
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

func (w *Worker) liveACME() bool {
	return acme.Directory() != "" && (w.Agent == nil || w.Agent.Root == "")
}

func (w *Worker) certificateNames(acc *store.Account, hostname string) []string {
	names := []string{hostname}
	if acc == nil || hostname == "" {
		return names
	}
	if primary := w.primaryDomain(acc); primary != nil && primary.ASCII == hostname {
		names = append(names, w.aliasesFor(acc, primary)...)
	}
	return names
}

func (w *Worker) renewCertificate(acc *store.Account, hostname string) error {
	if acc == nil || hostname == "" {
		return nil
	}
	var cert *store.Certificate
	for _, c := range w.Store.ListCerts(acc.ID) {
		if c.Hostname != hostname {
			continue
		}
		cp := c
		cert = &cp
		break
	}
	if cert == nil {
		cert = &store.Certificate{
			ID: store.NewID(), AccountID: acc.ID, Hostname: hostname,
			Kind: "domain", Status: "requested",
		}
		w.Store.PutCert(cert)
	}
	return w.issueStoredCertificate(cert)
}

func (w *Worker) ensureCertificate(acc *store.Account, hostname string) error {
	if acc == nil || hostname == "" {
		return nil
	}
	var cert *store.Certificate
	for _, c := range w.Store.ListCerts(acc.ID) {
		if c.Hostname != hostname {
			continue
		}
		cp := c
		cert = &cp
		break
	}
	if cert != nil && cert.Status == "active" && cert.NotAfter != nil && time.Until(*cert.NotAfter) > 30*24*time.Hour {
		return nil
	}
	if cert == nil {
		cert = &store.Certificate{
			ID: store.NewID(), AccountID: acc.ID, Hostname: hostname,
			Kind: "domain", Status: "requested",
		}
		w.Store.PutCert(cert)
	}
	return w.issueStoredCertificate(cert)
}

func (w *Worker) issueStoredCertificate(c *store.Certificate) error {
	if c == nil {
		return fmt.Errorf("certificate missing")
	}
	contact := "admin@localhost"
	if acc := w.Store.GetAccount(c.AccountID); acc != nil {
		if owner := w.Store.UserByID(acc.OwnerUserID); owner != nil && owner.Email != "" {
			contact = owner.Email
		}
	}
	directory := acme.Directory()
	names := []string{c.Hostname}
	if acc := w.Store.GetAccount(c.AccountID); acc != nil {
		names = w.certificateNames(acc, c.Hostname)
	}
	exp, err := acme.IssueNames(context.Background(), w.Agent, names, contact, directory)
	if err != nil {
		return err
	}
	c.Status = "active"
	c.NotAfter = &exp
	c.Issuer = acme.IssuerName(directory)
	if w.Agent != nil && w.Agent.Root != "" {
		c.Issuer = "panel-dev"
	}
	w.Store.PutCert(c)
	return nil
}

func (w *Worker) provisionCert(j *store.Job) error {
	c := w.Store.GetCert(str(j.Payload["certificate_id"]))
	if err := w.issueStoredCertificate(c); err != nil {
		return err
	}
	// Nginx only picks up a newly written certificate after ApplyWebsite.
	if acc := w.Store.GetAccount(c.AccountID); acc != nil {
		for _, site := range w.Store.ListWebsites(acc.ID) {
			d := w.Store.GetDomain(site.DomainID)
			if d == nil || d.ASCII != c.Hostname {
				continue
			}
			s := site
			s.HTTPSRedirect = true
			w.Store.PutWebsite(&s)
			if err := w.applyWebsiteDispatch(acc, &s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Worker) syncFTPUsers() error {
	var users []operations.FTPUser
	for _, f := range w.Store.ListAllFTP() {
		acc := w.Store.GetAccount(f.AccountID)
		if acc == nil {
			continue
		}
		switch acc.Status {
		case "terminating", "terminated", "suspended":
			continue
		default:
		}
		if f.Status != "active" || f.PasswordHash == "" || f.PasswordHash == "!" {
			continue
		}
		root := f.HomePath
		if root == "" {
			root = filepath.Join(acc.HomePath, "public_html")
		}
		users = append(users, operations.FTPUser{
			Username: f.Username, PasswordHash: f.PasswordHash,
			GuestUser: acc.Username, LocalRoot: root,
		})
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyFTPUsers",
		Params: mustJSON(map[string]any{"users": users}),
	})
	return err
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
	return w.applyMigratedData(j)
}

func (w *Worker) applyMigratedData(j *store.Job) error {
	if w.Agent == nil {
		return nil
	}
	acc := w.jobAccount(j)
	if acc == nil {
		return nil
	}
	src := str(j.Payload["copy_source"])
	if src == "" {
		src = str(j.Payload["source"])
	}
	dest := str(j.Payload["copy_dest"])
	if dest == "" {
		dest = str(j.Payload["dest"])
	}
	if dest == "" {
		dest = acc.HomePath
	}
	user := str(j.Payload["username"])
	if user == "" {
		user = acc.Username
	}
	if src != "" && src != dest {
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "CopyHomedir",
			Params: mustJSON(map[string]any{"username": user, "source": src, "dest": dest}),
		}); err != nil {
			return err
		}
	}
	if err := w.migrateDatabases(acc, j); err != nil {
		return err
	}
	if err := w.migrateMailboxes(j); err != nil {
		return err
	}
	return nil
}

func (w *Worker) migrateDatabases(acc *store.Account, j *store.Job) error {
	for _, item := range payloadObjects(j.Payload["databases"]) {
		engine := str(item["engine"])
		srcName := str(item["source"])
		destName := str(item["dest"])
		if engine == "" || destName == "" {
			continue
		}
		if srcName == "" {
			srcName = destName
		}
		if err := w.ensureHostedDBOnHost(acc, engine, destName); err != nil {
			return err
		}
		if dump := str(item["dump"]); dump != "" {
			if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "RestoreHostedDatabase",
				Params: mustJSON(map[string]any{
					"engine": engine, "name": destName, "source": dump, "extract": srcName,
				}),
			}); err != nil {
				return err
			}
			continue
		}
		srcHome := str(j.Payload["copy_source"])
		if srcHome == "" {
			srcHome = str(j.Payload["source"])
		}
		if !strings.HasPrefix(srcHome, "/home/") {
			continue
		}
		if srcName == destName && srcHome == acc.HomePath {
			continue
		}
		dumpID := strings.ReplaceAll(acc.ID+"-"+engine+"-"+srcName, "/", "-")
		dumpPath := "/var/lib/panel/backups/staging/migrate-" + dumpID + ".sql"
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "DumpHostedDatabase",
			Params: mustJSON(map[string]any{"engine": engine, "name": srcName, "dest": dumpPath}),
		}); err != nil {
			if srcName == destName {
				continue
			}
			return err
		}
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "RestoreHostedDatabase",
			Params: mustJSON(map[string]any{"engine": engine, "name": destName, "source": dumpPath}),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) migrateMailboxes(j *store.Job) error {
	for _, item := range payloadObjects(j.Payload["mailboxes"]) {
		src := str(item["source"])
		dest := str(item["dest"])
		if src == "" || dest == "" || src == dest {
			continue
		}
		dumpID := strings.ReplaceAll(strings.Trim(src, "/"), "/", "-")
		archive := "/var/lib/panel/backups/staging/migrate-mail-" + dumpID + ".tar.gz"
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "PackDirectory",
			Params: mustJSON(map[string]any{"source": src, "dest": archive}),
		}); err != nil {
			continue
		}
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "UnpackDirectory",
			Params: mustJSON(map[string]any{"archive": archive, "dest": dest}),
		}); err != nil {
			return err
		}
	}
	return nil
}

func payloadObjects(v any) []map[string]any {
	switch items := v.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
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
	homeTar, dumps, mailTrees, err := w.collectBackupParts(acc, b.ID, home)
	if err != nil {
		return err
	}
	repo, err := backup.Open(b.Destination, localRoot)
	if err != nil {
		return err
	}
	man, key, err := backup.BuildFull(context.Background(), w.Box, repo, acc, w.Store.ListDBs(acc.ID), w.Store.ListMailboxes(acc.ID), homeTar, dumps, mailTrees)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	b.State = "succeeded"
	b.FinishedAt = &now
	b.Checksum = man.Checksums["files.tar.gz"]
	b.Manifest = map[string]any{
		"format_version": man.FormatVersion, "key": key, "account_id": man.AccountID,
		"checksums": man.Checksums, "databases": man.Databases, "mailboxes": man.Mailboxes,
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
	man, raw, err := backup.OpenArchive(context.Background(), w.Box, repo, key)
	if err != nil {
		return err
	}
	if err := backup.Preflight(man, acc); err != nil {
		return err
	}
	homeTar := raw
	var dumps []backup.DBDump
	var mailTrees []backup.MailDump
	if man.FormatVersion == 2 {
		homeTar, dumps, mailTrees, err = backup.SplitV2(raw)
		if err != nil {
			return err
		}
	}
	if err := w.restoreHomeTar(acc, b.ID, home, homeTar); err != nil {
		return err
	}
	if err := w.restoreDatabaseDumps(acc, b.ID, dumps); err != nil {
		return err
	}
	if err := w.restoreMailboxTrees(acc, b.ID, mailTrees); err != nil {
		return err
	}
	_ = w.applyMailStack(acc.ID)
	w.syncWordPressDatabase(acc)
	return nil
}

func (w *Worker) hostPath(p string) string {
	if w.Agent != nil && w.Agent.Sock == "" && w.Agent.Root != "" {
		return filepath.Join(w.Agent.Root, strings.TrimPrefix(p, "/"))
	}
	return p
}

func (w *Worker) collectBackupParts(acc *store.Account, backupID, home string) ([]byte, []backup.DBDump, []backup.MailDump, error) {
	stagingHome := "/var/lib/panel/backups/staging/" + backupID + "-home.tar.gz"
	var homeTar []byte
	if w.Agent != nil {
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "PackDirectory",
			Params: mustJSON(map[string]any{"source": acc.HomePath, "dest": stagingHome}),
		}); err != nil {
			return nil, nil, nil, err
		}
		b, err := os.ReadFile(w.hostPath(stagingHome))
		if err != nil {
			return nil, nil, nil, err
		}
		homeTar = b
	} else {
		packed, err := backup.PackHome(home)
		if err != nil {
			return nil, nil, nil, err
		}
		homeTar = packed
	}
	var dumps []backup.DBDump
	for _, d := range w.Store.ListDBs(acc.ID) {
		if d.Name == "" {
			continue
		}
		dest := "/var/lib/panel/backups/staging/" + backupID + "-" + d.Engine + "-" + d.Name + ".sql"
		if w.Agent == nil {
			continue
		}
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "DumpHostedDatabase",
			Params: mustJSON(map[string]any{"engine": d.Engine, "name": d.Name, "dest": dest}),
		}); err != nil {
			return nil, nil, nil, err
		}
		sql, err := os.ReadFile(w.hostPath(dest))
		if err != nil {
			return nil, nil, nil, err
		}
		dumps = append(dumps, backup.DBDump{Engine: d.Engine, Name: d.Name, SQL: sql})
	}
	var mailTrees []backup.MailDump
	if w.Agent == nil {
		return homeTar, dumps, mailTrees, nil
	}
	for _, rec := range mail.Recipients(w.Store, acc.ID) {
		dest := "/var/lib/panel/backups/staging/" + backupID + "-mail-" + rec.Domain + "-" + rec.LocalPart + ".tar.gz"
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "PackDirectory",
			Params: mustJSON(map[string]any{"source": rec.Home, "dest": dest}),
		}); err != nil {
			continue
		}
		raw, err := os.ReadFile(w.hostPath(dest))
		if err != nil {
			continue
		}
		mailTrees = append(mailTrees, backup.MailDump{Domain: rec.Domain, Local: rec.LocalPart, TarGz: raw})
	}
	return homeTar, dumps, mailTrees, nil
}

func (w *Worker) restoreHomeTar(acc *store.Account, backupID, destHome string, homeTar []byte) error {
	if w.Agent != nil {
		staging := "/var/lib/panel/backups/staging/restore-" + backupID + "-home.tar.gz"
		if _, err := w.Agent.ApplyFile(staging, homeTar, 0o640); err != nil {
			return err
		}
		_, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "UnpackDirectory",
			Params: mustJSON(map[string]any{"archive": staging, "dest": acc.HomePath}),
		})
		return err
	}
	return backup.UnpackHomeBytes(homeTar, destHome)
}

func (w *Worker) restoreDatabaseDumps(acc *store.Account, backupID string, dumps []backup.DBDump) error {
	for _, d := range dumps {
		w.ensureRestoredDB(acc, d.Engine, d.Name)
		if w.Agent == nil {
			continue
		}
		src := "/var/lib/panel/backups/staging/restore-" + backupID + "-" + d.Engine + "-" + d.Name + ".sql"
		if _, err := w.Agent.ApplyFile(src, d.SQL, 0o640); err != nil {
			return err
		}
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "RestoreHostedDatabase",
			Params: mustJSON(map[string]any{"engine": d.Engine, "name": d.Name, "source": src}),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) ensureRestoredDB(acc *store.Account, engine, name string) {
	_ = w.ensureHostedDBOnHost(acc, engine, name)
}

func (w *Worker) ensureHostedDBOnHost(acc *store.Account, engine, name string) error {
	var row *store.HostedDatabase
	for _, existing := range w.Store.ListDBs(acc.ID) {
		if existing.Name == name {
			e := existing
			row = &e
			break
		}
	}
	if row == nil {
		row = &store.HostedDatabase{ID: store.NewID(), AccountID: acc.ID, Engine: engine, Name: name, Status: "provisioning"}
		w.Store.PutDB(row)
	}
	if w.Agent == nil {
		row.Status = "active"
		w.Store.PutDB(row)
		return nil
	}
	dbUser, pw, reset := w.hostedDBCredentials(acc, engine)
	if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{
			"engine": engine, "name": name, "username": dbUser, "password": pw, "reset_password": reset,
		}),
	}); err != nil {
		return err
	}
	note := fmt.Sprintf("engine=%s\nname=%s\nusername=%s\npassword=%s\nhost=127.0.0.1\n", engine, name, dbUser, pw)
	_, _ = w.Agent.ApplyFile("/home/"+acc.Username+"/.panel-database."+engine+"."+name, []byte(note), 0o600)
	row.Status = "active"
	w.Store.PutDB(row)
	return nil
}

func (w *Worker) restoreMailboxTrees(acc *store.Account, backupID string, trees []backup.MailDump) error {
	if w.Agent == nil {
		return nil
	}
	for _, m := range trees {
		staging := "/var/lib/panel/backups/staging/restore-" + backupID + "-mail-" + m.Domain + "-" + m.Local + ".tar.gz"
		if _, err := w.Agent.ApplyFile(staging, m.TarGz, 0o640); err != nil {
			return err
		}
		dest := "/var/vmail/" + m.Domain + "/" + m.Local
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "UnpackDirectory",
			Params: mustJSON(map[string]any{"archive": staging, "dest": dest}),
		}); err != nil {
			return err
		}
	}
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

func (w *Worker) websiteEnabled(acc *store.Account) bool {
	if acc == nil {
		return false
	}
	switch acc.Status {
	case "suspended", "terminating", "terminated":
		return false
	default:
		return true
	}
}

func (w *Worker) primaryDomain(acc *store.Account) *store.Domain {
	if acc == nil {
		return nil
	}
	for _, d := range w.Store.ListDomains(acc.ID) {
		if d.Type == "primary" || d.ASCII == acc.PrimaryDomain {
			cp := d
			return &cp
		}
	}
	return nil
}

func (w *Worker) aliasesFor(acc *store.Account, siteDomain *store.Domain) []string {
	if acc == nil || siteDomain == nil {
		return nil
	}
	primary := w.primaryDomain(acc)
	if primary == nil || (siteDomain.ID != primary.ID && siteDomain.ASCII != acc.PrimaryDomain) {
		return nil
	}
	var out []string
	for _, d := range w.Store.ListDomains(acc.ID) {
		if d.Type == "alias" && d.ASCII != "" {
			out = append(out, d.ASCII)
		}
	}
	return out
}

func (w *Worker) concurrentWebRequests(acc *store.Account) int {
	if acc == nil {
		return 0
	}
	pkg := w.Store.GetPackage(acc.PackageID)
	if pkg == nil || pkg.ConcurrentWebRequests < 1 {
		return 0
	}
	return pkg.ConcurrentWebRequests
}

func (w *Worker) bandwidthHold(acc *store.Account) bool {
	if acc == nil {
		return false
	}
	pkg := w.Store.GetPackage(acc.PackageID)
	if pkg == nil || pkg.BandwidthBytesMonthly <= 0 {
		return false
	}
	u := w.Store.GetUsage(acc.ID)
	if u == nil {
		return false
	}
	return u.BandwidthBytes >= pkg.BandwidthBytesMonthly
}

func (w *Worker) applyWebsiteDispatch(acc *store.Account, site *store.Website, d *store.Domain) error {
	if acc == nil || site == nil || d == nil || w.Agent == nil {
		return fmt.Errorf("website apply missing account, site, or domain")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyWebsite",
		Params: mustJSON(map[string]any{
			"website_id": site.ID, "account": acc.Username, "domain": d.ASCII,
			"document_root": site.DocumentRoot, "runtime": site.Runtime,
			"https_redirect": site.HTTPSRedirect, "enabled": w.websiteEnabled(acc),
			"bandwidth_hold":          w.bandwidthHold(acc),
			"concurrent_web_requests": w.concurrentWebRequests(acc),
			"aliases":                 w.aliasesFor(acc, d),
		}),
	})
	return err
}

func (w *Worker) applySuspendedHost(acc *store.Account) {
	if acc == nil {
		return
	}
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "LockLinuxUser", Params: mustJSON(map[string]any{"username": acc.Username})})
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{Method: "FreezeAccount", Params: mustJSON(map[string]any{"username": acc.Username, "freeze": true})})
	w.reapplyAccountWebsites(acc)
}

func (w *Worker) reapplyAccountWebsites(acc *store.Account) {
	if acc == nil {
		return
	}
	for _, site := range w.Store.ListWebsites(acc.ID) {
		d := w.Store.GetDomain(site.DomainID)
		if d == nil {
			continue
		}
		s := site
		_ = w.applyWebsiteDispatch(acc, &s, d)
	}
}

func (w *Worker) recordUsage(acc *store.Account) {
	if acc == nil {
		return
	}
	wasHold := w.bandwidthHold(acc)
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
				u.BandwidthBytes = got.BandwidthBytes
				w.Store.PutUsage(u)
				_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
					Method: "EnforceAccountDisk",
					Params: mustJSON(map[string]any{"username": acc.Username, "home": acc.HomePath}),
				})
				hold := w.bandwidthHold(acc)
				_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
					Method: "EnforceAccountBandwidth",
					Params: mustJSON(map[string]any{"username": acc.Username, "hold": hold}),
				})
				if wasHold != hold {
					w.reapplyAccountWebsites(acc)
				}
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

func (w *Worker) peekHostedDBPassword(acc *store.Account, engine string) (username, password string, ok bool) {
	if acc == nil {
		return "", "", false
	}
	username = acc.Username + "_u"
	if w.Box == nil {
		return username, "", false
	}
	for _, u := range w.Store.ListDBUsers(acc.ID) {
		if u.Username != username || u.Engine != engine {
			continue
		}
		if len(u.PasswordEnc) == 0 {
			continue
		}
		plain, err := w.Box.Decrypt(u.PasswordEnc)
		if err != nil || len(plain) == 0 {
			continue
		}
		return username, string(plain), true
	}
	return username, "", false
}

func (w *Worker) syncWordPressDatabase(acc *store.Account) {
	if acc == nil || w.Agent == nil {
		return
	}
	dbUser, pw, ok := w.peekHostedDBPassword(acc, "mariadb")
	if !ok {
		dbUser, pw, _ = w.hostedDBCredentials(acc, "mariadb")
	}
	dbName := acc.Username + "_wp"
	for _, d := range w.Store.ListDBs(acc.ID) {
		if d.Engine == "mariadb" || d.Engine == "mysql" {
			dbName = d.Name
			break
		}
	}
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "CreateHostedDatabase",
		Params: mustJSON(map[string]any{
			"engine": "mariadb", "name": dbName, "username": dbUser, "password": pw, "reset_password": true,
		}),
	})
	_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SyncWordPressDatabase",
		Params: mustJSON(map[string]any{
			"username": acc.Username, "db_user": dbUser, "db_password": pw, "db_host": "127.0.0.1",
		}),
	})
}

func (w *Worker) hostedDBCredentials(acc *store.Account, engine string) (username, password string, reset bool) {
	username = acc.Username + "_u"
	var existing *store.DatabaseUser
	for _, u := range w.Store.ListDBUsers(acc.ID) {
		if u.Username == username && u.Engine == engine {
			cp := u
			existing = &cp
			break
		}
	}
	if existing != nil && len(existing.PasswordEnc) > 0 && w.Box != nil {
		if plain, err := w.Box.Decrypt(existing.PasswordEnc); err == nil && len(plain) > 0 {
			return username, string(plain), false
		}
	}
	password = fmt.Sprintf("db-%s", store.NewID())
	var enc []byte
	if w.Box != nil {
		enc, _ = w.Box.Encrypt([]byte(password))
	}
	if existing != nil {
		existing.Engine = engine
		existing.PasswordEnc = enc
		w.Store.PutDBUser(existing)
	} else {
		w.Store.PutDBUser(&store.DatabaseUser{
			ID: store.NewID(), AccountID: acc.ID, Username: username, Engine: engine, PasswordEnc: enc,
		})
	}
	return username, password, true
}

func (w *Worker) collapseDomainWebsites(acc *store.Account, d *store.Domain, keep *store.Website) {
	if acc == nil || d == nil || keep == nil {
		return
	}
	for _, s := range w.Store.ListWebsites(acc.ID) {
		if s.DomainID != d.ID || s.ID == keep.ID {
			continue
		}
		if w.Agent != nil {
			_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "RetireWebsite",
				Params: mustJSON(map[string]any{"website_id": s.ID, "account": acc.Username}),
			})
		}
		w.Store.DeleteWebsite(s.ID)
	}
}

func publicIPv4() string {
	return netaddr.PublicIPv4()
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
