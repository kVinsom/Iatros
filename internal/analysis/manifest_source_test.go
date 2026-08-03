package analysis

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kVinsom/Iatros/internal/manifest"
)

func TestLocalManifestSourceReadsOnlyRegularConfinedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "package.json")
	if err := os.WriteFile(filePath, []byte(`{"name":"safe"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := NewLocalManifestSource(root)
	if err != nil {
		t.Fatalf("NewLocalManifestSource() error = %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })

	reader, size, err := source.Open(context.Background(), "package.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if int64(len(data)) != size || string(data) != `{"name":"safe"}` {
		t.Fatalf("Open() data = %q, size = %d", data, size)
	}

	if _, _, err := source.Open(context.Background(), "../package.json"); !errors.Is(err, manifest.ErrInvalidSnapshot) {
		t.Fatalf("Open(traversal) error = %v", err)
	}
}

func TestLocalManifestSourceRejectsSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "package.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	source, err := NewLocalManifestSource(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	if _, _, err := source.Open(context.Background(), "package.json"); err == nil {
		t.Fatal("Open() accepted a symbolic link")
	}
}

func TestLocalManifestSourceLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := NewLocalManifestSource(root)
	if err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := source.Open(canceled, "package.json"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open(canceled) error = %v, want context.Canceled", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if _, _, err := source.Open(t.Context(), "package.json"); !errors.Is(err, errManifestSourceClosed) {
		t.Fatalf("Open(after Close) error = %v, want errManifestSourceClosed", err)
	}

	var nilSource *LocalManifestSource
	if err := nilSource.Close(); err != nil {
		t.Fatalf("nil Close() error = %v", err)
	}
	if _, _, err := nilSource.Open(t.Context(), "package.json"); !errors.Is(err, errManifestSourceClosed) {
		t.Fatalf("nil Open() error = %v, want errManifestSourceClosed", err)
	}
}
