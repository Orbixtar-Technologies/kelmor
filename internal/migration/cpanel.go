package migration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
	"github.com/hosting-panel/panel/internal/store"
)

// FromCPanel reads an extracted cpmove-style tree (userdata, dnszones, mysql.sql, va)
// and produces a native HostingAccountExport. It does not execute cPanel binaries.
func FromCPanel(root, username string) (*HostingAccountExport, error) {
	if err := validate.Username(username); err != nil {
		return nil, err
	}
	ud, err := readUserdata(filepath.Join(root, "userdata"))
	if err != nil {
		return nil, err
	}
	domain := ud["DNS"]
	if domain == "" {
		domain = ud["DOMAIN"]
	}
	ascii, err := validate.NormalizeDomain(domain)
	if err != nil {
		return nil, fmt.Errorf("cpanel primary domain: %w", err)
	}
	accID := store.NewID()
	domID := store.NewID()
	mdID := store.NewID()
	exp := &HostingAccountExport{
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Account: store.Account{
			ID: accID, Username: username, PrimaryDomain: ascii,
			HomePath: "/home/" + username, Status: "provisioning", ShellClass: "sftp-only",
			DesiredRevision: 1,
		},
		Domains: []store.Domain{{
			ID: domID, AccountID: accID, FQDN: ascii, ASCII: ascii,
			Type: "primary", DocumentRoot: "/home/" + username + "/public_html", DNSManaged: true, Status: "provisioning",
		}},
		MailDomains: []store.MailDomain{{ID: mdID, AccountID: accID, DomainID: domID, CatchallPolicy: "reject", Status: "active"}},
	}
	exp.Databases = parseMySQLDump(filepath.Join(root, "mysql.sql"), accID)
	exp.Zones, exp.Records = parseDNSZones(filepath.Join(root, "dnszones"), accID, domID, ascii)
	exp.Mailboxes = parseCPanelMail(filepath.Join(root, "va"), accID, mdID)
	homedir, dump := stageCPanelPaths(root, username)
	exp.Homedir = homedir
	exp.MySQLDump = dump
	if len(exp.Mailboxes) == 0 {
		exp.Mailboxes = []store.Mailbox{{
			ID: store.NewID(), AccountID: accID, DomainID: mdID, LocalPart: "postmaster",
			QuotaBytes: 1 << 30, PasswordHash: "!", Status: "provisioning",
		}}
	}
	return exp, nil
}

func readUserdata(dir string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cpanel userdata: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				k, v, ok = strings.Cut(line, ": ")
			}
			if ok {
				out[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		_ = f.Close()
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cpanel userdata empty")
	}
	return out, nil
}

func parseMySQLDump(path, accountID string) []store.HostedDatabase {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []store.HostedDatabase
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		u := strings.ToUpper(strings.TrimSpace(line))
		if !strings.HasPrefix(u, "CREATE DATABASE") {
			continue
		}
		name := extractIdent(line)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, store.HostedDatabase{ID: store.NewID(), AccountID: accountID, Engine: "mariadb", Name: name, Status: "provisioning"})
	}
	return out
}

func parseDNSZones(dir, accountID, domainID, zoneName string) ([]store.DNSZone, []store.DNSRecord) {
	z := store.DNSZone{ID: store.NewID(), AccountID: accountID, DomainID: domainID, Name: zoneName, Provider: "powerdns", DesiredRevision: 1}
	var recs []store.DNSRecord
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []store.DNSZone{z}, nil
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "$") {
				continue
			}
			fs := strings.Fields(line)
			if len(fs) < 4 {
				continue
			}
			name, typ, content := fs[0], "", ""
			for i := 1; i < len(fs); i++ {
				u := strings.ToUpper(fs[i])
				if u == "IN" {
					continue
				}
				if typ == "" && len(u) <= 5 && strings.IndexFunc(u, func(r rune) bool { return r < 'A' || r > 'Z' }) < 0 {
					typ = u
					content = strings.Join(fs[i+1:], " ")
					break
				}
			}
			if typ == "" {
				continue
			}
			recs = append(recs, store.DNSRecord{ID: store.NewID(), ZoneID: z.ID, Name: strings.TrimSuffix(name, "."), Type: typ, Content: content, TTL: 3600})
		}
	}
	return []store.DNSZone{z}, recs
}

func parseCPanelMail(dir, accountID, mailDomainID string) []store.Mailbox {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []store.Mailbox
	for _, e := range entries {
		name := e.Name()
		local := name
		if i := strings.IndexByte(name, '@'); i > 0 {
			local = name[:i]
		}
		if err := validate.LocalPart(local); err != nil {
			continue
		}
		out = append(out, store.Mailbox{
			ID: store.NewID(), AccountID: accountID, DomainID: mailDomainID,
			LocalPart: local, QuotaBytes: 1 << 30, PasswordHash: "!", Status: "provisioning",
		})
	}
	return out
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func importRootAllowed(p string) bool {
	return strings.HasPrefix(p, "/var/tmp/panel-imports/") || strings.HasPrefix(p, "/var/lib/panel/imports/")
}

// stageCPanelPaths copies mysql.sql and homedir onto an agent-approved
// prefix when the extracted tree lives elsewhere (for example testdata).
func stageCPanelPaths(root, username string) (homedir, dump string) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	destRoot := absRoot
	if !importRootAllowed(absRoot+"/") && !importRootAllowed(absRoot) {
		destRoot = filepath.Join("/var/tmp/panel-imports", username)
		if err := os.MkdirAll(destRoot, 0o750); err != nil {
			destRoot = absRoot
		} else {
			if src := filepath.Join(absRoot, "mysql.sql"); fileExists(src) {
				_ = copyRegularFile(src, filepath.Join(destRoot, "mysql.sql"))
			}
			if src := filepath.Join(absRoot, "homedir"); dirExists(src) {
				_ = copyRegularTree(src, filepath.Join(destRoot, "homedir"))
			}
		}
	}
	if dirExists(filepath.Join(destRoot, "homedir")) {
		homedir = filepath.Join(destRoot, "homedir")
	}
	if fileExists(filepath.Join(destRoot, "mysql.sql")) {
		dump = filepath.Join(destRoot, "mysql.sql")
	}
	return homedir, dump
}

func copyRegularFile(src, dest string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	return os.WriteFile(dest, in, 0o640)
}

func copyRegularTree(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dest, 0o755)
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("homedir path escape")
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyRegularFile(path, target)
	})
}

func extractIdent(line string) string {
	for _, q := range []string{"`", "'", `"`} {
		if i := strings.Index(line, q); i >= 0 {
			rest := line[i+1:]
			if j := strings.Index(rest, q); j >= 0 {
				return rest[:j]
			}
		}
	}
	return ""
}
