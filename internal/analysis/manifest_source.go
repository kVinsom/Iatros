package analysis

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/kVinsom/Iatros/internal/manifest"
)

var (
	errManifestSourceClosed = errors.New("local manifest source is closed")
	errUnsafeManifestFile   = errors.New("manifest path is not a stable regular file")
)

// LocalManifestSource confines manifest reads to one validated local repository root.
type LocalManifestSource struct {
	mu   sync.RWMutex
	root *os.Root
}

var _ manifest.Source = (*LocalManifestSource)(nil)

// NewLocalManifestSource opens a confined, read-only source for manifest analysis.
func NewLocalManifestSource(path string) (*LocalManifestSource, error) {
	root, err := openLocalRoot(path)
	if err != nil {
		return nil, err
	}
	return &LocalManifestSource{root: root}, nil
}

// Open returns a regular manifest file only when its identity remains stable while opening it.
func (s *LocalManifestSource) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if s == nil {
		return nil, 0, errManifestSourceClosed
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.root == nil {
		return nil, 0, errManifestSourceClosed
	}
	if !validRelativePath(path) {
		return nil, 0, manifest.ErrInvalidSnapshot
	}

	info, err := s.root.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, errUnsafeManifestFile
	}

	file, err := s.root.Open(path)
	if err != nil {
		return nil, 0, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, 0, errUnsafeManifestFile
	}
	if err := ctx.Err(); err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	return file, openedInfo.Size(), nil
}

// Close releases the confined repository handle.
func (s *LocalManifestSource) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	err := s.root.Close()
	s.root = nil
	return err
}
