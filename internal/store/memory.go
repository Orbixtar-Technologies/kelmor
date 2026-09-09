package store

import (
	"crypto/subtle"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hosting-panel/panel/internal/id"
)

type Memory struct {
	mu        sync.RWMutex
	Users     map[string]*User
	Sessions  map[string]*Session
	Packages  map[string]*Package
	Features  map[string]*FeatureSet
	Resellers map[string]*Reseller
	Accounts  map[string]*Account
	Members   map[string][]string // accountID -> userIDs
	Domains   map[string]*Domain
	Websites  map[string]*Website
	Apps      map[string]*Application
	DBs       map[string]*HostedDatabase
	DBUsers   map[string]*DatabaseUser
	Zones     map[string]*DNSZone
	Records   map[string]*DNSRecord
	MailDom   map[string]*MailDomain
	Mailboxes map[string]*Mailbox
	Aliases   map[string]*MailAlias
	Certs     map[string]*Certificate
	Jobs      map[string]*Job
	Audit     []*AuditEvent
	Tokens    map[string]*APIToken
	Backups   map[string]*BackupRun
	Crons     map[string]*CronJob
	SSHKeys   map[string]*SSHKey
	FTPs      map[string]*FTPAccount
	Usage     map[string]*Usage
	NextUID   int
}

func NewMemory() *Memory {
	return &Memory{
		Users: map[string]*User{}, Sessions: map[string]*Session{},
		Packages: map[string]*Package{}, Features: map[string]*FeatureSet{},
		Resellers: map[string]*Reseller{}, Accounts: map[string]*Account{},
		Members: map[string][]string{}, Domains: map[string]*Domain{},
		Websites: map[string]*Website{}, Apps: map[string]*Application{},
		DBs: map[string]*HostedDatabase{}, DBUsers: map[string]*DatabaseUser{},
		Zones: map[string]*DNSZone{}, Records: map[string]*DNSRecord{},
		MailDom: map[string]*MailDomain{}, Mailboxes: map[string]*Mailbox{},
		Aliases: map[string]*MailAlias{},
		Certs:   map[string]*Certificate{}, Jobs: map[string]*Job{},
		Tokens: map[string]*APIToken{}, Backups: map[string]*BackupRun{},
		Crons: map[string]*CronJob{}, SSHKeys: map[string]*SSHKey{},
		FTPs: map[string]*FTPAccount{}, Usage: map[string]*Usage{},
		NextUID: 20000,
	}
}

func (m *Memory) AllocUID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.NextUID
	m.NextUID++
	return u
}

func (m *Memory) PutUser(u *User) { m.mu.Lock(); m.Users[u.ID] = u; m.mu.Unlock() }

func (m *Memory) UserByUsername(name string) *User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.Users {
		if u.Username == name {
			return cloneUser(u)
		}
	}
	return nil
}

func (m *Memory) UserByID(id string) *User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if u := m.Users[id]; u != nil {
		return cloneUser(u)
	}
	return nil
}

func (m *Memory) PutSession(s *Session) { m.mu.Lock(); m.Sessions[s.ID] = s; m.mu.Unlock() }

func (m *Memory) SessionByHash(hash []byte) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.Sessions {
		if subtle.ConstantTimeCompare(s.TokenHash, hash) == 1 && s.RevokedAt == nil && time.Now().Before(s.ExpiresAt) {
			cp := *s
			return &cp
		}
	}
	return nil
}

func (m *Memory) RevokeSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.Sessions[id]; s != nil {
		now := time.Now()
		s.RevokedAt = &now
	}
}

func (m *Memory) PutFeature(f *FeatureSet) { m.mu.Lock(); m.Features[f.ID] = f; m.mu.Unlock() }
func (m *Memory) ListFeatureSets() []FeatureSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]FeatureSet, 0, len(m.Features))
	for _, f := range m.Features {
		out = append(out, *f)
	}
	return out
}
func (m *Memory) PutPackage(p *Package) { m.mu.Lock(); m.Packages[p.ID] = p; m.mu.Unlock() }
func (m *Memory) GetPackage(id string) *Package {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p := m.Packages[id]; p != nil {
		cp := *p
		return &cp
	}
	return nil
}
func (m *Memory) ListPackages() []Package {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Package, 0, len(m.Packages))
	for _, p := range m.Packages {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (m *Memory) DeletePackage(id string) { m.mu.Lock(); delete(m.Packages, id); m.mu.Unlock() }

func (m *Memory) PutReseller(r *Reseller) { m.mu.Lock(); m.Resellers[r.ID] = r; m.mu.Unlock() }
func (m *Memory) GetReseller(id string) *Reseller {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if r := m.Resellers[id]; r != nil {
		cp := *r
		return &cp
	}
	return nil
}
func (m *Memory) ResellerByUser(userID string) *Reseller {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.Resellers {
		if r.UserID == userID {
			cp := *r
			return &cp
		}
	}
	return nil
}

func (m *Memory) ListResellers() []Reseller {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Reseller, 0, len(m.Resellers))
	for _, r := range m.Resellers {
		out = append(out, *r)
	}
	return out
}

func (m *Memory) PutAccount(a *Account) { m.mu.Lock(); m.Accounts[a.ID] = a; m.mu.Unlock() }
func (m *Memory) GetAccount(id string) *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a := m.Accounts[id]; a != nil {
		cp := *a
		return &cp
	}
	return nil
}
func (m *Memory) AccountByUsername(name string) *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.Accounts {
		if a.Username == name {
			cp := *a
			return &cp
		}
	}
	return nil
}
func (m *Memory) ListAccounts(q string, status string) []Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q = strings.ToLower(q)
	out := []Account{}
	for _, a := range m.Accounts {
		if status != "" && a.Status != status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Username), q) && !strings.Contains(strings.ToLower(a.PrimaryDomain), q) {
			continue
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

func (m *Memory) DomainTaken(ascii string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.Domains {
		if d.ASCII == ascii {
			return true
		}
	}
	return false
}

func (m *Memory) PutDomain(d *Domain) { m.mu.Lock(); m.Domains[d.ID] = d; m.mu.Unlock() }
func (m *Memory) GetDomain(id string) *Domain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d := m.Domains[id]; d != nil {
		cp := *d
		return &cp
	}
	return nil
}
func (m *Memory) ListDomains(accountID string) []Domain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Domain{}
	for _, d := range m.Domains {
		if accountID == "" || d.AccountID == accountID {
			out = append(out, *d)
		}
	}
	return out
}
func (m *Memory) DeleteDomain(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for wid, w := range m.Websites {
		if w != nil && w.DomainID == id {
			for aid, app := range m.Apps {
				if app != nil && app.WebsiteID == wid {
					delete(m.Apps, aid)
				}
			}
			delete(m.Websites, wid)
		}
	}
	var mailIDs []string
	for mid, md := range m.MailDom {
		if md != nil && md.DomainID == id {
			mailIDs = append(mailIDs, mid)
			delete(m.MailDom, mid)
		}
	}
	for _, mid := range mailIDs {
		for bid, mb := range m.Mailboxes {
			if mb != nil && mb.DomainID == mid {
				delete(m.Mailboxes, bid)
			}
		}
		for aid, al := range m.Aliases {
			if al != nil && al.DomainID == mid {
				delete(m.Aliases, aid)
			}
		}
	}
	for zid, z := range m.Zones {
		if z != nil && z.DomainID == id {
			for rid, rec := range m.Records {
				if rec != nil && rec.ZoneID == zid {
					delete(m.Records, rid)
				}
			}
			delete(m.Zones, zid)
		}
	}
	delete(m.Domains, id)
}

func (m *Memory) PutWebsite(w *Website) { m.mu.Lock(); m.Websites[w.ID] = w; m.mu.Unlock() }
func (m *Memory) GetWebsite(id string) *Website {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if w := m.Websites[id]; w != nil {
		cp := *w
		return &cp
	}
	return nil
}
func (m *Memory) ListWebsites(accountID string) []Website {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Website{}
	for _, w := range m.Websites {
		if accountID == "" || w.AccountID == accountID {
			out = append(out, *w)
		}
	}
	return out
}
func (m *Memory) DeleteWebsite(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for aid, app := range m.Apps {
		if app != nil && app.WebsiteID == id {
			delete(m.Apps, aid)
		}
	}
	delete(m.Websites, id)
}

func (m *Memory) PutApp(a *Application) { m.mu.Lock(); m.Apps[a.ID] = a; m.mu.Unlock() }
func (m *Memory) GetApp(id string) *Application {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a := m.Apps[id]; a != nil {
		cp := *a
		return &cp
	}
	return nil
}
func (m *Memory) DeleteApp(id string) {
	m.mu.Lock()
	delete(m.Apps, id)
	m.mu.Unlock()
}

func (m *Memory) ListApps(accountID string) []Application {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Application{}
	for _, a := range m.Apps {
		if accountID == "" || a.AccountID == accountID {
			out = append(out, *a)
		}
	}
	return out
}

func (m *Memory) PutDB(d *HostedDatabase) { m.mu.Lock(); m.DBs[d.ID] = d; m.mu.Unlock() }
func (m *Memory) GetDB(id string) *HostedDatabase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d := m.DBs[id]; d != nil {
		cp := *d
		return &cp
	}
	return nil
}
func (m *Memory) ListDBs(accountID string) []HostedDatabase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []HostedDatabase{}
	for _, d := range m.DBs {
		if accountID == "" || d.AccountID == accountID {
			out = append(out, *d)
		}
	}
	return out
}
func (m *Memory) DeleteDB(id string) {
	m.mu.Lock()
	delete(m.DBs, id)
	m.mu.Unlock()
}
func (m *Memory) PutDBUser(u *DatabaseUser) { m.mu.Lock(); m.DBUsers[u.ID] = u; m.mu.Unlock() }
func (m *Memory) ListDBUsers(accountID string) []DatabaseUser {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []DatabaseUser{}
	for _, u := range m.DBUsers {
		if accountID == "" || u.AccountID == accountID {
			out = append(out, *u)
		}
	}
	return out
}

func (m *Memory) PutZone(z *DNSZone) { m.mu.Lock(); m.Zones[z.ID] = z; m.mu.Unlock() }
func (m *Memory) GetZone(id string) *DNSZone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if z := m.Zones[id]; z != nil {
		cp := *z
		return &cp
	}
	return nil
}
func (m *Memory) ZoneByDomain(domainID string) *DNSZone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, z := range m.Zones {
		if z.DomainID == domainID {
			cp := *z
			return &cp
		}
	}
	return nil
}
func (m *Memory) ListZones(accountID string) []DNSZone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []DNSZone{}
	for _, z := range m.Zones {
		if accountID == "" || z.AccountID == accountID {
			out = append(out, *z)
		}
	}
	return out
}
func (m *Memory) PutRecord(r *DNSRecord) { m.mu.Lock(); m.Records[r.ID] = r; m.mu.Unlock() }
func (m *Memory) ListRecords(zoneID string) []DNSRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []DNSRecord{}
	for _, r := range m.Records {
		if r.ZoneID == zoneID {
			out = append(out, *r)
		}
	}
	return out
}
func (m *Memory) DeleteRecord(id string) { m.mu.Lock(); delete(m.Records, id); m.mu.Unlock() }

func (m *Memory) PutMailDomain(d *MailDomain) { m.mu.Lock(); m.MailDom[d.ID] = d; m.mu.Unlock() }
func (m *Memory) MailDomainByDomain(domainID string) *MailDomain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.MailDom {
		if d.DomainID == domainID {
			cp := *d
			return &cp
		}
	}
	return nil
}
func (m *Memory) ListMailDomains(accountID string) []MailDomain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []MailDomain{}
	for _, d := range m.MailDom {
		if accountID == "" || d.AccountID == accountID {
			out = append(out, *d)
		}
	}
	return out
}
func (m *Memory) PutMailbox(mb *Mailbox) { m.mu.Lock(); m.Mailboxes[mb.ID] = mb; m.mu.Unlock() }
func (m *Memory) GetMailbox(id string) *Mailbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mb := m.Mailboxes[id]; mb != nil {
		cp := *mb
		return &cp
	}
	return nil
}
func (m *Memory) ListMailboxes(accountID string) []Mailbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Mailbox{}
	for _, mb := range m.Mailboxes {
		if accountID == "" || mb.AccountID == accountID {
			out = append(out, *mb)
		}
	}
	return out
}
func (m *Memory) DeleteMailbox(id string) {
	m.mu.Lock()
	delete(m.Mailboxes, id)
	m.mu.Unlock()
}
func (m *Memory) PutMailAlias(a *MailAlias) { m.mu.Lock(); m.Aliases[a.ID] = a; m.mu.Unlock() }
func (m *Memory) GetMailAlias(id string) *MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a := m.Aliases[id]; a != nil {
		cp := *a
		return &cp
	}
	return nil
}
func (m *Memory) ListMailAliases(accountID string) []MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []MailAlias{}
	for _, a := range m.Aliases {
		if accountID == "" || a.AccountID == accountID {
			out = append(out, *a)
		}
	}
	return out
}
func (m *Memory) DeleteMailAlias(id string) {
	m.mu.Lock()
	delete(m.Aliases, id)
	m.mu.Unlock()
}

func (m *Memory) PutCert(c *Certificate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, existing := range m.Certs {
		if existing.AccountID == c.AccountID && existing.Hostname == c.Hostname && id != c.ID {
			delete(m.Certs, id)
		}
	}
	cp := *c
	m.Certs[c.ID] = &cp
}
func (m *Memory) GetCert(id string) *Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c := m.Certs[id]; c != nil {
		cp := *c
		return &cp
	}
	return nil
}
func (m *Memory) ListCerts(accountID string) []Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Certificate{}
	for _, c := range m.Certs {
		if accountID == "" || c.AccountID == accountID || (accountID == "" && c.AccountID == "") {
			out = append(out, *c)
		}
	}
	return out
}

func (m *Memory) EnqueueJob(j *Job) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j.IdempotencyKey != "" {
		for _, existing := range m.Jobs {
			if existing.IdempotencyKey == j.IdempotencyKey {
				cp := *existing
				return &cp, nil
			}
		}
	}
	if j.ID == "" {
		j.ID = id.New()
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	if j.RunAfter.IsZero() {
		j.RunAfter = time.Now().UTC()
	}
	if j.MaxAttempts == 0 {
		j.MaxAttempts = 5
	}
	if j.State == "" {
		j.State = "queued"
	}
	cp := *j
	m.Jobs[j.ID] = &cp
	return &cp, nil
}

func (m *Memory) ClaimJob(worker string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best *Job
	now := time.Now()
	for _, j := range m.Jobs {
		if (j.State == "queued" || j.State == "retrying") && !j.RunAfter.After(now) {
			if best == nil || j.Priority < best.Priority || (j.Priority == best.Priority && j.CreatedAt.Before(best.CreatedAt)) {
				best = j
			}
		}
	}
	if best == nil {
		return nil
	}
	best.State = "running"
	best.LockedBy = worker
	t := now
	best.StartedAt = &t
	best.HeartbeatAt = &t
	best.Attempts++
	cp := *best
	return &cp
}

func (m *Memory) UpdateJob(j *Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Jobs[j.ID] = j
}

func (m *Memory) GetJob(id string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j := m.Jobs[id]; j != nil {
		cp := *j
		return &cp
	}
	return nil
}

func (m *Memory) ListJobs(state string, limit int) []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Job{}
	for _, j := range m.Jobs {
		if state != "" && j.State != state {
			continue
		}
		out = append(out, *j)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (m *Memory) DriftedAccounts() []Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Account{}
	for _, a := range m.Accounts {
		if a.DesiredRevision > a.ObservedRevision && a.Status != "terminated" {
			out = append(out, *a)
		}
	}
	return out
}

func (m *Memory) AppendAudit(e AuditEvent) {
	if e.ID == "" {
		e.ID = id.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	m.mu.Lock()
	m.Audit = append(m.Audit, &e)
	m.mu.Unlock()
}

func (m *Memory) ListAudit(limit int) []AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AuditEvent, 0, len(m.Audit))
	for _, e := range m.Audit {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurredAt.After(out[j].OccurredAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (m *Memory) PutToken(t *APIToken) { m.mu.Lock(); m.Tokens[t.ID] = t; m.mu.Unlock() }
func (m *Memory) GetToken(id string) *APIToken {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if t := m.Tokens[id]; t != nil {
		cp := *t
		return &cp
	}
	return nil
}
func (m *Memory) DeleteToken(id string) {
	m.mu.Lock()
	delete(m.Tokens, id)
	m.mu.Unlock()
}
func (m *Memory) TokenByHash(hash []byte) *APIToken {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.Tokens {
		if subtle.ConstantTimeCompare(t.TokenHash, hash) == 1 && t.RevokedAt == nil {
			if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
				continue
			}
			cp := *t
			return &cp
		}
	}
	return nil
}
func (m *Memory) ListTokens(userID string) []APIToken {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []APIToken{}
	for _, t := range m.Tokens {
		if t.UserID == userID {
			out = append(out, *t)
		}
	}
	return out
}

func (m *Memory) PutBackup(b *BackupRun) { m.mu.Lock(); m.Backups[b.ID] = b; m.mu.Unlock() }
func (m *Memory) GetBackup(id string) *BackupRun {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b := m.Backups[id]; b != nil {
		cp := *b
		return &cp
	}
	return nil
}
func (m *Memory) ListBackups(accountID string) []BackupRun {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []BackupRun{}
	for _, b := range m.Backups {
		if accountID == "" || b.AccountID == accountID {
			out = append(out, *b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *Memory) PutCron(c *CronJob) { m.mu.Lock(); m.Crons[c.ID] = c; m.mu.Unlock() }
func (m *Memory) GetCron(id string) *CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c := m.Crons[id]; c != nil {
		cp := *c
		return &cp
	}
	return nil
}
func (m *Memory) DeleteCron(id string) {
	m.mu.Lock()
	delete(m.Crons, id)
	m.mu.Unlock()
}
func (m *Memory) ListCrons(accountID string) []CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []CronJob{}
	for _, c := range m.Crons {
		if c.AccountID == accountID {
			out = append(out, *c)
		}
	}
	return out
}
func (m *Memory) PutSSH(k *SSHKey) { m.mu.Lock(); m.SSHKeys[k.ID] = k; m.mu.Unlock() }
func (m *Memory) GetSSH(id string) *SSHKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if k := m.SSHKeys[id]; k != nil {
		cp := *k
		return &cp
	}
	return nil
}
func (m *Memory) DeleteSSH(id string) {
	m.mu.Lock()
	delete(m.SSHKeys, id)
	m.mu.Unlock()
}
func (m *Memory) ListSSH(accountID string) []SSHKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []SSHKey{}
	for _, k := range m.SSHKeys {
		if k.AccountID == accountID {
			out = append(out, *k)
		}
	}
	return out
}
func (m *Memory) PutFTP(f *FTPAccount) { m.mu.Lock(); m.FTPs[f.ID] = f; m.mu.Unlock() }
func (m *Memory) ListFTP(accountID string) []FTPAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []FTPAccount{}
	for _, f := range m.FTPs {
		if f.AccountID == accountID {
			out = append(out, *f)
		}
	}
	return out
}
func (m *Memory) ListAllFTP() []FTPAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []FTPAccount{}
	for _, f := range m.FTPs {
		out = append(out, *f)
	}
	return out
}
func (m *Memory) FTPUsernameTaken(username, exceptID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, f := range m.FTPs {
		if f.Username == username && f.ID != exceptID {
			return true
		}
	}
	return false
}
func (m *Memory) DeleteFTP(id string) {
	m.mu.Lock()
	delete(m.FTPs, id)
	m.mu.Unlock()
}

func (m *Memory) PutUsage(u *Usage) { m.mu.Lock(); m.Usage[u.AccountID] = u; m.mu.Unlock() }
func (m *Memory) GetUsage(accountID string) *Usage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if u := m.Usage[accountID]; u != nil {
		cp := *u
		return &cp
	}
	return nil
}

func (m *Memory) AddMember(accountID, userID string) {
	m.mu.Lock()
	m.Members[accountID] = append(m.Members[accountID], userID)
	m.mu.Unlock()
}

func (m *Memory) AccountsForUser(userID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for aid, users := range m.Members {
		for _, u := range users {
			if u == userID {
				ids = append(ids, aid)
			}
		}
	}
	for _, a := range m.Accounts {
		if a.OwnerUserID == userID {
			ids = append(ids, a.ID)
		}
	}
	return unique(ids)
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func cloneUser(u *User) *User { cp := *u; return &cp }

func (m *Memory) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	failedJobs := 0
	for _, j := range m.Jobs {
		if j.State == "failed" {
			failedJobs++
		}
	}
	return map[string]int{
		"users":      len(m.Users),
		"accounts":   len(m.Accounts),
		"domains":    len(m.Domains),
		"websites":   len(m.Websites),
		"mailboxes":  len(m.Mailboxes),
		"jobs":       len(m.Jobs),
		"failedJobs": failedJobs,
		"audit":      len(m.Audit),
	}
}

func RetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 0
	case 2:
		return 10 * time.Second
	case 3:
		return 30 * time.Second
	case 4:
		return 2 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func NewID() string { return id.New() }

func Require(cond bool, msg string) error {
	if !cond {
		return fmt.Errorf(msg)
	}
	return nil
}

var _ Store = (*Memory)(nil)
