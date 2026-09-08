package firewall

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// HostingTCP is the inbound TCP set for a dedicated hosting node.
var HostingTCP = []int{21, 22, 25, 53, 80, 443, 587, 993, 8443, 8444}

// HostingPASV is the vsftpd passive-mode range (inclusive).
var HostingPASV = [2]int{40000, 40100}

// Rules returns an nft script for table inet panel. extraTCP ports are
// merged so applying on a live lab node does not drop existing listeners
// (for example the cloud-agent tunnel).
func Rules(extraTCP []int) string {
	ports := uniquePorts(append(append([]int{}, HostingTCP...), extraTCP...))
	var b strings.Builder
	b.WriteString("#!/usr/sbin/nft -f\n")
	b.WriteString("table inet panel {\n")
	b.WriteString("  chain input {\n")
	b.WriteString("    type filter hook input priority 0; policy drop;\n")
	b.WriteString("    iif lo accept\n")
	b.WriteString("    ct state established,related accept\n")
	b.WriteString("    tcp dport { ")
	for i, p := range ports {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(p))
	}
	b.WriteString(" } accept\n")
	b.WriteString("    tcp dport ")
	b.WriteString(strconv.Itoa(HostingPASV[0]))
	b.WriteString("-")
	b.WriteString(strconv.Itoa(HostingPASV[1]))
	b.WriteString(" accept\n")
	b.WriteString("    udp dport { 53 } accept\n")
	b.WriteString("    icmp type echo-request accept\n")
	b.WriteString("    ip6 nexthdr icmpv6 accept\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func uniquePorts(in []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, p := range in {
		if p < 1 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// ExtraListeningTCP returns non-loopback LISTEN ports from /proc so a
// drop-policy table does not cut an already-bound management session.
func ExtraListeningTCP() []int {
	var extra []int
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		extra = append(extra, parseProcTCP(name)...)
	}
	return uniquePorts(extra)
}

func parseProcTCP(path string) []int {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var ports []int
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		// header
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		if fields[3] != "0A" {
			continue
		}
		host, port, err := splitHexAddr(fields[1])
		if err != nil {
			continue
		}
		if isLoopbackHex(host) {
			continue
		}
		ports = append(ports, port)
	}
	return ports
}

func splitHexAddr(s string) (host string, port int, err error) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return "", 0, fmt.Errorf("addr")
	}
	p, err := strconv.ParseInt(s[i+1:], 16, 32)
	if err != nil {
		return "", 0, err
	}
	return s[:i], int(p), nil
}

func isLoopbackHex(host string) bool {
	switch strings.ToUpper(host) {
	case "0100007F", "7F000001":
		return true
	case "00000000000000000000000001000000":
		return true
	default:
		return len(host) == 32 && strings.HasSuffix(strings.ToUpper(host), "0000000000000000000000000001")
	}
}
