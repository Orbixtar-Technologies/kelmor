package operations

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

//go:embed static_socket_server.cjs
var staticSocketServerSource []byte

var applicationUnitID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

func panelNodePath() string {
	for _, p := range []string{"/usr/local/bin/node", "/usr/bin/node"} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return p
		}
	}
	return "/usr/bin/node"
}

func panelPkgBin(logical string) string {
	for _, p := range []string{"/usr/local/bin/" + logical, "/usr/bin/" + logical} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return p
		}
	}
	return "/usr/bin/" + logical
}

// deployApplication deploys existing code; unlike website demo provisioning it
// never generates or replaces application source. It does install and build
// when the project metadata requires it, then starts the systemd unit.
func (h *Host) deployApplication(websiteID, account, runtime, workDir, command string) (Result, error) {
	if err := validate.Username(account); err != nil {
		return Result{}, err
	}
	if !applicationUnitID.MatchString(websiteID) {
		return Result{}, fmt.Errorf("invalid website ID")
	}
	canonical, err := policy.WithinAccount(account, workDir)
	if err != nil || canonical != workDir || containsSystemdControl(workDir) || strings.ContainsAny(workDir, "\"\\%") {
		return Result{}, fmt.Errorf("working directory must be canonical and within the account")
	}
	if runtime != "node" && runtime != "python" {
		return Result{}, fmt.Errorf("runtime must be node or python")
	}
	if containsSystemdControl(command) || strings.Contains(command, "%") {
		return Result{}, fmt.Errorf("start command cannot contain control characters or systemd specifiers")
	}

	detected, _ := h.DetectApplication(workDir)
	read := func(name string) ([]byte, error) { return h.readManaged(workDir+"/"+name, 1024*1024) }
	if command == "" {
		command = detected.StartCmd
	}
	if command == "" && runtime == "node" {
		manifest, readErr := read("package.json")
		if readErr == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(manifest, &pkg) != nil {
				return Result{}, fmt.Errorf("requirements: fix invalid package.json")
			}
			if strings.TrimSpace(pkg.Scripts["start"]) != "" {
				command = "/usr/bin/npm start"
			}
		} else if !os.IsNotExist(readErr) {
			return Result{}, fmt.Errorf("requirements: package.json cannot be read safely")
		}
		if command == "" {
			if _, err := read("server.js"); err != nil {
				return Result{}, fmt.Errorf("requirements: upload server.js or provide a package.json start script, a build script, or an explicit start command")
			}
			command = "/usr/bin/node server.js"
		}
	}
	if command == "" && runtime == "python" {
		if _, err := read("app.py"); err != nil {
			return Result{}, fmt.Errorf("requirements: upload app.py or provide an explicit Python start command")
		}
		command = "/usr/bin/python3 app.py"
	}
	if command == "" {
		return Result{}, fmt.Errorf("requirements: could not detect a start command")
	}
	if !strings.HasPrefix(command, "/") {
		return Result{}, fmt.Errorf("requirements: start command must begin with an absolute executable path")
	}
	if h.live() {
		binary := panelNodePath()
		if runtime == "python" {
			binary = "/usr/bin/python3"
		}
		for _, path := range []string{binary, strings.Fields(command)[0]} {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
				return Result{}, fmt.Errorf("requirements: install the required runtime or start executable before retrying")
			}
		}
	}
	if _, err := h.CreateDirectoryTree("/run/panel/apps", 0o1777); err != nil {
		return Result{}, err
	}
	if strings.Contains(command, panelStaticServerName) || detected.Kind == "spa" {
		helperPath := strings.TrimRight(workDir, "/") + "/" + panelStaticServerName
		if _, err := h.ApplyFile(helperPath, staticSocketServerSource, 0o644); err != nil {
			return Result{}, fmt.Errorf("requirements: could not install static socket server: %w", err)
		}
		// Normalize ExecStart to the workdir-local helper (absolute node, relative script).
		command = "/usr/bin/node " + panelStaticServerName
	}
	if h.live() {
		if err := h.prepareApplicationTree(account, workDir, detected); err != nil {
			return Result{}, err
		}
	}

	unit := "panel-app-" + websiteID + ".service"
	extraEnv := ""
	if detected.Kind == "spa" || strings.Contains(command, panelStaticServerName) {
		extraEnv = "Environment=STATIC_ROOT=dist\n"
	}
	body := fmt.Sprintf("[Unit]\nDescription=Kelmor application %s\nStartLimitIntervalSec=60\nStartLimitBurst=3\n[Service]\nUser=%s\nWorkingDirectory=%s\nExecStart=%s\nEnvironment=SOCKET_PATH=/run/panel/apps/%s.sock\n%sRestart=on-failure\nRestartSec=5\nNoNewPrivileges=yes\nSlice=panel-account-%s.slice\n[Install]\nWantedBy=multi-user.target\n", websiteID, account, workDir, command, websiteID, extraEnv, account)
	if _, err := h.ApplyFile("/etc/systemd/system/"+unit, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, ObservedState: "configured", Message: "Service configured in sandbox; execution not verified"}, nil
	}
	for _, args := range [][]string{{"daemon-reload"}, {"reset-failed", unit}, {"restart", unit}, {"is-active", unit}} {
		if _, err := runFixedEnv(h.commandContext(), "/bin/systemctl", nil, 30*time.Second, nil, args...); err != nil {
			if args[0] == "reset-failed" {
				continue
			}
			return Result{}, fmt.Errorf("deployment: service %s failed; inspect its journal, dependencies and environment, then retry", args[0])
		}
	}
	_, _ = runFixedEnv(h.commandContext(), "/bin/systemctl", nil, 15*time.Second, nil, "enable", unit)
	return Result{OK: true, ObservedState: "running", Message: "Application service is active; HTTP readiness is not verified"}, nil
}

func (h *Host) prepareApplicationTree(account, workDir string, detected DetectResult) error {
	h.chownTree(account, workDir)
	if detected.Runtime != "node" {
		return nil
	}
	manager := detected.Manager
	if manager == "" {
		manager = "npm"
	}
	bin := panelPkgBin(manager)
	home := "/home/" + account
	realDir, err := h.resolve(workDir)
	if err != nil {
		return err
	}
	storeDir := filepath.Join(home, "apps", ".pnpm-store")
	corepackHome := filepath.Join(home, ".cache", "node", "corepack")
	_ = os.MkdirAll(storeDir, 0o750)
	_ = os.MkdirAll(corepackHome, 0o750)
	h.chownTree(account, filepath.Join(home, "apps"))
	h.chownTree(account, filepath.Join(home, ".cache"))
	envPrefix := []string{
		"HOME=" + home,
		"USER=" + account,
		"LOGNAME=" + account,
		"CI=1",
		"NODE_OPTIONS=--max-old-space-size=384",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"PNPM_STORE_DIR=" + storeDir,
		"COREPACK_HOME=" + corepackHome,
	}
	run := func(pkgArgs ...string) ([]byte, error) {
		args := []string{"-u", account, "--", "/usr/bin/env"}
		args = append(args, envPrefix...)
		args = append(args, bin)
		if manager == "npm" {
			args = append(args, "--prefix", realDir)
		} else {
			args = append(args, "--dir", realDir)
		}
		args = append(args, pkgArgs...)
		return runFixedEnv(h.commandContext(), "/usr/sbin/runuser", nil, 5*time.Minute, nil, args...)
	}
	pkgFail := func(kind string, out []byte, err error) error {
		msg := strings.TrimSpace(string(out))
		if msg == "" && err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "unknown error"
		}
		// Keep job UI readable: strip ANSI and collapse whitespace.
		msg = stripANSI(msg)
		if len(msg) > 800 {
			msg = msg[:800] + "…"
		}
		return fmt.Errorf("requirements: %s failed: %s", kind, msg)
	}
	if detected.InstallCmd != "" {
		var pkgArgs []string
		switch manager {
		case "pnpm":
			// --force avoids interactive "reinstall node_modules?" prompts under CI.
			pkgArgs = []string{"install", "--frozen-lockfile", "--force"}
		case "yarn":
			pkgArgs = []string{"install", "--frozen-lockfile"}
		default:
			pkgArgs = []string{"ci"}
		}
		out, err := run(pkgArgs...)
		if err != nil {
			return pkgFail("dependency install", out, err)
		}
	}
	if detected.BuildCmd != "" {
		out, err := run("run", "build")
		if err != nil {
			return pkgFail("build", out, err)
		}
	}
	h.chownTree(account, workDir)
	return nil
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				i = j
				continue
			}
		}
		if s[i] == '\r' {
			continue
		}
		b.WriteByte(s[i])
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (h *Host) chownTree(account, workDir string) {
	if account == "" {
		return
	}
	realDir, err := h.resolve(workDir)
	if err != nil {
		return
	}
	u, err := user.Lookup(account)
	if err != nil {
		return
	}
	uid, err1 := strconv.Atoi(u.Uid)
	gid, err2 := strconv.Atoi(u.Gid)
	if err1 != nil || err2 != nil {
		return
	}
	_ = filepath.Walk(realDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		_ = os.Chown(path, uid, gid)
		return nil
	})
}
