package phases

import (
	"encoding/json"
	"fmt"
	"os"
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
		named{"repositories", checkNoop, applyNoop, verifyNoop},
		named{"system_packages", checkNoop, applyNoop, verifyNoop},
		named{"panel_users", checkNoop, applyUsers, verifyNoop},
		named{"control_database", checkNoop, applyControlDB, verifyNoop},
		named{"control_plane", checkNoop, applyControlPlane, verifyNoop},
		named{"web_stack", checkNoop, applyDir("etc/nginx/panel-sites"), verifyNoop},
		named{"database_stack", checkNoop, applyNoop, verifyNoop},
		named{"dns", checkNoop, applyNoop, verifyNoop},
		named{"mail", checkNoop, applyNoop, verifyNoop},
		named{"security", checkNoop, applyNoop, verifyNoop},
		named{"firewall", checkNoop, applyNoop, verifyNoop},
		named{"runtime_versions", checkNoop, applyNoop, verifyNoop},
		named{"templates", checkNoop, applyTemplates, verifyNoop},
		named{"tls", checkNoop, applyNoop, verifyNoop},
		named{"systemd", checkNoop, applyNoop, verifyNoop},
		named{"administrator", checkNoop, applyNoop, verifyNoop},
		named{"health_checks", checkNoop, applyNoop, verifyNoop},
		named{"installation_report", checkNoop, applyNoop, verifyNoop},
	}
}

type named struct {
	name   string
	check  func(Config) error
	apply  func(Config) error
	verify func(Config) error
}

func (n named) Name() string                 { return n.name }
func (n named) Check(c Config) error         { return n.check(c) }
func (n named) Apply(c Config) error         { return n.apply(c) }
func (n named) Verify(c Config) error        { return n.verify(c) }

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
	return os.MkdirAll(root(c, "var/lib/panel"), 0o750)
}

func applyControlDB(c Config) error {
	dir := root(c, "var/lib/panel/control")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	if !c.Dev {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "dsn"), []byte("postgres:///panel_control?host=/var/run/postgresql\n"), 0o640)
}

func applyControlPlane(c Config) error {
	for _, d := range []string{"bin", "run", "jobs", "backups", "releases", "secrets"} {
		if err := os.MkdirAll(root(c, "var/lib/panel/"+d), 0o750); err != nil {
			return err
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
