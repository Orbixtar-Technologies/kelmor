package operations

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
