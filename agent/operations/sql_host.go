package operations

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	mysqlUpgradePath = "/var/lib/panel/mysql-upgrade.json"
	postgresConfName = "kelmor.conf"
)

type PostgresStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Listen    string `json:"listen,omitempty"`
	Auth      string `json:"auth,omitempty"`
	Message   string `json:"message,omitempty"`
}

type MySQLUpgradeStatus struct {
	Installed   bool     `json:"installed"`
	Engine      string   `json:"engine,omitempty"`
	Version     string   `json:"version,omitempty"`
	Major       string   `json:"major,omitempty"`
	UpgradeTool string   `json:"upgrade_tool,omitempty"`
	Targets     []string `json:"targets"`
	Message     string   `json:"message,omitempty"`
}

func (h *Host) sandboxPath(p string) string {
	if h == nil || h.Root == "" {
		return p
	}
	return filepath.Join(h.Root, strings.TrimPrefix(p, "/"))
}

func postgresAuthOK(auth string) bool {
	switch auth {
	case "peer", "md5", "scram-sha-256":
		return true
	default:
		return false
	}
}

func postgresListenOK(listen string) bool {
	listen = strings.TrimSpace(listen)
	if listen == "" || strings.ContainsAny(listen, " \t\n;|&$`'\"") {
		return false
	}
	switch listen {
	case "*", "localhost", "0.0.0.0", "::", "::1":
		return true
	default:
		return net.ParseIP(listen) != nil
	}
}

func postgresHostAuth(auth string) string {
	if auth == "peer" {
		return "scram-sha-256"
	}
	return auth
}

func (h *Host) postgresClusters() []string {
	base := h.sandboxPath("/etc/postgresql")
	ents, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []string
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		cluster := filepath.Join("/etc/postgresql", ent.Name(), "main")
		if _, err := os.Stat(h.sandboxPath(filepath.Join(cluster, "pg_hba.conf"))); err == nil {
			out = append(out, cluster)
		}
	}
	return out
}

func (h *Host) probePostgres() PostgresStatus {
	clusters := h.postgresClusters()
	if len(clusters) == 0 {
		return PostgresStatus{
			Installed: false,
			Message:   "PostgreSQL is not installed on this host",
		}
	}
	cluster := clusters[0]
	status := PostgresStatus{
		Installed: true,
		Version:   postgresVersionFromCluster(cluster),
		Cluster:   cluster,
		Listen:    "127.0.0.1",
		Auth:      "scram-sha-256",
	}
	if raw, err := h.readManaged(filepath.Join(cluster, "conf.d", postgresConfName), 1<<16); err == nil {
		status.Listen = parsePostgresListen(string(raw), status.Listen)
	}
	if raw, err := h.readManaged(filepath.Join(cluster, "pg_hba.conf"), 1<<20); err == nil {
		status.Auth = parsePostgresHostAuth(string(raw), status.Auth)
	}
	return status
}

func postgresVersionFromCluster(cluster string) string {
	rest := strings.TrimPrefix(cluster, "/etc/postgresql/")
	version, _, _ := strings.Cut(rest, "/")
	return version
}

func parsePostgresListen(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "listen_addresses" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "'\"")
		if value != "" {
			return value
		}
	}
	return fallback
}

func parsePostgresHostAuth(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[0] != "host" && fields[0] != "hostssl" && fields[0] != "hostnossl" {
			continue
		}
		method := fields[4]
		if postgresAuthOK(method) {
			return method
		}
	}
	return fallback
}

func (h *Host) applyPostgresConfig(spec HostConfigSpec) error {
	listen := strings.TrimSpace(spec.PostgresListen)
	if listen == "" {
		listen = "127.0.0.1"
	}
	auth := strings.TrimSpace(spec.PostgresAuth)
	if auth == "" {
		auth = "scram-sha-256"
	}
	if !postgresListenOK(listen) {
		return fmt.Errorf("invalid PostgreSQL listen address")
	}
	if !postgresAuthOK(auth) {
		return fmt.Errorf("unsupported PostgreSQL auth method")
	}
	clusters := h.postgresClusters()
	if len(clusters) == 0 {
		return fmt.Errorf("PostgreSQL is not installed on this host")
	}
	hostAuth := postgresHostAuth(auth)
	conf := "# Kelmor PostgreSQL listen — written by ApplyHostConfig\nlisten_addresses = '" + listen + "'\n"
	for _, cluster := range clusters {
		if _, err := h.ApplyFile(filepath.Join(cluster, "conf.d", postgresConfName), []byte(conf), 0o644); err != nil {
			return err
		}
		existing := ""
		if raw, err := h.readManaged(filepath.Join(cluster, "pg_hba.conf"), 1<<20); err == nil {
			existing = string(raw)
		}
		body := rewritePgHBA(existing, hostAuth, listen)
		if _, err := h.ApplyFile(filepath.Join(cluster, "pg_hba.conf"), []byte(body), 0o640); err != nil {
			return err
		}
	}
	if h.live() {
		_ = reloadNamedService("postgresql")
	}
	return nil
}

func rewritePgHBA(existing, hostAuth, listen string) string {
	var b strings.Builder
	if existing == "" {
		b.WriteString("local   all             postgres                                peer\n")
		b.WriteString("local   all             all                                     peer\n")
	}
	sawHost := false
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if existing != "" {
				b.WriteString(line)
				if !strings.HasSuffix(line, "\n") && line != "" {
					b.WriteByte('\n')
				} else if line == "" {
					b.WriteByte('\n')
				}
			}
			continue
		}
		fields := strings.Fields(trimmed)
		if fields[0] == "local" {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if fields[0] == "host" || fields[0] == "hostssl" || fields[0] == "hostnossl" {
			sawHost = true
			if len(fields) >= 5 {
				fmt.Fprintf(&b, "%-7s %-15s %-15s %-23s %s\n", fields[0], fields[1], fields[2], fields[3], hostAuth)
			} else {
				b.WriteString(line)
				b.WriteByte('\n')
			}
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if !sawHost {
		b.WriteString(fmt.Sprintf("host    all             all             127.0.0.1/32            %s\n", hostAuth))
		b.WriteString(fmt.Sprintf("host    all             all             ::1/128                 %s\n", hostAuth))
	}
	if listen == "*" || listen == "0.0.0.0" {
		body := b.String()
		if !strings.Contains(body, "0.0.0.0/0") {
			b.WriteString(fmt.Sprintf("host    all             all             0.0.0.0/0               %s\n", hostAuth))
		}
		if !strings.Contains(body, "::/0") {
			b.WriteString(fmt.Sprintf("host    all             all             ::/0                    %s\n", hostAuth))
		}
	}
	return b.String()
}

func mysqlUpgradeTargets() []string {
	return []string{"10.11", "11.4"}
}

func mysqlTargetOK(target string) bool {
	for _, allowed := range mysqlUpgradeTargets() {
		if target == allowed {
			return true
		}
	}
	return false
}

func (h *Host) probeMySQL() MySQLUpgradeStatus {
	status := MySQLUpgradeStatus{
		Targets: mysqlUpgradeTargets(),
	}
	if h.filePresent("/usr/sbin/mariadbd") || h.filePresent("/usr/bin/mariadb-upgrade") || h.filePresent("/usr/bin/mariadb") {
		status.Installed = true
		status.Engine = "mariadb"
		status.UpgradeTool = "/usr/bin/mariadb-upgrade"
	} else if h.filePresent("/usr/sbin/mysqld") || h.filePresent("/usr/bin/mysql_upgrade") {
		status.Installed = true
		status.Engine = "mysql"
		status.UpgradeTool = "/usr/bin/mysql_upgrade"
	}
	if !status.Installed {
		status.Message = "MariaDB/MySQL is not installed on this host"
		return status
	}
	status.Version, status.Major = h.mysqlVersion(status.Engine)
	if !h.filePresent(status.UpgradeTool) {
		status.UpgradeTool = ""
		status.Message = status.Engine + " is installed but the upgrade helper is missing"
		return status
	}
	status.Message = "Host " + status.Engine + " can be upgraded through a typed Agent job"
	return status
}

func (h *Host) filePresent(path string) bool {
	_, err := os.Stat(h.sandboxPath(path))
	return err == nil
}

func (h *Host) mysqlVersion(engine string) (version, major string) {
	if !h.live() {
		return "", ""
	}
	bin := "/usr/sbin/mariadbd"
	if engine == "mysql" {
		bin = "/usr/sbin/mysqld"
	}
	out, err := runFixed(bin, "--version")
	if err != nil {
		return "", ""
	}
	return parseMySQLVersion(string(out))
}

func parseMySQLVersion(raw string) (version, major string) {
	fields := strings.Fields(raw)
	for _, field := range fields {
		field = strings.Trim(field, ",")
		parts := strings.Split(field, ".")
		if len(parts) < 2 {
			continue
		}
		ok := true
		for _, part := range parts {
			if part == "" {
				ok = false
				break
			}
			for _, r := range part {
				if r < '0' || r > '9' {
					ok = false
					break
				}
			}
		}
		if !ok {
			continue
		}
		version = field
		major = parts[0] + "." + parts[1]
		return version, major
	}
	return "", ""
}

func (h *Host) upgradeMySQL(target string) (Result, error) {
	target = strings.TrimSpace(target)
	if !mysqlTargetOK(target) {
		return Result{}, fmt.Errorf("unsupported MariaDB target version")
	}
	status := h.probeMySQL()
	if !status.Installed {
		return Result{}, fmt.Errorf("MariaDB/MySQL is not installed on this host")
	}
	if !h.live() {
		body, err := json.MarshalIndent(map[string]any{
			"engine": status.Engine, "target": target, "state": "staged",
		}, "", "  ")
		if err != nil {
			return Result{}, err
		}
		if _, err := h.ApplyFile(mysqlUpgradePath, append(body, '\n'), 0o644); err != nil {
			return Result{}, err
		}
		return Result{
			OK:            true,
			Message:       status.Engine + " upgrade to " + target + " staged",
			ObservedState: "staged",
		}, nil
	}
	if status.Major != "" && status.Major != target {
		if err := h.installMariaDBTarget(target, status); err != nil {
			return Result{}, err
		}
	}
	tool := status.UpgradeTool
	if tool == "" {
		return Result{}, fmt.Errorf("%s upgrade helper is not installed", status.Engine)
	}
	out, err := runFixed(tool, "--force")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %s", filepath.Base(tool), strings.TrimSpace(string(out)))
	}
	return Result{
		OK:            true,
		Message:       status.Engine + " upgrade completed",
		ObservedState: "upgraded",
	}, nil
}

func (h *Host) installMariaDBTarget(target string, status MySQLUpgradeStatus) error {
	if status.Engine != "mariadb" {
		return fmt.Errorf("Oracle MySQL major upgrades are not supported; this host runs %s %s", status.Engine, status.Version)
	}
	out, err := runFixed("/usr/bin/apt-cache", "policy", "mariadb-server")
	if err != nil {
		return fmt.Errorf("apt-cache: %s", strings.TrimSpace(string(out)))
	}
	candidate := aptCandidateVersion(string(out))
	if candidate == "" || !strings.HasPrefix(candidate, target) {
		if candidate == "" {
			candidate = "none"
		}
		return fmt.Errorf("MariaDB %s is not available from this host's apt sources (candidate %s). Installed engine is %s %s", target, candidate, status.Engine, status.Version)
	}
	install, err := runFixedEnv(
		h.commandContext(),
		"/usr/bin/apt-get",
		[]string{"DEBIAN_FRONTEND=noninteractive"},
		15*time.Minute,
		nil,
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "Dpkg::Lock::Timeout=120",
		"install", "-y", "mariadb-server",
	)
	if err != nil {
		return fmt.Errorf("apt-get: %s", strings.TrimSpace(string(install)))
	}
	return nil
}

func aptCandidateVersion(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Candidate:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "Candidate:"))
	}
	return ""
}
