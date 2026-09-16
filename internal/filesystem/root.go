package filesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

const secureResolve = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS

var (
	ErrGuaranteesUnavailable = errors.New("required openat2 guarantees are unavailable")
	temporarySequence        atomic.Uint64
)

type Root struct {
	path string
	file *os.File
}

func Create(rootPath, description string) error {
	if !filepath.IsAbs(rootPath) {
		return fmt.Errorf("%s must be an absolute path", description)
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	current := os.NewFile(uintptr(fd), "/")
	defer func() {
		_ = current.Close()
	}()
	for _, component := range strings.Split(strings.TrimPrefix(filepath.ToSlash(filepath.Clean(rootPath)), "/"), "/") {
		if component == "" {
			continue
		}
		mkdirErr := unix.Mkdirat(int(current.Fd()), component, 0o755)
		created := mkdirErr == nil
		if mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
			return mkdirErr
		}
		if err := current.Sync(); err != nil {
			return err
		}
		next, err := unix.Openat(int(current.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return classifyOpen(err, description+" or an ancestor")
		}
		if created {
			if err := unix.Fchmod(next, 0o755); err != nil {
				_ = unix.Close(next)
				return err
			}
			if err := unix.Fsync(next); err != nil {
				_ = unix.Close(next)
				return err
			}
		}
		if err := current.Close(); err != nil {
			_ = unix.Close(next)
			return err
		}
		current = os.NewFile(uintptr(next), component)
	}
	return nil
}

func Open(rootPath, description string) (*Root, error) {
	if !filepath.IsAbs(rootPath) {
		return nil, fmt.Errorf("%s must be an absolute path", description)
	}
	canonical := filepath.Clean(rootPath)
	base, err := os.Open("/")
	if err != nil {
		return nil, err
	}
	defer base.Close()
	relative := strings.TrimPrefix(filepath.ToSlash(canonical), "/")
	fd, err := unix.Openat2(int(base.Fd()), relative, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: secureResolve,
	})
	if err != nil {
		return nil, classifyOpen(err, description)
	}
	return &Root{path: canonical, file: os.NewFile(uintptr(fd), canonical)}, nil
}

func OpenNested(parent *Root, relative, description string) (*Root, error) {
	if !ValidRelative(relative) {
		return nil, fmt.Errorf("invalid relative path %q", relative)
	}
	fd, err := unix.Openat2(int(parent.file.Fd()), relative, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: secureResolve | unix.RESOLVE_NO_XDEV,
	})
	if err != nil {
		return nil, classifyOpen(err, description)
	}
	return &Root{
		path: filepath.Join(parent.path, filepath.FromSlash(relative)),
		file: os.NewFile(uintptr(fd), relative),
	}, nil
}

func (root *Root) Path() string { return root.path }

func (root *Root) Close() error {
	return root.file.Close()
}

func (root *Root) OpenFile(relative string, flags int, mode uint32) (*os.File, error) {
	if !ValidRelative(relative) {
		return nil, fmt.Errorf("invalid relative path %q", relative)
	}
	fd, err := unix.Openat2(int(root.file.Fd()), relative, &unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC),
		Mode:    uint64(mode),
		Resolve: secureResolve,
	})
	if err != nil {
		return nil, classifyOpen(err, relative)
	}
	return os.NewFile(uintptr(fd), filepath.Join(root.path, filepath.FromSlash(relative))), nil
}

func (root *Root) Parent(relative string, create bool, mode uint32) (*os.File, string, error) {
	if !ValidRelative(relative) {
		return nil, "", fmt.Errorf("invalid relative path %q", relative)
	}
	components := strings.Split(relative, "/")
	name := components[len(components)-1]
	current, err := unix.Dup(int(root.file.Fd()))
	if err != nil {
		return nil, "", err
	}
	for _, component := range components[:len(components)-1] {
		created := false
		if create {
			mkdirErr := unix.Mkdirat(current, component, mode)
			created = mkdirErr == nil
			if mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = unix.Close(current)
				return nil, "", mkdirErr
			}
			if err := unix.Fsync(current); err != nil {
				_ = unix.Close(current)
				return nil, "", err
			}
		}
		next, err := unix.Openat(current, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			_ = unix.Close(current)
			return nil, "", classifyOpen(err, relative)
		}
		if created {
			if err := unix.Fchmod(next, mode); err != nil {
				_ = unix.Close(next)
				_ = unix.Close(current)
				return nil, "", err
			}
			if err := unix.Fsync(next); err != nil {
				_ = unix.Close(next)
				_ = unix.Close(current)
				return nil, "", err
			}
		}
		_ = unix.Close(current)
		current = next
	}
	return os.NewFile(uintptr(current), filepath.Dir(filepath.Join(root.path, filepath.FromSlash(relative)))), name, nil
}

func (root *Root) MkdirAll(relative string, mode uint32) error {
	if relative == "" || relative == "." {
		return nil
	}
	parent, name, err := root.Parent(path.Join(relative, ".keep"), true, mode)
	if err != nil {
		return err
	}
	_ = name
	defer parent.Close()
	return parent.Sync()
}

func (root *Root) ReadFile(relative string, limit int64) ([]byte, error) {
	file, err := root.OpenFile(relative, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", relative)
	}
	reader := io.Reader(file)
	if limit >= 0 {
		reader = io.LimitReader(file, limit+1)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && int64(len(raw)) > limit {
		return nil, fmt.Errorf("%q exceeds size limit", relative)
	}
	return raw, nil
}

func (root *Root) WriteAtomic(relative string, content []byte, mode uint32) error {
	parent, name, err := root.Parent(relative, true, 0o755)
	if err != nil {
		return err
	}
	defer parent.Close()
	temporary := "." + name + ".tmp-" + strconv.Itoa(os.Getpid()) + "-" +
		strconv.FormatUint(temporarySequence.Add(1), 10)
	fd, err := unix.Openat(
		int(parent.Fd()), temporary,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		mode,
	)
	if err != nil {
		return classifyOpen(err, temporary)
	}
	if err := unix.Fchmod(fd, mode); err != nil {
		_ = unix.Close(fd)
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	file := os.NewFile(uintptr(fd), temporary)
	cleanup := func() {
		_ = file.Close()
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
	}
	if _, err := file.Write(content); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	if err := unix.Renameat(int(parent.Fd()), temporary, int(parent.Fd()), name); err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	return parent.Sync()
}

func (root *Root) CopyAtomic(source, destination string, mode uint32) error {
	return root.CopyAtomicWithDirMode(source, destination, mode, 0o755)
}

func (root *Root) CopyAtomicWithDirMode(source, destination string, mode, directoryMode uint32) error {
	input, err := root.OpenFile(source, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", source)
	}
	return root.CopyReaderAtomicWithDirMode(input, destination, mode, directoryMode)
}

func (root *Root) CopyReaderAtomic(input io.Reader, destination string, mode uint32) error {
	return root.CopyReaderAtomicWithDirMode(input, destination, mode, 0o755)
}

func (root *Root) CopyReaderAtomicWithDirMode(input io.Reader, destination string, mode, directoryMode uint32) error {
	parent, name, err := root.Parent(destination, true, directoryMode)
	if err != nil {
		return err
	}
	defer parent.Close()
	temporary := "." + name + ".tmp-" + strconv.Itoa(os.Getpid()) + "-" +
		strconv.FormatUint(temporarySequence.Add(1), 10)
	fd, err := unix.Openat(
		int(parent.Fd()), temporary,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		mode,
	)
	if err != nil {
		return classifyOpen(err, temporary)
	}
	if err := unix.Fchmod(fd, mode); err != nil {
		_ = unix.Close(fd)
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	output := os.NewFile(uintptr(fd), temporary)
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	if err := output.Close(); err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	if err := unix.Renameat(int(parent.Fd()), temporary, int(parent.Fd()), name); err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), temporary, 0)
		return err
	}
	return parent.Sync()
}

func (root *Root) Rename(source, destination string) error {
	sourceParent, sourceName, err := root.Parent(source, false, 0)
	if err != nil {
		return err
	}
	defer sourceParent.Close()
	destinationParent, destinationName, err := root.Parent(destination, true, 0o755)
	if err != nil {
		return err
	}
	defer destinationParent.Close()
	if err := unix.Renameat(
		int(sourceParent.Fd()), sourceName,
		int(destinationParent.Fd()), destinationName,
	); err != nil {
		return err
	}
	if err := destinationParent.Sync(); err != nil {
		return err
	}
	if sourceParent.Fd() != destinationParent.Fd() {
		return sourceParent.Sync()
	}
	return nil
}

func (root *Root) Stat(relative string) (os.FileInfo, error) {
	file, err := root.OpenFile(relative, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

func (root *Root) Remove(relative string) error {
	parent, name, err := root.Parent(relative, false, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	defer parent.Close()
	if err := unix.Unlinkat(int(parent.Fd()), name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	return parent.Sync()
}

func (root *Root) RemoveTree(relative string) error {
	parent, name, err := root.Parent(relative, false, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	defer parent.Close()
	if err := removeTreeAt(int(parent.Fd()), name); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	return parent.Sync()
}

func (root *Root) Chmod(relative string, mode uint32) error {
	file, err := root.OpenFile(relative, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return unix.Fchmod(int(file.Fd()), mode)
}

func removeTreeAt(parentFD int, name string) error {
	var stat unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return unix.Unlinkat(parentFD, name, 0)
	}
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return classifyOpen(err, name)
	}
	directory := os.NewFile(uintptr(fd), name)
	entries, err := directory.ReadDir(-1)
	if err != nil {
		_ = directory.Close()
		return err
	}
	for _, entry := range entries {
		if err := removeTreeAt(fd, entry.Name()); err != nil {
			_ = directory.Close()
			return err
		}
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	if err := directory.Close(); err != nil {
		return err
	}
	return unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR)
}

func classifyOpen(err error, name string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOTSUP) {
		return fmt.Errorf("%w for %s: %v", ErrGuaranteesUnavailable, name, err)
	}
	if errors.Is(err, unix.ELOOP) {
		return fmt.Errorf("%s contains a symlink: %w", name, err)
	}
	return fmt.Errorf("%s: %w", name, err)
}
