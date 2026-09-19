package operations

import (
	"os"
	"strings"
)

const diagnosticsFileCap = 64 << 10

type DiagnosticFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type DiagnosticsBundle struct {
	Files []DiagnosticFile `json:"files"`
}

func (h *Host) collectDiagnostics() (DiagnosticsBundle, error) {
	paths := []struct {
		name string
		path string
	}{
		{"etc-panel/server-profile", "/etc/panel/server-profile"},
		{"etc-panel/cluster.json", "/etc/panel/cluster.json"},
		{"etc-panel/language-modules.json", languageModulesPath},
		{"etc-panel/compiler-access", "/etc/panel/compiler-access"},
		{"etc-panel/resolvers.conf", "/etc/panel/resolvers.conf"},
		{"var-lib-panel/portal-hostname", "/var/lib/panel/portal-hostname"},
		{"var-lib-panel/timezone", "/var/lib/panel/timezone"},
		{"logs/api.jsonl", "/var/lib/panel/logs/api.jsonl"},
		{"logs/mail.log", "/var/log/mail.log"},
		{"logs/nginx-error.log", "/var/log/nginx/error.log"},
	}
	out := DiagnosticsBundle{Files: []DiagnosticFile{}}
	for _, entry := range paths {
		raw, err := h.readManaged(entry.path, diagnosticsFileCap)
		if err != nil {
			if resolved, resErr := h.resolve(entry.path); resErr == nil {
				raw, err = readTail(resolved, diagnosticsFileCap)
			}
		}
		if err != nil || len(raw) == 0 {
			continue
		}
		out.Files = append(out.Files, DiagnosticFile{
			Name:    entry.name,
			Content: redactDiagnostic(string(raw)),
		})
	}
	if info, err := h.GetSystemInfo(); err == nil {
		out.Files = append(out.Files, DiagnosticFile{
			Name:    "host/system.txt",
			Content: info.Hostname + "\n" + info.OS + "\n" + info.Kernel + "\n",
		})
	}
	return out, nil
}

func readTail(path string, limit int64) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st.Size() > limit {
		if _, err := f.Seek(st.Size()-limit, 0); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

func redactDiagnostic(raw string) string {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if diagnosticSecretLine(line) {
			b.WriteString("[redacted]\n")
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRightFunc(b.String(), func(r rune) bool { return r == '\n' }) + "\n"
}

func diagnosticSecretLine(line string) bool {
	lower := strings.ToLower(line)
	for _, key := range []string{"password", "secret", "private_key", "authorization", "api_key", "token="} {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}
