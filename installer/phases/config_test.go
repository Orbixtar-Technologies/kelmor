package phases

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInstallFileReadsDeclaredFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.yaml")
	body := "hostname: panel.example.net\n" +
		"admin_email: ops@example.net\n" +
		"channel: beta\n" +
		"acme: staging\n" +
		"non_interactive: true\n" +
		"dev: false\n" +
		"root: /srv/kelmor\n"
	if err := writeFile(t, path, body); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadInstallFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hostname != "panel.example.net" || cfg.AdminEmail != "ops@example.net" {
		t.Fatalf("identity: %#v", cfg)
	}
	if cfg.Channel != "beta" || cfg.ACMEMode != "staging" {
		t.Fatalf("channel/acme: %#v", cfg)
	}
	if !cfg.NonInteractive || cfg.Dev || cfg.Root != "/srv/kelmor" {
		t.Fatalf("flags: %#v", cfg)
	}
}

func TestLoadInstallFileRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.yaml")
	if err := writeFile(t, path, "hostname: a.test\nunexpected: yes\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadInstallFile(path); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestMergeInstallConfigCLIWinsWhenFlagSet(t *testing.T) {
	file := Config{Hostname: "file.test", AdminEmail: "file@test", Channel: "beta", ACMEMode: "staging"}
	cli := Config{Hostname: "cli.test", Channel: "stable"}
	got := MergeInstallConfig(file, cli, map[string]bool{"hostname": true, "channel": true})
	if got.Hostname != "cli.test" {
		t.Fatalf("hostname %q", got.Hostname)
	}
	if got.Channel != "stable" {
		t.Fatalf("channel %q", got.Channel)
	}
	if got.AdminEmail != "file@test" || got.ACMEMode != "staging" {
		t.Fatalf("file fields dropped: %#v", got)
	}
}

func TestDiscoverInstallFilePrefersExplicitThenCwdThenExe(t *testing.T) {
	root := t.TempDir()
	explicit := filepath.Join(root, "explicit.yaml")
	cwdFile := filepath.Join(root, "cwd", "install.yaml")
	exeFile := filepath.Join(root, "exe", "install.yaml")
	if err := writeFile(t, explicit, "hostname: explicit.test\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(t, cwdFile, "hostname: cwd.test\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(t, exeFile, "hostname: exe.test\n"); err != nil {
		t.Fatal(err)
	}
	if got := DiscoverInstallFile(explicit, filepath.Dir(exeFile), filepath.Dir(cwdFile)); got != explicit {
		t.Fatalf("explicit %q", got)
	}
	if got := DiscoverInstallFile("", filepath.Dir(exeFile), filepath.Dir(cwdFile)); got != cwdFile {
		t.Fatalf("cwd %q", got)
	}
	if got := DiscoverInstallFile("", filepath.Dir(exeFile), t.TempDir()); got != exeFile {
		t.Fatalf("exe %q", got)
	}
}

func TestPromptMissingFillsBlankFields(t *testing.T) {
	cfg := Config{}
	in := strings.NewReader("lab.kelmor.host\nops@kelmor.host\nstaging\n")
	var out bytes.Buffer
	if err := PromptMissing(&cfg, in, &out); err != nil {
		t.Fatal(err)
	}
	if cfg.Hostname != "lab.kelmor.host" || cfg.AdminEmail != "ops@kelmor.host" || cfg.ACMEMode != "staging" {
		t.Fatalf("prompted: %#v", cfg)
	}
	if !strings.Contains(out.String(), "Hostname") {
		t.Fatalf("prompts: %s", out.String())
	}
}

func TestPromptMissingSkipsWhenNonInteractive(t *testing.T) {
	cfg := Config{NonInteractive: true}
	if err := PromptMissing(&cfg, strings.NewReader("should-not-read\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if cfg.Hostname != "" {
		t.Fatalf("prompted despite non-interactive: %#v", cfg)
	}
}

func writeFile(t *testing.T, path, body string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}
