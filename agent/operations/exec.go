package operations

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var allowedBins = map[string]bool{
	"/usr/sbin/nft":         true,
	"/usr/bin/setfacl":      true,
	"/usr/bin/pgrep":        true,
	"/usr/sbin/useradd":     true,
	"/usr/sbin/userdel":     true,
	"/usr/sbin/usermod":     true,
	"/usr/sbin/groupadd":    true,
	"/usr/sbin/nologin":     true,
	"/usr/sbin/nginx":       true,
	"/usr/sbin/php-fpm8.3":  true,
	"/usr/sbin/php-fpm8.4":  true,
	"/usr/sbin/php-fpm8.5":  true,
	"/usr/bin/nginx":        true,
	"/bin/systemctl":        true,
	"/usr/bin/systemctl":    true,
	"/usr/sbin/setquota":    true,
	"/usr/bin/mysql":        true,
	"/usr/bin/mariadb":      true,
	"/usr/bin/mariadb-dump": true,
	"/usr/bin/mysqldump":    true,
	"/usr/bin/pg_dump":      true,
	"/usr/bin/psql":         true,
	"/usr/sbin/runuser":     true,
	"/usr/bin/pdnsutil":     true,
	"/usr/sbin/postqueue":   true,
	"/usr/sbin/postsuper":   true,
	"/usr/sbin/postmap":     true,
	"/usr/sbin/postfix":     true,
	"/usr/bin/doveadm":      true,
	"/usr/bin/pdns_control": true,
	"/usr/bin/node":         true,
	"/usr/bin/nodejs":       true,
	"/exec-daemon/node":     true,
	"/usr/bin/python3":      true,
	"/usr/sbin/vsftpd":      true,
	"/sbin/shutdown":        true,
	"/usr/sbin/shutdown":    true,
}

var allowedServices = map[string]bool{
	"nginx": true, "php8.3-fpm": true, "php8.4-fpm": true, "php8.5-fpm": true,
	"postfix": true, "dovecot": true, "pdns": true, "mariadb": true, "mysql": true,
	"postgresql": true, "redis-server": true, "rspamd": true, "clamav-daemon": true,
	"panel-api": true, "panel-worker": true, "panel-agent": true,
	"panel-smtp-policy": true,
	"vsftpd":            true,
}

func runFixed(bin string, args ...string) ([]byte, error) {
	return runFixedIO(bin, nil, args...)
}

func runFixedIO(bin string, stdin []byte, args ...string) ([]byte, error) {
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
	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}
	return cmd.CombinedOutput()
}

func runFixedStdout(bin string, args ...string) ([]byte, error) {
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
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()+" "+err.Error()))
	}
	return []byte(stdout.String()), nil
}

func startDetached(bin, dir string, args ...string) (int, error) {
	bin = filepath.Clean(bin)
	if !allowedBins[bin] {
		return 0, fmt.Errorf("executable not allow-listed")
	}
	for _, a := range args {
		if strings.ContainsAny(a, ";|&$`\n") {
			return 0, fmt.Errorf("illegal argument")
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "LC_ALL=C", "HOME=" + dir}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	if cmd.Process != nil {
		return cmd.Process.Pid, nil
	}
	return 0, nil
}

func (h *Host) live() bool {
	return h.Root == "" && os.Geteuid() == 0
}

func probeService(name string) map[string]any {
	if err := validateService(name); err != nil {
		return map[string]any{"name": name, "health": "unknown", "running": false, "error": err.Error()}
	}
	running := false
	switch name {
	case "nginx":
		running = listening("tcp", "127.0.0.1:80") || pidAlive("/run/nginx.pid")
	case "php8.3-fpm", "php-fpm":
		running = pidAlive("/run/php/php8.3-fpm.pid")
	case "postfix":
		running = listening("tcp", "127.0.0.1:25")
	case "dovecot":
		running = listening("tcp", "127.0.0.1:993")
	case "pdns":
		running = listening("tcp", "127.0.0.1:53") || listening("udp", "127.0.0.1:53")
	case "mariadb", "mysql":
		running = listening("tcp", "127.0.0.1:3306") || fileExists("/run/mysqld/mysqld.sock")
	case "postgresql":
		running = fileExists("/var/run/postgresql/.s.PGSQL.5432") || dirHas("/var/run/postgresql")
	case "rspamd":
		running = listening("tcp", "127.0.0.1:11332")
	case "clamav-daemon":
		running = fileExists("/run/clamav/clamd.ctl") || listening("tcp", "127.0.0.1:3310")
	case "panel-api":
		running = listening("tcp", "127.0.0.1:18080")
	case "panel-agent":
		running = fileExists("/run/panel/agent.sock")
	case "panel-worker":
		running = pidOf("panel-worker")
	case "vsftpd":
		running = listening("tcp", "127.0.0.1:21") || pidOf("vsftpd")
	default:
		running = pidOf(name)
	}
	health := "stopped"
	if running {
		health = "healthy"
	}
	return map[string]any{"name": name, "health": health, "running": running}
}

func pidAlive(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(b))
	if pid == "" {
		return false
	}
	_, err = os.Stat("/proc/" + pid)
	return err == nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirHas(p string) bool {
	ents, err := os.ReadDir(p)
	return err == nil && len(ents) > 0
}

func listening(network, addr string) bool {
	c, err := net.DialTimeout(network, addr, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func pidOf(name string) bool {
	out, err := runFixed("/usr/bin/pgrep", "-x", name)
	return err == nil && len(bytesTrimSpace(out)) > 0
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func validateService(name string) error {
	if !allowedServices[name] {
		return fmt.Errorf("unexpected service name")
	}
	return nil
}

func reloadNamedService(name string) error {
	if name == "nginx" {
		if out, err := runFixed("/usr/sbin/nginx", "-s", "reload"); err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if out, err := runFixed("/bin/systemctl", "reload-or-restart", name); err == nil {
		return nil
	} else if strings.Contains(string(out), "not been booted with systemd") || strings.Contains(string(out), "Host is down") {
		return nil
	} else {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
}
