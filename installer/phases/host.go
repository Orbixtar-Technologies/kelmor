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
	_ = os.MkdirAll("/run/panel", 0o775)
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
	return nil
}

func verifyHealth(c Config) error {
	if c.Dev {
		return nil
	}
	for _, addr := range []string{"127.0.0.1:80", "127.0.0.1:25"} {
		con, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err != nil {
			return fmt.Errorf("health %s: %w", addr, err)
		}
		_ = con.Close()
	}
	return nil
}
