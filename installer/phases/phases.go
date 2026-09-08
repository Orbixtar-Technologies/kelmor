package phases

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hosting-panel/panel/internal/id"
)

type Config struct {
	Hostname       string
	AdminEmail     string
	Channel        string
	NonInteractive bool
	Dev            bool
	ConfigPath     string
}

type Phase interface {
	Name() string
	Check(Config) error
	Apply(Config) error
	Verify(Config) error
}

type Rollbacker interface {
	Rollback(Config) error
}

type State struct {
	InstallationID string            `json:"installation_id"`
	Release        string            `json:"release"`
	Phases         map[string]string `json:"phases"`
}

func LoadState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{InstallationID: id.New(), Release: "0.1.0", Phases: map[string]string{}}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Phases == nil {
		s.Phases = map[string]string{}
	}
	return &s, nil
}

func (s *State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func All() []Phase {
	return []Phase{
		named{"preflight", checkPreflight, applyNoop, verifyNoop},
		named{"repositories", checkNoop, applyRepositories, verifyNoop},
		named{"system_packages", checkNoop, applySystemPackages, verifyNoop},
		named{"panel_users", checkNoop, applyUsers, verifyUsers},
		named{"control_database", checkNoop, applyControlDB, verifyControlDB},
		named{"control_plane", checkNoop, applyControlPlane, verifyControlPlane},
		named{"web_stack", checkNoop, applyWebStack, verifyWeb},
		named{"database_stack", checkNoop, applyDatabaseStack, verifyNoop},
		named{"dns", checkNoop, applyDNS, verifyDNS},
		named{"mail", checkNoop, applyMail, verifyMail},
		named{"security", checkNoop, applySecurity, verifySecurity},
		named{"firewall", checkNoop, applyFirewall, verifyFirewall},
		named{"runtime_versions", checkNoop, applyRuntimeVersions, verifyNoop},
		named{"templates", checkNoop, applyTemplates, verifyNoop},
		named{"tls", checkNoop, applyTLS, verifyNoop},
		named{"systemd", checkNoop, applySystemd, verifySystemd},
		named{"host_runtime", checkNoop, applyHostRuntime, verifyHostRuntime},
		named{"administrator", checkNoop, applyAdministrator, verifyNoop},
		named{"health_checks", checkNoop, applyHealth, verifyHealth},
		named{"installation_report", checkNoop, applyReport, verifyNoop},
	}
}

type named struct {
	name   string
	check  func(Config) error
	apply  func(Config) error
	verify func(Config) error
}

func (n named) Name() string          { return n.name }
func (n named) Check(c Config) error  { return n.check(c) }
func (n named) Apply(c Config) error  { return n.apply(c) }
func (n named) Verify(c Config) error { return n.verify(c) }

func checkPreflight(c Config) error {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
	}
	if !c.Dev {
		if b, err := os.ReadFile("/etc/os-release"); err == nil {
			s := string(b)
			if !strings.Contains(s, "Ubuntu") || !strings.Contains(s, "24.04") {
				return fmt.Errorf("Ubuntu 24.04 LTS required")
			}
		}
	}
	if c.Hostname == "" {
		return fmt.Errorf("hostname required")
	}
	return nil
}

func checkNoop(Config) error  { return nil }
func applyNoop(Config) error  { return nil }
func verifyNoop(Config) error { return nil }

func applyUsers(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel"), 0o750); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	_ = exec.Command("/usr/sbin/groupadd", "--system", "panel").Run()
	_ = exec.Command("/usr/sbin/useradd", "--system", "-g", "panel", "-d", "/var/lib/panel", "-s", "/usr/sbin/nologin", "panel").Run()
	_ = exec.Command("/usr/sbin/usermod", "-aG", "panel", "panel").Run()
	if _, err := user.Lookup("ubuntu"); err == nil {
		_ = exec.Command("/usr/sbin/usermod", "-aG", "panel", "ubuntu").Run()
	}
	return nil
}

func applyControlDB(c Config) error {
	dir := root(c, "var/lib/panel/control")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	dsn := "postgres:///panel_control?host=/var/run/postgresql\n"
	if !c.Dev {
		_ = exec.Command("/usr/bin/sudo", "-u", "postgres", "/usr/bin/createdb", "panel_control").Run()
		_ = exec.Command("/usr/bin/sudo", "-u", "postgres", "/usr/bin/psql", "-d", "panel_control", "-c", "SELECT 1").Run()
	}
	return os.WriteFile(filepath.Join(dir, "dsn"), []byte(dsn), 0o640)
}

func applyControlPlane(c Config) error {
	for _, d := range []string{"bin", "run", "jobs", "backups", "releases", "secrets"} {
		if err := os.MkdirAll(root(c, "var/lib/panel/"+d), 0o750); err != nil {
			return err
		}
	}
	if c.Dev {
		return nil
	}
	dest := "/usr/local/panel/bin"
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	src := controlPlaneSource()
	if src == "" {
		return fmt.Errorf("control plane binaries not found (looked next to panel-install and ./dist/bin)")
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	copied := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "panel-") {
			continue
		}
		in := filepath.Join(src, e.Name())
		b, err := os.ReadFile(in)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, e.Name()), b, 0o755); err != nil {
			return err
		}
		copied++
	}
	if copied == 0 {
		return fmt.Errorf("no panel-* binaries copied from %s", src)
	}
	return nil
}

func controlPlaneSource() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "panel-agent")); err == nil {
			return dir
		}
	}
	if _, err := os.Stat("dist/bin/panel-agent"); err == nil {
		return "dist/bin"
	}
	return ""
}

func verifyUsers(c Config) error {
	if c.Dev {
		return nil
	}
	if err := exec.Command("/usr/bin/getent", "passwd", "panel").Run(); err != nil {
		return fmt.Errorf("panel user missing")
	}
	if err := exec.Command("/usr/bin/getent", "group", "panel").Run(); err != nil {
		return fmt.Errorf("panel group missing")
	}
	return nil
}

func verifyControlDB(c Config) error {
	if _, err := os.Stat(root(c, "var/lib/panel/control/dsn")); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	cmd := exec.Command("/usr/bin/psql", "-d", "panel_control", "-c", "SELECT 1")
	cmd.Env = append(os.Environ(), "PGUSER=postgres")
	if err := cmd.Run(); err != nil {
		if err := exec.Command("/usr/bin/sudo", "-u", "postgres", "/usr/bin/psql", "-d", "panel_control", "-c", "SELECT 1").Run(); err != nil {
			return fmt.Errorf("panel_control database missing")
		}
	}
	return nil
}

func verifyControlPlane(c Config) error {
	if c.Dev {
		return nil
	}
	for _, n := range []string{"panel-api", "panel-agent", "panel-worker", "panel-install"} {
		if _, err := os.Stat("/usr/local/panel/bin/" + n); err != nil {
			return fmt.Errorf("missing %s", n)
		}
	}
	return nil
}

func applyTemplates(c Config) error {
	for _, d := range []string{"nginx", "php", "postfix", "dovecot", "powerdns", "systemd"} {
		if err := os.MkdirAll(root(c, "etc/panel/templates/"+d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func applyDir(rel string) func(Config) error {
	return func(c Config) error { return os.MkdirAll(root(c, rel), 0o755) }
}

func root(c Config, p string) string {
	if c.Dev {
		return filepath.Join("var", "panel", "host", p)
	}
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
