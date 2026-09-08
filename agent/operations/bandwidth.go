package operations

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func parseNginxBodyBytes(line string) (when time.Time, n int64, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return time.Time{}, 0, false
	}
	if t, n, ok := parseCombinedAccess(line); ok {
		return t, n, true
	}
	return parsePanelBandwidth(line)
}

func parseCombinedAccess(line string) (time.Time, int64, bool) {
	i := strings.Index(line, "[")
	j := strings.Index(line, "]")
	if i < 0 || j <= i+1 {
		return time.Time{}, 0, false
	}
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", line[i+1:j])
	if err != nil {
		return time.Time{}, 0, false
	}
	close := strings.Index(line, "] ")
	if close < 0 {
		return time.Time{}, 0, false
	}
	rest := strings.TrimSpace(line[close+2:])
	if !strings.HasPrefix(rest, `"`) {
		return time.Time{}, 0, false
	}
	endReq := strings.Index(rest[1:], `"`)
	if endReq < 0 {
		return time.Time{}, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(rest[endReq+2:]))
	if len(fields) < 2 {
		return time.Time{}, 0, false
	}
	if fields[1] == "-" {
		return t, 0, true
	}
	n, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || n < 0 {
		return time.Time{}, 0, false
	}
	return t, n, true
}

func parsePanelBandwidth(line string) (time.Time, int64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return time.Time{}, 0, false
	}
	t, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return time.Time{}, 0, false
	}
	if fields[1] == "-" {
		return t, 0, true
	}
	n, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || n < 0 {
		return time.Time{}, 0, false
	}
	return t, n, true
}

func sameCalendarMonth(a, b time.Time) bool {
	au, bu := a.UTC(), b.UTC()
	return au.Year() == bu.Year() && au.Month() == bu.Month()
}

func (h *Host) websiteIDsForAccount(username string) []string {
	if validate.Username(username) != nil {
		return nil
	}
	dir, err := h.resolve("/etc/nginx/panel-sites")
	if err != nil {
		return nil
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	home := "/home/" + username + "/"
	sock := "panel-" + username + ".sock"
	var ids []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".conf") {
			continue
		}
		id := strings.TrimSuffix(name, ".conf")
		if id == "" || strings.ContainsAny(id, "/\\") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		s := string(body)
		if !strings.Contains(s, home) && !strings.Contains(s, sock) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (h *Host) sumNginxBandwidth(websiteIDs []string, now time.Time) int64 {
	var total int64
	for _, id := range websiteIDs {
		if id == "" || strings.ContainsAny(id, "/\\") {
			continue
		}
		for _, suffix := range []string{".access.log", ".access.log.1"} {
			p, err := h.resolve("/var/log/nginx/" + id + suffix)
			if err != nil {
				continue
			}
			f, err := os.Open(p)
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				when, n, ok := parseNginxBodyBytes(sc.Text())
				if !ok || !sameCalendarMonth(when, now) {
					continue
				}
				total += n
			}
			_ = f.Close()
		}
	}
	return total
}

func (h *Host) persistBandwidthTotal(username string, month time.Time, n int64) {
	if validate.Username(username) != nil {
		return
	}
	stamp := month.UTC().Format("200601")
	_, _ = h.ApplyFile(
		"/var/lib/panel/bandwidth/"+username+"/"+stamp,
		[]byte(strconv.FormatInt(n, 10)+"\n"),
		0o644,
	)
}

func (h *Host) enforceAccountBandwidth(username string, hold bool) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	flag := "0\n"
	if hold {
		flag = "1\n"
	}
	if _, err := h.ApplyFile("/var/lib/panel/bandwidth/"+username+"/hold", []byte(flag), 0o644); err != nil {
		return Result{}, err
	}
	state := "ok"
	if hold {
		state = "held"
	}
	return Result{OK: true, Message: "bandwidth hold updated", ObservedState: state}, nil
}

func (h *Host) clearBandwidthFiles(username string) {
	if validate.Username(username) != nil {
		return
	}
	if dir, err := h.resolve("/var/lib/panel/bandwidth/" + username); err == nil {
		_ = os.RemoveAll(dir)
	}
}
