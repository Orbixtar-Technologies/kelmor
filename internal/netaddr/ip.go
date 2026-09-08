package netaddr

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PublicIPv4 is the address published in DNS A records and bound by
// PowerDNS. PANEL_PUBLIC_IPV4 wins; otherwise the UDP source address
// toward 1.1.1.1 is used; then /var/lib/panel/public.env. Loopback is
// the last-resort lab default.
func PublicIPv4() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_PUBLIC_IPV4")); v != "" {
		if ip := net.ParseIP(v); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	c, err := net.DialTimeout("udp", "1.1.1.1:53", 400*time.Millisecond)
	if err == nil {
		defer c.Close()
		if addr, ok := c.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil && !addr.IP.IsLoopback() {
			return addr.IP.To4().String()
		}
	}
	if v := readPublicEnvFile(); v != "" {
		return v
	}
	return "127.0.0.1"
}

func publicEnvPath() string {
	if p := strings.TrimSpace(os.Getenv("PANEL_PUBLIC_ENV")); p != "" {
		return p
	}
	if d := strings.TrimSpace(os.Getenv("PANEL_STATE_DIR")); d != "" {
		return filepath.Join(d, "public.env")
	}
	return "/var/lib/panel/public.env"
}

func readPublicEnvFile() string {
	f, err := os.Open(publicEnvPath())
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != "PANEL_PUBLIC_IPV4" {
			continue
		}
		if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	return ""
}

// DNSListenIPv4 is 127.0.0.1 plus PublicIPv4 when that address is
// non-loopback, so the resolver stays reachable locally and on the NIC.
func DNSListenIPv4() []string {
	pub := PublicIPv4()
	if pub == "127.0.0.1" {
		return []string{"127.0.0.1"}
	}
	return []string{"127.0.0.1", pub}
}
