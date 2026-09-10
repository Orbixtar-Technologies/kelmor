package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	transactionDirectory = ".update-transaction"
	stagingDirectory     = ".update-staging"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type snapshotEntry struct {
	Target string `json:"target"`
	Exists bool   `json:"exists"`
}

type snapshotMetadata struct {
	State                string          `json:"state"`
	Targets              []snapshotEntry `json:"targets"`
	CurrentReleaseExists bool            `json:"current_release_exists"`
}

type operationLock struct {
	root *secureRoot
	file *os.File
}

func acquireInstallLock(installRoot string) (*operationLock, error) {
	return acquireOperationLock(installRoot, unix.LOCK_EX|unix.LOCK_NB)
}

func acquireOperationLock(installRoot string, flags int) (*operationLock, error) {
	root, err := openSecureRoot(installRoot, "install root")
	if err != nil {
		return nil, err
	}
	file, err := root.open(".update.lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), flags); err != nil {
		_ = file.Close()
		_ = root.Close()
		if flags&unix.LOCK_NB != 0 && errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("update operation already in progress")
		}
		return nil, err
	}
	return &operationLock{root: root, file: file}, nil
}

// WithInstallOperationLock runs operation while holding the updater's
// canonical install-root advisory lock. The callback must not call another
// updater operation that acquires the same lock.
func WithInstallOperationLock(installRoot string, operation func() error) (err error) {
	if operation == nil {
		return fmt.Errorf("update operation is required")
	}
	lock, err := acquireOperationLock(installRoot, unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, lock.Close())
	}()
	return operation()
}

func (lock *operationLock) Close() error {
	unlockErr := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	fileErr := lock.file.Close()
	rootErr := lock.root.Close()
	return errors.Join(unlockErr, fileErr, rootErr)
}

func Apply(bundleDir, installRoot string, publicKey ed25519.PublicKey) error {
	if err := createSecureRoot(installRoot, "install root"); err != nil {
		return err
	}
	lock, err := acquireInstallLock(installRoot)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := recoverInterruptedTransaction(lock.root, nil, false); err != nil {
		return fmt.Errorf("recover interrupted update: %w", err)
	}
	if err := lock.root.removeTree(stagingDirectory); err != nil {
		return err
	}
	bundle, err := openSecureRoot(bundleDir, "bundle root")
	if err != nil {
		return err
	}
	defer bundle.Close()

	raw, err := bundle.readFile("manifest.json", maxManifestSize)
	if err != nil {
		return err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if err := Verify(&manifest, publicKey); err != nil {
		return err
	}
	if !validRelativePath(manifest.Release) || strings.Contains(manifest.Release, "/") {
		return fmt.Errorf("invalid release path")
	}
	artifacts, err := openVerifiedLegacyBundle(bundle, &manifest)
	if err != nil {
		return err
	}
	defer closeFiles(artifacts)
	return applyLegacyLocked(lock.root, artifacts, &manifest)
}

func openVerifiedLegacyBundle(bundle *secureRoot, manifest *Manifest) (map[string]*os.File, error) {
	artifacts := make(map[string]*os.File, len(manifest.Files))
	for relative, want := range manifest.Files {
		if !validRelativePath(relative) {
			closeFiles(artifacts)
			return nil, fmt.Errorf("illegal path %s", relative)
		}
		file, err := bundle.open(relative, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
		if err != nil {
			closeFiles(artifacts)
			if errors.Is(err, unix.ELOOP) {
				return nil, fmt.Errorf("artifact path %q is a symlink: %w", relative, err)
			}
			return nil, err
		}
		info, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			closeFiles(artifacts)
			return nil, statErr
		}
		if !info.Mode().IsRegular() {
			_ = file.Close()
			closeFiles(artifacts)
			return nil, fmt.Errorf("artifact %q is not a regular file", relative)
		}
		sum := sha256.New()
		_, copyErr := io.Copy(sum, file)
		if copyErr != nil {
			_ = file.Close()
			closeFiles(artifacts)
			return nil, copyErr
		}
		if !strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), want) {
			_ = file.Close()
			closeFiles(artifacts)
			return nil, fmt.Errorf("hash mismatch for %s", relative)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = file.Close()
			closeFiles(artifacts)
			return nil, err
		}
		artifacts[relative] = file
	}
	return artifacts, nil
}

func closeFiles(files map[string]*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func applyLegacyLocked(root *secureRoot, artifacts map[string]*os.File, manifest *Manifest) error {
	if err := root.removeTree("rollback"); err != nil {
		return err
	}
	if err := root.mkdirAll("rollback", 0o700); err != nil {
		return err
	}
	if err := root.mkdirAll(stagingDirectory, 0o700); err != nil {
		return err
	}
	transactionArtifacts := make([]Artifact, 0, len(manifest.Files))
	for relative := range manifest.Files {
		source := artifacts[relative]
		releaseTarget := path.Join("releases", manifest.Release, relative)
		if err := root.copyReaderAtomic(source, releaseTarget, 0o755); err != nil {
			return err
		}
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return err
		}
		target := path.Join("bin", path.Base(relative))
		if err := root.copyReaderAtomicWithDirMode(
			source,
			path.Join(stagingDirectory, target),
			0o755,
			0o700,
		); err != nil {
			return err
		}
		transactionArtifacts = append(transactionArtifacts, Artifact{Target: target})
		if info, err := root.stat(target); err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("target %q is not a regular file", target)
			}
			if err := root.copyAtomic(target, path.Join("rollback", path.Base(relative)), uint32(info.Mode().Perm())); err != nil {
				return err
			}
		} else if !errors.Is(err, unix.ENOENT) {
			return err
		}
	}
	metadata, err := beginTransaction(root, transactionArtifacts)
	if err != nil {
		_ = cleanupTransaction(root)
		return err
	}
	metadata.State = "applying"
	if err := writeTransactionMetadata(root, metadata); err != nil {
		return rollbackLegacyTransaction(root, err)
	}
	if err := installStagedArtifacts(root, transactionArtifacts); err != nil {
		return rollbackLegacyTransaction(root, err)
	}
	if err := root.writeAtomic("current-release", []byte(manifest.Release+"\n"), 0o644); err != nil {
		return rollbackLegacyTransaction(root, err)
	}
	metadata.State = "committed"
	if err := writeTransactionMetadata(root, metadata); err != nil {
		return rollbackLegacyTransaction(root, err)
	}
	return cleanupTransaction(root)
}

func rollbackLegacyTransaction(root *secureRoot, applyErr error) error {
	if rollbackErr := rollbackTransaction(root); rollbackErr != nil {
		return fmt.Errorf("%w; rollback failed: %v", applyErr, rollbackErr)
	}
	if cleanupErr := cleanupTransaction(root); cleanupErr != nil {
		return fmt.Errorf("%w; rollback cleanup failed: %v", applyErr, cleanupErr)
	}
	return applyErr
}

func Rollback(installRoot string) error {
	lock, err := acquireInstallLock(installRoot)
	if err != nil {
		return err
	}
	defer lock.Close()
	if exists, err := transactionExists(lock.root); err != nil {
		return err
	} else if exists {
		if err := rollbackTransaction(lock.root); err != nil {
			return err
		}
		return cleanupTransaction(lock.root)
	}
	return rollbackLegacyLocked(lock.root)
}

func rollbackLegacyLocked(root *secureRoot) error {
	directory, err := root.open("rollback", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("no rollback snapshot: %w", err)
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := root.copyAtomic(
			path.Join("rollback", entry.Name()),
			path.Join("bin", entry.Name()),
			uint32(info.Mode().Perm()),
		); err != nil {
			return err
		}
	}
	return root.writeAtomic("current-release", []byte("rolled-back\n"), 0o644)
}

func Install(ctx context.Context, config Config, runner Runner) (*Status, error) {
	status := Status{
		State: "error", InstalledRelease: config.InstalledRelease,
		Automatic: config.Automatic, Channel: config.Channel,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if config.InstallRoot == "" {
		return finishStatus(config.StatusPath, status, fmt.Errorf("install root is required"))
	}
	if runner == nil {
		return finishStatus(config.StatusPath, status, fmt.Errorf("update runner is required"))
	}
	lock, err := acquireInstallLock(config.InstallRoot)
	if err != nil {
		return &status, err
	}
	defer lock.Close()
	return installWithOperationLock(ctx, config, runner, lock)
}

func installWithOperationLock(
	ctx context.Context,
	config Config,
	runner Runner,
	lock *operationLock,
) (*Status, error) {
	status := Status{
		State: "error", InstalledRelease: config.InstalledRelease,
		Automatic: config.Automatic, Channel: config.Channel,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if runner == nil {
		return finishStatus(config.StatusPath, status, fmt.Errorf("update runner is required"))
	}
	if err := recoverInterruptedTransaction(lock.root, runner, true); err != nil {
		return finishStatus(config.StatusPath, status, fmt.Errorf("recover interrupted update: %w", err))
	}
	if err := reloadInstalledRelease(lock.root, &config); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	status.InstalledRelease = config.InstalledRelease
	if err := lock.root.removeTree(stagingDirectory); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}

	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	manifest, err := fetchManifest(ctx, config)
	if err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := validateManifest(manifest, config); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	status.AvailableRelease = manifest.Release
	if err := lock.root.mkdirAll(stagingDirectory, 0o700); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := downloadArtifacts(ctx, config, manifest, lock.root); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	metadata, err := beginTransaction(lock.root, manifest.Artifacts)
	if err != nil {
		_ = cleanupTransaction(lock.root)
		return finishStatus(config.StatusPath, status, err)
	}
	metadata.State = "applying"
	if err := writeTransactionMetadata(lock.root, metadata); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, err)
	}
	if err := installStagedArtifacts(lock.root, manifest.Artifacts); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, err)
	}
	if err := runActivationAndHealthChecks(ctx, runner); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, err)
	}
	if err := lock.root.writeAtomic("current-release", []byte(manifest.Release+"\n"), 0o644); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, err)
	}
	metadata.State = "activated"
	if err := writeTransactionMetadata(lock.root, metadata); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, err)
	}

	status.State = "installed"
	status.InstalledRelease = manifest.Release
	status.Error = ""
	if config.StatusPath != "" {
		if err := WriteStatus(config.StatusPath, status); err != nil {
			return rollbackFailedInstall(config, runner, lock.root, status, fmt.Errorf("persist success status: %w", err))
		}
	}
	metadata.State = "committed"
	if err := writeTransactionMetadata(lock.root, metadata); err != nil {
		return rollbackFailedInstall(config, runner, lock.root, status, fmt.Errorf("commit update journal: %w", err))
	}
	_ = cleanupTransaction(lock.root)
	return &status, nil
}

func reloadInstalledRelease(root *secureRoot, config *Config) error {
	raw, err := root.readFile("current-release", 1024)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read installed release: %w", err)
	}
	release := strings.TrimSpace(string(raw))
	if release == "" {
		return fmt.Errorf("installed release marker is empty")
	}
	config.InstalledRelease = release
	return nil
}

func downloadArtifacts(ctx context.Context, config Config, manifest *Manifest, root *secureRoot) error {
	base, err := validateFeedURL(config.FeedURL)
	if err != nil {
		return err
	}
	client := *http.DefaultClient
	client.CheckRedirect = sameHostRedirectPolicy(base)
	if client.Timeout == 0 || client.Timeout > artifactRequestTimeout {
		client.Timeout = artifactRequestTimeout
	}
	for _, artifact := range manifest.Artifacts {
		artifactURL, err := url.JoinPath(base.String(), config.Channel, manifest.Release, artifact.Path)
		if err != nil {
			return err
		}
		requestContext, cancel := context.WithTimeout(ctx, artifactRequestTimeout)
		request, err := http.NewRequestWithContext(requestContext, http.MethodGet, artifactURL, nil)
		if err != nil {
			cancel()
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			cancel()
			return fmt.Errorf("download artifact %q: %w", artifact.Path, err)
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			cancel()
			return fmt.Errorf("download artifact %q: HTTP %d", artifact.Path, response.StatusCode)
		}
		reader := &hashingReader{
			reader: io.LimitReader(response.Body, artifact.Size+1),
			hash:   sha256.New(),
		}
		target := path.Join(stagingDirectory, artifact.Target)
		copyErr := root.copyReaderAtomicWithDirMode(reader, target, artifact.Mode, 0o700)
		bodyCloseErr := response.Body.Close()
		cancel()
		switch {
		case copyErr != nil:
			return fmt.Errorf("download artifact %q: %w", artifact.Path, copyErr)
		case bodyCloseErr != nil:
			return bodyCloseErr
		case reader.count != artifact.Size:
			return fmt.Errorf("artifact %q size mismatch: got %d, want %d", artifact.Path, reader.count, artifact.Size)
		case !strings.EqualFold(hex.EncodeToString(reader.hash.Sum(nil)), artifact.SHA256):
			return fmt.Errorf("artifact %q hash mismatch", artifact.Path)
		}
	}
	return nil
}

type hashingReader struct {
	reader io.Reader
	hash   hash.Hash
	count  int64
}

func (reader *hashingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	if count > 0 {
		_, _ = reader.hash.Write(buffer[:count])
		reader.count += int64(count)
	}
	return count, err
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

func beginTransaction(root *secureRoot, artifacts []Artifact) (snapshotMetadata, error) {
	if err := root.removeTree(transactionDirectory); err != nil {
		return snapshotMetadata{}, err
	}
	if err := root.mkdirAll(path.Join(transactionDirectory, "targets"), 0o700); err != nil {
		return snapshotMetadata{}, err
	}
	metadata := snapshotMetadata{
		State:   "prepared",
		Targets: make([]snapshotEntry, 0, len(artifacts)),
	}
	for _, artifact := range artifacts {
		info, err := root.stat(artifact.Target)
		if errors.Is(err, unix.ENOENT) {
			metadata.Targets = append(metadata.Targets, snapshotEntry{Target: artifact.Target})
			continue
		}
		if err != nil {
			return snapshotMetadata{}, err
		}
		if !info.Mode().IsRegular() {
			return snapshotMetadata{}, fmt.Errorf("target %q is not a regular file", artifact.Target)
		}
		metadata.Targets = append(metadata.Targets, snapshotEntry{Target: artifact.Target, Exists: true})
		if err := root.copyAtomicWithDirMode(
			artifact.Target,
			path.Join(transactionDirectory, "targets", artifact.Target),
			uint32(info.Mode().Perm()),
			0o700,
		); err != nil {
			return snapshotMetadata{}, err
		}
	}
	if info, err := root.stat("current-release"); err == nil {
		metadata.CurrentReleaseExists = true
		if err := root.copyAtomic(
			"current-release",
			path.Join(transactionDirectory, "current-release"),
			uint32(info.Mode().Perm()),
		); err != nil {
			return snapshotMetadata{}, err
		}
	} else if !errors.Is(err, unix.ENOENT) {
		return snapshotMetadata{}, err
	}
	if err := writeTransactionMetadata(root, metadata); err != nil {
		return snapshotMetadata{}, err
	}
	return metadata, nil
}

func writeTransactionMetadata(root *secureRoot, metadata snapshotMetadata) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return root.writeAtomic(path.Join(transactionDirectory, "metadata.json"), raw, 0o600)
}

func installStagedArtifacts(root *secureRoot, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		if err := root.rename(path.Join(stagingDirectory, artifact.Target), artifact.Target); err != nil {
			return err
		}
	}
	return nil
}

func transactionExists(root *secureRoot) (bool, error) {
	file, err := root.open(transactionDirectory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, file.Close()
}

func recoverInterruptedTransaction(root *secureRoot, runner Runner, requireServiceRecovery bool) error {
	exists, err := transactionExists(root)
	if err != nil || !exists {
		return err
	}
	raw, err := root.readFile(path.Join(transactionDirectory, "metadata.json"), maxManifestSize)
	if errors.Is(err, unix.ENOENT) {
		return cleanupTransaction(root)
	}
	if err != nil {
		return err
	}
	var metadata snapshotMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return err
	}
	if metadata.State == "committed" {
		return cleanupTransaction(root)
	}
	if runner == nil && requireServiceRecovery {
		return fmt.Errorf("recovery runner is required while an interrupted transaction is pending")
	}
	if err := rollbackTransactionWithMetadata(root, metadata); err != nil {
		return err
	}
	if runner != nil {
		recoveryContext, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
		defer cancel()
		if err := runActivationAndHealthChecks(recoveryContext, runner); err != nil {
			return err
		}
	}
	return cleanupTransaction(root)
}

func rollbackTransaction(root *secureRoot) error {
	raw, err := root.readFile(path.Join(transactionDirectory, "metadata.json"), maxManifestSize)
	if err != nil {
		return err
	}
	var metadata snapshotMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return err
	}
	return rollbackTransactionWithMetadata(root, metadata)
}

func rollbackTransactionWithMetadata(root *secureRoot, metadata snapshotMetadata) error {
	metadata.State = "rolling-back"
	if err := writeTransactionMetadata(root, metadata); err != nil {
		return err
	}
	for _, entry := range metadata.Targets {
		if !validTarget(entry.Target) {
			return fmt.Errorf("invalid rollback target %q", entry.Target)
		}
		if !entry.Exists {
			if err := root.remove(entry.Target); err != nil {
				return err
			}
			continue
		}
		snapshot := path.Join(transactionDirectory, "targets", entry.Target)
		info, err := root.stat(snapshot)
		if err != nil {
			return err
		}
		if err := root.copyAtomic(snapshot, entry.Target, uint32(info.Mode().Perm())); err != nil {
			return err
		}
	}
	if metadata.CurrentReleaseExists {
		info, err := root.stat(path.Join(transactionDirectory, "current-release"))
		if err != nil {
			return err
		}
		return root.copyAtomic(
			path.Join(transactionDirectory, "current-release"),
			"current-release",
			uint32(info.Mode().Perm()),
		)
	}
	return root.remove("current-release")
}

func cleanupTransaction(root *secureRoot) error {
	if err := root.removeTree(stagingDirectory); err != nil {
		return err
	}
	return root.removeTree(transactionDirectory)
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

func rollbackFailedInstall(
	config Config,
	runner Runner,
	root *secureRoot,
	status Status,
	installErr error,
) (*Status, error) {
	recoveryContext, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
	defer cancel()
	rollbackErr := rollbackTransaction(root)
	if rollbackErr == nil {
		rollbackErr = runActivationAndHealthChecks(recoveryContext, runner)
	}
	if rollbackErr == nil {
		rollbackErr = cleanupTransaction(root)
	}
	if rollbackErr != nil {
		status.State = "critical"
		status.Error = fmt.Sprintf("%v; rollback failed: %v", installErr, rollbackErr)
		if config.StatusPath != "" {
			_ = WriteStatus(config.StatusPath, status)
		}
		return &status, errors.New(status.Error)
	}
	status.State = "rolled-back"
	status.InstalledRelease = config.InstalledRelease
	status.Error = installErr.Error()
	if config.StatusPath != "" {
		if err := WriteStatus(config.StatusPath, status); err != nil {
			return &status, fmt.Errorf("%w; rollback succeeded but status persistence failed: %v", installErr, err)
		}
	}
	return &status, installErr
}
