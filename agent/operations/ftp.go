package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type FTPUser struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	GuestUser    string `json:"guest_user"`
	LocalRoot    string `json:"local_root"`
}

func (h *Host) applyFTPUsers(users []FTPUser) (Result, error) {
	passwdPath, err := h.resolve("/var/lib/panel/ftp/passwd")
	if err != nil {
		return Result{}, err
	}
	confDir, err := h.resolve("/var/lib/panel/ftp/user_conf")
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return Result{}, err
	}

	keep := map[string]bool{}
	var passwd strings.Builder
	for _, u := range users {
		if err := validate.Username(u.Username); err != nil {
			return Result{}, err
		}
		if err := validate.Username(u.GuestUser); err != nil {
			return Result{}, err
		}
		if !strings.HasPrefix(u.PasswordHash, "$6$") {
			return Result{}, fmt.Errorf("ftp user %s: SHA-512 crypt hash required", u.Username)
		}
		root, err := policy.WithinAccount(u.GuestUser, u.LocalRoot)
		if err != nil {
			return Result{}, err
		}
		keep[u.Username] = true
		fmt.Fprintf(&passwd, "%s:%s\n", u.Username, u.PasswordHash)
		body := fmt.Sprintf("guest_username=%s\nlocal_root=%s\nwrite_enable=YES\nanon_world_readable_only=NO\n",
			u.GuestUser, root)
		dst := filepath.Join(confDir, u.Username)
		if err := os.WriteFile(dst, []byte(body), 0o640); err != nil {
			return Result{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(passwdPath), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(passwdPath, []byte(passwd.String()), 0o640); err != nil {
		return Result{}, err
	}

	ents, err := os.ReadDir(confDir)
	if err != nil {
		return Result{}, err
	}
	for _, e := range ents {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		_ = os.Remove(filepath.Join(confDir, e.Name()))
	}

	if h.live() {
		if err := h.ensureVsftpd(); err != nil {
			return Result{}, err
		}
	}
	return Result{OK: true, ObservedState: "ftp-maps-applied", Message: fmt.Sprintf("%d users", len(keep))}, nil
}

func (h *Host) ensureVsftpd() error {
	if err := os.MkdirAll("/var/run/vsftpd/empty", 0o755); err != nil {
		return err
	}
	if pidOf("vsftpd") {
		_ = reloadNamedService("vsftpd")
		return nil
	}
	if _, err := os.Stat("/usr/sbin/vsftpd"); err != nil {
		return nil
	}
	_, err := startDetached("/usr/sbin/vsftpd", "/", "/etc/vsftpd.conf")
	return err
}
