package operations

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func reloadFPM(version string) {
	pidFile := "/run/php/php" + version + "-fpm.pid"
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 1 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGUSR2)
}

func (h *Host) applyPHPPool(account, version string, maxChildren int) (Result, error) {
	if err := validate.Username(account); err != nil {
		return Result{}, err
	}
	switch version {
	case "8.3", "8.4", "8.5", "":
	default:
		return Result{}, fmt.Errorf("unknown PHP version")
	}
	if version == "" {
		version = "8.3"
	}
	groupName := account
	if h.live() {
		u, err := user.Lookup(account)
		if err != nil {
			return Result{}, fmt.Errorf("unix user %s not ready", account)
		}
		if g, err := user.LookupGroupId(u.Gid); err == nil {
			groupName = g.Name
		} else if _, err := user.LookupGroup(account); err != nil {
			return Result{}, fmt.Errorf("unix group for %s not ready", account)
		}
	}
	body := configuration.PHPPoolFor(account, groupName, version, maxChildren)
	path := fmt.Sprintf("/etc/php/%s/fpm/pool.d/panel-%s.conf", version, account)
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	if h.live() {
		ensureFPM(version)
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func ensureFPM(version string) {
	pidFile := "/run/php/php" + version + "-fpm.pid"
	if b, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
			if err := syscall.Kill(pid, 0); err == nil {
				reloadFPM(version)
				return
			}
		}
	}
	bin := "/usr/sbin/php-fpm" + version
	_, _ = startDetached(bin, "/")
}

func (h *Host) applySlice(username string, cpu int, memory int64, tasks, ioWeight, iops int) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if tasks < 1 {
		tasks = 100
	}
	body := configuration.SystemdSlice(username, cpu, memory, tasks, ioWeight, iops)
	_, err := h.ApplyFile("/etc/systemd/system/panel-account-"+username+".slice", []byte(body), 0o644)
	if err != nil {
		return Result{}, err
	}
	if err := h.applyCgroupLimits(username, cpu, memory, tasks, ioWeight, iops); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func (h *Host) setQuota(username string, bytes int64) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if err := h.persistQuota(username, bytes); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, ObservedState: "applied"}, nil
	}
	if _, err := os.Stat("/usr/sbin/setquota"); err != nil {
		return Result{OK: true, ObservedState: "skipped"}, nil
	}
	blocks := bytes / 1024
	soft := fmt.Sprintf("%d", blocks)
	hard := fmt.Sprintf("%d", blocks)
	if out, err := runFixed("/usr/sbin/setquota", "-u", username, soft, hard, "0", "0", "/var/lib/panel/homes"); err == nil {
		return Result{OK: true, ObservedState: "applied"}, nil
	} else if strings.Contains(string(out), "No such file") || strings.Contains(string(out), "not found") {
		// fall through to -a
	}
	out, err := runFixed("/usr/sbin/setquota", "-u", username, soft, hard, "0", "0", "-a")
	if err != nil {
		return Result{OK: true, Message: strings.TrimSpace(string(out)), ObservedState: "skipped"}, nil
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func (h *Host) createHostedDatabase(engine, name, dbUser, password string) (Result, error) {
	if !ident(name) || !ident(dbUser) {
		return Result{}, fmt.Errorf("invalid database identifier")
	}
	if password == "" {
		return Result{}, fmt.Errorf("password required")
	}
	switch engine {
	case "mariadb", "mysql":
		if !h.live() {
			return Result{OK: true, ObservedState: "recorded"}, nil
		}
		stmts := []string{
			fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", name),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'", dbUser, escapeSQL(password)),
			fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'", dbUser, escapeSQL(password)),
			fmt.Sprintf("GRANT ALL ON %s.* TO '%s'@'localhost'", name, dbUser),
			"FLUSH PRIVILEGES",
		}
		for _, stmt := range stmts {
			out, err := runFixed("/usr/bin/mariadb", "-e", stmt)
			if err != nil {
				return Result{}, fmt.Errorf("mariadb: %s", strings.TrimSpace(string(out)))
			}
		}
	case "postgres":
		if !h.live() {
			return Result{OK: true, ObservedState: "recorded"}, nil
		}
		_, _ = runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE USER "+dbUser+" PASSWORD '"+escapeSQL(password)+"'")
		_, _ = runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "ALTER USER "+dbUser+" PASSWORD '"+escapeSQL(password)+"'")
		if out, err := runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE DATABASE "+name+" OWNER "+dbUser); err != nil {
			if !strings.Contains(string(out), "already exists") {
				return Result{}, fmt.Errorf("psql: %s", strings.TrimSpace(string(out)))
			}
		}
	default:
		return Result{}, fmt.Errorf("unsupported engine")
	}
	return Result{OK: true, ObservedState: "active"}, nil
}

func (h *Host) dropHostedDatabase(engine, name, dbUser string) (Result, error) {
	if !ident(name) || !ident(dbUser) {
		return Result{}, fmt.Errorf("invalid database identifier")
	}
	if !h.live() {
		return Result{OK: true, ObservedState: "absent"}, nil
	}
	switch engine {
	case "mariadb", "mysql":
		stmts := []string{
			fmt.Sprintf("DROP DATABASE IF EXISTS %s", name),
			fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'", dbUser),
			"FLUSH PRIVILEGES",
		}
		for _, stmt := range stmts {
			out, err := runFixed("/usr/bin/mariadb", "-e", stmt)
			if err != nil {
				return Result{}, fmt.Errorf("mariadb: %s", strings.TrimSpace(string(out)))
			}
		}
	case "postgres":
		if out, err := runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "DROP DATABASE IF EXISTS "+name); err != nil {
			return Result{}, fmt.Errorf("psql: %s", strings.TrimSpace(string(out)))
		}
		_, _ = runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "DROP USER IF EXISTS "+dbUser)
	default:
		return Result{}, fmt.Errorf("unsupported engine")
	}
	return Result{OK: true, ObservedState: "absent"}, nil
}

func ident(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
		if i == 0 && r >= '0' && r <= '9' {
			return false
		}
	}
	return true
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
