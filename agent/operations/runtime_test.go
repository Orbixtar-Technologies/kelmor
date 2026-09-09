package operations

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestProcessInDirSeesCurrentWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("/bin/sleep", "30")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	if !processInDir(dir) {
		t.Fatal("expected a child process in dir")
	}
	other := t.TempDir()
	if processInDir(other) {
		t.Fatal("empty directory should not look occupied")
	}
}

func TestIdent(t *testing.T) {
	if !ident("livehost_shop") || !ident("livehost_u") {
		t.Fatal("valid identifiers rejected")
	}
	if ident("1bad") || ident("bad-name") || ident("") {
		t.Fatal("invalid identifiers accepted")
	}
}

func TestDropHostedDatabaseRejectsIdent(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.dropHostedDatabase("mariadb", "bad-name", "okuser", true); err == nil {
		t.Fatal("expected invalid identifier")
	}
	res, err := h.dropHostedDatabase("mariadb", "okdb", "okuser", true)
	if err != nil || !res.OK {
		t.Fatalf("%v %#v", err, res)
	}
}

func TestApplyPHPPoolWritesSandboxConfig(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	res, err := h.applyPHPPool("acme42", "8.3", 6)
	if err != nil || !res.OK {
		t.Fatalf("%v %#v", err, res)
	}
	body, err := os.ReadFile(root + "/etc/php/8.3/fpm/pool.d/panel-acme42.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("listen = /run/php/panel-acme42.sock")) {
		t.Fatalf("%s", body)
	}
}

func TestApplyAppUnitRejectsBoundaryAndSystemdInjection(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	tests := []struct {
		name      string
		websiteID string
		runtime   string
		workDir   string
		command   string
	}{
		{name: "outside account", websiteID: "site-1", runtime: "node", workDir: "/home/other/app", command: "npm start"},
		{name: "noncanonical path", websiteID: "site-1", runtime: "node", workDir: "/home/acme/app/../escape", command: "npm start"},
		{name: "website newline", websiteID: "site-1\n[Service]", runtime: "node", workDir: "/home/acme/app", command: "npm start"},
		{name: "runtime newline", websiteID: "site-1", runtime: "node\nUser=root", workDir: "/home/acme/app", command: "npm start"},
		{name: "command newline", websiteID: "site-1", runtime: "node", workDir: "/home/acme/app", command: "npm start\nUser=root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := h.applyAppUnit(tt.websiteID, "acme", tt.runtime, tt.workDir, tt.command); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if result, err := h.applyAppUnit("site-1", "acme", "node", "/home/acme/app", "npm start"); err != nil || !result.OK {
		t.Fatalf("valid app unit: result=%+v err=%v", result, err)
	}
}

func TestCreateHostedDatabaseRejectsIdent(t *testing.T) {
	h := &Host{}
	if _, err := h.createHostedDatabase("mariadb", "bad-name", "okuser", "pw", false); err == nil {
		t.Fatal("expected invalid identifier")
	}
	res, err := h.createHostedDatabase("mariadb", "okdb", "okuser", "pw", true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal(res)
	}
}

func TestRunFixedRejectsSQLSemicolon(t *testing.T) {
	if _, err := runFixed("/usr/bin/mariadb", "-e", "SELECT 1; SELECT 2"); err == nil {
		t.Fatal("semicolon batch must be rejected — issue one statement per call")
	}
	if _, err := runFixed("/usr/bin/mariadb", "-e", "CREATE DATABASE `x`"); err == nil {
		t.Fatal("backticks must be rejected — use validated unquoted identifiers")
	}
}
