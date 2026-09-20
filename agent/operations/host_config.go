package operations

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/firewall"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type HostConfigSpec struct {
	Hostname          string            `json:"hostname,omitempty"`
	Resolvers         []string          `json:"resolvers,omitempty"`
	Timezone          string            `json:"timezone,omitempty"`
	IndexFiles        string            `json:"index_files,omitempty"`
	HTTPSRedirect     bool              `json:"https_redirect"`
	Gzip              bool              `json:"gzip"`
	ClientMaxBody     string            `json:"client_max_body_size,omitempty"`
	KeepaliveTimeout  string            `json:"keepalive_timeout,omitempty"`
	PHPIni            map[string]string `json:"php_ini,omitempty"`
	SpamEnabled       bool              `json:"spam_enabled"`
	RejectScore       int               `json:"reject_score,omitempty"`
	GreylistEnabled   bool              `json:"greylist_enabled"`
	GreylistMinutes   int               `json:"greylist_minutes,omitempty"`
	CountryAction     string            `json:"country_action,omitempty"`
	Countries         []string          `json:"countries,omitempty"`
	DenyDomains       []string          `json:"deny_domains,omitempty"`
	AllowDomains      []string          `json:"allow_domains,omitempty"`
	RequireAuth       bool              `json:"require_auth"`
	Milter            bool              `json:"milter"`
	RestrictSMTP      bool              `json:"restrict_smtp"`
	FTPEnabled        bool              `json:"ftp_enabled"`
	PasvMin           int               `json:"pasv_min,omitempty"`
	PasvMax           int               `json:"pasv_max,omitempty"`
	FTPBanner         string            `json:"ftp_banner,omitempty"`
	LogRetainDays     int               `json:"log_retain_days,omitempty"`
	AllowCIDRs        []string          `json:"allow_cidrs,omitempty"`
	DenyCIDRs         []string          `json:"deny_cidrs,omitempty"`
	RootSSHKeys       string            `json:"root_ssh_keys,omitempty"`
	WheelMembers      []string          `json:"wheel_members,omitempty"`
	CompilerAllow     bool              `json:"compiler_allow"`
	BackupSchedule    string            `json:"backup_schedule,omitempty"`
	BackupUsers       []string          `json:"backup_users,omitempty"`
	BackupDestination string            `json:"backup_destination,omitempty"`
	BackupRetention   int               `json:"backup_retention_days,omitempty"`
	Nameservers       []string          `json:"nameservers,omitempty"`
	IPPool            []string          `json:"ip_pool,omitempty"`
	IPv6Ranges        []string          `json:"ipv6_ranges,omitempty"`
	Relayers          []string          `json:"relayers,omitempty"`
	NS1IP             string            `json:"ns1_ip,omitempty"`
	NS2IP             string            `json:"ns2_ip,omitempty"`
	MaxEmailsHour     int               `json:"max_emails_hour,omitempty"`
	Profile           string            `json:"profile,omitempty"`
	ClusterPeers      []string          `json:"cluster_peers,omitempty"`
	WriteCluster      bool              `json:"write_cluster,omitempty"`
	LinkedNodes       []string          `json:"linked_nodes,omitempty"`
	WriteLinkedNodes  bool              `json:"write_linked_nodes,omitempty"`
	ExternalAuth      *ExternalAuthSpec `json:"external_auth,omitempty"`
	TwoFactorRequired bool              `json:"two_factor_required,omitempty"`
	WriteTwoFactor    bool              `json:"write_two_factor,omitempty"`
	InitialQuotaBytes int64             `json:"initial_quota_bytes,omitempty"`
	WriteInitialQuota bool              `json:"write_initial_quota,omitempty"`
	QuotaEnforce      bool              `json:"quota_enforce,omitempty"`
	PostgresListen    string            `json:"postgres_listen,omitempty"`
	PostgresAuth      string            `json:"postgres_auth,omitempty"`
	WritePostgres     bool              `json:"write_postgres,omitempty"`
}

type ExternalAuthSpec struct {
	Provider        string `json:"provider,omitempty"`
	Issuer          string `json:"issuer,omitempty"`
	ClientID        string `json:"client_id,omitempty"`
	LDAPURL         string `json:"ldap_url,omitempty"`
	LDAPUserDN      string `json:"ldap_user_dn,omitempty"`
	Enabled         bool   `json:"enabled"`
	RequireExternal bool   `json:"require_external"`
}

func (h *Host) applyHostConfig(spec HostConfigSpec) (Result, error) {
	applied := []string{}
	if spec.Hostname != "" {
		if _, err := validate.NormalizeDomain(spec.Hostname); err != nil {
			return Result{}, fmt.Errorf("hostname: %w", err)
		}
		if _, err := h.ApplyFile("/var/lib/panel/portal-hostname", []byte(spec.Hostname+"\n"), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "hostname")
		if h.live() {
			_, _ = runFixed("/usr/bin/hostnamectl", "set-hostname", spec.Hostname)
			_, _ = runFixed("/usr/sbin/postconf", "-e", "myhostname="+spec.Hostname)
		}
	}
	if len(spec.Resolvers) > 0 {
		var b strings.Builder
		for _, resolver := range spec.Resolvers {
			if net.ParseIP(resolver) == nil {
				continue
			}
			b.WriteString("nameserver ")
			b.WriteString(resolver)
			b.WriteByte('\n')
		}
		if b.Len() > 0 {
			if _, err := h.ApplyFile("/etc/panel/resolvers.conf", []byte(b.String()), 0o644); err != nil {
				return Result{}, err
			}
			applied = append(applied, "resolvers")
		}
	}
	if spec.Timezone != "" {
		if strings.ContainsAny(spec.Timezone, " \t\n;") {
			return Result{}, fmt.Errorf("timezone not permitted")
		}
		if _, err := h.ApplyFile("/var/lib/panel/timezone", []byte(spec.Timezone+"\n"), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "timezone")
		if h.live() {
			_, _ = runFixed("/usr/bin/timedatectl", "set-timezone", spec.Timezone)
		}
	}
	nginxBody := nginxDefaults(spec)
	if _, err := h.ApplyFile("/etc/nginx/conf.d/panel-defaults.conf", []byte(nginxBody), 0o644); err != nil {
		return Result{}, err
	}
	applied = append(applied, "nginx")
	if err := h.writePHPIni(spec.PHPIni); err != nil {
		return Result{}, err
	}
	if len(spec.PHPIni) > 0 {
		applied = append(applied, "php")
	}
	if err := h.writeRspamdPolicy(spec); err != nil {
		return Result{}, err
	}
	applied = append(applied, "rspamd")
	if spec.LogRetainDays > 0 {
		body := fmt.Sprintf("/var/log/nginx/*.log /var/log/mail.log /var/lib/panel/logs/*.log {\n  daily\n  rotate %d\n  missingok\n  notifempty\n  compress\n}\n", spec.LogRetainDays)
		if _, err := h.ApplyFile("/etc/panel/logrotate-panel", []byte(body), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "logrotate")
	}
	if spec.RootSSHKeys != "" {
		if _, err := h.ApplyFile("/var/lib/panel/ssh/root.authorized_keys", []byte(spec.RootSSHKeys+"\n"), 0o600); err != nil {
			return Result{}, err
		}
		applied = append(applied, "root-ssh")
	}
	if len(spec.WheelMembers) > 0 {
		body := strings.Join(spec.WheelMembers, "\n") + "\n"
		if _, err := h.ApplyFile("/etc/panel/wheel-members", []byte(body), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "wheel")
	}
	compiler := "deny\n"
	if spec.CompilerAllow {
		compiler = "allow\n"
	}
	if _, err := h.ApplyFile("/etc/panel/compiler-access", []byte(compiler), 0o644); err != nil {
		return Result{}, err
	}
	applied = append(applied, "compiler")
	if spec.BackupSchedule != "" {
		line := spec.BackupSchedule + " root /usr/bin/panel-cli backup schedule >/dev/null 2>&1\n"
		if _, err := h.ApplyFile("/etc/cron.d/panel-backup", []byte(line), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "backup-cron")
	}
	if (spec.PasvMin > 0 && spec.PasvMax >= spec.PasvMin) || spec.FTPBanner != "" {
		if err := h.applyFTPServerConfig(spec); err != nil {
			return Result{}, err
		}
		applied = append(applied, "ftp")
	}
	if len(spec.AllowCIDRs) > 0 || len(spec.DenyCIDRs) > 0 {
		if err := h.applyFirewallWithCIDRs(spec.AllowCIDRs, spec.DenyCIDRs); err != nil {
			return Result{}, err
		}
		applied = append(applied, "firewall")
	}
	if len(spec.IPPool) > 0 || len(spec.IPv6Ranges) > 0 {
		pool := strings.Join(append(append([]string{}, spec.IPPool...), spec.IPv6Ranges...), "\n") + "\n"
		if _, err := h.ApplyFile("/etc/panel/ip-pool", []byte(pool), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "ip-pool")
	}
	if len(spec.Relayers) > 0 {
		if _, err := h.ApplyFile("/etc/panel/relayers", []byte(strings.Join(spec.Relayers, "\n")+"\n"), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "relayers")
	}
	if spec.NS1IP != "" || spec.NS2IP != "" {
		body := "ns1=" + spec.NS1IP + "\nns2=" + spec.NS2IP + "\n"
		if _, err := h.ApplyFile("/etc/panel/nameserver-ips", []byte(body), 0o644); err != nil {
			return Result{}, err
		}
		applied = append(applied, "nameserver-ips")
	}
	if spec.RestrictSMTP && h.live() {
		_, _ = runFixed("/usr/sbin/postconf", "-e", "smtpd_sender_restrictions=reject_unknown_sender_domain")
	}
	if spec.MaxEmailsHour > 0 && h.live() {
		_, _ = runFixed("/usr/sbin/postconf", "-e", fmt.Sprintf("default_destination_rate_delay=%ds", 3600/spec.MaxEmailsHour))
	}
	if spec.Profile != "" {
		if err := h.applyServerProfile(spec.Profile); err != nil {
			return Result{}, err
		}
		applied = append(applied, "server-profile")
	}
	if spec.WriteCluster {
		if err := h.writeClusterMembership(spec.ClusterPeers); err != nil {
			return Result{}, err
		}
		applied = append(applied, "cluster")
	}
	if spec.WriteLinkedNodes {
		if err := h.writeLinkedNodes(spec.LinkedNodes); err != nil {
			return Result{}, err
		}
		applied = append(applied, "linked-nodes")
	}
	if spec.ExternalAuth != nil {
		if err := h.writeExternalAuth(*spec.ExternalAuth); err != nil {
			return Result{}, err
		}
		applied = append(applied, "external-auth")
	}
	if spec.WriteTwoFactor {
		if err := h.writeTwoFactorPolicy(spec.TwoFactorRequired); err != nil {
			return Result{}, err
		}
		applied = append(applied, "two-factor")
	}
	if spec.WriteInitialQuota {
		if err := h.writeInitialQuotaPolicy(spec.InitialQuotaBytes, spec.QuotaEnforce); err != nil {
			return Result{}, err
		}
		applied = append(applied, "initial-quota")
	}
	if spec.WritePostgres {
		if err := h.applyPostgresConfig(spec); err != nil {
			return Result{}, err
		}
		applied = append(applied, "postgres")
	}
	if h.live() {
		_ = reloadNamedService("nginx")
		_ = controlNamedService("rspamd", "reload")
	}
	return Result{
		OK:            true,
		Message:       "host config applied: " + strings.Join(applied, ","),
		ObservedState: "applied",
	}, nil
}

func nginxDefaults(spec HostConfigSpec) string {
	body := spec.ClientMaxBody
	if body == "" {
		body = "64m"
	}
	keep := spec.KeepaliveTimeout
	if keep == "" {
		keep = "65"
	}
	indexes := spec.IndexFiles
	if indexes == "" {
		indexes = "index.php index.html"
	}
	var b strings.Builder
	b.WriteString("# Kelmor panel defaults — written by ApplyHostConfig\n")
	b.WriteString("# index " + indexes + "\n")
	b.WriteString("client_max_body_size " + body + ";\n")
	b.WriteString("keepalive_timeout " + keep + ";\n")
	// Ubuntu nginx.conf already sets `gzip on` in http{}. A second gzip
	// in conf.d/panel-defaults.conf makes nginx -t fail ("directive is
	// duplicate"). Record the nginx-manager setting as a comment only.
	if spec.Gzip {
		b.WriteString("# gzip on; not redeclared (http gzip already set in nginx.conf)\n")
	} else {
		b.WriteString("# gzip off; not redeclared (would duplicate nginx.conf gzip)\n")
	}
	if spec.HTTPSRedirect {
		b.WriteString("# https_redirect=on for new vhosts\n")
	}
	return b.String()
}

func (h *Host) writePHPIni(values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("; Kelmor MultiPHP defaults\n")
	for key, value := range values {
		if !phpIniKeyOK(key) || strings.ContainsAny(value, "\n;") {
			continue
		}
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(value)
		b.WriteByte('\n')
	}
	for _, ver := range []string{"8.2", "8.3", "8.4", "8.5"} {
		path := "/etc/php/" + ver + "/fpm/conf.d/99-panel.ini"
		if _, err := h.ApplyFile(path, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func phpIniKeyOK(key string) bool {
	switch key {
	case "memory_limit", "upload_max_filesize", "max_execution_time", "post_max_size":
		return true
	default:
		return false
	}
}

func (h *Host) writeRspamdPolicy(spec HostConfigSpec) error {
	score := spec.RejectScore
	if score < 1 {
		score = 15
	}
	enabled := "true"
	if !spec.SpamEnabled {
		enabled = "false"
	}
	actions := fmt.Sprintf("enabled = %s;\nactions {\n  reject = %d;\n}\n", enabled, score)
	if _, err := h.ApplyFile("/etc/rspamd/local.d/actions.conf", []byte(actions), 0o644); err != nil {
		return err
	}
	if spec.GreylistEnabled {
		delay := spec.GreylistMinutes
		if delay < 1 {
			delay = 5
		}
		body := fmt.Sprintf("enabled = true;\nexpire = %dm;\n", delay)
		if _, err := h.ApplyFile("/etc/rspamd/local.d/greylist.conf", []byte(body), 0o644); err != nil {
			return err
		}
	} else {
		if _, err := h.ApplyFile("/etc/rspamd/local.d/greylist.conf", []byte("enabled = false;\n"), 0o644); err != nil {
			return err
		}
	}
	var multimap strings.Builder
	if len(spec.DenyDomains) > 0 {
		if _, err := h.ApplyFile("/etc/rspamd/local.d/denied-domains.map", []byte(strings.Join(spec.DenyDomains, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		multimap.WriteString("PANEL_DENY {\n  type = \"from\";\n  filter = \"email:domain\";\n  map = \"/etc/rspamd/local.d/denied-domains.map\";\n  action = \"reject\";\n}\n")
	}
	if len(spec.AllowDomains) > 0 {
		if _, err := h.ApplyFile("/etc/rspamd/local.d/allowed-domains.map", []byte(strings.Join(spec.AllowDomains, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	if len(spec.Countries) > 0 {
		action := spec.CountryAction
		if action != "greylist" {
			action = "reject"
		}
		if _, err := h.ApplyFile("/etc/rspamd/local.d/countries.map", []byte(strings.Join(spec.Countries, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		multimap.WriteString("PANEL_COUNTRY {\n  type = \"country\";\n  map = \"/etc/rspamd/local.d/countries.map\";\n  action = \"" + action + "\";\n}\n")
	}
	if _, err := h.ApplyFile("/etc/rspamd/local.d/multimap.conf", []byte(multimap.String()), 0o644); err != nil {
		return err
	}
	milter := "false"
	if spec.Milter {
		milter = "true"
	}
	if _, err := h.ApplyFile("/etc/rspamd/local.d/worker-proxy.inc", []byte("enabled = "+milter+";\n"), 0o644); err != nil {
		return err
	}
	_ = spec.RequireAuth
	return nil
}

func (h *Host) applyFirewallWithCIDRs(allow, deny []string) error {
	extra := firewall.ExtraListeningTCP()
	body := firewall.RulesWithAccess(extra, allow, deny)
	path := "/etc/panel/nftables-panel.nft"
	if _, err := h.ApplyFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	if !h.live() {
		return nil
	}
	resolved, err := h.resolve(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return fmt.Errorf("nft not installed")
	}
	_, _ = runFixed("/usr/sbin/nft", "delete", "table", "inet", "panel")
	out, err := runFixed("/usr/sbin/nft", "-f", resolved)
	if err != nil {
		return fmt.Errorf("nft -f: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *Host) setUnixShell(username, shell string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	switch shell {
	case "/usr/sbin/nologin", "/bin/bash", "/usr/sbin/rssh", "":
	default:
		return Result{}, fmt.Errorf("shell not permitted")
	}
	if shell == "" {
		shell = "/usr/sbin/nologin"
	}
	if !h.live() {
		home, err := h.resolve("/home/" + username)
		if err != nil {
			return Result{}, err
		}
		meta := fmt.Sprintf("username=%s shell=%s updated=%s\n", username, shell, time.Now().UTC().Format(time.RFC3339))
		_ = os.WriteFile(filepath.Join(home, ".panel-identity"), []byte(meta), 0o640)
		return Result{OK: true, ObservedState: shell}, nil
	}
	if out, err := runFixed("/usr/sbin/usermod", "-s", shell, username); err != nil {
		return Result{}, fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, ObservedState: shell}, nil
}

func (h *Host) sendSystemMail(from, to, subject, body string) (Result, error) {
	if from == "" || to == "" || subject == "" {
		return Result{}, fmt.Errorf("from, to, and subject are required")
	}
	if strings.ContainsAny(from, "\n\r") || strings.ContainsAny(to, "\n\r") || strings.ContainsAny(subject, "\n\r") {
		return Result{}, fmt.Errorf("mail headers must be single line")
	}
	msg := "From: " + from + "\nTo: " + to + "\nSubject: " + subject + "\n\n" + body + "\n"
	if !h.live() {
		dir := "/var/lib/panel/mail-outbox"
		if resolved, err := h.resolve(dir); err == nil {
			_ = os.MkdirAll(resolved, 0o750)
			name := strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + ".eml"
			_ = os.WriteFile(filepath.Join(resolved, name), []byte(msg), 0o640)
		}
		return Result{OK: true, Message: "mail staged", ObservedState: "staged"}, nil
	}
	out, err := runFixedIO("/usr/sbin/sendmail", []byte(msg), "-t", "-f", from)
	if err != nil {
		return Result{}, fmt.Errorf("sendmail: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, ObservedState: "queued"}, nil
}
