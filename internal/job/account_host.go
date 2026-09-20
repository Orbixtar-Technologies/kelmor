package job

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/dnsinventory"
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/netaddr"
	"github.com/hosting-panel/panel/internal/store"
)

// posixShell maps Director shell_class to a permitted POSIX path.
// jailed → rssh; sftp-only and nologin stay nologin (SFTP subsystem only).
func posixShell(class string) string {
	return accountShellPath(class)
}

func accountShellPath(class string) string {
	switch strings.TrimSpace(class) {
	case "jailed":
		return "/usr/sbin/rssh"
	default:
		return "/usr/sbin/nologin"
	}
}

func parseAccountIP(value string) net.IP {
	return net.ParseIP(strings.TrimSpace(value))
}

func accountIP(acc *store.Account) string {
	if acc == nil {
		return ""
	}
	return strings.TrimSpace(acc.IPAddress)
}

func parseIPv4(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() == nil {
		return ""
	}
	return ip.To4().String()
}

func parseIPv6(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() != nil {
		return ""
	}
	return ip.String()
}

func accountPublishIPv4(acc *store.Account) string {
	if ip := parseIPv4(accountIP(acc)); ip != "" {
		return ip
	}
	return publicIPv4()
}

func accountPublishIPv6(acc *store.Account) string {
	return parseIPv6(accountIP(acc))
}

func managedDNSName(name string) bool {
	switch strings.TrimSpace(name) {
	case "@", "www":
		return true
	}
	for _, service := range configuration.AccountServiceHostnames() {
		if name == service {
			return true
		}
	}
	return false
}

func (w *Worker) stateDir() string {
	root := ""
	sockEmpty := true
	if w.Agent != nil {
		root = w.Agent.Root
		sockEmpty = w.Agent.Sock == ""
	}
	return hostconfig.StateDir(os.Getenv("PANEL_STATE_DIR"), root, sockEmpty)
}

func (w *Worker) loadHostSettings() hostconfig.File {
	f, err := hostconfig.Load(w.stateDir())
	if err != nil {
		return hostconfig.File{Values: map[string]map[string]string{}}
	}
	return f
}

func (w *Worker) writeClusterSnapshotJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "WriteClusterSnapshot",
		Params: mustJSON(map[string]any{"snapshot": j.Payload["snapshot"]}),
	})
	return err
}

func (w *Worker) probeClusterPeersJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ProbeClusterPeers",
		Params: mustJSON(map[string]any{"urls": j.Payload["urls"]}),
	})
	return err
}

func (w *Worker) applyClusterSnapshotJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyClusterSnapshot",
		Params: mustJSON(map[string]any{
			"urls": j.Payload["urls"], "snapshot": j.Payload["snapshot"],
		}),
	})
	return err
}

func (w *Worker) applyServiceCertificateJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	service := str(j.Payload["service"])
	if service == "" {
		service = str(j.Payload["target"])
	}
	if str(j.Payload["cert_pem"]) == "" {
		return nil
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "InstallServiceCertificate",
		Params: mustJSON(map[string]any{
			"service": service, "hostname": str(j.Payload["hostname"]),
			"cert_pem": str(j.Payload["cert_pem"]), "key_pem": str(j.Payload["key_pem"]),
		}),
	})
	return err
}

func (w *Worker) applyRemoteAccessKeyJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "WriteRemoteAccessKey",
		Params: mustJSON(map[string]any{
			"prefix":     str(j.Payload["prefix"]),
			"hash":       str(j.Payload["hash"]),
			"created_at": str(j.Payload["created_at"]),
			"revoked":    payloadBool(j.Payload["revoked"], false),
		}),
	})
	return err
}

func (w *Worker) setupInitialQuotaJob(j *store.Job) error {
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetupInitialQuota",
		Params: mustJSON(map[string]any{
			"bytes":   payloadInt64(j.Payload["bytes"]),
			"enforce": payloadBool(j.Payload["enforce"], true),
		}),
	})
	return err
}

func payloadInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func payloadBool(v any, fallback bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

func (w *Worker) applyHostConfigJob(j *store.Job) error {
	settings := w.loadHostSettings()
	spec := hostConfigSpec(settings)
	if w.Agent == nil {
		return fmt.Errorf("agent missing")
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyHostConfig",
		Params: mustJSON(spec),
	})
	if err != nil {
		return err
	}
	if hostname := hostconfig.Field(settings, "hostname", "hostname", ""); hostname != "" {
		_ = w.ensureHostnameA(hostname, hostconfig.Field(settings, "hostname_a", "address", ""))
	}
	if addr := hostconfig.Field(settings, "hostname_a", "address", ""); addr != "" {
		_ = w.ensureHostnameA(hostconfig.Field(settings, "hostname", "hostname", ""), addr)
	}
	return nil
}

func hostConfigSpec(settings hostconfig.File) operations.HostConfigSpec {
	php := map[string]string{}
	if row := settings.Values["php_ini"]; row != nil {
		for key, value := range row {
			if value != "" {
				php[key] = value
			}
		}
	}
	spec := operations.HostConfigSpec{
		Hostname:          hostconfig.Field(settings, "hostname", "hostname", ""),
		Resolvers:         hostconfig.Lines(settings, "resolvers", "nameservers"),
		Timezone:          hostconfig.Field(settings, "server_time", "timezone", ""),
		IndexFiles:        hostconfig.Field(settings, "directory_index", "indexes", ""),
		HTTPSRedirect:     hostconfig.BoolField(settings, "nginx_manager", "https_redirect", true),
		Gzip:              hostconfig.BoolField(settings, "nginx_manager", "gzip", true),
		ClientMaxBody:     hostconfig.Field(settings, "http_server", "client_max_body_size", ""),
		KeepaliveTimeout:  hostconfig.Field(settings, "http_server", "keepalive_timeout", ""),
		PHPIni:            php,
		SpamEnabled:       hostconfig.BoolField(settings, "spamd", "enabled", true),
		RejectScore:       hostconfig.IntField(settings, "spamd", "reject_score", 15),
		GreylistEnabled:   hostconfig.BoolField(settings, "greylisting", "enabled", false),
		GreylistMinutes:   hostconfig.IntField(settings, "greylisting", "delay_minutes", 5),
		CountryAction:     hostconfig.Field(settings, "mail_country_filter", "action", "reject"),
		Countries:         hostconfig.Lines(settings, "mail_country_filter", "countries"),
		DenyDomains:       hostconfig.Lines(settings, "mail_domain_filter", "deny"),
		AllowDomains:      hostconfig.Lines(settings, "mail_domain_filter", "allow"),
		RequireAuth:       hostconfig.BoolField(settings, "mail_config", "require_auth", true),
		Milter:            hostconfig.BoolField(settings, "mail_config", "milter", true),
		RestrictSMTP:      hostconfig.BoolField(settings, "smtp_restrictions", "restrict", true),
		FTPEnabled:        hostconfig.Field(settings, "ftp_selection", "daemon", "vsftpd") != "disabled",
		PasvMin:           hostconfig.IntField(settings, "ftp_server", "pasv_min", 40000),
		PasvMax:           hostconfig.IntField(settings, "ftp_server", "pasv_max", 40100),
		FTPBanner:         hostconfig.Field(settings, "ftp_server", "banner", ""),
		LogRetainDays:     hostconfig.IntField(settings, "log_rotation", "days", 14),
		AllowCIDRs:        hostconfig.ValidIPList(hostconfig.Lines(settings, "host_access", "allow")),
		DenyCIDRs:         hostconfig.ValidIPList(hostconfig.Lines(settings, "host_access", "deny")),
		RootSSHKeys:       hostconfig.Field(settings, "root_ssh_keys", "keys", ""),
		WheelMembers:      hostconfig.Lines(settings, "wheel_group", "members"),
		CompilerAllow:     hostconfig.BoolField(settings, "compiler_access", "allow", false),
		BackupSchedule:    hostconfig.Field(settings, "host_cron", "backup", ""),
		BackupUsers:       hostconfig.BackupUsernames(settings),
		BackupDestination: hostconfig.BackupDestination(settings),
		BackupRetention:   hostconfig.BackupRetentionDays(settings),
		Nameservers:       hostconfig.Lines(settings, "dns_cluster", "nameservers"),
		IPPool:            hostconfig.ValidIPList(hostconfig.Lines(settings, "ip_pool", "address")),
		IPv6Ranges:        hostconfig.ValidIPList(hostconfig.Lines(settings, "ipv6_ranges", "ranges")),
		Relayers:          hostconfig.Lines(settings, "relayers", "usernames"),
		NS1IP:             hostconfig.Field(settings, "nameserver_ips", "ns1_ip", ""),
		NS2IP:             hostconfig.Field(settings, "nameserver_ips", "ns2_ip", ""),
		MaxEmailsHour:     hostconfig.MaxEmailsHour(settings),
		Profile:           hostconfig.Field(settings, "server_profile", "profile", ""),
		ClusterPeers:      hostconfig.Lines(settings, "configuration_cluster", "peers"),
		WriteCluster:      settings.Values["configuration_cluster"] != nil,
		LinkedNodes:       hostconfig.Lines(settings, "linked_nodes", "nodes"),
		WriteLinkedNodes:  settings.Values["linked_nodes"] != nil,
		TwoFactorRequired: hostconfig.TwoFactorRequired(settings),
		WriteTwoFactor:    settings.Values["two_factor"] != nil || settings.Values["security_policies"] != nil,
		InitialQuotaBytes: hostconfig.InitialQuotaBytes(settings),
		WriteInitialQuota: settings.Values["initial_quota"] != nil,
		QuotaEnforce:      hostconfig.BoolField(settings, "initial_quota", "enforce", true),
		PostgresListen:    hostconfig.Field(settings, "postgres", "listen", ""),
		PostgresAuth:      hostconfig.Field(settings, "postgres", "auth", ""),
		WritePostgres:     settings.Values["postgres"] != nil,
	}
	if settings.Values["external_auth"] != nil {
		provider := hostconfig.ExternalAuthProvider(settings)
		spec.ExternalAuth = &operations.ExternalAuthSpec{
			Provider:        provider,
			Issuer:          hostconfig.Field(settings, "external_auth", "issuer", ""),
			ClientID:        hostconfig.Field(settings, "external_auth", "client_id", ""),
			LDAPURL:         hostconfig.Field(settings, "external_auth", "ldap_url", ""),
			LDAPUserDN:      hostconfig.Field(settings, "external_auth", "ldap_user_dn", "uid={username},ou=people,dc=example,dc=com"),
			Enabled:         hostconfig.BoolField(settings, "external_auth", "enabled", provider != "disabled"),
			RequireExternal: hostconfig.ExternalAuthRequired(settings),
		}
	}
	return spec
}

func (w *Worker) ensureHostnameA(hostname, address string) error {
	if hostname == "" || parseAccountIP(address) == nil || parseAccountIP(address).To4() == nil {
		return nil
	}
	for _, acc := range w.Store.ListAccounts("", "") {
		for _, d := range w.Store.ListDomains(acc.ID) {
			if d.ASCII != hostname && !strings.HasSuffix(hostname, "."+d.ASCII) {
				continue
			}
			z := w.Store.ZoneByDomain(d.ID)
			if z == nil {
				continue
			}
			name := "@"
			if hostname != d.ASCII {
				name = strings.TrimSuffix(hostname, "."+d.ASCII)
			}
			found := false
			for _, rec := range w.Store.ListRecords(z.ID) {
				if rec.Type == "A" && rec.Name == name {
					rec.Content = address
					w.Store.PutRecord(&rec)
					found = true
					break
				}
			}
			if !found {
				w.Store.PutRecord(&store.DNSRecord{
					ID: store.NewID(), ZoneID: z.ID, Name: name, Type: "A",
					Content: address, TTL: 3600,
				})
			}
			z.DesiredRevision++
			w.Store.PutZone(z)
			return w.writeZone(z)
		}
	}
	return nil
}

func (w *Worker) maybeScheduleBackups() {
	if !w.lastBackupScan.IsZero() && time.Since(w.lastBackupScan) < 24*time.Hour {
		return
	}
	w.lastBackupScan = time.Now()
	_ = w.scheduleSelectedBackups(w.loadHostSettings())
}

func (w *Worker) scheduleSelectedBackups(settings hostconfig.File) error {
	users := hostconfig.BackupUsernames(settings)
	if len(users) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, name := range users {
		want[strings.ToLower(name)] = true
	}
	dest := hostconfig.BackupDestination(settings)
	for _, acc := range w.Store.ListAccounts("", "") {
		if !want[strings.ToLower(acc.Username)] || acc.Status != "active" {
			continue
		}
		b := &store.BackupRun{
			ID: store.NewID(), AccountID: acc.ID, Kind: "full", State: "queued",
			Destination: dest, CreatedAt: time.Now().UTC(),
		}
		if _, err := w.Store.CreateBackupWithJob(b, &store.Job{
			Type: "backup.create", ResourceType: "backup", ResourceID: b.ID,
			Payload: map[string]any{"backup_id": b.ID, "account_id": acc.ID},
			State:   "queued",
		}, store.AuditEvent{Action: "backup.schedule", ResourceType: "backup", ResourceID: b.ID, Success: true}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) synchronizeAllZones(j *store.Job) error {
	payload := map[string]any{}
	if j != nil {
		payload = j.Payload
	}
	wanted := dnsinventory.PayloadZoneIDs(payload)
	for _, item := range dnsinventory.FilterByIDs(dnsinventory.ManagedZones(w.Store), wanted) {
		z := w.Store.GetZone(item.ID)
		if z == nil {
			continue
		}
		if err := w.writeZone(z); err != nil {
			return err
		}
		z.ObservedRevision = z.DesiredRevision
		w.Store.PutZone(z)
	}
	return nil
}

func (w *Worker) cleanupOrphanZones(j *store.Job) error {
	payload := map[string]any{}
	if j != nil {
		payload = j.Payload
	}
	wanted := dnsinventory.PayloadZoneIDs(payload)
	for _, item := range dnsinventory.FilterByIDs(dnsinventory.LeftoverZones(w.Store), wanted) {
		if item.AccountUsername == "" || item.Domain == "" {
			continue
		}
		if w.Agent != nil {
			_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
				Method: "RetireDomain",
				Params: mustJSON(map[string]any{"account": item.AccountUsername, "domain": item.Domain}),
			})
		}
	}
	return nil
}

func (w *Worker) notifyMailJob(j *store.Job) error {
	from := str(j.Payload["from"])
	subject := str(j.Payload["subject"])
	body := str(j.Payload["body"])
	audience := str(j.Payload["audience"])
	if from == "" || subject == "" {
		return fmt.Errorf("notify mail missing from or subject")
	}
	var recipients []string
	switch audience {
	case "resellers":
		for _, reseller := range w.Store.ListResellers() {
			if user := w.Store.UserByID(reseller.UserID); user != nil && user.Email != "" {
				recipients = append(recipients, user.Email)
			}
		}
	default:
		for _, acc := range w.Store.ListAccounts("", "") {
			if owner := w.Store.UserByID(acc.OwnerUserID); owner != nil && owner.Email != "" {
				recipients = append(recipients, owner.Email)
			}
		}
	}
	for _, to := range recipients {
		if _, err := w.Agent.Dispatch(context.Background(), operations.Request{
			Method: "SendSystemMail",
			Params: mustJSON(map[string]any{"from": from, "to": to, "subject": subject, "body": body}),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) republishAccountAddresses(z *store.DNSZone, acc *store.Account) {
	if z == nil || acc == nil {
		return
	}
	v4 := accountPublishIPv4(acc)
	v6 := accountPublishIPv6(acc)
	shared := publicIPv4()
	service := map[string]bool{"@": true, "www": true}
	for _, name := range configuration.AccountServiceHostnames() {
		service[name] = true
	}
	dedicatedV4 := parseIPv4(accountIP(acc)) != ""
	changed := false
	haveAAAA := map[string]bool{}
	for _, rec := range w.Store.ListRecords(z.ID) {
		switch rec.Type {
		case "A":
			if v4 == "" {
				continue
			}
			should := rec.Content != v4 && netaddr.IsPrivateIPv4String(rec.Content)
			if dedicatedV4 && service[rec.Name] && rec.Content != v4 {
				should = true
			}
			if !dedicatedV4 && service[rec.Name] && rec.Content != v4 && rec.Content == shared {
				should = true
			}
			if should {
				rec.Content = v4
				w.Store.PutRecord(&rec)
				changed = true
			}
		case "AAAA":
			haveAAAA[rec.Name] = true
			if v6 != "" && service[rec.Name] && rec.Content != v6 {
				rec.Content = v6
				w.Store.PutRecord(&rec)
				changed = true
			}
		case "TXT":
			if v4 == "" || netaddr.IsPrivateIPv4String(v4) {
				continue
			}
			next := netaddr.RewritePrivateIP4Tokens(rec.Content, v4)
			if dedicatedV4 && shared != "" && shared != v4 {
				next = strings.ReplaceAll(next, "ip4:"+shared, "ip4:"+v4)
			}
			if next != rec.Content {
				rec.Content = next
				w.Store.PutRecord(&rec)
				changed = true
			}
		}
	}
	if v6 != "" {
		for name := range service {
			if haveAAAA[name] {
				continue
			}
			w.Store.PutRecord(&store.DNSRecord{
				ID: store.NewID(), ZoneID: z.ID, Name: name, Type: "AAAA",
				Content: v6, TTL: 3600,
			})
			changed = true
		}
	}
	if changed {
		z.DesiredRevision++
		w.Store.PutZone(z)
	}
}

func restorePathPrefix(raw string) string {
	prefix := filepath.Clean(strings.TrimSpace(raw))
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "." || prefix == "" || strings.Contains(prefix, "..") {
		return ""
	}
	return prefix
}

func (w *Worker) applyAccountShell(acc *store.Account) error {
	if acc == nil || w.Agent == nil {
		return nil
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetLinuxShell",
		Params: mustJSON(map[string]any{
			"username": acc.Username,
			"shell":    accountShellPath(acc.ShellClass),
		}),
	})
	return err
}
