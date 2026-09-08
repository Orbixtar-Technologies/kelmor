package phases

import (
	"fmt"
	"os"
	"os/exec"
)

var allowedPackages = map[string]bool{
	"nginx": true, "php8.3-fpm": true, "php8.3-cli": true, "php8.3-mysql": true,
	"php8.3-pgsql": true, "php8.3-xml": true, "php8.3-mbstring": true, "php8.3-curl": true,
	"mariadb-server": true, "postgresql": true, "pdns-server": true, "pdns-backend-pgsql": true,
	"postfix": true, "dovecot-core": true, "dovecot-imapd": true, "dovecot-lmtpd": true,
	"redis-server": true, "rspamd": true, "clamav": true, "clamav-daemon": true,
	"fail2ban": true, "quota": true, "libnginx-mod-http-modsecurity": true,
	"nodejs": true, "python3": true, "openssh-server": true,
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
	args := append([]string{"-y", "install"}, names...)
	cmd := exec.Command("/usr/bin/apt-get", args...)
	cmd.Env = []string{"DEBIAN_FRONTEND=noninteractive", "PATH=/usr/sbin:/usr/bin:/bin"}
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
	})
}

func applyWebStack(c Config) error {
	if err := os.MkdirAll(root(c, "etc/nginx/panel-sites"), 0o755); err != nil {
		return err
	}
	include := "include /etc/nginx/panel-sites/*.conf;\n"
	if c.Dev {
		return os.WriteFile(root(c, "etc/nginx/panel-sites.conf"), []byte(include), 0o644)
	}
	return os.WriteFile("/etc/nginx/conf.d/panel-sites.conf", []byte(include), 0o644)
}

func applyDatabaseStack(c Config) error {
	if c.Dev {
		return os.MkdirAll(root(c, "var/lib/panel/db"), 0o750)
	}
	return nil
}

func verifyWeb(c Config) error {
	_, err := os.Stat(root(c, "etc/nginx/panel-sites"))
	return err
}
