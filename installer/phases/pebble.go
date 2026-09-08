package phases

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func startLocalACME(c Config) error {
	if c.Dev {
		return nil
	}
	dir := root(c, "var/lib/panel/pebble")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if _, err := os.Stat(cert); err != nil {
		out, err := exec.Command("/usr/bin/openssl", "req", "-x509", "-newkey", "ec",
			"-pkeyopt", "ec_paramgen_curve:prime256v1", "-nodes", "-days", "3650",
			"-subj", "/CN=pebble", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1",
			"-keyout", key, "-out", cert).CombinedOutput()
		if err != nil {
			return fmt.Errorf("pebble tls: %s", strings.TrimSpace(string(out)))
		}
	}
	cfg := `{
  "pebble": {
    "listenAddress": "127.0.0.1:14000",
    "managementListenAddress": "127.0.0.1:15000",
    "certificate": "` + cert + `",
    "privateKey": "` + key + `",
    "httpPort": 80,
    "tlsPort": 443
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o640); err != nil {
		return err
	}
	ensureLoopbackHost(c.Hostname)
	ensureLoopbackHost("livehost.test")
	bin := pebbleBinary()
	if bin == "" {
		return nil
	}
	if err := os.WriteFile(root(c, "var/lib/panel/acme.directory"), []byte("https://127.0.0.1:14000/dir\n"), 0o644); err != nil {
		return err
	}
	if exec.Command("/usr/bin/pgrep", "-x", "pebble").Run() == nil {
		return nil
	}
	cmd := exec.Command(bin, "-config", filepath.Join(dir, "config.json"), "-dnsserver", "127.0.0.1:53")
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "PEBBLE_VA_NOSLEEP=1", "PEBBLE_WFE_NONCEREJECT=0"}
	logf, err := os.OpenFile("/tmp/pebble.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		con, err := net.DialTimeout("tcp", "127.0.0.1:14000", 200*time.Millisecond)
		if err == nil {
			_ = con.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func pebbleBinary() string {
	for _, p := range []string{"/usr/local/panel/bin/pebble", "/usr/local/bin/pebble"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func ensureLoopbackHost(name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == "localhost" {
		return
	}
	b, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return
	}
	if strings.Contains(string(b), name) {
		return
	}
	f, err := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "127.0.0.1 %s\n", name)
	_ = f.Close()
}
