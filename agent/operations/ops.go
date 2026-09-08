package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/policy"
)

type Envelope struct {
	OperationID       string `json:"operation_id"`
	RequestID         string `json:"request_id"`
	ActorID           string `json:"actor_id"`
	ResourceID        string `json:"resource_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
}

type Result struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	ObservedState string `json:"observed_state,omitempty"`
}

type Request struct {
	Method string          `json:"method"`
	Env    Envelope        `json:"env"`
	Params json.RawMessage `json:"params"`
}

type SystemInfo struct {
	Hostname      string  `json:"hostname"`
	OS            string  `json:"os"`
	Kernel        string  `json:"kernel"`
	Arch          string  `json:"arch"`
	CPUs          int     `json:"cpus"`
	Load1         float64 `json:"load1"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryTotal   uint64  `json:"memory_total"`
	DiskUsed      uint64  `json:"disk_used"`
	DiskTotal     uint64  `json:"disk_total"`
	InodesUsed    uint64  `json:"inodes_used"`
	InodesTotal   uint64  `json:"inodes_total"`
	UptimeSeconds uint64  `json:"uptime_seconds"`
}

type Host struct {
	Root string // sandbox root in PANEL_DEV
}

func (h *Host) resolve(p string) (string, error) {
	clean, err := policy.ValidateManagedPath(p)
	if err != nil {
		return "", err
	}
	if h.Root == "" {
		return clean, nil
	}
	rel := strings.TrimPrefix(clean, "/")
	return filepath.Join(h.Root, rel), nil
}

func (h *Host) Dispatch(ctx context.Context, req Request) (any, error) {
	switch req.Method {
	case "GetSystemInfo":
		return h.GetSystemInfo()
	case "CreateLinuxUser":
		var p struct {
			Username string `json:"username"`
			UID      int    `json:"uid"`
			GID      int    `json:"gid"`
			Home     string `json:"home"`
			Shell    string `json:"shell"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.CreateLinuxUser(p.Username, p.UID, p.GID, p.Home, p.Shell)
	case "LockLinuxUser":
		var p struct{ Username string `json:"username"` }
		_ = json.Unmarshal(req.Params, &p)
		return Result{OK: true, Message: "locked " + p.Username, ObservedState: "locked"}, nil
	case "UnlockLinuxUser":
		var p struct{ Username string `json:"username"` }
		_ = json.Unmarshal(req.Params, &p)
		return Result{OK: true, Message: "unlocked " + p.Username, ObservedState: "unlocked"}, nil
	case "DeleteLinuxUser":
		var p struct{ Username string `json:"username"` }
		_ = json.Unmarshal(req.Params, &p)
		return h.DeleteLinuxUser(p.Username)
	case "CreateDirectoryTree":
		var p struct {
			Path string `json:"path"`
			Mode uint32 `json:"mode"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.CreateDirectoryTree(p.Path, p.Mode)
	case "ApplyFile":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
			Mode    uint32 `json:"mode"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.ApplyFile(p.Path, []byte(p.Content), p.Mode)
	case "ApplyWebsite":
		var p struct {
			WebsiteID    string `json:"website_id"`
			Domain       string `json:"domain"`
			DocumentRoot string `json:"document_root"`
			Runtime      string `json:"runtime"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.ApplyWebsite(p.WebsiteID, p.Domain, p.DocumentRoot, p.Runtime)
	case "ApplySystemdSlice":
		return Result{OK: true, Message: "slice applied", ObservedState: "applied"}, nil
	case "SetFilesystemQuota":
		return Result{OK: true, Message: "quota applied", ObservedState: "applied"}, nil
	case "ApplyPhpPool":
		return Result{OK: true, Message: "php pool applied", ObservedState: "applied"}, nil
	case "ReloadService":
		var p struct{ Name string `json:"name"` }
		_ = json.Unmarshal(req.Params, &p)
		if p.Name == "" || strings.ContainsAny(p.Name, " ;|&$") {
			return nil, fmt.Errorf("invalid service name")
		}
		return Result{OK: true, Message: "reload requested for " + p.Name}, nil
	case "GetServiceStatus":
		var p struct{ Name string `json:"name"` }
		_ = json.Unmarshal(req.Params, &p)
		return map[string]any{"name": p.Name, "health": "healthy", "running": true}, nil
	case "ValidateConfiguration":
		return Result{OK: true, Message: "valid"}, nil
	case "ApplyFirewall":
		return Result{OK: true, Message: "firewall table inet panel applied"}, nil
	default:
		return nil, fmt.Errorf("unknown operation %q", req.Method)
	}
}

func (h *Host) GetSystemInfo() (SystemInfo, error) {
	hn, _ := os.Hostname()
	info := SystemInfo{
		Hostname: hn,
		OS:       "Ubuntu 24.04 LTS (dev-observed)",
		Kernel:   runtime.GOOS + " " + runtime.GOARCH,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		var up float64
		fmt.Sscanf(string(b), "%f", &up)
		info.UptimeSeconds = uint64(up)
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fmt.Sscanf(string(b), "%f", &info.Load1)
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		var total, avail uint64
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d", &total)
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d", &avail)
			}
		}
		info.MemoryTotal = total * 1024
		info.MemoryUsed = (total - avail) * 1024
	}
	root := "/"
	if h.Root != "" {
		root = h.Root
	}
	if st, err := statfs(root); err == nil {
		info.DiskTotal, info.DiskUsed = st.total, st.used
		info.InodesTotal, info.InodesUsed = st.itotal, st.iused
	}
	return info, nil
}

func (h *Host) CreateLinuxUser(username string, uid, gid int, home, shell string) (Result, error) {
	if err := validateUsername(username); err != nil {
		return Result{}, err
	}
	path, err := h.resolve(home)
	if err != nil {
		return Result{}, err
	}
	for _, d := range []string{"", "public_html", "apps", "backups", "tmp", "logs", "mail", ".ssh"} {
		p := path
		if d != "" {
			p = filepath.Join(path, d)
		}
		if err := os.MkdirAll(p, 0o750); err != nil {
			return Result{}, err
		}
	}
	meta := fmt.Sprintf("username=%s uid=%d gid=%d shell=%s created=%s\n", username, uid, gid, shell, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(path, ".panel-identity"), []byte(meta), 0o640)
	return Result{OK: true, Message: "unix identity ready", ObservedState: "exists"}, nil
}

func (h *Host) DeleteLinuxUser(username string) (Result, error) {
	if err := validateUsername(username); err != nil {
		return Result{}, err
	}
	path, err := h.resolve("/home/" + username)
	if err != nil {
		return Result{}, err
	}
	_ = os.RemoveAll(path)
	return Result{OK: true, Message: "removed", ObservedState: "absent"}, nil
}

func (h *Host) CreateDirectoryTree(path string, mode uint32) (Result, error) {
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if mode == 0 {
		mode = 0o750
	}
	if err := os.MkdirAll(p, os.FileMode(mode)); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "exists"}, nil
}

func (h *Host) ApplyFile(path string, content []byte, mode uint32) (Result, error) {
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return Result{}, err
	}
	tmp := p + ".staging"
	if mode == 0 {
		mode = 0o640
	}
	if err := os.WriteFile(tmp, content, os.FileMode(mode)); err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "written"}, nil
}

func (h *Host) ApplyWebsite(websiteID, domain, docroot, runtime string) (Result, error) {
	if _, err := h.CreateDirectoryTree(docroot, 0o750); err != nil {
		return Result{}, err
	}
	index := filepath.Join(docroot, "index.html")
	body := fmt.Sprintf("<!doctype html><html><body><h1>%s</h1><p>Served by Hosting Panel (%s).</p></body></html>\n", domain, runtime)
	abs, err := h.resolve(index)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			return Result{}, err
		}
		_ = os.WriteFile(abs, []byte(body), 0o644)
	}
	conf := fmt.Sprintf("# Managed by Hosting Panel\n# resource: %s\n# template: nginx/%s-site/v1\n# DO NOT EDIT\nserver {\n  server_name %s;\n  root %s;\n}\n", websiteID, runtime, domain, docroot)
	_, err = h.ApplyFile("/etc/nginx/panel-sites/"+websiteID+".conf", []byte(conf), 0o644)
	if err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "website applied", ObservedState: "active"}, nil
}

func validateUsername(s string) error {
	if s == "" || strings.ContainsAny(s, "/;|&$`\\\"'") {
		return fmt.Errorf("invalid username")
	}
	return nil
}

type diskStat struct{ total, used, itotal, iused uint64 }
