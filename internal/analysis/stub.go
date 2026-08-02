package analysis

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// ErrNotImplemented indicates that the stable analysis contract exists without a scanner implementation.
	ErrNotImplemented = errors.New("local repository analysis is not implemented")
	// ErrInvalidTarget indicates that the selected analysis root cannot be safely accepted.
	ErrInvalidTarget = errors.New("analysis target is invalid")
)

// Request identifies the local root selected for analysis.
type Request struct {
	Root string
}

// LocalStub validates a local target without scanning repository content.
type LocalStub struct{}

// NewLocalStub creates the placeholder local analyzer.
func NewLocalStub() LocalStub {
	return LocalStub{}
}

// Analyze validates the selected root and returns the approved not-implemented report.
func (LocalStub) Analyze(ctx context.Context, request Request) (Report, error) {
	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}

	if err := validateLocalDirectory(request.Root); err != nil {
		if errors.Is(err, ErrInvalidTarget) {
			return NewInvalidTargetReport(), ErrInvalidTarget
		}
		return NewAnalysisFailedReport(), err
	}

	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}

	return NewNotImplementedReport(), ErrNotImplemented
}

func validateLocalDirectory(path string) error {
	root, err := openLocalRoot(path)
	if err != nil {
		return err
	}
	return root.Close()
}

func openLocalRoot(path string) (*os.Root, error) {
	if path == "" || isWindowsRemoteOrDevicePath(path) {
		return nil, ErrInvalidTarget
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil || isWindowsRemoteOrDevicePath(absolutePath) {
		return nil, ErrInvalidTarget
	}

	info, err := os.Lstat(absolutePath)
	if err != nil {
		return nil, ErrInvalidTarget
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidTarget
	}

	root, err := os.OpenRoot(absolutePath)
	if err != nil {
		return nil, classifyTargetAccessError(err)
	}
	openedInfo, statErr := root.Stat(".")
	if statErr != nil {
		_ = root.Close()
		return nil, classifyTargetAccessError(statErr)
	}
	// Comparing identities detects target replacement between metadata validation
	// and directory-handle acquisition.
	if !os.SameFile(info, openedInfo) {
		_ = root.Close()
		return nil, ErrInvalidTarget
	}

	return root, nil
}

func classifyTargetAccessError(err error) error {
	if errors.Is(err, fs.ErrInvalid) ||
		errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, fs.ErrPermission) {
		return ErrInvalidTarget
	}
	return err
}

func isWindowsRemoteOrDevicePath(path string) bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(filepath.ToSlash(path), "//")
}
