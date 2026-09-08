package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/limits"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func SFTPReadOnly(used, limit int64) bool {
	if limit <= 0 {
		return false
	}
	return used >= limit
}

func accountFromHomePath(p string) (string, bool) {
	clean := filepath.Clean(p)
	const prefix = "/home/"
	if !strings.HasPrefix(clean, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(clean, prefix)
	user, _, _ := strings.Cut(rest, "/")
	if validate.Username(user) != nil {
		return "", false
	}
	return user, true
}

func (h *Host) persistQuota(username string, bytes int64) error {
	if err := validate.Username(username); err != nil {
		return err
	}
	body := []byte(strconv.FormatInt(bytes, 10) + "\n")
	for _, p := range []string{
		"/var/lib/panel/quotas/" + username,
		"/home/" + username + "/.panel-quota",
	} {
		rp, err := h.resolve(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(rp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(rp, body, 0o444); err != nil {
			return err
		}
		_ = os.Chmod(rp, 0o444)
	}
	return nil
}

func (h *Host) readQuota(username string) (int64, error) {
	if err := validate.Username(username); err != nil {
		return 0, err
	}
	for _, p := range []string{
		"/var/lib/panel/quotas/" + username,
		"/home/" + username + "/.panel-quota",
	} {
		rp, err := h.resolve(p)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(rp)
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			continue
		}
		return n, nil
	}
	return 0, nil
}

func (h *Host) rejectHomeWrite(path string, incoming int64) error {
	if strings.HasSuffix(path, "/.panel-quota") || strings.HasSuffix(path, "/.panel-readonly") {
		return nil
	}
	username, ok := accountFromHomePath(path)
	if !ok {
		return nil
	}
	limit, err := h.readQuota(username)
	if err != nil || limit <= 0 {
		return nil
	}
	used, err := h.measureAccountUsage(username, "/home/"+username)
	if err != nil {
		return limits.Check{Kind: "disk_bytes", Limit: limit, Used: limit}
	}
	var old int64
	if rp, err := h.resolve(path); err == nil {
		if st, err := os.Stat(rp); err == nil {
			old = st.Size()
		}
	}
	return limits.DiskWouldExceed(used.DiskBytes-old, incoming, limit)
}

func (h *Host) enforceAccountDisk(username, home string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if home == "" {
		home = "/home/" + username
	}
	if _, err := policy.WithinAccount(username, home); err != nil {
		return Result{}, err
	}
	limit, err := h.readQuota(username)
	if err != nil {
		return Result{}, err
	}
	used, err := h.measureAccountUsage(username, home)
	if err != nil {
		return Result{}, err
	}
	lock := SFTPReadOnly(used.DiskBytes, limit)
	if err := h.setHomeWriteLock(username, lock); err != nil {
		return Result{}, err
	}
	if err := h.setQuotaReadOnlyMarker(username, lock); err != nil {
		return Result{}, err
	}
	if err := h.rewriteSFTPQuotaSSHD(); err != nil {
		return Result{}, err
	}
	state := "writable"
	if lock {
		state = "readonly"
	}
	return Result{OK: true, ObservedState: state, Message: fmt.Sprintf("disk %d/%d", used.DiskBytes, limit)}, nil
}

func (h *Host) setHomeWriteLock(username string, lock bool) error {
	mode := os.FileMode(0o750)
	if lock {
		mode = 0o550
	}
	for _, d := range []string{"public_html", "apps", "backups", "tmp"} {
		p, err := h.resolve(filepath.Join("/home", username, d))
		if err != nil {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
	}
	return nil
}

func (h *Host) setQuotaReadOnlyMarker(username string, lock bool) error {
	p, err := h.resolve("/var/lib/panel/quotas/" + username + ".ro")
	if err != nil {
		return err
	}
	if !lock {
		_ = os.Remove(p)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte("1\n"), 0o644)
}

func (h *Host) rewriteSFTPQuotaSSHD() error {
	dir, err := h.resolve("/var/lib/panel/quotas")
	if err != nil {
		return err
	}
	var ro []string
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range ents {
		name := e.Name()
		if !strings.HasSuffix(name, ".ro") {
			continue
		}
		u := strings.TrimSuffix(name, ".ro")
		if validate.Username(u) != nil {
			continue
		}
		ro = append(ro, u)
	}
	sort.Strings(ro)
	body := "# managed by panel-agent — SFTP read-only when over package disk_bytes\n"
	if len(ro) > 0 {
		body += "Match User " + strings.Join(ro, ",") + "\n    ForceCommand internal-sftp -R\n"
	}
	dst, err := h.resolve("/etc/ssh/sshd_config.d/zz-panel-sftp-quota.conf")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
		return err
	}
	h.reloadSSHD()
	return nil
}

func (h *Host) reloadSSHD() {
	if !h.live() {
		return
	}
	if out, err := runFixed("/usr/sbin/sshd", "-t"); err != nil {
		_ = out
		return
	}
	for _, pidf := range []string{"/run/sshd.pid", "/var/run/sshd.pid"} {
		b, err := os.ReadFile(pidf)
		if err != nil {
			continue
		}
		pid := strings.TrimSpace(string(b))
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}
		_, _ = runFixed("/bin/kill", "-HUP", pid)
		return
	}
}

func (h *Host) clearQuotaFiles(username string) {
	h.removeManaged("/var/lib/panel/quotas/" + username)
	h.removeManaged("/var/lib/panel/quotas/" + username + ".ro")
	_ = h.rewriteSFTPQuotaSSHD()
}
