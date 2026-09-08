package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedBins = map[string]bool{
	"/usr/sbin/useradd":    true,
	"/usr/sbin/userdel":    true,
	"/usr/sbin/usermod":    true,
	"/usr/sbin/groupadd":   true,
	"/usr/sbin/nologin":    true,
	"/usr/sbin/nginx":      true,
	"/usr/sbin/php-fpm8.3": true,
	"/usr/sbin/php-fpm8.4": true,
	"/usr/sbin/php-fpm8.5": true,
	"/usr/bin/nginx":       true,
	"/bin/systemctl":       true,
	"/usr/bin/systemctl":   true,
	"/usr/sbin/setquota":   true,
	"/usr/bin/mysql":       true,
	"/usr/bin/mariadb":     true,
	"/usr/bin/psql":        true,
	"/usr/bin/pdnsutil":    true,
	"/usr/sbin/postqueue":  true,
	"/usr/sbin/postsuper":  true,
	"/usr/sbin/postmap":    true,
}

var allowedServices = map[string]bool{
	"nginx": true, "php8.3-fpm": true, "php8.4-fpm": true, "php8.5-fpm": true,
	"postfix": true, "dovecot": true, "pdns": true, "mariadb": true, "mysql": true,
	"postgresql": true, "redis-server": true, "rspamd": true, "clamav-daemon": true,
	"panel-api": true, "panel-worker": true, "panel-agent": true,
}

func runFixed(bin string, args ...string) ([]byte, error) {
	bin = filepath.Clean(bin)
	if !allowedBins[bin] {
		return nil, fmt.Errorf("executable not allow-listed")
	}
	for _, a := range args {
		if strings.ContainsAny(a, ";|&$`\n") {
			return nil, fmt.Errorf("illegal argument")
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "LC_ALL=C"}
	return cmd.CombinedOutput()
}

func (h *Host) live() bool {
	return h.Root == "" && os.Geteuid() == 0
}

func validateService(name string) error {
	if !allowedServices[name] {
		return fmt.Errorf("unexpected service name")
	}
	return nil
}
