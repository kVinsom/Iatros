package analysis

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalStubReturnsNotImplementedForDirectory(t *testing.T) {
	t.Parallel()

	report, err := NewLocalStub().Analyze(t.Context(), Request{Root: t.TempDir()})

	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Analyze() error = %v, want ErrNotImplemented", err)
	}
	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", report.SchemaVersion, SchemaVersion)
	}
	if report.Status != StatusNotImplemented {
		t.Fatalf("Status = %q, want %q", report.Status, StatusNotImplemented)
	}
	if report.Target.Path != "." {
		t.Fatalf("Target.Path = %q, want relative root", report.Target.Path)
	}
	if len(report.Ecosystems) != 0 || len(report.Findings) != 0 {
		t.Fatal("stub report must not contain ecosystems or findings")
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "IATROS_ANALYSIS_NOT_IMPLEMENTED" {
		t.Fatalf("Diagnostics = %#v, want not-implemented diagnostic", report.Diagnostics)
	}
}

func TestLocalStubRejectsWindowsRemoteAndDevicePaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path syntax test")
	}

	tests := []string{
		`\\server\share\repository`,
		`//server/share/repository`,
		`\\?\C:\repository`,
		`\\.\C:\repository`,
	}
	for _, target := range tests {
		report, err := NewLocalStub().Analyze(t.Context(), Request{Root: target})
		if !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("target %q: Analyze() error = %v, want ErrInvalidTarget", target, err)
		}
		if report.Status != StatusFailed {
			t.Fatalf("target %q: Status = %q, want %q", target, report.Status, StatusFailed)
		}
	}
}

func TestLocalStubAcceptsDirectoryBelowLinkedAncestor(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realParent := filepath.Join(base, "real-parent")
	target := filepath.Join(realParent, "repository")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	report, err := NewLocalStub().Analyze(
		t.Context(),
		Request{Root: filepath.Join(linkedParent, "repository")},
	)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Analyze() error = %v, want ErrNotImplemented", err)
	}
	if report.Status != StatusNotImplemented {
		t.Fatalf("Status = %q, want %q", report.Status, StatusNotImplemented)
	}
}

func TestClassifyTargetAccessError(t *testing.T) {
	t.Parallel()

	for _, err := range []error{fs.ErrInvalid, fs.ErrNotExist, fs.ErrPermission} {
		if classified := classifyTargetAccessError(err); !errors.Is(classified, ErrInvalidTarget) {
			t.Fatalf("classifyTargetAccessError(%v) = %v, want ErrInvalidTarget", err, classified)
		}
	}

	operationalErr := errors.New("resource exhausted")
	if classified := classifyTargetAccessError(operationalErr); !errors.Is(classified, operationalErr) {
		t.Fatalf("classifyTargetAccessError() = %v, want original operational error", classified)
	}
}

func TestLocalStubRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target func(*testing.T) string
	}{
		{
			name: "empty path",
			target: func(*testing.T) string {
				return ""
			},
		},
		{
			name: "missing path",
			target: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing")
			},
		},
		{
			name: "regular file",
			target: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "target.txt")
				if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				return path
			},
		},
		{
			name: "regular file ancestor",
			target: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "target.txt")
				if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				return filepath.Join(path, "child")
			},
		},
		{
			name: "symbolic link",
			target: func(t *testing.T) string {
				base := t.TempDir()
				target := filepath.Join(base, "target")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v", err)
				}

				link := filepath.Join(base, "link")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symbolic links are unavailable: %v", err)
				}
				return link
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report, err := NewLocalStub().Analyze(t.Context(), Request{Root: test.target(t)})
			if !errors.Is(err, ErrInvalidTarget) {
				t.Fatalf("Analyze() error = %v, want ErrInvalidTarget", err)
			}
			if report.Status != StatusFailed {
				t.Fatalf("Status = %q, want %q", report.Status, StatusFailed)
			}
			if len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "IATROS_TARGET_INVALID" {
				t.Fatalf("Diagnostics = %#v, want invalid-target diagnostic", report.Diagnostics)
			}
		})
	}
}

func TestLocalStubHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	report, err := NewLocalStub().Analyze(ctx, Request{Root: t.TempDir()})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze() error = %v, want context.Canceled", err)
	}
	if report.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", report.Status, StatusFailed)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "IATROS_ANALYSIS_CANCELLED" {
		t.Fatalf("Diagnostics = %#v, want cancellation diagnostic", report.Diagnostics)
	}
}
