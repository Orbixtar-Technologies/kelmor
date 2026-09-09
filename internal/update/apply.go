package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type snapshotEntry struct {
	Target string `json:"target"`
	Exists bool   `json:"exists"`
}

type snapshotMetadata struct {
	Targets              []snapshotEntry `json:"targets"`
	CurrentReleaseExists bool            `json:"current_release_exists"`
}

func Apply(bundleDir, installRoot string, pub ed25519.PublicKey) error {
	m, err := Load(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		return err
	}
	if err := Verify(m, pub); err != nil {
		return err
	}
	if err := VerifyFileHashes(m, bundleDir); err != nil {
		return err
	}
	relDir := filepath.Join(installRoot, "releases", m.Release)
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		return err
	}
	binDir := filepath.Join(installRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	rbDir := filepath.Join(installRoot, "rollback")
	_ = os.RemoveAll(rbDir)
	if err := os.MkdirAll(rbDir, 0o755); err != nil {
		return err
	}
	for rel := range m.Files {
		if strings.Contains(rel, "..") {
			return fmt.Errorf("illegal path %s", rel)
		}
		src := filepath.Join(bundleDir, rel)
		if err := copyFile(src, filepath.Join(relDir, rel)); err != nil {
			return err
		}
		cur := filepath.Join(binDir, filepath.Base(rel))
		if _, err := os.Stat(cur); err == nil {
			if err := copyFile(cur, filepath.Join(rbDir, filepath.Base(rel))); err != nil {
				return err
			}
		}
		if err := copyFile(src, cur); err != nil {
			_ = Rollback(installRoot)
			return err
		}
	}
	return os.WriteFile(filepath.Join(installRoot, "current-release"), []byte(m.Release+"\n"), 0o644)
}

func Rollback(installRoot string) error {
	rbDir := filepath.Join(installRoot, "rollback")
	metadataPath := filepath.Join(rbDir, "metadata.json")
	if raw, err := os.ReadFile(metadataPath); err == nil {
		var metadata snapshotMetadata
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return fmt.Errorf("invalid rollback metadata: %w", err)
		}
		for _, entry := range metadata.Targets {
			target := filepath.Join(installRoot, filepath.FromSlash(entry.Target))
			if !entry.Exists {
				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					return err
				}
				continue
			}
			source := filepath.Join(rbDir, "targets", filepath.FromSlash(entry.Target))
			info, err := os.Stat(source)
			if err != nil {
				return err
			}
			if err := copyFileWithMode(source, target, info.Mode().Perm()); err != nil {
				return err
			}
		}
		currentRelease := filepath.Join(installRoot, "current-release")
		if metadata.CurrentReleaseExists {
			info, err := os.Stat(filepath.Join(rbDir, "current-release"))
			if err != nil {
				return err
			}
			return copyFileWithMode(filepath.Join(rbDir, "current-release"), currentRelease, info.Mode().Perm())
		}
		if err := os.Remove(currentRelease); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	binDir := filepath.Join(installRoot, "bin")
	entries, err := os.ReadDir(rbDir)
	if err != nil {
		return fmt.Errorf("no rollback snapshot: %w", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(rbDir, e.Name()), filepath.Join(binDir, e.Name())); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(installRoot, "current-release"), []byte("rolled-back\n"), 0o644)
}

func Install(ctx context.Context, config Config, runner Runner) (*Status, error) {
	status := Status{
		State: "error", InstalledRelease: config.InstalledRelease,
		Automatic: config.Automatic, Channel: config.Channel,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	unlock, err := acquireInstallLock(config)
	if err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	defer unlock()
	if config.InstallRoot == "" {
		return finishStatus(config.StatusPath, status, fmt.Errorf("install root is required"))
	}
	if runner == nil {
		return finishStatus(config.StatusPath, status, fmt.Errorf("update runner is required"))
	}

	manifest, err := fetchManifest(ctx, config)
	if err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := validateManifest(manifest, config); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	status.AvailableRelease = manifest.Release

	staging, err := os.MkdirTemp(config.InstallRoot, ".update-staging-")
	if err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o700); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := downloadArtifacts(ctx, config, manifest, staging); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := createSnapshot(config.InstallRoot, manifest.Artifacts); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := installStagedArtifacts(config.InstallRoot, staging, manifest.Artifacts); err != nil {
		return rollbackFailedInstall(ctx, config, runner, status, err)
	}
	if err := runActivationAndHealthChecks(ctx, runner); err != nil {
		return rollbackFailedInstall(ctx, config, runner, status, err)
	}
	if err := writeCurrentRelease(config.InstallRoot, manifest.Release); err != nil {
		return rollbackFailedInstall(ctx, config, runner, status, err)
	}

	status.State = "installed"
	status.InstalledRelease = manifest.Release
	status.Error = ""
	return finishStatus(config.StatusPath, status, nil)
}

func installLockPath(config Config) string {
	if config.StatusPath != "" {
		return filepath.Join(filepath.Dir(config.StatusPath), "update.lock")
	}
	return filepath.Join(config.InstallRoot, ".update.lock")
}

func acquireInstallLock(config Config) (func(), error) {
	path := installLockPath(config)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil, fmt.Errorf("update operation already in progress")
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() {
		_ = os.Remove(path)
	}, nil
}

func downloadArtifacts(ctx context.Context, config Config, manifest *Manifest, staging string) error {
	base, err := validateFeedURL(config.FeedURL)
	if err != nil {
		return err
	}
	client := *http.DefaultClient
	client.CheckRedirect = sameHostRedirectPolicy(base)
	for _, artifact := range manifest.Artifacts {
		artifactURL, err := url.JoinPath(base.String(), config.Channel, manifest.Release, artifact.Path)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("download artifact %q: %w", artifact.Path, err)
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return fmt.Errorf("download artifact %q: HTTP %d", artifact.Path, response.StatusCode)
		}
		target := filepath.Join(staging, filepath.FromSlash(artifact.Target))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			_ = response.Body.Close()
			return err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(artifact.Mode))
		if err != nil {
			_ = response.Body.Close()
			return err
		}
		hash := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, artifact.Size+1))
		closeErr := file.Close()
		bodyCloseErr := response.Body.Close()
		switch {
		case copyErr != nil:
			return fmt.Errorf("download artifact %q: %w", artifact.Path, copyErr)
		case closeErr != nil:
			return closeErr
		case bodyCloseErr != nil:
			return bodyCloseErr
		case written != artifact.Size:
			return fmt.Errorf("artifact %q size mismatch: got %d, want %d", artifact.Path, written, artifact.Size)
		case !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), artifact.SHA256):
			return fmt.Errorf("artifact %q hash mismatch", artifact.Path)
		}
	}
	return nil
}

func sameHostRedirectPolicy(base *url.URL) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if request.URL.Scheme != "https" || request.URL.Host != base.Host {
			return fmt.Errorf("redirect to a different scheme or host is refused")
		}
		return nil
	}
}

func createSnapshot(root string, artifacts []Artifact) error {
	rollback := filepath.Join(root, "rollback")
	if err := os.RemoveAll(rollback); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rollback, "targets"), 0o700); err != nil {
		return err
	}
	metadata := snapshotMetadata{Targets: make([]snapshotEntry, 0, len(artifacts))}
	for _, artifact := range artifacts {
		if err := rejectSymlinks(root, artifact.Target); err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(artifact.Target))
		info, err := os.Lstat(target)
		if os.IsNotExist(err) {
			metadata.Targets = append(metadata.Targets, snapshotEntry{Target: artifact.Target})
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("target %q is not a regular file", artifact.Target)
		}
		metadata.Targets = append(metadata.Targets, snapshotEntry{Target: artifact.Target, Exists: true})
		snapshot := filepath.Join(rollback, "targets", filepath.FromSlash(artifact.Target))
		if err := copyFileWithMode(target, snapshot, info.Mode().Perm()); err != nil {
			return err
		}
	}
	currentRelease := filepath.Join(root, "current-release")
	if info, err := os.Stat(currentRelease); err == nil {
		metadata.CurrentReleaseExists = true
		if err := copyFileWithMode(currentRelease, filepath.Join(rollback, "current-release"), info.Mode().Perm()); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rollback, "metadata.json"), raw, 0o600)
}

func installStagedArtifacts(root, staging string, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		if err := rejectSymlinks(root, artifact.Target); err != nil {
			return err
		}
		source := filepath.Join(staging, filepath.FromSlash(artifact.Target))
		target := filepath.Join(root, filepath.FromSlash(artifact.Target))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Chmod(source, os.FileMode(artifact.Mode)); err != nil {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	return nil
}

func rejectSymlinks(root, target string) error {
	current := root
	for _, component := range strings.Split(filepath.FromSlash(target), string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("target %q contains a symlink", target)
		}
	}
	return nil
}

func runActivationAndHealthChecks(ctx context.Context, runner Runner) error {
	commands := [][]string{
		{"/bin/systemctl", "daemon-reload"},
		{"/bin/systemctl", "restart", "panel-api", "panel-worker", "panel-agent", "nginx"},
		{"/bin/systemctl", "is-active", "panel-api", "panel-worker", "panel-agent"},
		{"/usr/bin/curl", "-fsS", "http://127.0.0.1:18080/healthz"},
		{"/usr/bin/curl", "-kfsS", "https://127.0.0.1:8443/"},
		{"/usr/bin/curl", "-kfsS", "https://127.0.0.1:8444/"},
		{"/usr/bin/curl", "-fsS", "-H", "X-API-Key: panel-loopback", "http://127.0.0.1:8081/api/v1/servers/localhost"},
	}
	for index, command := range commands {
		if err := runner.Run(ctx, command[0], command[1:]...); err != nil {
			if index >= 2 {
				return fmt.Errorf("health check failed: %w", err)
			}
			return fmt.Errorf("service activation failed: %w", err)
		}
	}
	return nil
}

func writeCurrentRelease(root, release string) error {
	path := filepath.Join(root, "current-release")
	tmp := path + ".staging"
	if err := os.WriteFile(tmp, []byte(release+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func rollbackFailedInstall(
	ctx context.Context,
	config Config,
	runner Runner,
	status Status,
	installErr error,
) (*Status, error) {
	rollbackErr := Rollback(config.InstallRoot)
	if rollbackErr == nil {
		rollbackErr = runActivationAndHealthChecks(ctx, runner)
	}
	if rollbackErr != nil {
		status.State = "critical"
		status.Error = fmt.Sprintf("%v; rollback failed: %v", installErr, rollbackErr)
		if config.StatusPath != "" {
			if err := WriteStatus(config.StatusPath, status); err != nil {
				return &status, fmt.Errorf("%s; write status: %v", status.Error, err)
			}
		}
		return &status, fmt.Errorf("%s", status.Error)
	}
	status.State = "rolled-back"
	status.Error = installErr.Error()
	if config.StatusPath != "" {
		if err := WriteStatus(config.StatusPath, status); err != nil {
			return &status, fmt.Errorf("%w; write status: %v", installErr, err)
		}
	}
	return &status, installErr
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".staging"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func copyFileWithMode(src, dest string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, input); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dest)
}
