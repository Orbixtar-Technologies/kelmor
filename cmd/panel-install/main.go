package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hosting-panel/panel/installer/phases"
)

func main() {
	hostname := flag.String("hostname", "", "server FQDN")
	adminEmail := flag.String("admin-email", "", "administrator email")
	channel := flag.String("channel", "stable", "release channel")
	nonInteractive := flag.Bool("non-interactive", false, "unattended install")
	config := flag.String("config", "", "install.yaml")
	dev := flag.Bool("dev", false, "development sandbox install")
	acme := flag.String("acme", "", "letsencrypt, staging, pebble/lab (default: keep an existing lab directory, otherwise Let's Encrypt)")
	installRoot := flag.String("root", os.Getenv("PANEL_INSTALL_ROOT"), "filesystem prefix for packaged file writes")
	flag.Parse()
	phases.LoadValidationEnv()

	statePath := "/var/lib/panel/install-state.json"
	if *dev || os.Getenv("PANEL_DEV") == "1" {
		statePath = filepath.Join("var", "panel", "install-state.json")
	} else if strings.TrimSpace(*installRoot) != "" {
		statePath = filepath.Join(*installRoot, "var/lib/panel/install-state.json")
	}
	st, err := phases.LoadState(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "state: %v\n", err)
		os.Exit(1)
	}
	cfg := phases.Config{
		Hostname: *hostname, AdminEmail: *adminEmail, Channel: *channel,
		NonInteractive: *nonInteractive, Dev: *dev || os.Getenv("PANEL_DEV") == "1",
		ConfigPath: *config, ACMEMode: *acme, Root: strings.TrimSpace(*installRoot),
	}
	if cfg.Hostname == "" {
		if h := phases.ValidationHostname(); h != "" {
			cfg.Hostname = h
		} else {
			cfg.Hostname, _ = os.Hostname()
		}
	}
	if cfg.AdminEmail == "" {
		cfg.AdminEmail = "admin@" + cfg.Hostname
	}
	logf := filepath.Join(filepath.Dir(statePath), "install.log.jsonl")
	_ = os.MkdirAll(filepath.Dir(statePath), 0o750)
	log, _ := os.OpenFile(logf, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	defer log.Close()

	for _, p := range phases.All() {
		if st.Phases[p.Name()] == "complete" {
			if err := p.Verify(cfg); err == nil {
				continue
			}
			writeLog(log, p.Name(), "reapply", map[string]any{"reason": "verify failed after claimed complete"})
		}
		st.Phases[p.Name()] = "running"
		_ = st.Save(statePath)
		writeLog(log, p.Name(), "start", nil)
		if err := p.Check(cfg); err != nil {
			fail(st, statePath, log, p.Name(), err)
			os.Exit(1)
		}
		if err := p.Apply(cfg); err != nil {
			if rb, ok := p.(phases.Rollbacker); ok {
				_ = rb.Rollback(cfg)
			}
			fail(st, statePath, log, p.Name(), err)
			os.Exit(1)
		}
		if err := p.Verify(cfg); err != nil {
			fail(st, statePath, log, p.Name(), err)
			os.Exit(1)
		}
		st.Phases[p.Name()] = "complete"
		_ = st.Save(statePath)
		writeLog(log, p.Name(), "complete", nil)
	}
	fmt.Printf(`Installation successful

Kelmor Director URL: https://%s:8443/
Kelmor Control URL:  https://%s:8444/
Administrator:      admin
Installation ID:    %s
Go/arch:            %s/%s
Channel:            %s
Firewall:           table inet panel (dev sandbox)
DNS nameservers:    ns1.%s ns2.%s
Mail:               SMTP relay from .run/validation/smtp.env when present; otherwise configure a relay if port 25 is blocked

Administrator password was generated or taken from PANEL_ADMIN_PASSWORD and is not written to this report.
`, cfg.Hostname, cfg.Hostname, st.InstallationID, runtime.Version(), runtime.GOARCH, cfg.Channel, cfg.Hostname, cfg.Hostname)
}

func fail(st *phases.State, path string, log *os.File, name string, err error) {
	st.Phases[name] = "failed"
	_ = st.Save(path)
	writeLog(log, name, "failed", map[string]any{"error": err.Error()})
	fmt.Fprintf(os.Stderr, "phase %s failed: %v\n", name, err)
}

func writeLog(f *os.File, phase, event string, extra map[string]any) {
	rec := map[string]any{"timestamp": time.Now().UTC().Format(time.RFC3339Nano), "phase": phase, "event": event}
	for k, v := range extra {
		rec[k] = v
	}
	b, _ := json.Marshal(rec)
	_, _ = f.Write(append(b, '\n'))
}

func _() { _ = strings.TrimSpace }
