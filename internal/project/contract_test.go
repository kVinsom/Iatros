package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
	"github.com/kVinsom/Iatros/internal/project"
)

func TestDiscoveryInventoryFitsProjectDetector(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relativePath := range []string{
		"go.work",
		"services/api/go.mod",
		"infrastructure/main.tf",
		".git/generated/go.mod",
	} {
		filename := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", relativePath, err)
		}
		if err := os.WriteFile(filename, []byte("fixture content is not read"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", relativePath, err)
		}
	}

	discovery, err := analysis.NewLocalDiscovery(analysis.DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("NewLocalDiscovery() error = %v", err)
	}
	inventory, err := discovery.Discover(t.Context(), root)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	detector, err := project.NewDetector(project.DefaultLimits())
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}
	model, err := detector.Detect(t.Context(), project.Snapshot{
		Files:   inventory.Files,
		Partial: inventory.Partial,
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if model.Partial || len(model.Projects) != 2 || len(model.Workspaces) != 1 {
		t.Fatalf("model = %#v, want two projects and one complete workspace", model)
	}
	if model.Projects[0].Root != "infrastructure" ||
		model.Projects[1].Root != "services/api" ||
		model.Workspaces[0].Root != "." {
		t.Fatalf("model = %#v, want sorted inventory-backed boundaries", model)
	}
}
