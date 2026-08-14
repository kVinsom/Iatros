package artifactvalidation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

var (
	// ErrArtifactNotFound indicates that a source does not contain the requested artifact.
	ErrArtifactNotFound = errors.New("artifact content was not found")
	// ErrUnsafeSourcePath indicates that a filesystem artifact resolves outside its configured root.
	ErrUnsafeSourcePath = errors.New("artifact source path is unsafe")
	// ErrArtifactSourceClosed indicates that a filesystem source no longer owns an open root handle.
	ErrArtifactSourceClosed = errors.New("artifact source is closed")
)

// MemorySource owns detached generated or preloaded artifact content.
type MemorySource struct {
	contents map[string][]byte
}

// NewMemorySource copies artifact content so callers cannot mutate validation input concurrently.
func NewMemorySource(contents map[string][]byte) *MemorySource {
	detached := make(map[string][]byte, len(contents))
	for artifactID, content := range contents {
		detached[artifactID] = bytes.Clone(content)
	}
	return &MemorySource{contents: detached}
}

// Open returns a reader for the content associated with artifact.ID.
func (s *MemorySource) Open(ctx context.Context, artifact Artifact) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrArtifactNotFound
	}
	content, exists := s.contents[artifact.ID]
	if !exists {
		return nil, ErrArtifactNotFound
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

// DirectorySource reads existing artifacts below one canonical filesystem root.
type DirectorySource struct {
	mu   sync.RWMutex
	root *os.Root
}

// NewDirectorySource opens one confined root handle and rejects links or non-directory roots.
func NewDirectorySource(root string) (*DirectorySource, error) {
	if root == "" {
		return nil, ErrUnsafeSourcePath
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("make artifact source root absolute: %w", err)
	}
	rootInfo, err := os.Lstat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect artifact source root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafeSourcePath
	}
	confinedRoot, err := os.OpenRoot(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("open artifact source root: %w", err)
	}
	openedInfo, err := confinedRoot.Stat(".")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("inspect opened artifact source root: %w", err), confinedRoot.Close())
	}
	if !os.SameFile(rootInfo, openedInfo) {
		return nil, errors.Join(ErrUnsafeSourcePath, confinedRoot.Close())
	}
	return &DirectorySource{root: confinedRoot}, nil
}

// Open confines the path, rejects links, and verifies stable regular-file identity.
func (s *DirectorySource) Open(ctx context.Context, artifact Artifact) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || !repositorypath.IsValidFile(artifact.Path) {
		return nil, ErrUnsafeSourcePath
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.root == nil {
		return nil, ErrArtifactSourceClosed
	}
	filePath := filepath.FromSlash(artifact.Path)
	fileInfo, err := inspectSourcePath(s.root, artifact.Path)
	if err != nil {
		return nil, err
	}
	artifactFile, err := s.root.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open artifact: %w", err)
	}
	artifactInfo, err := artifactFile.Stat()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("inspect artifact: %w", err), artifactFile.Close())
	}
	if !artifactInfo.Mode().IsRegular() || !os.SameFile(fileInfo, artifactInfo) {
		return nil, errors.Join(ErrUnsafeSourcePath, artifactFile.Close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, artifactFile.Close())
	}
	return artifactFile, nil
}

func inspectSourcePath(root *os.Root, artifactPath string) (os.FileInfo, error) {
	components := strings.Split(artifactPath, "/")
	currentPath := ""
	for index, component := range components {
		currentPath = filepath.Join(currentPath, component)
		fileInfo, err := root.Lstat(currentPath)
		if err != nil {
			return nil, fmt.Errorf("inspect artifact path: %w", err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return nil, ErrUnsafeSourcePath
		}
		isFinalComponent := index == len(components)-1
		if !isFinalComponent && !fileInfo.IsDir() {
			return nil, ErrUnsafeSourcePath
		}
		if isFinalComponent {
			if !fileInfo.Mode().IsRegular() {
				return nil, ErrUnsafeSourcePath
			}
			return fileInfo, nil
		}
	}
	return nil, ErrUnsafeSourcePath
}

// Close releases the confined root handle. It is safe to call more than once.
func (s *DirectorySource) Close() error {
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
