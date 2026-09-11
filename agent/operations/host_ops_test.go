package operations

import (
	"strings"
	"testing"
)

func TestSetMariaDBRootPasswordRejectsDashPrefix(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.setMariaDBRootPassword("", "-DbRoot!2026"); err == nil {
		t.Fatal("expected dash-prefix password to be rejected before live()")
	}
	if _, err := h.setMariaDBRootPassword("-current", "DbRoot!2026"); err == nil {
		t.Fatal("expected dash-prefix current password to be rejected")
	}
	res, err := h.setMariaDBRootPassword("", "DbRoot!2026")
	if err != nil || res.ObservedState != "staged" {
		t.Fatalf("valid staged password: %v %#v", err, res)
	}
}

func TestRedactProcessCommandHidesPasswordArgv(t *testing.T) {
	cmd := redactProcessCommand("mysqladmin", "mysqladmin -uroot -pSecret password next")
	if strings.Contains(cmd, "Secret") || strings.Contains(cmd, "next") {
		t.Fatalf("password leaked: %s", cmd)
	}
	if redactProcessCommand("nginx", "nginx -t") != "nginx -t" {
		t.Fatal("unrelated commands must stay visible")
	}
	psql := redactProcessCommand("psql", "psql -c CREATE USER shop PASSWORD 'TenantSecret'")
	if strings.Contains(psql, "TenantSecret") {
		t.Fatalf("psql password leaked: %s", psql)
	}
	runuser := redactProcessCommand("runuser", "runuser -u postgres -- psql -c ALTER USER shop PASSWORD 'x'")
	if strings.Contains(runuser, "PASSWORD") {
		t.Fatalf("runuser password leaked: %s", runuser)
	}
}

func TestEnsurePHPRuntimeRejectsUnknownVersion(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.ensurePHPRuntime("7.4"); err == nil {
		t.Fatal("expected unsupported version")
	}
}
