package operations

import (
	"fmt"
	"strings"
	"unicode"
)

const maxCommandOutput = 1 << 20

var allowedEnvKeys = map[string]bool{
	"DEBIAN_FRONTEND": true,
}

var allowedFlags = map[string]map[string]bool{
	"/usr/sbin/nft":         {"f": true},
	"/usr/bin/setfacl":      {"m": true},
	"/usr/bin/pgrep":        {"x": true},
	"/usr/sbin/useradd":     {"u": true, "g": true, "d": true, "s": true, "M": true, "r": true, "m": true},
	"/usr/sbin/userdel":     {"f": true, "r": true},
	"/usr/sbin/usermod":     {"aG": true, "L": true, "U": true, "s": true, "f": true, "a": true, "G": true},
	"/usr/sbin/groupadd":    {"g": true},
	"/usr/sbin/nginx":       {"s": true, "t": true, "c": true},
	"/usr/bin/nginx":        {"s": true, "t": true, "c": true},
	"/bin/systemctl":        {},
	"/usr/bin/systemctl":    {},
	"/usr/sbin/setquota":    {"u": true, "a": true},
	"/usr/bin/mysql":        {"e": true},
	"/usr/bin/mysqladmin":   {},
	"/usr/bin/apt-get":      {"o": true, "y": true},
	"/usr/bin/mariadb":      {"e": true},
	"/usr/bin/mariadb-dump": {"single-transaction": true},
	"/usr/bin/mysqldump":    {"single-transaction": true},
	"/usr/bin/pg_dump":      {"d": true, "no-owner": true, "clean": true, "if-exists": true},
	"/usr/bin/psql":         {"d": true, "v": true, "c": true},
	"/usr/sbin/runuser":     {"u": true},
	"/usr/bin/pdnsutil":     {},
	"/usr/sbin/postqueue":   {},
	"/usr/sbin/postsuper":   {},
	"/usr/sbin/postmap":     {},
	"/usr/sbin/postconf":    {},
	"/usr/sbin/postfix":     {},
	"/usr/bin/doveadm":      {},
	"/usr/bin/pdns_control": {},
	"/sbin/shutdown":        {"r": true},
	"/usr/sbin/shutdown":    {"r": true},
	"/usr/sbin/sshd":        {"t": true},
	"/bin/kill":             {"HUP": true},
}

func validateFixedCommand(bin string, env []string, args []string) error {
	if !allowedBins[bin] {
		return fmt.Errorf("executable not allow-listed")
	}
	flags := allowedFlags[bin]
	for _, arg := range args {
		if err := validateCommandArg(bin, arg, flags); err != nil {
			return err
		}
	}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !allowedEnvKeys[key] {
			return fmt.Errorf("hostile environment")
		}
		if containsCommandControl(value) || strings.ContainsAny(value, ";|&$`") {
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
