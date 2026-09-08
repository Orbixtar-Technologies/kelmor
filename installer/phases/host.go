package phases

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

func applyHostRuntime(c Config) error {
	if c.Dev {
		return nil
	}
	_ = os.MkdirAll("/run/panel", 0o751)
	_ = os.Chmod("/run/panel", 0o751)
	_ = os.MkdirAll("/var/lib/panel/mail", 0o755)
	_ = os.Chmod("/var/lib/panel", 0o755)
	starts := [][]string{
		{"/usr/sbin/php-fpm8.3"},
		{"/usr/sbin/nginx"},
		{"/usr/sbin/postfix", "start"},
		{"/usr/sbin/dovecot"},
		{"/usr/sbin/pdns_server", "--daemon"},
		{"/usr/sbin/clamd"},
		{"/usr/bin/freshclam", "--daemon"},
	}
	for _, args := range starts {
		if _, err := os.Stat(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "DEBIAN_FRONTEND=noninteractive"}
		_ = cmd.Start()
	}
	startControlPlane()
	startAccountApps()
	return nil
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

func startControlPlane() {
	if _, err := os.Stat("/usr/local/panel/bin/panel-agent"); err != nil {
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
		cmd := exec.Command("/usr/bin/sudo", "-u", "panel", "-g", "panel", "env",
			"PANEL_STATE_DIR=/var/lib/panel",
			"PANEL_AGENT_SOCK=/run/panel/agent.sock",
			"PANEL_API_ADDR=127.0.0.1:18080",
			"PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql",
			"/usr/local/panel/bin/"+name)
		_ = cmd.Start()
	}
}

func verifyHealth(c Config) error {
	if c.Dev {
		return nil
	}
	addrs := []string{"127.0.0.1:80", "127.0.0.1:25"}
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
