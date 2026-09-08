package operations

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) packDirectory(source, dest string) (Result, error) {
	src, err := h.resolve(source)
	if err != nil {
		return Result{}, err
	}
	out, err := h.resolve(dest)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o775); err != nil {
		return Result{}, err
	}
	f, err := os.Create(out)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return nil
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil || rel == "." {
			return nil
		}
		hdr, hdrErr := tar.FileInfoHeader(info, "")
		if hdrErr != nil {
			return hdrErr
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rf, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, rf)
		_ = rf.Close()
		return copyErr
	})
	_ = tw.Close()
	_ = gz.Close()
	if err != nil {
		return Result{}, err
	}
	_ = os.Chmod(out, 0o644)
	return Result{OK: true, ObservedState: "packed"}, nil
}

func (h *Host) unpackDirectory(archive, dest string) (Result, error) {
	src, err := h.resolve(archive)
	if err != nil {
		return Result{}, err
	}
	out, err := h.resolve(dest)
	if err != nil {
		return Result{}, err
	}
	f, err := os.Open(src)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return Result{}, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	if err := os.MkdirAll(out, hostingDirMode(out)); err != nil {
		return Result{}, err
	}
	out = filepath.Clean(out)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return Result{OK: true, ObservedState: "unpacked"}, nil
		}
		if err != nil {
			return Result{}, err
		}
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return Result{}, fmt.Errorf("archive traversal")
		}
		target := filepath.Join(out, name)
		if !strings.HasPrefix(target, out+string(os.PathSeparator)) && target != out {
			return Result{}, fmt.Errorf("archive traversal")
		}
		if hdr.FileInfo().IsDir() {
			_ = os.MkdirAll(target, hostingDirMode(target))
			_ = os.Chmod(target, hostingDirMode(target))
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), hostingDirMode(filepath.Dir(target)))
		wf, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
		if err != nil {
			return Result{}, err
		}
		if _, err := io.Copy(wf, tr); err != nil {
			_ = wf.Close()
			return Result{}, err
		}
		_ = wf.Close()
	}
}

func (h *Host) copyHomedir(username, source, dest string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if dest == "" {
		dest = "/home/" + username
	}
	destClean, err := policy.WithinAccount(username, dest)
	if err != nil {
		return Result{}, err
	}
	srcClean, err := policy.ValidateManagedPath(source)
	if err != nil {
		return Result{}, err
	}
	src, err := h.resolve(srcClean)
	if err != nil {
		return Result{}, err
	}
	dst, err := h.resolve(destClean)
	if err != nil {
		return Result{}, err
	}
	st, err := os.Stat(src)
	if err != nil || !st.IsDir() {
		return Result{}, fmt.Errorf("homedir source is not a directory")
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return Result{}, err
	}
	copied := 0
	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("homedir path escape")
		}
		target := filepath.Join(dst, rel)
		if !strings.HasPrefix(target, dst+string(os.PathSeparator)) {
			return fmt.Errorf("homedir path escape")
		}
		if info.IsDir() {
			mode := hostingDirMode(target)
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
			return os.Chmod(target, mode)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), hostingDirMode(filepath.Dir(target))); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		_ = out.Close()
		if copyErr != nil {
			return copyErr
		}
		copied++
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	pub := filepath.Join(dst, "public_html")
	_ = os.Chmod(pub, hostingDirMode(pub))
	return Result{OK: true, Message: fmt.Sprintf("copied %d files", copied), ObservedState: "copied"}, nil
}
