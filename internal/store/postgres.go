package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PG struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, dsn string) (*PG, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &PG{pool: pool}, nil
}

func (p *PG) Close() { p.pool.Close() }

func (p *PG) ctx() context.Context { return context.Background() }

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (p *PG) PutUser(u *User) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO users (id, username, email, password_hash, display_name, status, totp_enabled, must_change_password, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,COALESCE($9, now()), now())
		ON CONFLICT (id) DO UPDATE SET
			username=EXCLUDED.username, email=EXCLUDED.email, password_hash=EXCLUDED.password_hash,
			display_name=EXCLUDED.display_name, status=EXCLUDED.status, totp_enabled=EXCLUDED.totp_enabled,
			must_change_password=EXCLUDED.must_change_password, updated_at=now()`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.DisplayName, u.Status, u.TOTPEnabled, u.MustChangePassword, u.CreatedAt)
	p.ensureRoles(u.Roles)
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM user_roles WHERE user_id=$1`, u.ID)
	for _, role := range u.Roles {
		_, _ = p.pool.Exec(p.ctx(), `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name=$2`, u.ID, role)
	}
}

func (p *PG) ensureRoles(names []string) {
	for _, n := range names {
		_, _ = p.pool.Exec(p.ctx(), `INSERT INTO roles (id, name) VALUES ($1,$2) ON CONFLICT (name) DO NOTHING`, id.New(), n)
	}
}

func (p *PG) scanUser(row pgx.Row) *User {
	u := &User{}
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Status, &u.TOTPEnabled, &u.MustChangePassword, &u.CreatedAt); err != nil {
		return nil
	}
	rows, err := p.pool.Query(p.ctx(), `SELECT r.name FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=$1`, u.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			_ = rows.Scan(&name)
			u.Roles = append(u.Roles, name)
		}
	}
	return u
}

func (p *PG) UserByUsername(name string) *User {
	return p.scanUser(p.pool.QueryRow(p.ctx(), `
		SELECT id, username, email, password_hash, display_name, status, totp_enabled, must_change_password, created_at
		FROM users WHERE username=$1`, name))
}

func (p *PG) UserByID(uid string) *User {
	return p.scanUser(p.pool.QueryRow(p.ctx(), `
		SELECT id, username, email, password_hash, display_name, status, totp_enabled, must_change_password, created_at
		FROM users WHERE id=$1`, uid))
}

func (p *PG) PutSession(s *Session) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at, source_ip, user_agent, impersonator_id, impersonation_reason)
		VALUES ($1,$2,$3,now(),$4,NULLIF($5,'')::inet,$6,NULLIF($7,'')::uuid,$8)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, s.SourceIP, s.UserAgent, s.ImpersonatorID, s.ImpersonationReason)
}

func (p *PG) SessionByHash(hash []byte) *Session {
	s := &Session{}
	var ip *string
	err := p.pool.QueryRow(p.ctx(), `
		SELECT id, user_id, token_hash, expires_at, revoked_at, COALESCE(source_ip::text,''), COALESCE(user_agent,''), COALESCE(impersonator_id::text,''), COALESCE(impersonation_reason,'')
		FROM sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at > now()`, hash).
		Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.RevokedAt, &s.SourceIP, &s.UserAgent, &s.ImpersonatorID, &s.ImpersonationReason)
	if err != nil {
		return nil
	}
	_ = ip
	return s
}

func (p *PG) RevokeSession(sid string) {
	_, _ = p.pool.Exec(p.ctx(), `UPDATE sessions SET revoked_at=now() WHERE id=$1`, sid)
}

func (p *PG) PutFeature(f *FeatureSet) {
	b, _ := json.Marshal(f.Features)
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO feature_sets (id, name, features) VALUES ($1,$2,$3)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, features=EXCLUDED.features`, f.ID, f.Name, b)
}

func (p *PG) ListFeatureSets() []FeatureSet {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, name, features FROM feature_sets ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []FeatureSet
	for rows.Next() {
		var f FeatureSet
		var raw []byte
		_ = rows.Scan(&f.ID, &f.Name, &raw)
		_ = json.Unmarshal(raw, &f.Features)
		out = append(out, f)
	}
	return out
}

func (p *PG) PutPackage(pkg *Package) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO packages (id, reseller_id, name, feature_set_id, disk_bytes, bandwidth_bytes_monthly,
			domains, subdomains, alias_domains, databases, database_users, mailboxes, mailbox_storage_bytes,
			ftp_users, cron_jobs, application_instances, backup_retention_days, cpu_percent, memory_bytes,
			process_limit, io_weight, iops, concurrent_web_requests, email_daily_limit)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, disk_bytes=EXCLUDED.disk_bytes, cpu_percent=EXCLUDED.cpu_percent, memory_bytes=EXCLUDED.memory_bytes`,
		pkg.ID, pkg.ResellerID, pkg.Name, pkg.FeatureSetID, pkg.DiskBytes, pkg.BandwidthBytesMonthly,
		pkg.Domains, pkg.Subdomains, pkg.AliasDomains, pkg.Databases, pkg.DatabaseUsers, pkg.Mailboxes, pkg.MailboxStorageBytes,
		pkg.FTPUsers, pkg.CronJobs, pkg.ApplicationInstances, pkg.BackupRetentionDays, pkg.CPUPercent, pkg.MemoryBytes,
		pkg.ProcessLimit, pkg.IOWeight, pkg.IOPS, pkg.ConcurrentWebRequests, pkg.EmailDailyLimit)
}

type scanner interface {
	Scan(dest ...any) error
}

func (p *PG) scanPackage(row scanner) *Package {
	pkg := &Package{}
	var reseller *string
	if err := row.Scan(&pkg.ID, &reseller, &pkg.Name, &pkg.FeatureSetID, &pkg.DiskBytes, &pkg.BandwidthBytesMonthly,
		&pkg.Domains, &pkg.Subdomains, &pkg.AliasDomains, &pkg.Databases, &pkg.DatabaseUsers, &pkg.Mailboxes, &pkg.MailboxStorageBytes,
		&pkg.FTPUsers, &pkg.CronJobs, &pkg.ApplicationInstances, &pkg.BackupRetentionDays, &pkg.CPUPercent, &pkg.MemoryBytes,
		&pkg.ProcessLimit, &pkg.IOWeight, &pkg.IOPS, &pkg.ConcurrentWebRequests, &pkg.EmailDailyLimit); err != nil {
		return nil
	}
	if reseller != nil {
		pkg.ResellerID = *reseller
	}
	return pkg
}

func (p *PG) GetPackage(pid string) *Package {
	return p.scanPackage(p.pool.QueryRow(p.ctx(), `SELECT id, reseller_id::text, name, feature_set_id, disk_bytes, bandwidth_bytes_monthly,
		domains, subdomains, alias_domains, databases, database_users, mailboxes, mailbox_storage_bytes,
		ftp_users, cron_jobs, application_instances, backup_retention_days, cpu_percent, memory_bytes,
		process_limit, io_weight, iops, concurrent_web_requests, email_daily_limit FROM packages WHERE id=$1`, pid))
}

func (p *PG) ListPackages() []Package {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, reseller_id::text, name, feature_set_id, disk_bytes, bandwidth_bytes_monthly,
		domains, subdomains, alias_domains, databases, database_users, mailboxes, mailbox_storage_bytes,
		ftp_users, cron_jobs, application_instances, backup_retention_days, cpu_percent, memory_bytes,
		process_limit, io_weight, iops, concurrent_web_requests, email_daily_limit FROM packages ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		if pkg := p.scanPackage(rows); pkg != nil {
			out = append(out, *pkg)
		}
	}
	return out
}

func (p *PG) PutReseller(r *Reseller) {
	if r.PrivilegeMask == nil {
		r.PrivilegeMask = []string{}
	}
	if r.Nameservers == nil {
		r.Nameservers = []string{}
	}
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO resellers (id, user_id, name, brand_name, privilege_mask, nameservers, status)
		VALUES ($1,$2,$3,$4,$5,$6,COALESCE(NULLIF($7,''),'active'))
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, status=EXCLUDED.status`,
		r.ID, r.UserID, r.Name, r.BrandName, r.PrivilegeMask, r.Nameservers, r.Status)
}

func (p *PG) GetReseller(rid string) *Reseller {
	r := &Reseller{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, user_id, name, COALESCE(brand_name,''), privilege_mask, nameservers, status FROM resellers WHERE id=$1`, rid).
		Scan(&r.ID, &r.UserID, &r.Name, &r.BrandName, &r.PrivilegeMask, &r.Nameservers, &r.Status); err != nil {
		return nil
	}
	return r
}

func (p *PG) ResellerByUser(userID string) *Reseller {
	r := &Reseller{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, user_id, name, COALESCE(brand_name,''), privilege_mask, nameservers, status FROM resellers WHERE user_id=$1`, userID).
		Scan(&r.ID, &r.UserID, &r.Name, &r.BrandName, &r.PrivilegeMask, &r.Nameservers, &r.Status); err != nil {
		return nil
	}
	return r
}

func (p *PG) ListResellers() []Reseller {
	rows, _ := p.pool.Query(p.ctx(), `SELECT id, user_id, name, COALESCE(brand_name,''), privilege_mask, nameservers, status FROM resellers`)
	if rows == nil {
		return nil
	}
	defer rows.Close()
	out := []Reseller{}
	for rows.Next() {
		var r Reseller
		_ = rows.Scan(&r.ID, &r.UserID, &r.Name, &r.BrandName, &r.PrivilegeMask, &r.Nameservers, &r.Status)
		out = append(out, r)
	}
	return out
}

func (p *PG) AllocUID() int {
	var uid int
	_ = p.pool.QueryRow(p.ctx(), `UPDATE id_allocators SET next_value=next_value+1 WHERE name='linux_uid' RETURNING next_value-1`).Scan(&uid)
	if uid == 0 {
		uid = 20000
	}
	return uid
}

func (p *PG) PutAccount(a *Account) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO accounts (id, reseller_id, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address, shell_class, login_disabled, desired_revision, observed_revision)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11,'')::inet, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			reseller_id=EXCLUDED.reseller_id, package_id=EXCLUDED.package_id, status=EXCLUDED.status,
			primary_domain=EXCLUDED.primary_domain, ip_address=EXCLUDED.ip_address, login_disabled=EXCLUDED.login_disabled,
			desired_revision=EXCLUDED.desired_revision, observed_revision=EXCLUDED.observed_revision, updated_at=now()`,
		a.ID, a.ResellerID, a.OwnerUserID, a.Username, a.PrimaryDomain, a.LinuxUID, a.LinuxGID, a.PackageID, a.Status, a.HomePath, a.IPAddress, a.ShellClass, a.LoginDisabled, a.DesiredRevision, a.ObservedRevision)
}

func (p *PG) scanAccount(row scanner) *Account {
	a := &Account{}
	var reseller, ip *string
	if err := row.Scan(&a.ID, &reseller, &a.OwnerUserID, &a.Username, &a.PrimaryDomain, &a.LinuxUID, &a.LinuxGID, &a.PackageID, &a.Status, &a.HomePath, &ip, &a.ShellClass, &a.LoginDisabled, &a.DesiredRevision, &a.ObservedRevision); err != nil {
		return nil
	}
	if reseller != nil {
		a.ResellerID = *reseller
	}
	if ip != nil {
		a.IPAddress = *ip
	}
	return a
}

func (p *PG) GetAccount(aid string) *Account {
	return p.scanAccount(p.pool.QueryRow(p.ctx(), `SELECT id, reseller_id::text, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address::text, shell_class, login_disabled, desired_revision, observed_revision FROM accounts WHERE id=$1`, aid))
}

func (p *PG) AccountByUsername(name string) *Account {
	return p.scanAccount(p.pool.QueryRow(p.ctx(), `SELECT id, reseller_id::text, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address::text, shell_class, login_disabled, desired_revision, observed_revision FROM accounts WHERE username=$1`, name))
}

func (p *PG) ListAccounts(q, status string) []Account {
	q = strings.ToLower(q)
	rows, err := p.pool.Query(p.ctx(), `SELECT id, reseller_id::text, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address::text, shell_class, login_disabled, desired_revision, observed_revision
		FROM accounts
		WHERE ($1='' OR status=$1)
		  AND ($2='' OR username ILIKE '%'||$2||'%' OR primary_domain ILIKE '%'||$2||'%')
		ORDER BY username`, status, q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		if a := p.scanAccount(rows); a != nil {
			out = append(out, *a)
		}
	}
	return out
}

func (p *PG) AddMember(accountID, userID string) {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO account_members (account_id, user_id, role) VALUES ($1,$2,'customer_owner') ON CONFLICT DO NOTHING`, accountID, userID)
}

func (p *PG) AccountsForUser(userID string) []string {
	rows, err := p.pool.Query(p.ctx(), `
		SELECT account_id::text FROM account_members WHERE user_id=$1
		UNION SELECT id::text FROM accounts WHERE owner_user_id=$1`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

func (p *PG) DriftedAccounts() []Account {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, reseller_id::text, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address::text, shell_class, login_disabled, desired_revision, observed_revision
		FROM accounts WHERE desired_revision > observed_revision AND status <> 'terminated'`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		if a := p.scanAccount(rows); a != nil {
			out = append(out, *a)
		}
	}
	return out
}

func (p *PG) DomainTaken(ascii string) bool {
	var n int
	_ = p.pool.QueryRow(p.ctx(), `SELECT COUNT(*) FROM domains WHERE ascii_fqdn=$1`, ascii).Scan(&n)
	return n > 0
}

func (p *PG) PutDomain(d *Domain) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO domains (id, account_id, fqdn, ascii_fqdn, type, document_root, dns_managed, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, document_root=EXCLUDED.document_root`,
		d.ID, d.AccountID, d.FQDN, d.ASCII, d.Type, d.DocumentRoot, d.DNSManaged, d.Status)
}

func (p *PG) GetDomain(did string) *Domain {
	d := &Domain{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, fqdn, ascii_fqdn, type, COALESCE(document_root,''), dns_managed, status FROM domains WHERE id=$1`, did).
		Scan(&d.ID, &d.AccountID, &d.FQDN, &d.ASCII, &d.Type, &d.DocumentRoot, &d.DNSManaged, &d.Status); err != nil {
		return nil
	}
	return d
}

func (p *PG) ListDomains(accountID string) []Domain {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, fqdn, ascii_fqdn, type, COALESCE(document_root,''), dns_managed, status FROM domains WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Domain
	for rows.Next() {
		var d Domain
		_ = rows.Scan(&d.ID, &d.AccountID, &d.FQDN, &d.ASCII, &d.Type, &d.DocumentRoot, &d.DNSManaged, &d.Status)
		out = append(out, d)
	}
	return out
}

func (p *PG) PutWebsite(w *Website) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO websites (id, account_id, domain_id, runtime, runtime_version, document_root, https_redirect, www_redirect, proxy_target, enabled, desired_revision, observed_revision)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE(NULLIF($8,''),'none'),$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET runtime=EXCLUDED.runtime, enabled=EXCLUDED.enabled, https_redirect=EXCLUDED.https_redirect, document_root=EXCLUDED.document_root, desired_revision=EXCLUDED.desired_revision, observed_revision=EXCLUDED.observed_revision`,
		w.ID, w.AccountID, w.DomainID, w.Runtime, w.RuntimeVersion, w.DocumentRoot, w.HTTPSRedirect, w.WWWRedirect, w.ProxyTarget, w.Enabled, w.DesiredRevision, w.ObservedRevision)
}

func (p *PG) scanWebsite(row scanner) *Website {
	w := &Website{}
	if err := row.Scan(&w.ID, &w.AccountID, &w.DomainID, &w.Runtime, &w.RuntimeVersion, &w.DocumentRoot, &w.HTTPSRedirect, &w.WWWRedirect, &w.ProxyTarget, &w.Enabled, &w.DesiredRevision, &w.ObservedRevision); err != nil {
		return nil
	}
	return w
}

func (p *PG) GetWebsite(wid string) *Website {
	return p.scanWebsite(p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, runtime, COALESCE(runtime_version,''), document_root, https_redirect, www_redirect, COALESCE(proxy_target,''), enabled, desired_revision, observed_revision FROM websites WHERE id=$1`, wid))
}

func (p *PG) ListWebsites(accountID string) []Website {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, domain_id, runtime, COALESCE(runtime_version,''), document_root, https_redirect, www_redirect, COALESCE(proxy_target,''), enabled, desired_revision, observed_revision FROM websites WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Website
	for rows.Next() {
		if w := p.scanWebsite(rows); w != nil {
			out = append(out, *w)
		}
	}
	return out
}

func (p *PG) PutApp(a *Application) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO applications (id, website_id, account_id, runtime, runtime_version, working_directory, start_command, listen_target, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, start_command=EXCLUDED.start_command`,
		a.ID, a.WebsiteID, a.AccountID, a.Runtime, a.RuntimeVersion, a.WorkingDirectory, a.StartCommand, a.ListenTarget, a.Status)
}

func (p *PG) GetApp(aid string) *Application {
	a := &Application{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, website_id, account_id, runtime, runtime_version, working_directory, start_command, listen_target, status FROM applications WHERE id=$1`, aid).
		Scan(&a.ID, &a.WebsiteID, &a.AccountID, &a.Runtime, &a.RuntimeVersion, &a.WorkingDirectory, &a.StartCommand, &a.ListenTarget, &a.Status); err != nil {
		return nil
	}
	return a
}

func (p *PG) ListApps(accountID string) []Application {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, website_id, account_id, runtime, runtime_version, working_directory, start_command, listen_target, status FROM applications WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		var a Application
		_ = rows.Scan(&a.ID, &a.WebsiteID, &a.AccountID, &a.Runtime, &a.RuntimeVersion, &a.WorkingDirectory, &a.StartCommand, &a.ListenTarget, &a.Status)
		out = append(out, a)
	}
	return out
}

func (p *PG) PutDB(d *HostedDatabase) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO hosted_databases (id, account_id, server_id, name, engine, status)
		VALUES ($1,$2, COALESCE((SELECT id FROM database_servers WHERE engine=$4 LIMIT 1), $1), $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status`,
		d.ID, d.AccountID, d.Name, d.Engine, d.Status)
}

func (p *PG) GetDB(did string) *HostedDatabase {
	d := &HostedDatabase{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, name, engine, status FROM hosted_databases WHERE id=$1`, did).
		Scan(&d.ID, &d.AccountID, &d.Name, &d.Engine, &d.Status); err != nil {
		return nil
	}
	return d
}

func (p *PG) ListDBs(accountID string) []HostedDatabase {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, name, engine, status FROM hosted_databases WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HostedDatabase
	for rows.Next() {
		var d HostedDatabase
		_ = rows.Scan(&d.ID, &d.AccountID, &d.Name, &d.Engine, &d.Status)
		out = append(out, d)
	}
	return out
}

func (p *PG) DeleteDB(id string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM hosted_databases WHERE id=$1`, id)
}

func (p *PG) PutDBUser(u *DatabaseUser) {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO database_users (id, account_id, username, engine, password_enc) VALUES ($1,$2,$3,$4,'') ON CONFLICT (id) DO NOTHING`, u.ID, u.AccountID, u.Username, u.Engine)
}

func (p *PG) ListDBUsers(accountID string) []DatabaseUser {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, username, engine FROM database_users WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DatabaseUser
	for rows.Next() {
		var u DatabaseUser
		_ = rows.Scan(&u.ID, &u.AccountID, &u.Username, &u.Engine)
		out = append(out, u)
	}
	return out
}

func (p *PG) PutZone(z *DNSZone) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO dns_zones (id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET desired_revision=EXCLUDED.desired_revision, observed_revision=EXCLUDED.observed_revision, dnssec_enabled=EXCLUDED.dnssec_enabled`,
		z.ID, z.AccountID, z.DomainID, z.Name, z.DNSSECEnabled, z.Provider, z.DesiredRevision, z.ObservedRevision)
}

func (p *PG) GetZone(zid string) *DNSZone {
	z := &DNSZone{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision FROM dns_zones WHERE id=$1`, zid).
		Scan(&z.ID, &z.AccountID, &z.DomainID, &z.Name, &z.DNSSECEnabled, &z.Provider, &z.DesiredRevision, &z.ObservedRevision); err != nil {
		return nil
	}
	return z
}

func (p *PG) ZoneByDomain(domainID string) *DNSZone {
	z := &DNSZone{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision FROM dns_zones WHERE domain_id=$1`, domainID).
		Scan(&z.ID, &z.AccountID, &z.DomainID, &z.Name, &z.DNSSECEnabled, &z.Provider, &z.DesiredRevision, &z.ObservedRevision); err != nil {
		return nil
	}
	return z
}

func (p *PG) ListZones(accountID string) []DNSZone {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision FROM dns_zones WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DNSZone
	for rows.Next() {
		var z DNSZone
		_ = rows.Scan(&z.ID, &z.AccountID, &z.DomainID, &z.Name, &z.DNSSECEnabled, &z.Provider, &z.DesiredRevision, &z.ObservedRevision)
		out = append(out, z)
	}
	return out
}

func (p *PG) PutRecord(r *DNSRecord) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO dns_records (id, zone_id, name, type, content, ttl, priority)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET content=EXCLUDED.content, ttl=EXCLUDED.ttl`,
		r.ID, r.ZoneID, r.Name, r.Type, r.Content, r.TTL, r.Priority)
}

func (p *PG) ListRecords(zoneID string) []DNSRecord {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, zone_id, name, type, content, ttl, priority FROM dns_records WHERE zone_id=$1`, zoneID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DNSRecord
	for rows.Next() {
		var r DNSRecord
		_ = rows.Scan(&r.ID, &r.ZoneID, &r.Name, &r.Type, &r.Content, &r.TTL, &r.Priority)
		out = append(out, r)
	}
	return out
}

func (p *PG) DeleteRecord(rid string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM dns_records WHERE id=$1`, rid)
}

func (p *PG) PutMailDomain(d *MailDomain) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO mail_domains (id, account_id, domain_id, catchall_policy, status)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, catchall_policy=EXCLUDED.catchall_policy`,
		d.ID, d.AccountID, d.DomainID, d.CatchallPolicy, d.Status)
}

func (p *PG) MailDomainByDomain(domainID string) *MailDomain {
	d := &MailDomain{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, catchall_policy, status FROM mail_domains WHERE domain_id=$1`, domainID).
		Scan(&d.ID, &d.AccountID, &d.DomainID, &d.CatchallPolicy, &d.Status); err != nil {
		return nil
	}
	return d
}

func (p *PG) ListMailDomains(accountID string) []MailDomain {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, domain_id, catchall_policy, status FROM mail_domains WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []MailDomain
	for rows.Next() {
		var d MailDomain
		_ = rows.Scan(&d.ID, &d.AccountID, &d.DomainID, &d.CatchallPolicy, &d.Status)
		out = append(out, d)
	}
	return out
}

func (p *PG) PutMailbox(mb *Mailbox) {
	hash := mb.PasswordHash
	if hash == "" {
		hash = "!"
	}
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO mailboxes (id, account_id, domain_id, local_part, quota_bytes, password_hash, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, quota_bytes=EXCLUDED.quota_bytes, password_hash=EXCLUDED.password_hash`,
		mb.ID, mb.AccountID, mb.DomainID, mb.LocalPart, mb.QuotaBytes, hash, mb.Status)
}

func (p *PG) GetMailbox(mid string) *Mailbox {
	mb := &Mailbox{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, local_part, quota_bytes, password_hash, status FROM mailboxes WHERE id=$1`, mid).
		Scan(&mb.ID, &mb.AccountID, &mb.DomainID, &mb.LocalPart, &mb.QuotaBytes, &mb.PasswordHash, &mb.Status); err != nil {
		return nil
	}
	return mb
}

func (p *PG) ListMailboxes(accountID string) []Mailbox {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, domain_id, local_part, quota_bytes, password_hash, status FROM mailboxes WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Mailbox
	for rows.Next() {
		var mb Mailbox
		_ = rows.Scan(&mb.ID, &mb.AccountID, &mb.DomainID, &mb.LocalPart, &mb.QuotaBytes, &mb.PasswordHash, &mb.Status)
		out = append(out, mb)
	}
	return out
}

func (p *PG) DeleteMailbox(id string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM mailboxes WHERE id=$1`, id)
}

func (p *PG) PutMailAlias(a *MailAlias) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO mail_aliases (id, account_id, domain_id, address, destination)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET address=EXCLUDED.address, destination=EXCLUDED.destination`,
		a.ID, a.AccountID, a.DomainID, a.Address, a.Destination)
}

func (p *PG) GetMailAlias(id string) *MailAlias {
	a := &MailAlias{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, domain_id, address, destination FROM mail_aliases WHERE id=$1`, id).
		Scan(&a.ID, &a.AccountID, &a.DomainID, &a.Address, &a.Destination); err != nil {
		return nil
	}
	return a
}

func (p *PG) ListMailAliases(accountID string) []MailAlias {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, domain_id, address, destination FROM mail_aliases WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []MailAlias
	for rows.Next() {
		var a MailAlias
		_ = rows.Scan(&a.ID, &a.AccountID, &a.DomainID, &a.Address, &a.Destination)
		out = append(out, a)
	}
	return out
}

func (p *PG) DeleteMailAlias(id string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM mail_aliases WHERE id=$1`, id)
}

func (p *PG) PutCert(c *Certificate) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO certificates (id, account_id, hostname, kind, status, not_after, issuer)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, not_after=EXCLUDED.not_after, issuer=EXCLUDED.issuer`,
		c.ID, c.AccountID, c.Hostname, c.Kind, c.Status, c.NotAfter, c.Issuer)
}

func (p *PG) GetCert(cid string) *Certificate {
	c := &Certificate{}
	var acc *string
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id::text, hostname, kind, status, not_after, COALESCE(issuer,'') FROM certificates WHERE id=$1`, cid).
		Scan(&c.ID, &acc, &c.Hostname, &c.Kind, &c.Status, &c.NotAfter, &c.Issuer); err != nil {
		return nil
	}
	if acc != nil {
		c.AccountID = *acc
	}
	return c
}

func (p *PG) ListCerts(accountID string) []Certificate {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id::text, hostname, kind, status, not_after, COALESCE(issuer,'') FROM certificates WHERE $1='' OR account_id::text=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Certificate
	for rows.Next() {
		var c Certificate
		var acc *string
		_ = rows.Scan(&c.ID, &acc, &c.Hostname, &c.Kind, &c.Status, &c.NotAfter, &c.Issuer)
		if acc != nil {
			c.AccountID = *acc
		}
		out = append(out, c)
	}
	return out
}

func (p *PG) EnqueueJob(j *Job) (*Job, error) {
	if j.IdempotencyKey != "" {
		if existing := p.jobByIdem(j.IdempotencyKey); existing != nil {
			return existing, nil
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
	if j.Logs == nil {
		j.Logs = []string{}
	}
	payload, _ := json.Marshal(j.Payload)
	_, err := p.pool.Exec(p.ctx(), `
		INSERT INTO jobs (id, type, resource_type, resource_id, payload, state, priority, attempts, max_attempts, progress, run_after, idempotency_key, actor_id, request_id, created_at, logs)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),NULLIF($13,'')::uuid,NULLIF($14,'')::uuid,$15,$16)`,
		j.ID, j.Type, j.ResourceType, j.ResourceID, payload, j.State, j.Priority, j.Attempts, j.MaxAttempts, j.Progress, j.RunAfter, j.IdempotencyKey, j.ActorID, j.RequestID, j.CreatedAt, j.Logs)
	if err != nil {
		if existing := p.jobByIdem(j.IdempotencyKey); existing != nil {
			return existing, nil
		}
		return nil, err
	}
	cp := *j
	return &cp, nil
}

func (p *PG) jobByIdem(key string) *Job {
	if key == "" {
		return nil
	}
	return p.scanJob(p.pool.QueryRow(p.ctx(), jobSelect+" WHERE idempotency_key=$1", key))
}

const jobSelect = `SELECT id, type, COALESCE(resource_type,''), COALESCE(resource_id::text,''), payload, state, priority, attempts, max_attempts, progress, run_after, COALESCE(locked_by,''), heartbeat_at, COALESCE(idempotency_key,''), COALESCE(last_error::text,''), COALESCE(actor_id::text,''), COALESCE(request_id::text,''), created_at, started_at, finished_at, logs FROM jobs`

func (p *PG) scanJob(row scanner) *Job {
	j := &Job{}
	var payload []byte
	if err := row.Scan(&j.ID, &j.Type, &j.ResourceType, &j.ResourceID, &payload, &j.State, &j.Priority, &j.Attempts, &j.MaxAttempts, &j.Progress, &j.RunAfter, &j.LockedBy, &j.HeartbeatAt, &j.IdempotencyKey, &j.LastError, &j.ActorID, &j.RequestID, &j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.Logs); err != nil {
		return nil
	}
	_ = json.Unmarshal(payload, &j.Payload)
	return j
}

func (p *PG) ClaimJob(worker string) *Job {
	tx, err := p.pool.Begin(p.ctx())
	if err != nil {
		return nil
	}
	defer tx.Rollback(p.ctx())
	row := tx.QueryRow(p.ctx(), jobSelect+`
		WHERE state IN ('queued','retrying') AND run_after <= now()
		ORDER BY priority, created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1`)
	j := p.scanJob(row)
	if j == nil {
		return nil
	}
	now := time.Now().UTC()
	j.State = "running"
	j.LockedBy = worker
	j.StartedAt = &now
	j.HeartbeatAt = &now
	j.Attempts++
	if _, err := tx.Exec(p.ctx(), `UPDATE jobs SET state='running', locked_by=$2, locked_at=now(), heartbeat_at=now(), started_at=now(), attempts=$3 WHERE id=$1`, j.ID, worker, j.Attempts); err != nil {
		return nil
	}
	if err := tx.Commit(p.ctx()); err != nil {
		return nil
	}
	return j
}

func (p *PG) UpdateJob(j *Job) {
	payload, _ := json.Marshal(j.Payload)
	var last any
	if j.LastError != "" {
		b, _ := json.Marshal(map[string]string{"error": j.LastError})
		last = b
	}
	_, _ = p.pool.Exec(p.ctx(), `
		UPDATE jobs SET type=$2, payload=$3, state=$4, priority=$5, attempts=$6, progress=$7, run_after=$8, locked_by=NULLIF($9,''), heartbeat_at=$10, last_error=$11, started_at=$12, finished_at=$13, logs=$14
		WHERE id=$1`,
		j.ID, j.Type, payload, j.State, j.Priority, j.Attempts, j.Progress, j.RunAfter, j.LockedBy, j.HeartbeatAt, last, j.StartedAt, j.FinishedAt, j.Logs)
}

func (p *PG) GetJob(jid string) *Job {
	return p.scanJob(p.pool.QueryRow(p.ctx(), jobSelect+" WHERE id=$1", jid))
}

func (p *PG) ListJobs(state string, limit int) []Job {
	if limit <= 0 {
		limit = 100
	}
	rows, err := p.pool.Query(p.ctx(), jobSelect+` WHERE ($1='' OR state=$1) ORDER BY created_at DESC LIMIT $2`, state, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		if j := p.scanJob(rows); j != nil {
			out = append(out, *j)
		}
	}
	return out
}

func (p *PG) AppendAudit(e AuditEvent) {
	if e.ID == "" {
		e.ID = id.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	before, _ := json.Marshal(e.Before)
	after, _ := json.Marshal(e.After)
	meta, _ := json.Marshal(e.Metadata)
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO audit_events (id, occurred_at, actor_type, actor_id, effective_actor_id, account_id, action, resource_type, resource_id, source_ip, user_agent, request_id, success, before_state, after_state, metadata)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,NULLIF($9,'')::uuid,NULLIF($10,'')::inet,$11,$12,$13,$14,$15,$16)`,
		e.ID, e.OccurredAt, e.ActorType, e.ActorID, e.EffectiveActor, e.AccountID, e.Action, e.ResourceType, e.ResourceID, e.SourceIP, e.UserAgent, e.RequestID, e.Success, before, after, meta)
}

func (p *PG) ListAudit(limit int) []AuditEvent {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, occurred_at, actor_type, COALESCE(actor_id::text,''), COALESCE(effective_actor_id::text,''), COALESCE(account_id::text,''), action, COALESCE(resource_type,''), COALESCE(resource_id::text,''), COALESCE(source_ip::text,''), COALESCE(user_agent,''), request_id, success, before_state, after_state, metadata
		FROM audit_events ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var before, after, meta []byte
		_ = rows.Scan(&e.ID, &e.OccurredAt, &e.ActorType, &e.ActorID, &e.EffectiveActor, &e.AccountID, &e.Action, &e.ResourceType, &e.ResourceID, &e.SourceIP, &e.UserAgent, &e.RequestID, &e.Success, &before, &after, &meta)
		_ = json.Unmarshal(before, &e.Before)
		_ = json.Unmarshal(after, &e.After)
		_ = json.Unmarshal(meta, &e.Metadata)
		out = append(out, e)
	}
	return out
}

func (p *PG) PutToken(t *APIToken) {
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO api_tokens (id, user_id, name, prefix, token_hash, scope, account_id, capabilities)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,$8)`,
		t.ID, t.UserID, t.Name, t.Prefix, t.TokenHash, t.Scope, t.AccountID, t.Capabilities)
}

func (p *PG) TokenByHash(hash []byte) *APIToken {
	t := &APIToken{}
	var acc *string
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, user_id, name, prefix, token_hash, scope, account_id::text, capabilities, expires_at, revoked_at FROM api_tokens WHERE token_hash=$1 AND revoked_at IS NULL`, hash).
		Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.TokenHash, &t.Scope, &acc, &t.Capabilities, &t.ExpiresAt, &t.RevokedAt); err != nil {
		return nil
	}
	if acc != nil {
		t.AccountID = *acc
	}
	return t
}

func (p *PG) ListTokens(userID string) []APIToken {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, user_id, name, prefix, scope, COALESCE(account_id::text,''), capabilities FROM api_tokens WHERE user_id=$1`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var t APIToken
		_ = rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.Scope, &t.AccountID, &t.Capabilities)
		out = append(out, t)
	}
	return out
}

func (p *PG) PutBackup(b *BackupRun) {
	man, _ := json.Marshal(b.Manifest)
	_, _ = p.pool.Exec(p.ctx(), `
		INSERT INTO backup_runs (id, account_id, kind, state, destination, checksum, size_bytes, created_at, finished_at, manifest)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET state=EXCLUDED.state, checksum=EXCLUDED.checksum, finished_at=EXCLUDED.finished_at, manifest=EXCLUDED.manifest`,
		b.ID, b.AccountID, b.Kind, b.State, b.Destination, b.Checksum, b.SizeBytes, b.CreatedAt, b.FinishedAt, man)
}

func (p *PG) GetBackup(bid string) *BackupRun {
	b := &BackupRun{}
	var man []byte
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, kind, state, destination, COALESCE(checksum,''), COALESCE(size_bytes,0), created_at, finished_at, manifest FROM backup_runs WHERE id=$1`, bid).
		Scan(&b.ID, &b.AccountID, &b.Kind, &b.State, &b.Destination, &b.Checksum, &b.SizeBytes, &b.CreatedAt, &b.FinishedAt, &man); err != nil {
		return nil
	}
	_ = json.Unmarshal(man, &b.Manifest)
	return b
}

func (p *PG) ListBackups(accountID string) []BackupRun {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, kind, state, destination, COALESCE(checksum,''), COALESCE(size_bytes,0), created_at, finished_at FROM backup_runs WHERE $1='' OR account_id::text=$1 ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		var b BackupRun
		_ = rows.Scan(&b.ID, &b.AccountID, &b.Kind, &b.State, &b.Destination, &b.Checksum, &b.SizeBytes, &b.CreatedAt, &b.FinishedAt)
		out = append(out, b)
	}
	return out
}

func (p *PG) PutCron(c *CronJob) {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO cron_jobs (id, account_id, schedule, command, working_directory, enabled) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET enabled=EXCLUDED.enabled`,
		c.ID, c.AccountID, c.Schedule, c.Command, c.WorkingDirectory, c.Enabled)
}

func (p *PG) ListCrons(accountID string) []CronJob {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, schedule, command, working_directory, enabled FROM cron_jobs WHERE account_id=$1`, accountID)
	if err != nil {
		return []CronJob{}
	}
	defer rows.Close()
	out := []CronJob{}
	for rows.Next() {
		var c CronJob
		_ = rows.Scan(&c.ID, &c.AccountID, &c.Schedule, &c.Command, &c.WorkingDirectory, &c.Enabled)
		out = append(out, c)
	}
	return out
}

func (p *PG) PutSSH(k *SSHKey) {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO ssh_keys (id, account_id, label, public_key, fingerprint, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`,
		k.ID, k.AccountID, k.Label, k.PublicKey, k.Fingerprint, k.CreatedAt)
}

func (p *PG) GetSSH(id string) *SSHKey {
	k := &SSHKey{}
	if err := p.pool.QueryRow(p.ctx(), `SELECT id, account_id, label, public_key, fingerprint, created_at FROM ssh_keys WHERE id=$1`, id).
		Scan(&k.ID, &k.AccountID, &k.Label, &k.PublicKey, &k.Fingerprint, &k.CreatedAt); err != nil {
		return nil
	}
	return k
}

func (p *PG) DeleteSSH(id string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM ssh_keys WHERE id=$1`, id)
}

func (p *PG) ListSSH(accountID string) []SSHKey {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, label, public_key, fingerprint, created_at FROM ssh_keys WHERE account_id=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []SSHKey
	for rows.Next() {
		var k SSHKey
		_ = rows.Scan(&k.ID, &k.AccountID, &k.Label, &k.PublicKey, &k.Fingerprint, &k.CreatedAt)
		out = append(out, k)
	}
	return out
}

func (p *PG) PutFTP(f *FTPAccount) {
	hash := f.PasswordHash
	if hash == "" {
		hash = "!"
	}
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO ftp_accounts (id, account_id, username, home_path, password_hash, status)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET username=EXCLUDED.username, home_path=EXCLUDED.home_path,
			password_hash=EXCLUDED.password_hash, status=EXCLUDED.status`,
		f.ID, f.AccountID, f.Username, f.HomePath, hash, f.Status)
}

func (p *PG) scanFTP(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) []FTPAccount {
	var out []FTPAccount
	for rows.Next() {
		var f FTPAccount
		_ = rows.Scan(&f.ID, &f.AccountID, &f.Username, &f.HomePath, &f.PasswordHash, &f.Status)
		out = append(out, f)
	}
	return out
}

func (p *PG) ListFTP(accountID string) []FTPAccount {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, username, home_path, password_hash, status FROM ftp_accounts WHERE account_id=$1`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return p.scanFTP(rows)
}

func (p *PG) ListAllFTP() []FTPAccount {
	rows, err := p.pool.Query(p.ctx(), `SELECT id, account_id, username, home_path, password_hash, status FROM ftp_accounts`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return p.scanFTP(rows)
}

func (p *PG) FTPUsernameTaken(username, exceptID string) bool {
	var n int
	err := p.pool.QueryRow(p.ctx(), `SELECT COUNT(1) FROM ftp_accounts WHERE username=$1 AND id<>COALESCE(NULLIF($2,'')::uuid, '00000000-0000-0000-0000-000000000000')`, username, exceptID).Scan(&n)
	return err == nil && n > 0
}

func (p *PG) DeleteFTP(id string) {
	_, _ = p.pool.Exec(p.ctx(), `DELETE FROM ftp_accounts WHERE id=$1`, id)
}

func (p *PG) PutUsage(u *Usage) {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO resource_usage (id, account_id, collected_at, disk_bytes, inode_count, bandwidth_bytes, cpu_percent, memory_bytes, process_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id.New(), u.AccountID, u.CollectedAt, u.DiskBytes, u.InodeCount, u.BandwidthBytes, u.CPUPercent, u.MemoryBytes, u.ProcessCount)
}

func (p *PG) GetUsage(accountID string) *Usage {
	u := &Usage{AccountID: accountID}
	if err := p.pool.QueryRow(p.ctx(), `SELECT collected_at, disk_bytes, inode_count, bandwidth_bytes, cpu_percent, memory_bytes, process_count FROM resource_usage WHERE account_id=$1 ORDER BY collected_at DESC LIMIT 1`, accountID).
		Scan(&u.CollectedAt, &u.DiskBytes, &u.InodeCount, &u.BandwidthBytes, &u.CPUPercent, &u.MemoryBytes, &u.ProcessCount); err != nil {
		return nil
	}
	return u
}

func (p *PG) Stats() map[string]int {
	out := map[string]int{}
	var n int
	scan := func(q string, key string) {
		n = 0
		_ = p.pool.QueryRow(p.ctx(), q).Scan(&n)
		out[key] = n
	}
	scan(`SELECT COUNT(*) FROM users`, "users")
	scan(`SELECT COUNT(*) FROM accounts`, "accounts")
	scan(`SELECT COUNT(*) FROM domains`, "domains")
	scan(`SELECT COUNT(*) FROM websites`, "websites")
	scan(`SELECT COUNT(*) FROM mailboxes`, "mailboxes")
	scan(`SELECT COUNT(*) FROM jobs`, "jobs")
	scan(`SELECT COUNT(*) FROM jobs WHERE state='failed'`, "failedJobs")
	scan(`SELECT COUNT(*) FROM audit_events`, "audit")
	return out
}

func (p *PG) SeedDatabaseServers() {
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO database_servers (id, engine, name, host, port) VALUES ($1,'mariadb','local-mariadb','127.0.0.1',3306) ON CONFLICT (name) DO NOTHING`, id.New())
	_, _ = p.pool.Exec(p.ctx(), `INSERT INTO database_servers (id, engine, name, host, port) VALUES ($1,'postgres','local-postgres','127.0.0.1',5432) ON CONFLICT (name) DO NOTHING`, id.New())
}

var _ = fmt.Sprintf
var _ Store = (*PG)(nil)
