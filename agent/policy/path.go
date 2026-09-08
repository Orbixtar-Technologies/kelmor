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
	"/etc/ssh/sshd_config.d/",
	"/var/tmp/panel-imports/",
	"/var/log/nginx/",
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
			if strings.HasPrefix(clean, "/etc/php/") && strings.Contains(clean, "/fpm/pool.d/") {
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
	if !ok {
		return "", fmt.Errorf("path outside approved prefixes")
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path escape")
	}
	return clean, nil
}

func AccountRoot(username string) string {
	return filepath.Join("/home", username)
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
