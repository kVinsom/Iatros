package artifactvalidation

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySourceReadsOnlyRegularContainedArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	source, err := NewDirectorySource(root)
	if err != nil {
		t.Fatalf("NewDirectorySource() error = %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })
	artifact := Artifact{
		ID: "config", Path: "config.json", Origin: OriginExisting, Kind: KindGeneric,
		Format: FormatJSON, Producer: "iatros.test", ProducerVersion: "1.0",
	}
	reader, err := source.Open(t.Context(), artifact)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	content, err := io.ReadAll(reader)
	if err != nil || string(content) != `{"ok":true}` {
		t.Fatalf("ReadAll() = (%q, %v)", content, err)
	}

	artifact.Path = "../outside.json"
	if _, err := source.Open(t.Context(), artifact); err == nil {
		t.Fatal("Open(traversal) error = nil, want rejection")
	}
	artifact.Path = "."
	if _, err := source.Open(t.Context(), artifact); err == nil {
		t.Fatal("Open(directory) error = nil, want rejection")
	}
	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	artifact.Path = "config.json"
	if _, err := source.Open(t.Context(), artifact); !errors.Is(err, ErrArtifactSourceClosed) {
		t.Fatalf("Open(closed) error = %v, want ErrArtifactSourceClosed", err)
	}
}

func TestDirectorySourceRejectsImplicitAndNonDirectoryRoots(t *testing.T) {
	t.Parallel()

	if _, err := NewDirectorySource(""); !errors.Is(err, ErrUnsafeSourcePath) {
		t.Fatalf("NewDirectorySource(empty) error = %v, want ErrUnsafeSourcePath", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := NewDirectorySource(file); !errors.Is(err, ErrUnsafeSourcePath) {
		t.Fatalf("NewDirectorySource(file) error = %v, want ErrUnsafeSourcePath", err)
	}
}

func TestDirectorySourceRejectsLinksInArtifactPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDirectory, "config.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(realDirectory, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	if err := os.Symlink(
		filepath.Join(realDirectory, "config.json"),
		filepath.Join(root, "config-link.json"),
	); err != nil {
		t.Fatalf("Symlink(file) error = %v", err)
	}
	source, err := NewDirectorySource(root)
	if err != nil {
		t.Fatalf("NewDirectorySource() error = %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })
	artifact := Artifact{ID: "config", Path: "linked/config.json"}
	if _, err := source.Open(t.Context(), artifact); !errors.Is(err, ErrUnsafeSourcePath) {
		t.Fatalf("Open(intermediate link) error = %v, want ErrUnsafeSourcePath", err)
	}
	artifact.Path = "config-link.json"
	if _, err := source.Open(t.Context(), artifact); !errors.Is(err, ErrUnsafeSourcePath) {
		t.Fatalf("Open(final link) error = %v, want ErrUnsafeSourcePath", err)
	}
}

func TestMemorySourceOwnsDetachedContent(t *testing.T) {
	t.Parallel()

	content := []byte("original")
	source := NewMemorySource(map[string][]byte{"artifact": content})
	content[0] = 'X'
	reader, err := source.Open(t.Context(), Artifact{ID: "artifact"})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	got, err := io.ReadAll(reader)
	if err != nil || string(got) != "original" {
		t.Fatalf("ReadAll() = (%q, %v), want detached original", got, err)
	}
}
