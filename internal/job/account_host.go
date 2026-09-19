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
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/netaddr"
	"github.com/hosting-panel/panel/internal/store"
)

func posixShell(class string) string {
	switch class {
	case "sftp-only":
		return "/usr/sbin/rssh"
	case "jailed":
		return "/bin/bash"
	default:
		return "/usr/sbin/nologin"
	}
}

func parseAccountIP(value string) net.IP {
	return net.ParseIP(strings.TrimSpace(value))
}

func accountPublishIPv4(acc *store.Account) string {
	if acc != nil {
		if ip := parseAccountIP(acc.IPAddress); ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}
	return publicIPv4()
}

func accountPublishIPv6(acc *store.Account) string {
	if acc == nil {
		return ""
	}
	if ip := parseAccountIP(acc.IPAddress); ip != nil && ip.To4() == nil {
		return ip.String()
	}
	return ""
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
	return operations.HostConfigSpec{
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
	}
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

func (w *Worker) synchronizeAllZones() error {
	for _, acc := range w.Store.ListAccounts("", "") {
		for _, d := range w.Store.ListDomains(acc.ID) {
			z := w.Store.ZoneByDomain(d.ID)
			if z == nil {
				continue
			}
			if err := w.writeZone(z); err != nil {
				return err
			}
			z.ObservedRevision = z.DesiredRevision
			w.Store.PutZone(z)
		}
	}
	return nil
}

func (w *Worker) cleanupOrphanZones() error {
	live := map[string]bool{}
	for _, acc := range w.Store.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, d := range w.Store.ListDomains(acc.ID) {
			if z := w.Store.ZoneByDomain(d.ID); z != nil {
				live[z.ID] = true
			}
		}
	}
	for _, acc := range w.Store.ListAccounts("", "terminated") {
		for _, d := range w.Store.ListDomains(acc.ID) {
			z := w.Store.ZoneByDomain(d.ID)
			if z == nil || live[z.ID] {
				continue
			}
			if w.Agent != nil {
				_, _ = w.Agent.Dispatch(context.Background(), operations.Request{
					Method: "RetireDomain",
					Params: mustJSON(map[string]any{"account": acc.Username, "domain": d.ASCII}),
				})
			}
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
	dedicatedV4 := parseAccountIP(acc.IPAddress) != nil && parseAccountIP(acc.IPAddress).To4() != nil
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
	clean := filepath.ToSlash(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || strings.Contains(clean, "..") {
		return ""
	}
	return clean
}
