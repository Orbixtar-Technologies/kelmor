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

// AddressIsLocal reports whether ipv4 is assigned to a local interface.
func AddressIsLocal(ipv4 string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipv4))
	if ip == nil || ip.To4() == nil {
		return false
	}
	want := ip.To4().String()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP == nil || n.IP.To4() == nil {
			continue
		}
		if n.IP.To4().String() == want {
			return true
		}
	}
	return false
}

// DNSListenIPv4 is the PowerDNS local-address list. Always include
// 127.0.0.1. Add PublicIPv4 when it is on a NIC. A floating / 1:1 NAT
// address is not bindable and 0.0.0.0:53 collides with systemd-resolved,
// so bind the UDP egress address instead.
func DNSListenIPv4() []string {
	bind := bindableNonLoopback()
	if bind == "" || bind == "127.0.0.1" {
		return []string{"127.0.0.1"}
	}
	return []string{"127.0.0.1", bind}
}

func bindableNonLoopback() string {
	pub := PublicIPv4()
	if pub != "127.0.0.1" && AddressIsLocal(pub) {
		return pub
	}
	return egressIPv4()
}

func egressIPv4() string {
	c, err := net.DialTimeout("udp", "1.1.1.1:53", 400*time.Millisecond)
	if err != nil {
		return ""
	}
	defer c.Close()
	addr, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.To4() == nil || addr.IP.IsLoopback() {
		return ""
	}
	return addr.IP.To4().String()
}
