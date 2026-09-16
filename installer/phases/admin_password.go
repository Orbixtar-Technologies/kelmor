package phases

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/store"
)

var applyLiveAdminPassword = updateLiveAdminPassword

func updateLiveAdminPassword(c Config, password string) error {
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	admin := strings.TrimSpace(os.Getenv("PANEL_ADMIN_USER"))
	if admin == "" {
		admin = "admin"
	}
	storeErr := resetAdminPasswordViaStore(admin, password)
	if storeErr == nil || errors.Is(storeErr, store.ErrAdminMissing) {
		return nil
	}
	peerErr := resetAdminPasswordViaPeer(admin, password)
	if peerErr == nil || errors.Is(peerErr, store.ErrAdminMissing) {
		return nil
	}
	return fmt.Errorf("control database password was not updated (peer %v; direct %v)", peerErr, storeErr)
}

func resetAdminPasswordViaStore(admin, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	dsn := os.Getenv("PANEL_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	pg, err := store.OpenPostgres(ctx, dsn)
	if err != nil {
		return err
	}
	defer pg.Close()
	return store.ResetAdminPassword(pg, admin, password)
}

func resetAdminPasswordViaPeer(admin, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	out, err := runControlSQL(adminPasswordSQL(admin, hash))
	if err != nil {
		return err
	}
	return parseAdminPasswordResult(out)
}

func adminPasswordSQL(admin, hash string) string {
	quotedAdmin := pgLiteral(admin)
	quotedHash := pgLiteral(hash)
	return strings.Join([]string{
		"BEGIN;",
		"UPDATE users SET password_hash = " + quotedHash + ", must_change_password = false, updated_at = now() WHERE username = " + quotedAdmin + ";",
		"UPDATE sessions SET revoked_at = now() WHERE revoked_at IS NULL AND user_id IN (SELECT id FROM users WHERE username = " + quotedAdmin + ");",
		"COMMIT;",
		"SELECT CASE WHEN EXISTS (SELECT 1 FROM users WHERE username = " + quotedAdmin + ") THEN 'updated' ELSE 'missing' END;",
	}, "\n")
}

func pgLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func parseAdminPasswordResult(out string) error {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || strings.TrimSpace(out) == "" {
		return fmt.Errorf("empty psql result")
	}
	switch strings.TrimSpace(lines[len(lines)-1]) {
	case "updated":
		return nil
	case "missing":
		return store.ErrAdminMissing
	default:
		return fmt.Errorf("unexpected psql result %q", strings.TrimSpace(out))
	}
}

func runControlSQL(sql string) (string, error) {
	psql := []string{"/usr/bin/psql", "-d", "panel_control", "-v", "ON_ERROR_STOP=1", "-At", "-q"}
	attempts := [][]string{
		psql,
		peerExecArgs("panel", psql...),
		peerExecArgs("postgres", psql...),
	}
	var last error
	for _, args := range attempts {
		if len(args) == 0 {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(sql)
		cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=5")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
		last = fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	if last == nil {
		return "", fmt.Errorf("psql is not available")
	}
	return "", last
}

func peerExecArgs(osUser string, args ...string) []string {
	if os.Geteuid() != 0 && currentUsername() != osUser {
		return nil
	}
	if _, err := os.Stat("/usr/bin/sudo"); err == nil {
		return append([]string{"/usr/bin/sudo", "-n", "-u", osUser, "--"}, args...)
	}
	if _, err := os.Stat("/usr/sbin/runuser"); err == nil {
		return append([]string{"/usr/sbin/runuser", "-u", osUser, "--"}, args...)
	}
	return nil
}

func currentUsername() string {
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("LOGNAME")); u != "" {
		return u
	}
	return ""
}
