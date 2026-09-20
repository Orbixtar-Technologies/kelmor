package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

var allowedRoots = []string{
	"/home/",
	"/etc/nginx/panel-sites/",
	"/etc/php/",
	"/etc/systemd/system/",
	"/var/lib/panel/",
	"/run/panel/",
	"/var/vmail/",
	"/etc/panel/",
	"/etc/skel/",
	"/etc/ssh/sshd_config.d/",
	"/var/tmp/panel-imports/",
	"/var/log/nginx/",
	"/var/www/panel-acme/",
}

func ValidateManagedPath(p string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("empty or NUL path")
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be absolute")
	}
	clean := filepath.Clean(p)
	if strings.Contains(clean, "\x00") {
		return "", fmt.Errorf("NUL path")
	}
	ok := false
	for _, root := range allowedRoots {
		if root == "/etc/php/" {
			if strings.HasPrefix(clean, "/etc/php/") && (strings.Contains(clean, "/fpm/pool.d/") || strings.HasSuffix(clean, "/fpm/conf.d/99-panel.ini")) {
				ok = true
				break
			}
			continue
		}
		if strings.HasPrefix(clean, strings.TrimSuffix(root, "/")) &&
			(clean == strings.TrimSuffix(root, "/") || strings.HasPrefix(clean, root) || strings.HasPrefix(clean+"/", root)) {
			ok = true
			break
		}
	}
	if strings.HasPrefix(clean, "/etc/cron.d/panel-") {
		ok = true
	}
	if strings.HasPrefix(clean, "/etc/nginx/conf.d/panel-") {
		ok = true
	}
	if strings.HasPrefix(clean, "/etc/rspamd/local.d/") {
		ok = true
	}
	if strings.HasPrefix(clean, "/etc/phpmyadmin/conf.d/panel-") {
		ok = true
	}
	if clean == "/etc/roundcube/config.panel.inc.php" || clean == "/etc/roundcube/config.inc.php" {
		ok = true
	}
	if postgresPathOK(clean) {
		ok = true
	}
	if clean == "/etc/vsftpd.conf" {
		ok = true
	}
	if nameserverPathOK(clean) || mailserverPathOK(clean) {
		ok = true
	}
	if clean == "/etc/skel" || strings.HasPrefix(clean, "/etc/skel/") {
		ok = true
	}
	if clean == "/var/log/mail.log" || strings.HasPrefix(clean, "/var/log/mail.log.") {
		ok = true
	}
	if !ok {
		return "", fmt.Errorf("path outside approved prefixes")
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path escape")
	}
	return clean, nil
}

func postgresPathOK(clean string) bool {
	const prefix = "/etc/postgresql/"
	if !strings.HasPrefix(clean, prefix) {
		return false
	}
	rest := strings.TrimPrefix(clean, prefix)
	version, rem, ok := strings.Cut(rest, "/")
	if !ok || !postgresVersionOK(version) {
		return false
	}
	return rem == "main/pg_hba.conf" || rem == "main/conf.d/kelmor.conf"
}

func nameserverPathOK(clean string) bool {
	return clean == "/etc/powerdns/pdns.conf" || clean == "/etc/powerdns/pdns.d/99-panel-listen.conf"
}

func mailserverPathOK(clean string) bool {
	return clean == "/etc/dovecot/conf.d/99-panel-ports.conf" || clean == "/etc/postfix/master.cf"
}

func splitExactManaged(clean string) (prefix, rel string, ok bool) {
	switch clean {
	case "/etc/powerdns/pdns.conf":
		return "/etc/powerdns", "pdns.conf", true
	case "/etc/powerdns/pdns.d/99-panel-listen.conf":
		return "/etc/powerdns", "pdns.d/99-panel-listen.conf", true
	case "/etc/dovecot/conf.d/99-panel-ports.conf":
		return "/etc/dovecot", "conf.d/99-panel-ports.conf", true
	case "/etc/postfix/master.cf":
		return "/etc/postfix", "master.cf", true
	default:
		return "", "", false
	}
}

func postgresVersionOK(version string) bool {
	if version == "" {
		return false
	}
	for _, r := range version {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func AccountRoot(username string) string {
	return filepath.Join("/home", username)
}

const skeletonRoot = "/etc/skel"

func WithinSkeleton(requested string) (string, error) {
	if requested == "" || requested == "/" {
		return skeletonRoot, nil
	}
	abs := requested
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(skeletonRoot, strings.TrimPrefix(requested, "/"))
	}
	clean, err := ValidateManagedPath(abs)
	if err != nil {
		return "", err
	}
	if clean != skeletonRoot && !strings.HasPrefix(clean, skeletonRoot+"/") {
		return "", fmt.Errorf("path outside skeleton directory")
	}
	return clean, nil
}

func WithinAccount(username, requested string) (string, error) {
	clean, err := ValidateManagedPath(requested)
	if err != nil {
		return "", err
	}
	root := AccountRoot(username)
	if clean != root && !strings.HasPrefix(clean, root+"/") {
		return "", fmt.Errorf("path outside account boundary")
	}
	return clean, nil
}

func AccountRelative(clean string) (username, relative string, ok bool) {
	const prefix = "/home/"
	if !strings.HasPrefix(clean, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(clean, prefix)
	username, remainder, found := strings.Cut(rest, "/")
	if username == "" {
		return "", "", false
	}
	if !found {
		return username, "", true
	}
	return username, remainder, true
}

func SplitManaged(clean string) (prefix, relative string, err error) {
	candidates := append([]string{}, allowedRoots...)
	candidates = append(candidates,
		"/etc/cron.d/",
		"/etc/nginx/conf.d/",
		"/etc/rspamd/local.d/",
		"/etc/phpmyadmin/conf.d/",
		"/etc/roundcube/",
		"/etc/postgresql/",
	)
	best := ""
	for _, root := range candidates {
		base := strings.TrimSuffix(root, "/")
		if clean == base || strings.HasPrefix(clean, root) || strings.HasPrefix(clean+"/", root) {
			if len(base) > len(best) {
				best = base
			}
		}
	}
	if clean == "/etc/vsftpd.conf" {
		return "/etc", "vsftpd.conf", nil
	}
	if prefix, rel, ok := splitExactManaged(clean); ok {
		return prefix, rel, nil
	}
	if best == "" {
		return "", "", fmt.Errorf("path outside approved prefixes")
	}
	if clean == best {
		return "", "", fmt.Errorf("managed path requires a relative name")
	}
	rel := strings.TrimPrefix(clean, best+"/")
	if rel == "" || rel == clean || strings.Contains(rel, "..") {
		return "", "", fmt.Errorf("invalid managed relative path")
	}
	return best, rel, nil
}
