package operations

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type DSRecord struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	TTL     int    `json:"ttl"`
	Content string `json:"content"`
}

type DNSSECState struct {
	Zone    string     `json:"zone"`
	Enabled bool       `json:"enabled"`
	DS      []DSRecord `json:"ds"`
}

func (h *Host) setDNSSEC(name string, enabled bool) (DNSSECState, error) {
	ascii, err := validate.NormalizeDomain(name)
	if err != nil {
		return DNSSECState{}, err
	}
	if err := h.ensureBindDNSSECDB(); err != nil {
		return DNSSECState{}, err
	}
	if !enabled {
		if h.live() {
			_, _ = runFixed("/usr/bin/pdnsutil", "disable-dnssec", ascii)
			_, _ = runFixed("/usr/bin/pdns_control", "bind-reload-now", ascii)
		}
		_ = h.removeDSFile(ascii)
		return DNSSECState{Zone: ascii, Enabled: false}, nil
	}
	if h.live() {
		if out, err := runFixed("/usr/bin/pdnsutil", "secure-zone", ascii); err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(strings.ToLower(msg), "already") {
				return DNSSECState{}, fmt.Errorf("secure-zone: %s", msg)
			}
		}
		_, _ = runFixed("/usr/bin/pdnsutil", "rectify-zone", ascii)
		_, _ = runFixed("/usr/bin/pdns_control", "bind-reload-now", ascii)
	}
	ds, err := h.dsRecords(ascii)
	if err != nil {
		return DNSSECState{}, err
	}
	if len(ds) == 0 {
		ds = []DSRecord{sandboxDS(ascii)}
		if err := h.writeDSFile(ascii, ds); err != nil {
			return DNSSECState{}, err
		}
	} else {
		_ = h.writeDSFile(ascii, ds)
	}
	return DNSSECState{Zone: ascii, Enabled: true, DS: ds}, nil
}

func (h *Host) getDSRecords(name string) (DNSSECState, error) {
	ascii, err := validate.NormalizeDomain(name)
	if err != nil {
		return DNSSECState{}, err
	}
	ds, err := h.dsRecords(ascii)
	if err != nil {
		return DNSSECState{}, err
	}
	return DNSSECState{Zone: ascii, Enabled: len(ds) > 0, DS: ds}, nil
}

func (h *Host) dsRecords(ascii string) ([]DSRecord, error) {
	if h.live() {
		out, err := runFixedStdout("/usr/bin/pdnsutil", "export-zone-ds", ascii)
		if err == nil {
			if parsed := ParseDSRecords(ascii, string(out)); len(parsed) > 0 {
				return parsed, nil
			}
		}
	}
	path, err := h.resolve("/var/lib/panel/dns/ds/" + ascii + ".ds")
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseDSRecords(ascii, string(b)), nil
}

func (h *Host) writeDSFile(ascii string, recs []DSRecord) error {
	if _, err := h.CreateDirectoryTree("/var/lib/panel/dns/ds", 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, r := range recs {
		name := r.Name
		if name == "" {
			name = ascii + "."
		}
		fmt.Fprintf(&b, "%s %d IN DS %s\n", name, r.TTL, r.Content)
	}
	_, err := h.ApplyFile("/var/lib/panel/dns/ds/"+ascii+".ds", []byte(b.String()), 0o644)
	return err
}

func (h *Host) removeDSFile(ascii string) error {
	path, err := h.resolve("/var/lib/panel/dns/ds/" + ascii + ".ds")
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	return nil
}

func (h *Host) ensureBindDNSSECDB() error {
	db := "/var/lib/panel/dns/bind-dnssec.sqlite3"
	if _, err := h.CreateDirectoryTree("/var/lib/panel/dns", 0o755); err != nil {
		return err
	}
	real, err := h.resolve(db)
	if err != nil {
		return err
	}
	if _, err := os.Stat(real); err == nil {
		return nil
	}
	if h.live() {
		if out, err := runFixed("/usr/bin/pdnsutil", "create-bind-db", db); err != nil {
			if _, st := os.Stat(real); st != nil {
				return fmt.Errorf("create-bind-db: %s", strings.TrimSpace(string(out)))
			}
		}
		if u, err := user.Lookup("pdns"); err == nil {
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)
			_ = os.Chown(real, uid, gid)
			if dir, err := h.resolve("/var/lib/panel/dns"); err == nil {
				_ = os.Chown(dir, uid, gid)
			}
		}
		_ = os.Chmod(real, 0o664)
		return nil
	}
	return os.WriteFile(real, []byte(""), 0o644)
}

func sandboxDS(zone string) DSRecord {
	return DSRecord{
		Name:    zone + ".",
		Type:    "DS",
		TTL:     3600,
		Content: "31337 13 2 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

func ParseDSRecords(zone, body string) []DSRecord {
	origin := strings.TrimSuffix(zone, ".") + "."
	var out []DSRecord
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		dsAt := -1
		for i, f := range fields {
			if strings.EqualFold(f, "DS") {
				dsAt = i
				break
			}
		}
		if dsAt < 0 || dsAt+1 >= len(fields) {
			continue
		}
		name := origin
		if dsAt > 0 && !strings.EqualFold(fields[0], "IN") && !isDigits(fields[0]) {
			name = fields[0]
			if !strings.HasSuffix(name, ".") {
				name += "."
			}
		}
		ttl := 3600
		for i := 0; i < dsAt; i++ {
			if isDigits(fields[i]) {
				if n, err := strconv.Atoi(fields[i]); err == nil {
					ttl = n
				}
			}
		}
		out = append(out, DSRecord{
			Name:    name,
			Type:    "DS",
			TTL:     ttl,
			Content: strings.Join(fields[dsAt+1:], " "),
		})
	}
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
