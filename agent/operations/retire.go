package operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type retireJournal struct {
	Completed []string `json:"completed"`
}

func (h *Host) retireApplication(websiteID, account string) (Result, error) {
	websiteID = strings.TrimSpace(websiteID)
	if websiteID == "" || strings.ContainsAny(websiteID, "/\\.;|&$`\n") {
		return Result{}, fmt.Errorf("invalid website id")
	}
	if err := validate.Username(account); err != nil {
		return Result{}, err
	}
	journalPath := "/var/lib/panel/retire/app-" + websiteID + ".json"
	done := h.loadRetireJournal(journalPath)
	steps := []struct {
		name string
		fn   func() error
	}{
		{"stop_unit", func() error {
			if h.live() {
				_, _ = runFixed("/bin/systemctl", "stop", "panel-app-"+websiteID+".service")
				_, _ = runFixed("/bin/systemctl", "disable", "panel-app-"+websiteID+".service")
			}
			return nil
		}},
		{"remove_socket", func() error {
			h.removeManaged("/run/panel/apps/" + websiteID + ".sock")
			return nil
		}},
		{"remove_unit", func() error {
			h.removeManaged("/etc/systemd/system/panel-app-" + websiteID + ".service")
			if h.live() {
				_, _ = runFixed("/bin/systemctl", "daemon-reload")
			}
			return nil
		}},
		{"remove_files", func() error {
			h.removeManaged("/home/" + account + "/apps/" + websiteID)
			return nil
		}},
	}
	for _, step := range steps {
		if retireJournalHas(done, step.name) {
			continue
		}
		if h.FailRetireAfter == step.name {
			return Result{}, fmt.Errorf("retire crashed at %s", step.name)
		}
		if err := step.fn(); err != nil {
			return Result{}, err
		}
		done = append(done, step.name)
		if err := h.writeRetireJournal(journalPath, done); err != nil {
			return Result{}, err
		}
	}
	h.removeManaged(journalPath)
	return Result{OK: true, ObservedState: "absent", Message: "application retired"}, nil
}

func (h *Host) loadRetireJournal(path string) []string {
	abs, err := h.resolve(path)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	var journal retireJournal
	if json.Unmarshal(raw, &journal) != nil {
		return nil
	}
	return journal.Completed
}

func (h *Host) writeRetireJournal(path string, completed []string) error {
	body, err := json.Marshal(retireJournal{Completed: completed})
	if err != nil {
		return err
	}
	_, err = h.ApplyFile(path, body, 0o640)
	return err
}

func retireJournalHas(completed []string, name string) bool {
	for _, item := range completed {
		if item == name {
			return true
		}
	}
	return false
}

func (h *Host) retireAccount(username string, websiteIDs, domains []string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	for _, id := range websiteIDs {
		id = strings.TrimSpace(id)
		if id == "" || strings.ContainsAny(id, "/\\") {
			continue
		}
		h.removeManaged("/etc/nginx/panel-sites/" + id + ".conf")
		h.removeManaged("/run/panel/apps/" + id + ".sock")
		h.removeManaged("/etc/systemd/system/panel-app-" + id + ".service")
		if h.live() {
			_, _ = runFixed("/bin/systemctl", "stop", "panel-app-"+id+".service")
		}
	}
	h.retireAccountSites(username)
	for _, ver := range []string{"8.3", "8.4", "8.5"} {
		h.removeManaged(fmt.Sprintf("/etc/php/%s/fpm/pool.d/panel-%s.conf", ver, username))
	}
	h.removeManaged("/etc/systemd/system/panel-account-" + username + ".slice")
	h.removeManaged("/etc/cron.d/panel-" + username)
	h.removeManaged("/var/lib/panel/cron/" + username)
	h.clearQuotaFiles(username)
	h.clearBandwidthFiles(username)
	h.removeManaged("/var/lib/panel/cgroup/" + username)
	for _, domain := range domains {
		ascii, err := validate.NormalizeDomain(domain)
		if err != nil {
			continue
		}
		h.removeManaged("/var/lib/panel/certs/" + ascii + ".crt")
		h.removeManaged("/var/lib/panel/certs/" + ascii + ".key")
		h.removeManaged("/var/lib/panel/dns/zones/" + ascii + ".zone")
		h.removeMailboxTree(ascii)
		h.clearDKIM(ascii)
		h.dropNamedZone(ascii)
		if h.live() {
			_, _ = runFixed("/usr/bin/pdnsutil", "delete-zone", ascii)
		}
	}
	if h.live() {
		_ = h.freezeAccount(username, false)
		h.killAccountProcesses(username)
		h.removeCgroup(username)
		reloadFPM("8.3")
		if err := h.testNginx(); err != nil {
			return Result{}, err
		}
		_, _ = runFixed("/usr/sbin/nginx", "-s", "reload")
		_, _ = runFixed("/usr/bin/pdns_control", "rediscover")
	}
	return h.deleteUnixUser(username)
}

func (h *Host) retireDomain(account, ascii string, websiteIDs []string) (Result, error) {
	if err := validate.Username(account); err != nil {
		return Result{}, err
	}
	name, err := validate.NormalizeDomain(ascii)
	if err != nil {
		return Result{}, err
	}
	for _, id := range websiteIDs {
		if _, err := h.retireWebsite(id, account); err != nil {
			return Result{}, err
		}
	}
	h.removeManaged("/var/lib/panel/certs/" + name + ".crt")
	h.removeManaged("/var/lib/panel/certs/" + name + ".key")
	h.removeManaged("/var/lib/panel/dns/zones/" + name + ".zone")
	h.removeMailboxTree(name)
	h.clearDKIM(name)
	h.dropNamedZone(name)
	if h.live() {
		_, _ = runFixed("/usr/bin/pdnsutil", "delete-zone", name)
		_, _ = runFixed("/usr/bin/pdns_control", "rediscover")
	}
	if err := h.rewriteConnZone(); err != nil {
		return Result{}, err
	}
	if err := h.testNginx(); err != nil {
		return Result{}, err
	}
	if h.live() {
		if out, err := runFixed("/usr/sbin/nginx", "-s", "reload"); err != nil {
			return Result{}, fmt.Errorf("nginx reload: %s", strings.TrimSpace(string(out)))
		}
	}
	return Result{OK: true, ObservedState: "absent", Message: "domain retired"}, nil
}

func (h *Host) removeManaged(path string) {
	abs, err := h.resolve(path)
	if err != nil {
		return
	}
	_ = os.RemoveAll(abs)
}

func (h *Host) retireAccountSites(username string) {
	dir := "/etc/nginx/panel-sites"
	abs, err := h.resolve(dir)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return
	}
	markers := []string{
		"root /home/" + username + "/",
		"unix:/run/php/panel-" + username + ".sock",
	}
	for _, e := range entries {
		name := e.Name()
		if name == "00-acme.conf" || name == "01-modsec-probe.conf" {
			continue
		}
		p := filepath.Join(abs, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		body := string(b)
		for _, m := range markers {
			if strings.Contains(body, m) {
				_ = os.Remove(p)
				break
			}
		}
	}
}

func (h *Host) removeMailboxTree(domain string) {
	h.removeManaged("/var/vmail/" + domain)
}

func (h *Host) dropNamedZone(name string) {
	named := "/var/lib/panel/dns/named-zones.conf"
	rp, err := h.resolve(named)
	if err != nil {
		return
	}
	prev, err := os.ReadFile(rp)
	if err != nil {
		return
	}
	needle := `zone "` + name + `"`
	var b strings.Builder
	for _, line := range strings.Split(string(prev), "\n") {
		if strings.Contains(line, needle) {
			continue
		}
		if line == "" && b.Len() == 0 {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(rp, []byte(b.String()), 0o644)
}

func (h *Host) killAccountProcesses(username string) {
	if !h.live() {
		return
	}
	uid := unixUID(username)
	if uid == "" {
		return
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		st, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(st), "\n") {
			if !strings.HasPrefix(line, "Uid:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[1] == uid {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
			break
		}
	}
}

func unixUID(username string) string {
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(line, ":")
		if len(f) > 2 && f[0] == username {
			return f[2]
		}
	}
	return ""
}

func (h *Host) removeCgroup(username string) {
	if !h.live() {
		return
	}
	dir := accountCgroupDir(username)
	_ = os.WriteFile(filepath.Join(dir, "cgroup.freeze"), []byte("0\n"), 0o644)
	if b, err := os.ReadFile(filepath.Join(dir, "cgroup.procs")); err == nil {
		for _, line := range strings.Fields(string(b)) {
			_ = os.WriteFile(filepath.Join(panelCgroupRoot, "cgroup.procs"), []byte(line+"\n"), 0o644)
		}
	}
	_ = os.Remove(dir)
}
