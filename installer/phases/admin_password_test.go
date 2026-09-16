package phases

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/store"
)

func TestPgLiteralEscapesQuotes(t *testing.T) {
	if got := pgLiteral("a'b"); got != "'a''b'" {
		t.Fatalf("got %q", got)
	}
}

func TestAdminPasswordSQLQuotesArgonHash(t *testing.T) {
	hash, err := auth.HashPassword("OperatorPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	sql := adminPasswordSQL("admin", hash)
	if !strings.Contains(sql, "must_change_password = false") {
		t.Fatalf("missing must_change clear: %s", sql)
	}
	if !strings.Contains(sql, pgLiteral(hash)) {
		t.Fatalf("hash not quoted as a literal: %s", sql)
	}
	if strings.Contains(sql, hash+"") && strings.Contains(sql, "password_hash = "+hash) {
		t.Fatal("hash interpolated without quoting")
	}
}

func TestParseAdminPasswordResult(t *testing.T) {
	if err := parseAdminPasswordResult("BEGIN\nCOMMIT\nupdated\n"); err != nil {
		t.Fatal(err)
	}
	if err := parseAdminPasswordResult("missing\n"); !errors.Is(err, store.ErrAdminMissing) {
		t.Fatalf("got %v", err)
	}
	if err := parseAdminPasswordResult("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetAdministratorPasswordFailsWhenDatabaseWriteFails(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	original := applyLiveAdminPassword
	applyLiveAdminPassword = func(Config, string) error {
		return errors.New(`failed to connect to user=root database=panel_control: role "root" does not exist`)
	}
	t.Cleanup(func() { applyLiveAdminPassword = original })

	err = SetAdministratorPassword(Config{Hostname: "kelmor.host", Dev: true}, "OperatorPass!2026")
	if err == nil {
		t.Fatal("expected database write failure to fail the command")
	}
	if !strings.Contains(err.Error(), `role "root" does not exist`) {
		t.Fatalf("got %v", err)
	}
}

func TestSetAdministratorPasswordWritesSecretInDev(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cfg := Config{Hostname: "kelmor.host", Dev: true, AdminPassword: "OperatorPass!2026"}
	if err := SetAdministratorPassword(cfg, "OperatorPass!2026"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "var/panel/host/var/lib/panel/secrets/admin-bootstrap"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "OperatorPass!2026" {
		t.Fatalf("secret %q", body)
	}
}

func TestUpdateLiveAdminPasswordSkipsPrefixedInstalls(t *testing.T) {
	if err := updateLiveAdminPassword(Config{Dev: true}, "OperatorPass!2026"); err != nil {
		t.Fatal(err)
	}
	if err := updateLiveAdminPassword(Config{Root: "/tmp/panel-prefix"}, "OperatorPass!2026"); err != nil {
		t.Fatal(err)
	}
}
