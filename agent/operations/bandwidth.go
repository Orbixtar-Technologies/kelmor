package operations

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type bandwidthCheckpoint struct {
	Path   string `json:"path"`
	Inode  uint64 `json:"inode"`
	Size   int64  `json:"size"`
	Offset int64  `json:"offset"`
	Month  string `json:"month"`
	Bytes  int64  `json:"bytes"`
}

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

func (h *Host) sumNginxBandwidth(username string, websiteIDs []string, now time.Time) int64 {
	month := now.UTC().Format("200601")
	total := h.readPersistedMonthly(username, now)
	for _, id := range websiteIDs {
		if id == "" || strings.ContainsAny(id, "/\\") {
			continue
		}
		total += h.readBandwidthDelta(username, "/var/log/nginx/"+id+".access.log", month, now)
	}
	return total
}

func (h *Host) readPersistedMonthly(username string, month time.Time) int64 {
	if validate.Username(username) != nil {
		return 0
	}
	p, err := h.resolve("/var/lib/panel/bandwidth/" + username + "/" + month.UTC().Format("200601"))
	if err != nil {
		return 0
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (h *Host) readBandwidthDelta(username, rel, month string, now time.Time) int64 {
	p, err := h.resolve(rel)
	if err != nil {
		return 0
	}
	info, err := os.Stat(p)
	if err != nil {
		return 0
	}
	var inode uint64
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		inode = sys.Ino
	}
	cp := h.loadBandwidthCheckpoint(username, rel)
	start := int64(0)
	already := int64(0)
	if cp != nil && cp.Inode == inode && cp.Month == month && info.Size() >= cp.Offset {
		start = cp.Offset
		already = cp.Bytes
	}
	f, err := os.Open(p)
	if err != nil {
		return 0
	}
	defer f.Close()
	if start > 0 {
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return 0
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var delta int64
	for sc.Scan() {
		when, n, ok := parseNginxBodyBytes(sc.Text())
		if !ok || !sameCalendarMonth(when, now) {
			continue
		}
		delta += n
	}
	offset, _ := f.Seek(0, io.SeekCurrent)
	if offset == 0 {
		offset = info.Size()
	}
	h.saveBandwidthCheckpoint(username, bandwidthCheckpoint{
		Path: rel, Inode: inode, Size: info.Size(), Offset: offset, Month: month, Bytes: already + delta,
	})
	return delta
}

func (h *Host) checkpointPath(username, rel string) string {
	name := strings.ReplaceAll(strings.Trim(rel, "/"), "/", "_")
	return "/var/lib/panel/bandwidth/" + username + "/cp-" + name
}

func (h *Host) loadBandwidthCheckpoint(username, rel string) *bandwidthCheckpoint {
	p, err := h.resolve(h.checkpointPath(username, rel))
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cp bandwidthCheckpoint
	if json.Unmarshal(b, &cp) != nil {
		return nil
	}
	return &cp
}

func (h *Host) saveBandwidthCheckpoint(username string, cp bandwidthCheckpoint) {
	body, err := json.Marshal(cp)
	if err != nil {
		return
	}
	_, _ = h.ApplyFile(h.checkpointPath(username, cp.Path), append(body, '\n'), 0o644)
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
