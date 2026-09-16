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
	adminPassword := flag.String("admin-password", os.Getenv("PANEL_ADMIN_PASSWORD"), "Director admin password (min 12 chars; generated if empty)")
	setAdminPassword := flag.String("set-admin-password", "", "set Director admin password and exit")
	channel := flag.String("channel", "stable", "release channel")
	nonInteractive := flag.Bool("non-interactive", false, "unattended install")
	config := flag.String("config", "", "install.yaml")
	dev := flag.Bool("dev", false, "development sandbox install")
	acme := flag.String("acme", "", "letsencrypt, staging, pebble/lab (default: keep an existing lab directory, otherwise Let's Encrypt)")
	installRoot := flag.String("root", os.Getenv("PANEL_INSTALL_ROOT"), "filesystem prefix for packaged file writes")
	flag.Parse()
	phases.LoadValidationEnv()
	if strings.TrimSpace(*adminPassword) == "" {
		*adminPassword = os.Getenv("PANEL_ADMIN_PASSWORD")
	}

	setFlags := map[string]bool{}
	flag.CommandLine.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	cwd, _ := os.Getwd()
	configPath := phases.DiscoverInstallFile(*config, exeDir, cwd)
	file := phases.Config{}
	if configPath != "" {
		loaded, err := phases.LoadInstallFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
		file = loaded
		file.ConfigPath = configPath
	}
	cli := phases.Config{
		Hostname: *hostname, AdminEmail: *adminEmail, AdminPassword: strings.TrimSpace(*adminPassword),
		Channel: *channel, NonInteractive: *nonInteractive, Dev: *dev || os.Getenv("PANEL_DEV") == "1",
		ConfigPath: *config, ACMEMode: *acme, Root: strings.TrimSpace(*installRoot),
	}
	if os.Getenv("PANEL_DEV") == "1" {
		cli.Dev = true
	}
	cfg := phases.MergeInstallConfig(file, cli, setFlags)
	if !cfg.NonInteractive && stdinIsTTY() {
		if err := phases.PromptMissing(&cfg, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "prompt: %v\n", err)
			os.Exit(1)
		}
	}

	statePath := "/var/lib/panel/install-state.json"
	if cfg.Dev {
		statePath = filepath.Join("var", "panel", "install-state.json")
	} else if strings.TrimSpace(cfg.Root) != "" {
		statePath = filepath.Join(cfg.Root, "var/lib/panel/install-state.json")
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
	if pass := strings.TrimSpace(*setAdminPassword); pass != "" {
		if err := phases.SetAdministratorPassword(cfg, pass); err != nil {
			fmt.Fprintf(os.Stderr, "admin password: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Administrator password updated for user admin.")
		fmt.Println("Also stored at /var/lib/panel/secrets/admin-bootstrap (root-only).")
		os.Exit(0)
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
	secrets, err := phases.PeekInstallSecrets(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin password: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(`Installation successful

Kelmor Director URL: https://%s:2087/
Kelmor Control URL:  https://%s:2083/
Administrator:      admin
Password:           %s
Installation ID:    %s
Release:            %s
Go/arch:            %s/%s
Channel:            %s
Firewall:           table inet panel (dev sandbox)
DNS nameservers:    ns1.%s ns2.%s
Mail:               SMTP relay from .run/validation/smtp.env when present; otherwise configure a relay if port 25 is blocked

Change the password later with:
  sudo panel-install --set-admin-password 'your-new-password'
The bootstrap secret is also at /var/lib/panel/secrets/admin-bootstrap (root-only).
`, cfg.Hostname, cfg.Hostname, secrets.AdminPassword, st.InstallationID, st.Release, runtime.Version(), runtime.GOARCH, cfg.Channel, cfg.Hostname, cfg.Hostname)
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
