package operations

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const maxCommandOutput = 1 << 20

var allowedEnvKeys = map[string]bool{
	"CI": true,
	"DEBIAN_FRONTEND": true,
	"GIT_TERMINAL_PROMPT": true,
	"HOME": true,
	"LOGNAME": true,
	"NODE_OPTIONS": true,
	"PATH": true,
	"PNPM_STORE_DIR": true,
	"COREPACK_HOME": true,
	"USER": true,
	"npm_config_cache": true,
	// Narrow GIT_CONFIG_* allow-list for safe.directory only (see validateGitConfigEnv).
	"GIT_CONFIG_COUNT":   true,
	"GIT_CONFIG_KEY_0":   true,
	"GIT_CONFIG_VALUE_0": true,
}

var allowedFlags = map[string]map[string]bool{
	"/usr/sbin/nft":            {"f": true},
	"/usr/bin/setfacl":         {"m": true},
	"/usr/bin/pgrep":           {"x": true},
	"/usr/sbin/useradd":        {"u": true, "g": true, "G": true, "d": true, "s": true, "M": true, "r": true, "m": true},
	"/usr/sbin/userdel":        {"f": true, "r": true},
	"/usr/sbin/usermod":        {"aG": true, "L": true, "U": true, "s": true, "f": true, "a": true, "G": true},
	"/usr/sbin/groupadd":       {"g": true},
	"/usr/sbin/nginx":          {"s": true, "t": true, "c": true},
	"/usr/bin/nginx":           {"s": true, "t": true, "c": true},
	"/bin/systemctl":           {},
	"/usr/bin/systemctl":       {},
	"/usr/bin/git":               {"C": true, "depth": true, "branch": true, "hard": true, "f": true},
	"/usr/sbin/setquota":       {"u": true, "a": true},
	"/usr/bin/mysql":           {"e": true},
	"/usr/bin/mysqladmin":      {},
	"/usr/bin/apt-get":         {"o": true, "y": true},
	"/usr/bin/apt-cache":       {},
	"/usr/bin/mariadb-upgrade": {"force": true},
	"/usr/bin/mysql_upgrade":   {"force": true},
	"/usr/sbin/mariadbd":       {"version": true},
	"/usr/sbin/mysqld":         {"version": true},
	"/usr/bin/pecl":            {},
	"/usr/bin/pear":            {},
	"/usr/bin/gem":             {"no-document": true},
	"/usr/bin/cpan":            {"T": true},
	"/usr/bin/mariadb":         {"e": true},
	"/usr/bin/mariadb-dump":    {"single-transaction": true},
	"/usr/bin/mysqldump":       {"single-transaction": true},
	"/usr/bin/pg_dump":         {"d": true, "no-owner": true, "clean": true, "if-exists": true},
	"/usr/bin/psql":            {"d": true, "v": true, "c": true},
	"/usr/sbin/runuser":        {"u": true},
	"/usr/bin/pdnsutil":        {},
	"/usr/sbin/postqueue":      {"p": true, "j": true, "f": true},
	"/usr/sbin/postsuper":      {},
	"/usr/sbin/postmap":        {},
	"/usr/sbin/postconf":       {},
	"/usr/sbin/postfix":        {},
	"/usr/bin/doveadm":         {},
	"/usr/bin/pdns_control":    {},
	"/usr/bin/env":             {},
	"/usr/bin/npm":             {"prefix": true, "frozen-lockfile": true, "force": true, "yes": true},
	"/usr/local/bin/npm":       {"prefix": true, "frozen-lockfile": true, "force": true, "yes": true},
	"/usr/bin/pnpm":            {"dir": true, "frozen-lockfile": true, "store-dir": true, "force": true, "yes": true},
	"/usr/local/bin/pnpm":      {"dir": true, "frozen-lockfile": true, "store-dir": true, "force": true, "yes": true},
	"/usr/bin/yarn":            {"frozen-lockfile": true},
	"/usr/local/bin/yarn":      {"frozen-lockfile": true},
	"/usr/local/bin/node":      {},
	"/sbin/shutdown":           {"r": true},
	"/usr/sbin/shutdown":       {"r": true},
	"/usr/sbin/sshd":           {"t": true},
	"/bin/kill":                {"HUP": true},
}

func validateFixedCommand(bin string, env []string, args []string) error {
	if !allowedBins[bin] {
		return fmt.Errorf("executable not allow-listed")
	}
	if err := validateEnvPairs(env); err != nil {
		return err
	}
	switch bin {
	case "/usr/sbin/runuser":
		return validateRunuserArgs(args)
	case "/usr/bin/env":
		return validateEnvArgs(args)
	}
	flags := allowedFlags[bin]
	for _, arg := range args {
		if err := validateCommandArg(bin, arg, flags); err != nil {
			return err
		}
	}
	return nil
}

func validateEnvPairs(env []string) error {
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !allowedEnvKeys[key] {
			return fmt.Errorf("hostile environment")
		}
		if containsCommandControl(value) || strings.ContainsAny(value, ";|&$`") {
			return fmt.Errorf("hostile environment")
		}
		if err := validateEnvValue(key, value); err != nil {
			return err
		}
		if err := validateGitConfigEnv(key, value); err != nil {
			return err
		}
	}
	return nil
}

// validateEnvValue applies key-specific constraints. PATH is allow-listed to
// fixed system directories so callers cannot inject PATH=/tmp into git/runuser.
func validateEnvValue(key, value string) error {
	switch key {
	case "PATH":
		for _, part := range strings.Split(value, ":") {
			if part == "" {
				continue
			}
			switch part {
			case "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin":
			default:
				return fmt.Errorf("hostile environment")
			}
		}
	case "CI":
		if value != "1" && value != "true" {
			return fmt.Errorf("hostile environment")
		}
	}
	return nil
}

// validateRunuserArgs checks runuser's own flags, then recursively validates the
// command after "--" so nested package-manager flags (e.g. pnpm --dir) are checked
// against the nested binary instead of runuser's tiny allow-list.
func validateRunuserArgs(args []string) error {
	flags := allowedFlags["/usr/sbin/runuser"]
	i := 0
	for ; i < len(args); i++ {
		if args[i] == "--" {
			i++
			break
		}
		if err := validateCommandArg("/usr/sbin/runuser", args[i], flags); err != nil {
			return err
		}
	}
	if i >= len(args) {
		return fmt.Errorf("runuser missing command")
	}
	nested := filepath.Clean(args[i])
	return validateFixedCommand(nested, nil, args[i+1:])
}

// validateEnvArgs accepts KEY=VALUE assignments then validates the nested binary.
func validateEnvArgs(args []string) error {
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("leading option")
		}
		if key, value, ok := strings.Cut(arg, "="); ok {
			if !allowedEnvKeys[key] {
				return fmt.Errorf("hostile environment")
			}
			if containsCommandControl(value) || strings.ContainsAny(value, ";|&$`") {
				return fmt.Errorf("hostile environment")
			}
			if err := validateEnvValue(key, value); err != nil {
				return err
			}
			if err := validateGitConfigEnv(key, value); err != nil {
				return err
			}
			continue
		}
		break
	}
	if i >= len(args) {
		return fmt.Errorf("env missing command")
	}
	nested := filepath.Clean(args[i])
	return validateFixedCommand(nested, nil, args[i+1:])
}

// validateGitConfigEnv restricts GIT_CONFIG_* to safe.directory overrides only so
// callers cannot inject core.sshCommand or similar via the config env protocol.
func validateGitConfigEnv(key, value string) error {
	switch key {
	case "GIT_CONFIG_COUNT":
		if value != "1" {
			return fmt.Errorf("hostile environment")
		}
	case "GIT_CONFIG_KEY_0":
		if value != "safe.directory" {
			return fmt.Errorf("hostile environment")
		}
	case "GIT_CONFIG_VALUE_0":
		if value != "*" && !strings.HasPrefix(value, "/") {
			return fmt.Errorf("hostile environment")
		}
	}
	return nil
}

func validateCommandArg(bin, arg string, flags map[string]bool) error {
	if containsCommandControl(arg) {
		return fmt.Errorf("illegal argument")
	}
	if strings.ContainsAny(arg, ";|&$`") {
		return fmt.Errorf("illegal argument")
	}
	if arg == "--" {
		return nil
	}
	if strings.HasPrefix(arg, "-") {
		if flags == nil {
			return fmt.Errorf("leading option")
		}
		key := flagName(arg)
		if !flags[key] {
			return fmt.Errorf("leading option")
		}
	}
	_ = bin
	return nil
}

func flagName(arg string) string {
	trimmed := strings.TrimLeft(arg, "-")
	if i := strings.IndexByte(trimmed, '='); i >= 0 {
		trimmed = trimmed[:i]
	}
	return trimmed
}

func containsCommandControl(value string) bool {
	for _, r := range value {
		if r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func validateWorkingDir(dir string) error {
	if dir == "" {
		return nil
	}
	if containsCommandControl(dir) || strings.Contains(dir, "..") || strings.ContainsAny(dir, ";|&$`") {
		return fmt.Errorf("illegal working directory")
	}
	return nil
}
