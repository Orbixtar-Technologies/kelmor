package operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

const vhostPolicyDir = "/var/lib/panel/vhost-policy"

type VhostRedirect struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Status   int    `json:"status"`
	Wildcard bool   `json:"wildcard,omitempty"`
}

type VhostHotlink struct {
	Enabled         bool     `json:"enabled"`
	AllowDirect     bool     `json:"allow_direct"`
	Extensions      []string `json:"extensions"`
	AllowedReferers []string `json:"allowed_referers"`
}

type VhostPolicy struct {
	WebsiteID string          `json:"website_id"`
	Redirects []VhostRedirect `json:"redirects"`
	Hotlink   VhostHotlink    `json:"hotlink"`
}

func defaultVhostPolicy(websiteID string) VhostPolicy {
	return VhostPolicy{
		WebsiteID: websiteID,
		Redirects: []VhostRedirect{},
		Hotlink: VhostHotlink{
			Enabled:     false,
			AllowDirect: true,
			Extensions:  []string{"jpg", "jpeg", "png", "gif", "webp", "svg"},
		},
	}
}

func vhostPolicyPath(websiteID string) (string, error) {
	id := strings.TrimSpace(websiteID)
	if id == "" || strings.ContainsAny(id, "/\\.;|&$`\n") {
		return "", fmt.Errorf("invalid website id")
	}
	return vhostPolicyDir + "/" + id + ".json", nil
}

func (h *Host) readVhostPolicy(websiteID string) (VhostPolicy, error) {
	policy := defaultVhostPolicy(websiteID)
	path, err := vhostPolicyPath(websiteID)
	if err != nil {
		return policy, err
	}
	abs, err := h.resolve(path)
	if err != nil {
		return policy, nil
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return policy, nil
		}
		return policy, err
	}
	if err := json.Unmarshal(body, &policy); err != nil {
		return defaultVhostPolicy(websiteID), nil
	}
	policy.WebsiteID = websiteID
	if policy.Redirects == nil {
		policy.Redirects = []VhostRedirect{}
	}
	return policy, nil
}

func (h *Host) writeVhostPolicy(policy VhostPolicy) (VhostPolicy, error) {
	path, err := vhostPolicyPath(policy.WebsiteID)
	if err != nil {
		return VhostPolicy{}, err
	}
	if policy.Redirects == nil {
		policy.Redirects = []VhostRedirect{}
	}
	body, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return VhostPolicy{}, err
	}
	if _, err := h.CreateDirectoryTree(vhostPolicyDir, 0o750); err != nil {
		return VhostPolicy{}, err
	}
	if err := h.writeManaged(path, body, 0o640); err != nil {
		return VhostPolicy{}, err
	}
	return policy, nil
}

func (h *Host) applyLoadedVhostPolicy(spec *configuration.WebsiteSpec) {
	if spec == nil {
		return
	}
	policy, err := h.readVhostPolicy(spec.WebsiteID)
	if err != nil {
		return
	}
	for _, item := range policy.Redirects {
		spec.Redirects = append(spec.Redirects, configuration.PathRedirect{
			Source: item.Source, Target: item.Target, Status: item.Status, Wildcard: item.Wildcard,
		})
	}
	spec.Hotlink = &configuration.HotlinkPolicy{
		Enabled: policy.Hotlink.Enabled, AllowDirect: policy.Hotlink.AllowDirect,
		Extensions: policy.Hotlink.Extensions, AllowedReferers: policy.Hotlink.AllowedReferers,
	}
}

type GitRepo struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type ImageFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (h *Host) listGitRepos(home string) ([]GitRepo, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		return []GitRepo{}, fmt.Errorf("home required")
	}
	if err := validate.Username(filepath.Base(strings.TrimSuffix(home, "/"))); err != nil {
		return []GitRepo{}, err
	}
	abs, err := h.resolve(home)
	if err != nil {
		return []GitRepo{}, nil
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return []GitRepo{}, nil
	}
	var out []GitRepo
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == ".cache") {
			return filepath.SkipDir
		}
		if d.IsDir() && depth > 5 {
			return filepath.SkipDir
		}
		if d.IsDir() && d.Name() == ".git" {
			parent := filepath.Dir(rel)
			if parent == "." {
				parent = "/"
			} else {
				parent = "/" + filepath.ToSlash(parent)
			}
			out = append(out, GitRepo{Path: parent, Name: filepath.Base(parent)})
			return filepath.SkipDir
		}
		if len(out) >= 100 {
			return filepath.SkipAll
		}
		return nil
	})
	if out == nil {
		out = []GitRepo{}
	}
	return out, nil
}

func (h *Host) listImages(root string) ([]ImageFile, error) {
	if root == "" {
		return []ImageFile{}, nil
	}
	abs, err := h.resolve(root)
	if err != nil {
		return []ImageFile{}, nil
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return []ImageFile{}, nil
	}
	var out []ImageFile
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isImageName(d.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		size := int64(0)
		if st, err := d.Info(); err == nil {
			size = st.Size()
		}
		out = append(out, ImageFile{
			Path: "/" + filepath.ToSlash(rel),
			Name: d.Name(),
			Size: size,
		})
		if len(out) >= 200 {
			return filepath.SkipAll
		}
		return nil
	})
	if out == nil {
		out = []ImageFile{}
	}
	return out, nil
}

func isImageName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico":
		return true
	default:
		return false
	}
}

type MailQueueResult struct {
	Items   []MailQueueEntry `json:"items"`
	Source  string           `json:"source,omitempty"`
	Partial bool             `json:"partial,omitempty"`
	Message string           `json:"message,omitempty"`
}

func (h *Host) listMailQueue() MailQueueResult {
	items, source, err := h.readMailQueue()
	if items == nil {
		items = []MailQueueEntry{}
	}
	result := MailQueueResult{Items: items, Source: source}
	if err != nil {
		result.Partial = true
		result.Message = "Could not read the live Postfix queue. Showing an empty list."
		return result
	}
	return result
}
