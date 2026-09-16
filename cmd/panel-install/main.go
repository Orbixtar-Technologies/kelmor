package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	logPath := filepath.Join(filepath.Dir(statePath), "install.log.jsonl")
	if err := phases.Run(cfg, statePath, logPath, phases.All()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	st, err := phases.LoadState(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "state: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(`Installation successful

Kelmor Director URL: https://%s:8443/
Kelmor Control URL:  https://%s:8444/
Administrator:      admin
Installation ID:    %s
Release:            %s
Go/arch:            %s/%s
Channel:            %s
Firewall:           table inet panel (dev sandbox)
DNS nameservers:    ns1.%s ns2.%s
Mail:               SMTP relay from .run/validation/smtp.env when present; otherwise configure a relay if port 25 is blocked

Administrator password was generated or taken from PANEL_ADMIN_PASSWORD and is not written to this report.
`, cfg.Hostname, cfg.Hostname, st.InstallationID, st.Release, runtime.Version(), runtime.GOARCH, cfg.Channel, cfg.Hostname, cfg.Hostname)
}
