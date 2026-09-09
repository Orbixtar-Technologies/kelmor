package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDumpAndRestoreSandboxSQL(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	dest := "/var/lib/panel/backups/staging/acme_site.sql"
	if _, err := h.dumpHostedDatabase("mariadb", "acme_site", dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "var/lib/panel/backups/staging/acme_site.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("empty dump")
	}
	if _, err := h.restoreHostedDatabase("mariadb", "acme_site", dest, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dumpHostedDatabase("mariadb", "acme_site", "/tmp/evil.sql"); err == nil {
		t.Fatal("expected path reject")
	}
}

func TestRunFixedRejectsSemicolonSQLArg(t *testing.T) {
	if _, err := runFixed("/usr/bin/psql", "-c", "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err == nil {
		t.Fatal("semicolon SQL in argv must be rejected")
	}
}

func TestDumpAndRestoreSandboxPostgres(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	dest := "/var/lib/panel/backups/staging/acme_pg.sql"
	if _, err := h.dumpHostedDatabase("postgres", "acme_pg", dest); err != nil {
		t.Fatal(err)
	}
	if _, err := h.restoreHostedDatabase("postgres", "acme_pg", dest, ""); err != nil {
		t.Fatal(err)
	}
}
