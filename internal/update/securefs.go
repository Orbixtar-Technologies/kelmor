package update

import (
	"io"
	"os"

	"github.com/hosting-panel/panel/internal/filesystem"
)

type secureRoot struct {
	inner *filesystem.Root
}

func createSecureRoot(rootPath, description string) error {
	return filesystem.Create(rootPath, description)
}

func openSecureRoot(rootPath, description string) (*secureRoot, error) {
	inner, err := filesystem.Open(rootPath, description)
	if err != nil {
		return nil, err
	}
	return &secureRoot{inner: inner}, nil
}

func (root *secureRoot) Close() error { return root.inner.Close() }

func (root *secureRoot) open(relative string, flags int, mode uint32) (*os.File, error) {
	return root.inner.OpenFile(relative, flags, mode)
}

func (root *secureRoot) mkdirAll(relative string, mode uint32) error {
	return root.inner.MkdirAll(relative, mode)
}

func (root *secureRoot) readFile(relative string, limit int64) ([]byte, error) {
	return root.inner.ReadFile(relative, limit)
}

func (root *secureRoot) writeAtomic(relative string, content []byte, mode uint32) error {
	return root.inner.WriteAtomic(relative, content, mode)
}

func (root *secureRoot) copyAtomic(source, destination string, mode uint32) error {
	return root.inner.CopyAtomic(source, destination, mode)
}

func (root *secureRoot) copyAtomicWithDirMode(source, destination string, mode, directoryMode uint32) error {
	return root.inner.CopyAtomicWithDirMode(source, destination, mode, directoryMode)
}

func (root *secureRoot) copyReaderAtomic(input io.Reader, destination string, mode uint32) error {
	return root.inner.CopyReaderAtomic(input, destination, mode)
}

func (root *secureRoot) copyReaderAtomicWithDirMode(input io.Reader, destination string, mode, directoryMode uint32) error {
	return root.inner.CopyReaderAtomicWithDirMode(input, destination, mode, directoryMode)
}

func (root *secureRoot) rename(source, destination string) error {
	return root.inner.Rename(source, destination)
}

func (root *secureRoot) stat(relative string) (os.FileInfo, error) {
	return root.inner.Stat(relative)
}

func (root *secureRoot) remove(relative string) error {
	return root.inner.Remove(relative)
}

func (root *secureRoot) removeTree(relative string) error {
	return root.inner.RemoveTree(relative)
}
