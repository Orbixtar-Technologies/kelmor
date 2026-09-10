package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	stage := flag.String("stage", "", "release staging directory")
	release := flag.String("release", "0.2.0", "release version")
	channel := flag.String("channel", "stable", "release channel")
	privPath := flag.String("priv", "", "ed25519 private key hex file")
	feedRoot := flag.String("feed-root", "", "feed root containing channel/manifest.json")
	flag.Parse()
	if *stage == "" || *privPath == "" || *feedRoot == "" {
		flag.Usage()
		os.Exit(2)
	}
	privRaw, err := os.ReadFile(*privPath)
	if err != nil {
		fatal(err)
	}
	privBytes, err := hex.DecodeString(strings.TrimSpace(string(privRaw)))
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		fatal(fmt.Errorf("invalid private key"))
	}
	priv := ed25519.PrivateKey(privBytes)

	var artifacts []update.Artifact
	err = filepath.WalkDir(*stage, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(*stage, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "manifest.json" {
			return nil
		}
		target, ok := mapArtifactTarget(rel)
		if !ok {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, update.Artifact{
			Path: rel, Target: target, Size: int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]), Mode: uint32(info.Mode().Perm()),
		})
		return nil
	})
	if err != nil {
		fatal(err)
	}
	manifest := &update.Manifest{
		Release: *release, Channel: *channel, MinimumRelease: "0.1.0",
		Artifacts: artifacts,
	}
	if err := update.Sign(manifest, priv); err != nil {
		fatal(err)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(*stage, "manifest.json"), raw, 0o644); err != nil {
		fatal(err)
	}
	channelDir := filepath.Join(*feedRoot, *channel)
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(channelDir, "manifest.json"), raw, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("signed %d artifacts for %s\n", len(artifacts), *release)
}

func mapArtifactTarget(rel string) (string, bool) {
	switch {
	case strings.HasPrefix(rel, "bin/"):
		return rel, true
	case strings.HasPrefix(rel, "share/portals/server/"):
		return rel, true
	case strings.HasPrefix(rel, "share/portals/account/"):
		return rel, true
	default:
		return "", false
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
