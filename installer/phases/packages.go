package phases

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedPackages = map[string]bool{
	"nginx": true, "php8.3-fpm": true, "php8.3-cli": true, "php8.3-mysql": true,
	"php8.3-pgsql": true, "php8.3-xml": true, "php8.3-mbstring": true, "php8.3-curl": true,
	"mariadb-server": true, "postgresql": true, "pdns-server": true, "pdns-backend-pgsql": true,
	"postfix": true, "dovecot-core": true, "dovecot-imapd": true, "dovecot-lmtpd": true,
	"redis-server": true, "rspamd": true, "clamav": true, "clamav-daemon": true,
	"fail2ban": true, "quota": true, "libnginx-mod-http-modsecurity": true,
	"nodejs": true, "python3": true, "openssh-server": true,
	"nftables": true, "acl": true,
	"vsftpd": true, "libpam-pwdfile": true,
	"curl": true, "ca-certificates": true,
}

func InstallPackages(names []string) error {
	for _, n := range names {
		if !allowedPackages[n] {
			return fmt.Errorf("package %q is not allow-listed", n)
		}
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("root required to install packages")
	}
	env := []string{"DEBIAN_FRONTEND=noninteractive", "PATH=/usr/sbin:/usr/bin:/bin"}
	update := exec.Command("/usr/bin/apt-get", "update")
	update.Env = env
	if out, err := update.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get update: %s", string(out))
	}
	args := append([]string{"-y", "install"}, names...)
	cmd := exec.Command("/usr/bin/apt-get", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get: %s", string(out))
	}
	return nil
}

func applySystemPackages(c Config) error {
	if c.Dev {
		return nil
	}
	return InstallPackages([]string{
		"nginx", "php8.3-fpm", "php8.3-cli", "php8.3-mysql", "php8.3-xml",
		"mariadb-server", "postgresql", "pdns-server", "pdns-backend-pgsql",
		"postfix", "dovecot-core", "dovecot-imapd", "dovecot-lmtpd",
		"redis-server", "rspamd", "clamav-daemon", "fail2ban", "quota",
		"libnginx-mod-http-modsecurity", "nodejs", "python3", "openssh-server",
		"nftables", "acl", "vsftpd", "libpam-pwdfile",
	})
}

func applyWebStack(c Config) error {
	if err := os.MkdirAll(root(c, "etc/nginx/panel-sites"), 0o755); err != nil {
		return err
	}
	include := "include /etc/nginx/panel-sites/*.conf;\n"
	modsec := "modsecurity on;\nmodsecurity_rules_file /etc/nginx/modsec/panel.conf;\n"
	conn := "map $host $panel_account {\n    default \"\";\n}\nlimit_conn_zone $panel_account zone=panel_acct:10m;\n"
	if err := os.MkdirAll(root(c, "etc/nginx/conf.d"), 0o755); err != nil {
		return err
	}
	if c.Dev {
		if err := os.WriteFile(root(c, "etc/nginx/panel-sites.conf"), []byte(include), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(root(c, "etc/nginx/panel-modsec.conf"), []byte(modsec), 0o644); err != nil {
			return err
		}
		return os.WriteFile(root(c, "etc/nginx/conf.d/panel-conn-limit.conf"), []byte(conn), 0o644)
	}
	if err := os.WriteFile(root(c, "etc/nginx/conf.d/panel-sites.conf"), []byte(include), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(root(c, "etc/nginx/conf.d/panel-modsec.conf"), []byte(modsec), 0o644); err != nil {
		return err
	}
	return os.WriteFile(root(c, "etc/nginx/conf.d/panel-conn-limit.conf"), []byte(conn), 0o644)
}

func applyDatabaseStack(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/db"), 0o750); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	for _, args := range [][]string{
		{"/usr/sbin/mysqld", "--user=mysql"},
		{"/usr/lib/postgresql/16/bin/postgres", "-D", "/var/lib/postgresql/16/main"},
	} {
		if _, err := os.Stat(args[0]); err != nil {
			continue
		}
		name := filepath.Base(args[0])
		if exec.Command("/usr/bin/pgrep", "-x", name).Run() == nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin"}
		_ = cmd.Start()
	}
	return nil
}

func verifySystemPackages(c Config) error {
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	for _, n := range []string{"nginx", "postfix", "dovecot-core", "postgresql", "vsftpd"} {
		cmd := exec.Command("/usr/bin/dpkg-query", "-W", "-f=${Status}", n)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "ok installed") {
			return fmt.Errorf("required package %s is not installed", n)
		}
	}
	return nil
}

func verifyDatabaseStack(c Config) error {
	if c.Dev || installPrefix(c) != "" {
		_, err := os.Stat(root(c, "var/lib/panel/db"))
		return err
	}
	if _, err := os.Stat("/var/run/mysqld/mysqld.sock"); err == nil {
		return nil
	}
	if _, err := os.Stat("/var/run/postgresql"); err == nil {
		return nil
	}
	return fmt.Errorf("neither MariaDB nor PostgreSQL is listening")
}

func verifyWeb(c Config) error {
	_, err := os.Stat(root(c, "etc/nginx/panel-sites"))
	return err
}
