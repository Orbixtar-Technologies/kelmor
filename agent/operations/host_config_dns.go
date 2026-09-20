package operations

import (
	"fmt"
	"net"
	"strings"
)

func nameserverListenOK(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	if strings.ContainsAny(raw, "\n\r;|&$`'\"") {
		return false
	}
	parts := strings.Fields(strings.ReplaceAll(raw, ",", " "))
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if net.ParseIP(part) == nil {
			return false
		}
	}
	return true
}

func normalizeNameserverListen(raw string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.TrimSpace(raw), ",", " "))
	return strings.Join(parts, ",")
}

func (h *Host) applyNameserverSelection(spec HostConfigSpec) error {
	software := strings.TrimSpace(spec.NameserverSoftware)
	if software == "" {
		software = "pdns"
	}
	if software != "pdns" && software != "disabled" {
		return fmt.Errorf("nameserver software is not permitted")
	}
	listen := strings.TrimSpace(spec.NameserverListen)
	if !nameserverListenOK(listen) {
		return fmt.Errorf("invalid PowerDNS listen address")
	}
	record := "software=" + software + "\n"
	if listen != "" {
		record += "listen_address=" + normalizeNameserverListen(listen) + "\n"
	}
	if _, err := h.ApplyFile("/etc/panel/nameserver-selection", []byte(record), 0o644); err != nil {
		return err
	}
	if listen != "" {
		if err := h.writePowerDNSListen(normalizeNameserverListen(listen)); err != nil {
			return err
		}
	}
	if h.live() {
		if software == "disabled" {
			_ = controlNamedService("pdns", "stop")
		} else {
			_ = controlNamedService("pdns", "enable")
		}
	}
	return nil
}

func (h *Host) writePowerDNSListen(listen string) error {
	dropIn := "# Kelmor PowerDNS listen — written by ApplyHostConfig\nlocal-address=" + listen + "\n"
	if _, err := h.ApplyFile("/etc/powerdns/pdns.d/99-panel-listen.conf", []byte(dropIn), 0o644); err != nil {
		return err
	}
	existing, err := h.readManaged("/etc/powerdns/pdns.conf", 1<<20)
	if err != nil {
		return fmt.Errorf("PowerDNS configuration is not present")
	}
	merged := upsertPdnsKeys(string(existing), map[string]string{"local-address": listen})
	_, err = h.ApplyFile("/etc/powerdns/pdns.conf", []byte(merged), 0o640)
	return err
}

func upsertPdnsKeys(existing string, keys map[string]string) string {
	seen := map[string]bool{}
	var b strings.Builder
	for _, line := range strings.Split(existing, "\n") {
		candidate := strings.TrimSpace(line)
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "#"))
		key, _, ok := strings.Cut(candidate, "=")
		key = strings.TrimSpace(key)
		value, managed := keys[key]
		if !ok || !managed {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	for _, key := range []string{"local-address"} {
		if seen[key] {
			continue
		}
		value := keys[key]
		if value == "" {
			continue
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	out := b.String()
	return strings.TrimSuffix(out, "\n") + "\n"
}
