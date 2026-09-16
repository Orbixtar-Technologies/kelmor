package filesystem

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	defaultMaxArchiveFiles = 10000
	defaultMaxArchiveBytes = 1 << 30
)

type ArchiveLimits struct {
	MaxFiles int
	MaxBytes int64
}

func (root *Root) ExtractTar(reader io.Reader, dest string, limits ArchiveLimits) error {
	if dest != "" && dest != "." && !ValidRelative(dest) {
		return fmt.Errorf("invalid archive destination %q", dest)
	}
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = defaultMaxArchiveFiles
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = defaultMaxArchiveBytes
	}
	tr := tar.NewReader(reader)
	seen := map[string]bool{}
	var total int64
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.ReplaceAll(hdr.Name, "\\", "/"))
		name = strings.TrimPrefix(name, "./")
		if name == "." || name == "" {
			continue
		}
		if path.IsAbs(name) || strings.HasPrefix(name, "../") || strings.Contains(name, ":") {
			return fmt.Errorf("archive traversal")
		}
		if !ValidRelative(name) {
			return fmt.Errorf("archive traversal")
		}
		if seen[name] {
			return fmt.Errorf("duplicate archive entry %q", name)
		}
		seen[name] = true
		rel := name
		if dest != "" && dest != "." {
			rel = path.Join(dest, name)
			if !ValidRelative(rel) {
				return fmt.Errorf("archive traversal")
			}
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(rel, 0o750); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			count++
			if count > limits.MaxFiles {
				return fmt.Errorf("archive file count exceeds limit")
			}
			if hdr.Size < 0 || hdr.Size > limits.MaxBytes {
				return fmt.Errorf("archive member exceeds size limit")
			}
			total += hdr.Size
			if total > limits.MaxBytes {
				return fmt.Errorf("archive expansion exceeds size limit")
			}
			if info, err := root.Stat(rel); err == nil && !info.Mode().IsRegular() {
				return fmt.Errorf("archive replacement target is not a regular file")
			}
			limited := io.LimitReader(tr, hdr.Size)
			if err := root.CopyReaderAtomic(limited, rel, 0o640); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return fmt.Errorf("archive contains unsupported entry %q", name)
		default:
			return fmt.Errorf("archive contains unsupported entry %q", name)
		}
	}
}
