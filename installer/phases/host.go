package phases

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type hostService struct {
	Comm   string
	Listen string
	Args   []string
}

func hostServices() []hostService {
	return []hostService{
		{Comm: "mysqld", Listen: "127.0.0.1:3306", Args: []string{"/usr/sbin/mysqld", "--user=mysql"}},
		{Comm: "postgres", Listen: "127.0.0.1:5432", Args: []string{"/usr/sbin/runuser", "-u", "postgres", "--", "/usr/lib/postgresql/16/bin/postgres", "-D", "/var/lib/postgresql/16/main"}},
		{Comm: "php-fpm8.3", Args: []string{"/usr/sbin/php-fpm8.3"}},
		{Comm: "nginx", Listen: "127.0.0.1:80", Args: []string{"/usr/sbin/nginx"}},
		{Comm: "master", Listen: "127.0.0.1:25", Args: []string{"/usr/sbin/postfix", "start"}},
		{Comm: "dovecot", Listen: "127.0.0.1:993", Args: []string{"/usr/sbin/dovecot"}},
		{Comm: "pdns_server", Listen: "127.0.0.1:53", Args: []string{"/usr/sbin/pdns_server", "--daemon=yes", "--guardian=no", "--config-dir=/etc/powerdns"}},
		{Comm: "vsftpd", Listen: "127.0.0.1:21", Args: []string{"/usr/sbin/vsftpd", "/etc/vsftpd.conf"}},
		{Comm: "clamd", Args: []string{"/usr/sbin/clamd"}},
		{Comm: "freshclam", Args: []string{"/usr/bin/freshclam", "--daemon"}},
		{Comm: "rspamd", Args: []string{"/usr/bin/rspamd", "-c", "/etc/rspamd/rspamd.conf"}},
		{Comm: "fail2ban-server", Args: []string{"/usr/bin/fail2ban-server", "-xf", "start"}},
	}
}

func startHostService(svc hostService) {
	if len(svc.Args) == 0 {
		return
	}
	if _, err := os.Stat(svc.Args[0]); err != nil {
		return
	}
	if svc.Listen != "" {
		if con, err := net.DialTimeout("tcp", svc.Listen, 150*time.Millisecond); err == nil {
			_ = con.Close()
			return
		}
	}
	if svc.Comm != "" && exec.Command("/usr/bin/pgrep", "-x", svc.Comm).Run() == nil {
		return
	}
	cmd := exec.Command(svc.Args[0], svc.Args[1:]...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "DEBIAN_FRONTEND=noninteractive"}
	_ = cmd.Start()
	if svc.Listen != "" {
		_ = waitListen(svc.Listen, 3*time.Second)
	}
}

func waitListen(addr string, d time.Duration) error {
	deadline := time.Now().Add(d)
	var last error
	for time.Now().Before(deadline) {
		con, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = con.Close()
			return nil
		}
		last = err
		time.Sleep(100 * time.Millisecond)
	}
	if last == nil {
		return fmt.Errorf("timeout waiting for %s", addr)
	}
	return fmt.Errorf("timeout waiting for %s: %w", addr, last)
}

func applyHostRuntime(c Config) error {
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	_ = os.MkdirAll("/run/panel", 0o751)
	_ = os.Chmod("/run/panel", 0o751)
	_ = os.MkdirAll("/var/lib/panel/mail", 0o755)
	_ = os.Chmod("/var/lib/panel", 0o755)
	for _, svc := range hostServices() {
		startHostService(svc)
	}
	startControlPlane()
	startSMTPPolicy()
	startObjectStore()
	_ = ensureACME(c)
	enablePanelUnits()
	startAccountApps()
	if _, err := os.Stat("/etc/panel/nftables-panel.nft"); err == nil {
		_ = applyLiveNFT("/etc/panel/nftables-panel.nft")
	}
	applyExistingCgroups()
	return nil
}

func applyExistingCgroups() {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return
	}
	root := "/sys/fs/cgroup/panel.accounts"
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(root+"/cgroup.subtree_control", []byte("+cpu +memory +pids +io\n"), 0o644)
	homes, err := os.ReadDir("/home")
	if err != nil {
		return
	}
	for _, h := range homes {
		if !h.IsDir() {
			continue
		}
		name := h.Name()
		if len(name) < 2 {
			continue
		}
		dir := root + "/" + name
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(dir+"/memory.max", []byte("2147483648\n"), 0o644)
		_ = os.WriteFile(dir+"/cpu.max", []byte("200000 100000\n"), 0o644)
		_ = os.WriteFile(dir+"/pids.max", []byte("100\n"), 0o644)
	}
}

func startAccountApps() {
	homes, err := os.ReadDir("/home")
	if err != nil {
		return
	}
	for _, h := range homes {
		if !h.IsDir() {
			continue
		}
		user := h.Name()
		apps := "/home/" + user + "/apps"
		entries, err := os.ReadDir(apps)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := apps + "/" + e.Name()
			if _, err := os.Stat(dir + "/app.py"); err == nil {
				if exec.Command("/usr/bin/pgrep", "-f", dir+"/app.py").Run() != nil {
					cmd := exec.Command("/usr/sbin/runuser", "-u", user, "--", "/usr/bin/python3", "app.py")
					cmd.Dir = dir
					cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/home/" + user}
					_ = cmd.Start()
				}
			}
			if _, err := os.Stat(dir + "/server.js"); err == nil {
				if exec.Command("/usr/bin/pgrep", "-f", dir+"/server.js").Run() != nil {
					cmd := exec.Command("/usr/sbin/runuser", "-u", user, "--", "/usr/bin/node", "server.js")
					cmd.Dir = dir
					cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/home/" + user}
					_ = cmd.Start()
				}
			}
		}
	}
}

func pid1IsSystemd() bool {
	b, err := os.ReadFile("/proc/1/comm")
	return err == nil && strings.TrimSpace(string(b)) == "systemd"
}

func startPanelUnit(name string) {
	_ = exec.Command("/bin/systemctl", "start", name+".service").Run()
}

func enablePanelUnits() {
	if !pid1IsSystemd() {
		return
	}
	for _, name := range []string{
		"panel-agent", "panel-api", "panel-worker", "panel-smtp-policy", "panel-object-store",
		"nginx", "php8.3-fpm", "postgresql", "mariadb", "postfix", "dovecot", "pdns", "vsftpd",
	} {
		_ = exec.Command("/bin/systemctl", "enable", "--now", name+".service").Run()
	}
}

func startObjectStore() {
	if _, err := os.Stat("/usr/local/panel/bin/panel-object-store"); err != nil {
		return
	}
	_ = os.MkdirAll("/var/lib/panel/objects", 0o750)
	_ = exec.Command("/bin/chown", "panel:panel", "/var/lib/panel/objects").Run()
	if pid1IsSystemd() {
		startPanelUnit("panel-object-store")
		_ = waitListen("127.0.0.1:19090", 3*time.Second)
		return
	}
	if con, err := net.DialTimeout("tcp", "127.0.0.1:19090", 150*time.Millisecond); err == nil {
		_ = con.Close()
		return
	}
	if exec.Command("/usr/bin/pgrep", "-f", "/panel-object-store").Run() == nil {
		return
	}
	args := []string{"-u", "panel", "-g", "panel", "env"}
	args = append(args, loadEnvPairs("/var/lib/panel/secrets/backup-s3.env")...)
	args = append(args, "/usr/local/panel/bin/panel-object-store")
	cmd := exec.Command("/usr/bin/sudo", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
	_ = waitListen("127.0.0.1:19090", 3*time.Second)
}

func loadEnvPairs(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "PANEL_") && strings.Contains(line, "=") {
			out = append(out, line)
		}
	}
	return out
}

func startSMTPPolicy() {
	if _, err := os.Stat("/usr/local/panel/bin/panel-smtp-policy"); err != nil {
		return
	}
	if pid1IsSystemd() {
		startPanelUnit("panel-smtp-policy")
		return
	}
	if exec.Command("/usr/bin/pgrep", "-f", "/panel-smtp-policy").Run() == nil {
		return
	}
	_ = os.MkdirAll("/var/lib/panel/mail/send-counts", 0o775)
	cmd := exec.Command("/usr/bin/sudo", "-u", "panel", "-g", "panel", "env",
		"PANEL_SMTP_POLICY_ADDR=127.0.0.1:10031",
		"PANEL_SMTP_LIMITS=/var/lib/panel/mail/send-limits",
		"PANEL_SMTP_COUNTS=/var/lib/panel/mail/send-counts",
		"/usr/local/panel/bin/panel-smtp-policy")
	_ = cmd.Start()
}

func startControlPlane() {
	if _, err := os.Stat("/usr/local/panel/bin/panel-agent"); err != nil {
		return
	}
	if pid1IsSystemd() {
		for _, name := range []string{"panel-agent", "panel-api", "panel-worker"} {
			startPanelUnit(name)
		}
		return
	}
	if exec.Command("/usr/bin/pgrep", "-x", "panel-agent").Run() != nil {
		cmd := exec.Command("/usr/local/panel/bin/panel-agent")
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "PANEL_AGENT_SOCK=/run/panel/agent.sock"}
		_ = cmd.Start()
		time.Sleep(300 * time.Millisecond)
	}
	for _, name := range []string{"panel-api", "panel-worker"} {
		if exec.Command("/usr/bin/pgrep", "-x", name).Run() == nil {
			continue
		}
		args := []string{"-u", "panel", "-g", "panel", "env",
			"PANEL_STATE_DIR=/var/lib/panel",
			"PANEL_AGENT_SOCK=/run/panel/agent.sock",
			"PANEL_API_ADDR=127.0.0.1:18080",
			"PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql",
			"PANEL_PDNS_URL=http://127.0.0.1:8081",
			"PANEL_PDNS_API_KEY=panel-loopback",
		}
		args = append(args, loadEnvPairs("/var/lib/panel/public.env")...)
		args = append(args, loadEnvPairs("/var/lib/panel/acme.env")...)
		args = append(args, loadEnvPairs("/var/lib/panel/secrets/backup-sftp.env")...)
		args = append(args, loadEnvPairs("/var/lib/panel/secrets/backup-s3.env")...)
		args = append(args, "/usr/local/panel/bin/"+name)
		cmd := exec.Command("/usr/bin/sudo", args...)
		_ = cmd.Start()
	}
}

func verifyHostRuntime(c Config) error {
	if err := verifyHealth(c); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	for _, name := range []string{"panel-agent", "panel-api", "panel-worker"} {
		if exec.Command("/usr/bin/pgrep", "-x", name).Run() != nil {
			return fmt.Errorf("%s is not running", name)
		}
	}
	if _, err := os.Stat("/run/panel/agent.sock"); err != nil {
		return fmt.Errorf("agent socket missing")
	}
	if exec.Command("/usr/bin/pgrep", "-f", "/panel-smtp-policy").Run() != nil {
		return fmt.Errorf("panel-smtp-policy is not running")
	}
	if pid1IsSystemd() && exec.Command("/bin/systemctl", "is-active", "--quiet", "panel-smtp-policy.service").Run() != nil {
		return fmt.Errorf("panel-smtp-policy.service is not active")
	}
	if _, err := os.Stat("/usr/local/panel/bin/panel-object-store"); err == nil {
		if err := waitListen("127.0.0.1:19090", 2*time.Second); err != nil {
			return fmt.Errorf("panel-object-store is not listening: %w", err)
		}
		if pid1IsSystemd() && exec.Command("/bin/systemctl", "is-active", "--quiet", "panel-object-store.service").Run() != nil {
			return fmt.Errorf("panel-object-store.service is not active")
		}
	}
	return nil
}

func verifyHealth(c Config) error {
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	addrs := []string{"127.0.0.1:80", "127.0.0.1:25", "127.0.0.1:53"}
	if _, err := os.Stat("/usr/sbin/mysqld"); err == nil {
		addrs = append(addrs, "127.0.0.1:3306")
	}
	if _, err := os.Stat("/usr/lib/postgresql/16/bin/postgres"); err == nil {
		addrs = append(addrs, "127.0.0.1:5432")
	}
	if _, err := os.Stat("/usr/local/panel/bin/panel-api"); err == nil {
		addrs = append(addrs, "127.0.0.1:18080")
	}
	var last error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		last = nil
		for _, addr := range addrs {
			con, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
			if err != nil {
				last = fmt.Errorf("health %s: %w", addr, err)
				break
			}
			_ = con.Close()
		}
		if last == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return last
}
