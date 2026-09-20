package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func LocalFeedRoot(installRoot string) string {
	if strings.TrimSpace(installRoot) == "" {
		installRoot = DefaultInstallRoot
	}
	return filepath.Join(installRoot, "share", "updates")
}

func LocalManifestPath(installRoot, channel string) string {
	channel = strings.TrimSpace(channel)
	if !validRelativePath(channel) || strings.Contains(channel, "/") {
		return ""
	}
	return filepath.Join(LocalFeedRoot(installRoot), channel, "manifest.json")
}

func WriteSignedCurrentReleaseFeed(destRoot, channel, release string, priv ed25519.PrivateKey) error {
	if !validRelativePath(channel) || strings.Contains(channel, "/") {
		return fmt.Errorf("invalid update channel")
	}
	if _, err := parseSemanticVersion(release); err != nil {
		return fmt.Errorf("invalid release: %w", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid signing key")
	}
	payload := []byte(strings.TrimSpace(release) + "\n")
	sum := sha256.Sum256(payload)
	manifest := &Manifest{
		Release:        strings.TrimSpace(release),
		Channel:        channel,
		MinimumRelease: "0.1.0",
		Artifacts: []Artifact{{
			Path:   "current-release",
			Target: "bin/panel-api",
			Size:   int64(len(payload)),
			SHA256: hex.EncodeToString(sum[:]),
			Mode:   0o755,
			Kind:   ArtifactRuntime,
		}},
	}
	if err := Sign(manifest, priv); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	channelDir := filepath.Join(destRoot, channel)
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(channelDir, "current-release"), payload, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(channelDir, "manifest.json"), raw, 0o644)
}

func CopyFeedTree(src, dest string) error {
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("feed source must be a directory")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func readLocalManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeManifestBytes(raw)
}
