package update

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	maxManifestSize = 1 << 20
	maxArtifactSize = 256 << 20
	maxDownloadSize = 1 << 30
)

type Config struct {
	FeedURL          string
	Channel          string
	InstalledRelease string
	PublicKey        ed25519.PublicKey
	InstallRoot      string
	StatusPath       string
	Automatic        bool
}

func Check(ctx context.Context, config Config) (*Status, error) {
	status := Status{
		State: "error", InstalledRelease: config.InstalledRelease,
		Automatic: config.Automatic, Channel: config.Channel,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	manifest, err := fetchManifest(ctx, config)
	if err != nil {
		return finishStatus(config.StatusPath, status, err)
	}
	if err := validateManifest(manifest, config); err != nil {
		return finishStatus(config.StatusPath, status, err)
	}

	status.State = "available"
	status.AvailableRelease = manifest.Release
	return finishStatus(config.StatusPath, status, nil)
}

func fetchManifest(ctx context.Context, config Config) (*Manifest, error) {
	base, err := validateFeedURL(config.FeedURL)
	if err != nil {
		return nil, err
	}
	if !validRelativePath(config.Channel) || strings.Contains(config.Channel, "/") {
		return nil, fmt.Errorf("invalid update channel")
	}
	manifestURL, err := url.JoinPath(base.String(), config.Channel, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("build manifest URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	client := *http.DefaultClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "https" || req.URL.Host != base.Host {
			return fmt.Errorf("redirect to a different scheme or host is refused")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch manifest: HTTP %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, maxManifestSize+1)
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if len(raw) > maxManifestSize {
		return nil, fmt.Errorf("manifest exceeds size limit")
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("decode manifest: trailing data")
	}
	return &manifest, nil
}

func validateManifest(manifest *Manifest, config Config) error {
	if len(config.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid pinned public key")
	}
	if err := Verify(manifest, config.PublicKey); err != nil {
		return err
	}
	if manifest.Channel != config.Channel {
		return fmt.Errorf("manifest channel %q does not match configured channel %q", manifest.Channel, config.Channel)
	}
	available, err := parseSemanticVersion(manifest.Release)
	if err != nil {
		return fmt.Errorf("invalid release: %w", err)
	}
	installed, err := parseSemanticVersion(config.InstalledRelease)
	if err != nil {
		return fmt.Errorf("invalid installed release: %w", err)
	}
	if compareVersions(available, installed) <= 0 {
		return fmt.Errorf("release %s is not newer than installed release %s", manifest.Release, config.InstalledRelease)
	}
	minimum, err := parseSemanticVersion(manifest.MinimumRelease)
	if err != nil {
		return fmt.Errorf("invalid minimum release: %w", err)
	}
	if compareVersions(installed, minimum) < 0 {
		return fmt.Errorf("installed release %s is below compatible minimum %s", config.InstalledRelease, manifest.MinimumRelease)
	}
	return validateArtifacts(manifest.Artifacts)
}

func validateArtifacts(artifacts []Artifact) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("manifest contains no artifacts")
	}
	paths := make(map[string]struct{}, len(artifacts))
	targets := make(map[string]struct{}, len(artifacts))
	var total int64
	for _, artifact := range artifacts {
		if !validRelativePath(artifact.Path) {
			return fmt.Errorf("invalid artifact path %q", artifact.Path)
		}
		if !validTarget(artifact.Target) {
			return fmt.Errorf("invalid artifact target %q", artifact.Target)
		}
		if _, exists := paths[artifact.Path]; exists {
			return fmt.Errorf("duplicate artifact path %q", artifact.Path)
		}
		paths[artifact.Path] = struct{}{}
		if _, exists := targets[artifact.Target]; exists {
			return fmt.Errorf("duplicate artifact target %q", artifact.Target)
		}
		targets[artifact.Target] = struct{}{}
		if artifact.Size < 0 || artifact.Size > maxArtifactSize {
			return fmt.Errorf("artifact %q size is outside safety limits", artifact.Path)
		}
		if total > maxDownloadSize-artifact.Size {
			return fmt.Errorf("total artifact size exceeds safety limit")
		}
		total += artifact.Size
		hash, err := hex.DecodeString(artifact.SHA256)
		if err != nil || len(hash) != 32 {
			return fmt.Errorf("invalid SHA-256 for artifact %q", artifact.Path)
		}
		if artifact.Mode == 0 || artifact.Mode&^uint32(0o777) != 0 {
			return fmt.Errorf("invalid mode for artifact %q", artifact.Path)
		}
	}
	return nil
}

func validateFeedURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid feed URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("update feed must use HTTPS")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("invalid HTTPS feed URL")
	}
	return parsed, nil
}

func validRelativePath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) {
		return false
	}
	unescaped, err := url.PathUnescape(value)
	if err != nil || unescaped != value {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func validTarget(target string) bool {
	if !validRelativePath(target) {
		return false
	}
	return strings.HasPrefix(target, "bin/") ||
		strings.HasPrefix(target, "share/portals/account/") ||
		strings.HasPrefix(target, "share/portals/server/")
}

func finishStatus(path string, status Status, operationErr error) (*Status, error) {
	if operationErr != nil {
		status.State = "error"
		status.Error = operationErr.Error()
	}
	if path != "" {
		if err := WriteStatus(path, status); err != nil {
			if operationErr != nil {
				return &status, fmt.Errorf("%w; write status: %v", operationErr, err)
			}
			return &status, fmt.Errorf("write status: %w", err)
		}
	}
	return &status, operationErr
}

type semanticVersion struct {
	core       [3]int64
	prerelease []string
}

func parseSemanticVersion(raw string) (semanticVersion, error) {
	var version semanticVersion
	raw = strings.TrimPrefix(raw, "v")
	if build := strings.IndexByte(raw, '+'); build >= 0 {
		if build == len(raw)-1 {
			return version, fmt.Errorf("empty build metadata")
		}
		raw = raw[:build]
	}
	if pre := strings.IndexByte(raw, '-'); pre >= 0 {
		if pre == len(raw)-1 {
			return version, fmt.Errorf("empty prerelease")
		}
		version.prerelease = strings.Split(raw[pre+1:], ".")
		raw = raw[:pre]
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return version, fmt.Errorf("%q is not major.minor.patch", raw)
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return version, fmt.Errorf("invalid numeric component %q", part)
		}
		number, err := strconv.ParseInt(part, 10, 64)
		if err != nil || number < 0 {
			return version, fmt.Errorf("invalid numeric component %q", part)
		}
		version.core[i] = number
	}
	for _, identifier := range version.prerelease {
		if identifier == "" {
			return version, fmt.Errorf("empty prerelease identifier")
		}
		for _, char := range identifier {
			if (char < '0' || char > '9') && (char < 'A' || char > 'Z') &&
				(char < 'a' || char > 'z') && char != '-' {
				return version, fmt.Errorf("invalid prerelease identifier %q", identifier)
			}
		}
	}
	return version, nil
}

func compareVersions(left, right semanticVersion) int {
	for i := range left.core {
		if left.core[i] < right.core[i] {
			return -1
		}
		if left.core[i] > right.core[i] {
			return 1
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) == 0 {
		return 0
	}
	if len(left.prerelease) == 0 {
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	for i := 0; i < len(left.prerelease) && i < len(right.prerelease); i++ {
		if comparison := comparePrereleaseIdentifier(left.prerelease[i], right.prerelease[i]); comparison != 0 {
			return comparison
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumber, leftErr := strconv.ParseUint(left, 10, 64)
	rightNumber, rightErr := strconv.ParseUint(right, 10, 64)
	switch {
	case leftErr == nil && rightErr == nil:
		if leftNumber < rightNumber {
			return -1
		}
		if leftNumber > rightNumber {
			return 1
		}
		return 0
	case leftErr == nil:
		return -1
	case rightErr == nil:
		return 1
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
