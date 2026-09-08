package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

type Manifest struct {
	FormatVersion int            `json:"format_version"`
	AccountID     string         `json:"account_id"`
	Username      string         `json:"username"`
	CreatedAt     string         `json:"created_at"`
	PanelVersion  string         `json:"panel_version"`
	Kind          string         `json:"kind"`
	Files         map[string]any `json:"files"`
	Databases     []string       `json:"databases"`
	Mailboxes     []string       `json:"mailboxes"`
	Checksums     map[string]string `json:"checksums"`
}

func Build(ctx context.Context, box *secret.Box, repo Repository, acc *store.Account, dbs []store.HostedDatabase, mail []store.Mailbox, home string) (Manifest, string, error) {
	man := Manifest{
		FormatVersion: 1,
		AccountID:     acc.ID,
		Username:      acc.Username,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		PanelVersion:  "0.1.0",
		Kind:          "full",
		Files:         map[string]any{"home": acc.HomePath},
		Checksums:     map[string]string{},
	}
	for _, d := range dbs {
		man.Databases = append(man.Databases, d.Name)
	}
	for _, m := range mail {
		man.Mailboxes = append(man.Mailboxes, m.LocalPart)
	}
	raw, err := packHome(home)
	if err != nil {
		return man, "", err
	}
	sum := sha256.Sum256(raw)
	man.Checksums["files.tar.gz"] = hex.EncodeToString(sum[:])
	mb, _ := json.Marshal(man)
	man.Checksums["manifest"] = hex.EncodeToString(sum[:])
	bundle := append([]byte("HPM1"), mb...)
	bundle = append(bundle, 0)
	bundle = append(bundle, raw...)
	enc, err := box.Encrypt(bundle)
	if err != nil {
		return man, "", err
	}
	key := fmt.Sprintf("%s/%s.hpm", acc.Username, man.CreatedAt)
	if err := repo.Put(ctx, key, enc); err != nil {
		return man, "", err
	}
	if err := repo.Verify(ctx, key, man.Checksums["files.tar.gz"]); err != nil {
		return man, "", err
	}
	_ = mb
	return man, key, nil
}

func Restore(ctx context.Context, box *secret.Box, repo Repository, key, destHome string) (Manifest, error) {
	enc, err := repo.Get(ctx, key)
	if err != nil {
		return Manifest{}, err
	}
	plain, err := box.Decrypt(enc)
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.HasPrefix(plain, []byte("HPM1")) {
		return Manifest{}, fmt.Errorf("unknown backup magic")
	}
	rest := plain[4:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return Manifest{}, fmt.Errorf("corrupt backup")
	}
	var man Manifest
	if err := json.Unmarshal(rest[:i], &man); err != nil {
		return Manifest{}, err
	}
	if man.FormatVersion != 1 {
		return Manifest{}, fmt.Errorf("unsupported backup format %d", man.FormatVersion)
	}
	raw := rest[i+1:]
	sum := sha256.Sum256(raw)
	if man.Checksums["files.tar.gz"] != hex.EncodeToString(sum[:]) {
		return Manifest{}, fmt.Errorf("checksum mismatch")
	}
	if err := unpackHome(raw, destHome); err != nil {
		return man, err
	}
	return man, nil
}

func Preflight(man Manifest, dest *store.Account) error {
	if dest != nil && dest.Username != "" && dest.Username != man.Username {
		return fmt.Errorf("username mismatch")
	}
	return nil
}

func packHome(home string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, _ := filepath.Rel(home, path)
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, _ = io.Copy(tw, f)
			_ = f.Close()
		}
		return nil
	})
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes(), nil
}

func unpackHome(raw []byte, dest string) error {
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	dest = filepath.Clean(dest)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("archive traversal")
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("archive traversal")
		}
		if hdr.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o750)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o750)
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
	}
}
