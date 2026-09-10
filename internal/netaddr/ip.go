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

// DNSListenIPv4 is 127.0.0.1 plus an address PowerDNS can bind on the
// host. PublicIPv4 is used when it is assigned locally; otherwise the
// primary non-loopback NIC address is used for NAT/cloud deployments.
func DNSListenIPv4() []string {
	addrs := []string{"127.0.0.1"}
	pub := PublicIPv4()
	if pub == "127.0.0.1" {
		return addrs
	}
	if isAssignedIPv4(pub) {
		return append(addrs, pub)
	}
	if nic := primaryInterfaceIPv4(); nic != "" {
		return append(addrs, nic)
	}
	return addrs
}

func isAssignedIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var host net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				host = v.IP
			case *net.IPAddr:
				host = v.IP
			default:
				continue
			}
			if host.To4() != nil && host.Equal(parsed) {
				return true
			}
		}
	}
	return false
}

func primaryInterfaceIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var host net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				host = v.IP
			case *net.IPAddr:
				host = v.IP
			default:
				continue
			}
			if v4 := host.To4(); v4 != nil && !v4.IsLoopback() {
				return v4.String()
			}
		}
	}
	return ""
}
