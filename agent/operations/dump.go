package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/policy"
)

func (h *Host) dumpHostedDatabase(engine, name, dest string) (Result, error) {
	if !ident(name) {
		return Result{}, fmt.Errorf("invalid database identifier")
	}
	if err := validateStagingSQL(dest); err != nil {
		return Result{}, err
	}
	out, err := h.resolve(dest)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o775); err != nil {
		return Result{}, err
	}
	if !h.live() {
		body := []byte("-- panel sandbox dump " + engine + " " + name + "\n")
		if err := os.WriteFile(out, body, 0o664); err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "dumped"}, nil
	}
	var raw []byte
	switch engine {
	case "mariadb", "mysql":
		bin := dumpBinary()
		raw, err = runFixedStdout(bin, "--single-transaction", name)
		if err != nil {
			return Result{}, fmt.Errorf("dump: %s", err)
		}
	case "postgres":
		raw, err = runFixedStdout("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/pg_dump", "-d", name, "--no-owner")
		if err != nil {
			return Result{}, fmt.Errorf("pg_dump: %s", err)
		}
	default:
		return Result{}, fmt.Errorf("unsupported engine")
	}
	if err := os.WriteFile(out, raw, 0o664); err != nil {
		return Result{}, err
	}
	_ = os.Chmod(out, 0o664)
	return Result{OK: true, ObservedState: "dumped"}, nil
}

func (h *Host) restoreHostedDatabase(engine, name, source string) (Result, error) {
	if !ident(name) {
		return Result{}, fmt.Errorf("invalid database identifier")
	}
	if err := validateStagingSQL(source); err != nil {
		return Result{}, err
	}
	in, err := h.resolve(source)
	if err != nil {
		return Result{}, err
	}
	sql, err := os.ReadFile(in)
	if err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, ObservedState: "restored"}, nil
	}
	switch engine {
	case "mariadb", "mysql":
		if out, err := runFixed("/usr/bin/mariadb", "-e", "CREATE DATABASE IF NOT EXISTS "+name); err != nil {
			return Result{}, fmt.Errorf("mariadb: %s", strings.TrimSpace(string(out)))
		}
		if out, err := runFixedIO("/usr/bin/mariadb", sql, name); err != nil {
			return Result{}, fmt.Errorf("mariadb restore: %s", strings.TrimSpace(string(out)))
		}
	case "postgres":
		_, _ = runFixed("/usr/sbin/runuser", "-u", "postgres", "--", "/usr/bin/psql", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE DATABASE "+name)
		if out, err := runFixedIO("/usr/sbin/runuser", sql, "-u", "postgres", "--", "/usr/bin/psql", "-d", name, "-v", "ON_ERROR_STOP=1"); err != nil {
			return Result{}, fmt.Errorf("psql restore: %s", strings.TrimSpace(string(out)))
		}
	default:
		return Result{}, fmt.Errorf("unsupported engine")
	}
	return Result{OK: true, ObservedState: "restored"}, nil
}

func dumpBinary() string {
	for _, p := range []string{"/usr/bin/mariadb-dump", "/usr/bin/mysqldump"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/bin/mariadb-dump"
}

func validateStagingSQL(p string) error {
	clean, err := policy.ValidateManagedPath(p)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(clean, "/var/lib/panel/backups/") {
		return fmt.Errorf("dump path must be under backup staging")
	}
	if !strings.HasSuffix(clean, ".sql") {
		return fmt.Errorf("unexpected dump suffix")
	}
	return nil
}
