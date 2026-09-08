package operations

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type Envelope struct {
	OperationID      string `json:"operation_id"`
	RequestID        string `json:"request_id"`
	ActorID          string `json:"actor_id"`
	ResourceID       string `json:"resource_id"`
	ExpectedRevision int64  `json:"expected_revision"`
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
	Sock string // unix agent socket; when set, Dispatch is forwarded
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
	if h.Sock != "" {
		return CallUnix(ctx, h.Sock, req)
	}
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
		return h.createUnixIdentity(p.Username, p.UID, p.GID, p.Home, p.Shell)
	case "LockLinuxUser":
		var p struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.lockUnixUser(p.Username)
	case "UnlockLinuxUser":
		var p struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.unlockUnixUser(p.Username)
	case "DeleteLinuxUser":
		var p struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.deleteUnixUser(p.Username)
	case "RetireAccount":
		var p struct {
			Username   string   `json:"username"`
			WebsiteIDs []string `json:"website_ids"`
			Domains    []string `json:"domains"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.retireAccount(p.Username, p.WebsiteIDs, p.Domains)
	case "FreezeAccount":
		var p struct {
			Username string `json:"username"`
			Freeze   bool   `json:"freeze"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if err := h.freezeAccount(p.Username, p.Freeze); err != nil {
			return nil, err
		}
		return Result{OK: true, ObservedState: map[bool]string{true: "frozen", false: "thawed"}[p.Freeze]}, nil
	case "SetLinuxPassword":
		var p struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.setLinuxPassword(p.Username, p.Password)
	case "CreateDirectoryTree":
		var p struct {
			Path string `json:"path"`
			Mode uint32 `json:"mode"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.CreateDirectoryTree(p.Path, p.Mode)
	case "ApplyFile":
		var p struct {
			Path       string `json:"path"`
			Content    string `json:"content"`
			ContentB64 string `json:"content_b64"`
			Mode       uint32 `json:"mode"`
		}
		_ = json.Unmarshal(req.Params, &p)
		body := []byte(p.Content)
		if p.ContentB64 != "" {
			dec, err := base64.StdEncoding.DecodeString(p.ContentB64)
			if err != nil {
				return nil, err
			}
			body = dec
		}
		return h.ApplyFile(p.Path, body, p.Mode)
	case "ApplyFileChunk":
		var p struct {
			Path       string `json:"path"`
			ContentB64 string `json:"content_b64"`
			Mode       uint32 `json:"mode"`
			Offset     int64  `json:"offset"`
			Last       bool   `json:"last"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		body, err := base64.StdEncoding.DecodeString(p.ContentB64)
		if err != nil {
			return nil, err
		}
		return h.applyFileChunk(p.Path, body, p.Mode, p.Offset, p.Last)
	case "ApplyWebsite":
		var p struct {
			WebsiteID             string   `json:"website_id"`
			Account               string   `json:"account"`
			Domain                string   `json:"domain"`
			DocumentRoot          string   `json:"document_root"`
			Runtime               string   `json:"runtime"`
			HTTPSRedirect         bool     `json:"https_redirect"`
			Enabled               *bool    `json:"enabled"`
			BandwidthHold         *bool    `json:"bandwidth_hold"`
			ConcurrentWebRequests int      `json:"concurrent_web_requests"`
			Aliases               []string `json:"aliases"`
		}
		_ = json.Unmarshal(req.Params, &p)
		enabled := true
		if p.Enabled != nil {
			enabled = *p.Enabled
		}
		hold := false
		if p.BandwidthHold != nil {
			hold = *p.BandwidthHold
		}
		return h.applyWebsite(p.WebsiteID, p.Account, p.Domain, p.DocumentRoot, p.Runtime, "", "", p.HTTPSRedirect, enabled, hold, p.ConcurrentWebRequests, p.Aliases)
	case "RetireWebsite":
		var p struct {
			WebsiteID string `json:"website_id"`
			Account   string `json:"account"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.retireWebsite(p.WebsiteID, p.Account)
	case "ApplyACMEChallenge":
		var p struct {
			Token string `json:"token"`
			Body  string `json:"body"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyACMEChallenge(p.Token, p.Body)
	case "ApplyAppUnit":
		var p struct {
			WebsiteID string `json:"website_id"`
			Account   string `json:"account"`
			Runtime   string `json:"runtime"`
			WorkDir   string `json:"working_directory"`
			Command   string `json:"start_command"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyAppUnit(p.WebsiteID, p.Account, p.Runtime, p.WorkDir, p.Command)
	case "ApplySystemdSlice":
		var p struct {
			Username    string `json:"username"`
			CPUPercent  int    `json:"cpu_percent"`
			MemoryBytes int64  `json:"memory_bytes"`
			TasksMax    int    `json:"tasks_max"`
			IOWeight    int    `json:"io_weight"`
			IOPS        int    `json:"iops"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applySlice(p.Username, p.CPUPercent, p.MemoryBytes, p.TasksMax, p.IOWeight, p.IOPS)
	case "SetFilesystemQuota":
		var p struct {
			Username string `json:"username"`
			Bytes    int64  `json:"bytes"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.setQuota(p.Username, p.Bytes)
	case "EnforceAccountDisk":
		var p struct {
			Username string `json:"username"`
			Home     string `json:"home"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.enforceAccountDisk(p.Username, p.Home)
	case "EnforceAccountBandwidth":
		var p struct {
			Username string `json:"username"`
			Hold     bool   `json:"hold"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.enforceAccountBandwidth(p.Username, p.Hold)
	case "ApplyPhpPool":
		var p struct {
			Account     string `json:"account"`
			Version     string `json:"version"`
			MaxChildren int    `json:"max_children"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyPHPPool(p.Account, p.Version, p.MaxChildren)
	case "CreateHostedDatabase":
		var p struct {
			Engine   string `json:"engine"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.createHostedDatabase(p.Engine, p.Name, p.Username, p.Password)
	case "DumpHostedDatabase":
		var p struct {
			Engine string `json:"engine"`
			Name   string `json:"name"`
			Dest   string `json:"dest"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.dumpHostedDatabase(p.Engine, p.Name, p.Dest)
	case "RestoreHostedDatabase":
		var p struct {
			Engine string `json:"engine"`
			Name   string `json:"name"`
			Source string `json:"source"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.restoreHostedDatabase(p.Engine, p.Name, p.Source)
	case "DropHostedDatabase":
		var p struct {
			Engine   string `json:"engine"`
			Name     string `json:"name"`
			Username string `json:"username"`
			DropUser bool   `json:"drop_user"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.dropHostedDatabase(p.Engine, p.Name, p.Username, p.DropUser)
	case "RemoveManagedFile":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		h.removeManaged(p.Path)
		return Result{OK: true, ObservedState: "absent"}, nil
	case "ApplyMailMaps":
		virtual, domains, passwd, uids, gids, sendLimits, aliases, err := decodeMaps(req.Params)
		if err != nil {
			return nil, err
		}
		return h.applyMailMaps(virtual, domains, passwd, uids, gids, sendLimits, aliases)
	case "EnsureDKIM":
		var p struct {
			Domain string `json:"domain"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.ensureDKIM(p.Domain)
	case "ApplyDKIMSigning":
		var p struct {
			Domains []string `json:"domains"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyDKIMSigning(p.Domains)
	case "ListDirectory":
		var p struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.listDirectory(p.Path)
	case "PackDirectory":
		var p struct {
			Source string `json:"source"`
			Dest   string `json:"dest"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.packDirectory(p.Source, p.Dest)
	case "UnpackDirectory":
		var p struct {
			Archive string `json:"archive"`
			Dest    string `json:"dest"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.unpackDirectory(p.Archive, p.Dest)
	case "CopyHomedir":
		var p struct {
			Username string `json:"username"`
			Source   string `json:"source"`
			Dest     string `json:"dest"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.copyHomedir(p.Username, p.Source, p.Dest)
	case "CreateMailboxHome":
		var p struct {
			Domain    string `json:"domain"`
			LocalPart string `json:"local_part"`
			UID       int    `json:"uid"`
			GID       int    `json:"gid"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.createMailboxHome(p.Domain, p.LocalPart, p.UID, p.GID)
	case "InstallWordPress":
		var p WordPressInstall
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.installWordPress(p)
	case "ApplyDNSZone":
		var p struct {
			Name string `json:"name"`
			Body string `json:"body"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyDNSZone(p.Name, p.Body)
	case "SetDNSSEC":
		var p struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.setDNSSEC(p.Name, p.Enabled)
	case "GetDSRecords":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.getDSRecords(p.Name)
	case "ApplyAccountCron":
		var p struct {
			Username string `json:"username"`
			Body     string `json:"body"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyAccountCron(p.Username, p.Body)
	case "ApplyAuthorizedKeys":
		var p struct {
			Username string `json:"username"`
			Body     string `json:"body"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.applyAuthorizedKeys(p.Username, p.Body)
	case "IssueDevCertificate":
		var p struct {
			Hostname string `json:"hostname"`
			Days     int    `json:"days"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.issueDevCertificate(p.Hostname, p.Days)
	case "ReloadService":
		var p struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if err := validateService(p.Name); err != nil {
			return nil, err
		}
		if h.live() {
			if err := reloadNamedService(p.Name); err != nil {
				return nil, err
			}
		}
		return Result{OK: true, Message: "reload requested for " + p.Name}, nil
	case "GetServiceStatus":
		var p struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return probeService(p.Name), nil
	case "MeasureAccountUsage":
		var p struct {
			Username string `json:"username"`
			Home     string `json:"home"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return h.measureAccountUsage(p.Username, p.Home)
	case "ValidateConfiguration":
		return Result{OK: true, Message: "valid"}, nil
	case "ApplyFirewall":
		return h.applyFirewall()
	case "ApplyFTPUsers":
		var p struct {
			Users []FTPUser `json:"users"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return h.applyFTPUsers(p.Users)
	case "RebootHost":
		return h.rebootHost()
	default:
		return nil, fmt.Errorf("unknown operation %q", req.Method)
	}
}

func (h *Host) GetSystemInfo() (SystemInfo, error) {
	if h.Sock != "" {
		v, err := CallUnix(context.Background(), h.Sock, Request{Method: "GetSystemInfo"})
		if err != nil {
			return SystemInfo{}, err
		}
		b, err := json.Marshal(v)
		if err != nil {
			return SystemInfo{}, err
		}
		var info SystemInfo
		if err := json.Unmarshal(b, &info); err != nil {
			return SystemInfo{}, err
		}
		return info, nil
	}
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
	if err := validate.Username(username); err != nil {
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
		mode := hostingDirMode(p)
		if err := os.MkdirAll(p, mode); err != nil {
			return Result{}, err
		}
		_ = os.Chmod(p, mode)
	}
	meta := fmt.Sprintf("username=%s uid=%d gid=%d shell=%s created=%s\n", username, uid, gid, shell, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(path, ".panel-identity"), []byte(meta), 0o640)
	return Result{OK: true, Message: "unix identity ready", ObservedState: "exists"}, nil
}

func (h *Host) DeleteLinuxUser(username string) (Result, error) {
	if err := validate.Username(username); err != nil {
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
	if h.Sock != "" {
		_, err := CallUnix(context.Background(), h.Sock, Request{
			Method: "CreateDirectoryTree",
			Params: mustRaw(map[string]any{"path": path, "mode": mode}),
		})
		if err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "exists"}, nil
	}
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if mode == 0 {
		mode = uint32(hostingDirMode(p))
	}
	if err := os.MkdirAll(p, os.FileMode(mode)); err != nil {
		return Result{}, err
	}
	_ = os.Chmod(p, os.FileMode(mode))
	h.chownAccountPath(p)
	return Result{OK: true, ObservedState: "exists"}, nil
}

func (h *Host) ApplyFile(path string, content []byte, mode uint32) (Result, error) {
	if err := h.rejectHomeWrite(path, int64(len(content))); err != nil {
		return Result{}, err
	}
	if h.Sock != "" {
		if err := h.applyFileOverSock(path, content, mode); err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "written"}, nil
	}
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
	_ = os.Chmod(tmp, os.FileMode(mode))
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return Result{}, err
	}
	_ = os.Chmod(p, os.FileMode(mode))
	h.chownAccountPath(p)
	return Result{OK: true, ObservedState: "written"}, nil
}

func (h *Host) chownAccountPath(p string) {
	if !h.live() {
		return
	}
	const prefix = "/home/"
	if !strings.HasPrefix(p, prefix) {
		return
	}
	name := strings.SplitN(strings.TrimPrefix(p, prefix), "/", 2)[0]
	if name == "" {
		return
	}
	u, err := user.Lookup(name)
	if err != nil {
		return
	}
	uid, err1 := strconv.Atoi(u.Uid)
	gid, err2 := strconv.Atoi(u.Gid)
	if err1 != nil || err2 != nil {
		return
	}
	if strings.Contains(p, "/public_html") {
		if wg := webServerGID(); wg > 0 {
			gid = wg
		}
	}
	_ = os.Chown(p, uid, gid)
}

func (h *Host) listDirectory(path string) (any, error) {
	p, err := h.resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return map[string]any{"path": path, "items": []any{}}, nil
	}
	items := []map[string]any{}
	for _, e := range entries {
		info, _ := e.Info()
		sz := int64(0)
		if info != nil {
			sz = info.Size()
		}
		items = append(items, map[string]any{"name": e.Name(), "dir": e.IsDir(), "size": sz})
	}
	return map[string]any{"path": path, "items": items}, nil
}

func (h *Host) ApplyWebsite(websiteID, domain, docroot, runtime string) (Result, error) {
	return h.applyWebsite(websiteID, "", domain, docroot, runtime, "", "", true, true, false, 0, nil)
}

type diskStat struct{ total, used, itotal, iused uint64 }

func mustRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
