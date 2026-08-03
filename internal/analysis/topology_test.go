package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
	"github.com/kVinsom/Iatros/internal/topology"
)

func TestLocalTopologyAnalyzerBuildsRepositoryModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyFixture(t, root, "package.json", `{
  "name": "root",
  "workspaces": ["packages/*"]
}`)
	writeTopologyFixture(t, root, "packages/api/package.json", `{
  "name": "@example/api",
  "dependencies": {"@example/shared": "workspace:*", "react": "^19"}
}`)
	writeTopologyFixture(t, root, "packages/shared/package.json", `{
  "name": "@example/shared"
}`)

	analyzer, err := NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatalf("NewLocalTopologyAnalyzer() error = %v", err)
	}
	model, err := analyzer.Analyze(context.Background(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if model.Partial || len(model.Issues) != 0 || len(model.Projects) != 3 ||
		len(model.Workspaces) != 1 || len(model.Dependencies) != 2 {
		t.Fatalf("Analyze() = %+v", model)
	}
	if model.Projects[0].Root != "." || model.Workspaces[0].Root != "." {
		t.Fatalf("Analyze() exposed unexpected roots: %+v", model)
	}
	internal := findTopologyDependency(t, model, "@example/shared")
	if internal.Resolution != topology.DependencyInternal ||
		len(internal.TargetProjects) != 1 || internal.TargetProjects[0] != "packages/shared" {
		t.Fatalf("internal dependency = %+v", internal)
	}
}

func TestLocalTopologyAnalyzerHandlesComplexNestedRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyFixture(t, root, ".gitignore", "generated/\n*.tmp\n")
	writeTopologyFixture(t, root, ".gitmodules", "[submodule \"vendor\"]\npath = vendor/library\n")
	writeTopologyFixture(t, root, "package.json", `{
  "name": "root",
  "workspaces": ["services/*"]
}`)
	writeTopologyFixture(t, root, "services/api/package.json", `{"name":"api"}`)
	writeTopologyFixture(t, root, "services/api/compose.yaml", "services: {}")
	writeTopologyFixture(t, root, "services/worker/go.mod", "module example.com/worker\n")
	writeTopologyFixture(t, root, "services/worker/generated/package.json", `{"name":"generated"}`)
	writeTopologyFixture(t, root, "vendor/library/package.json", `{"name":"submodule"}`)
	writeTopologyFixture(t, root, "tools/external/.git", "gitdir: external-metadata")
	writeTopologyFixture(t, root, "tools/external/Cargo.toml", "[package]\nname = \"external\"")

	analyzer, err := NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	model, err := analyzer.Analyze(t.Context(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if model.Partial || len(model.Issues) != 0 || len(model.Projects) != 3 ||
		len(model.Workspaces) != 1 {
		t.Fatalf("Analyze() = %+v", model)
	}
	roots := make([]string, 0, len(model.Projects))
	for _, value := range model.Projects {
		roots = append(roots, value.Root)
	}
	wantRoots := []string{".", "services/api", "services/worker"}
	if !slices.Equal(roots, wantRoots) {
		t.Fatalf("project roots = %#v, want %#v", roots, wantRoots)
	}
	wantNestedRepositories := []string{"tools/external", "vendor/library"}
	if !slices.Equal(model.NestedRepositories, wantNestedRepositories) {
		t.Fatalf(
			"nested repositories = %#v, want %#v",
			model.NestedRepositories,
			wantNestedRepositories,
		)
	}
	if model.Projects[1].Kind != "mixed" {
		t.Fatalf("services/api kind = %q, want mixed", model.Projects[1].Kind)
	}
}

func TestLocalTopologyAnalyzerPropagatesManifestFailureAsPartial(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyFixture(t, root, "package.json", `{"name":"first","name":"second"}`)
	analyzer, err := NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	model, err := analyzer.Analyze(context.Background(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !model.Partial || len(model.Projects) != 1 || len(model.Issues) != 1 ||
		model.Issues[0].Code != "IATROS_MANIFEST_PARSE_FAILED" {
		t.Fatalf("Analyze() = %+v", model)
	}
}

func TestLocalTopologyAnalyzerSkipsOversizedManifestContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyFixture(
		t,
		root,
		"package.json",
		`{"name":"a-manifest-that-is-larger-than-the-test-profile"}`,
	)
	profile := SmallScalingProfile()
	profile.Manifest.MaxFileBytes = 32
	profile.Manifest.MaxTotalBytes = 64
	profile.Manifest.MaxValueBytes = 16

	analyzer, err := NewLocalTopologyAnalyzerWithProfile(profile)
	if err != nil {
		t.Fatalf("NewLocalTopologyAnalyzerWithProfile() error = %v", err)
	}
	model, err := analyzer.Analyze(t.Context(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !model.Partial || len(model.Projects) != 1 || len(model.Projects[0].Components) != 0 ||
		len(model.Issues) != 1 || model.Issues[0].Code != manifest.IssueFileTooLarge {
		t.Fatalf("Analyze() = %+v, want bounded partial result", model)
	}
}

func TestLocalTopologyAnalyzerPreservesDiscoveryIssues(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	builder, err := topology.NewBuilder(topology.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	analyzer := LocalTopologyAnalyzer{
		discovery: topologyDiscoveryFunc(func(context.Context, string) (Inventory, error) {
			return Inventory{
				Directories: []string{"."},
				Files:       []string{},
				Issues: []DiscoveryIssue{{
					Code: DiscoveryIssuePathUnreadable, Path: "restricted",
					Message: "The path could not be read and was skipped.",
				}},
				Partial: true,
			}, nil
		}),
		projects: projectBoundaryDetectorFunc(func(context.Context, project.Snapshot) (project.Model, error) {
			return project.Model{Projects: []project.Project{}, Workspaces: []project.Workspace{}, Partial: true}, nil
		}),
		manifests: manifestFactAnalyzerFunc(func(context.Context, manifest.Source, manifest.Snapshot) (manifest.Result, error) {
			return manifest.Result{Manifests: []manifest.Manifest{}, Issues: []manifest.Issue{}, Partial: true}, nil
		}),
		topology: builder,
	}

	model, err := analyzer.Analyze(t.Context(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(model.Issues) != 1 || model.Issues[0].Code != DiscoveryIssuePathUnreadable ||
		model.Issues[0].Path != "restricted" {
		t.Fatalf("Issues = %#v, want preserved discovery issue", model.Issues)
	}
}

func TestLocalTopologyAnalyzerStopsBetweenCanceledStages(t *testing.T) {
	t.Parallel()

	t.Run("after discovery", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		projectsCalled := false
		analyzer := LocalTopologyAnalyzer{
			discovery: topologyDiscoveryFunc(func(context.Context, string) (Inventory, error) {
				cancel()
				return emptyInventory(), nil
			}),
			projects: projectBoundaryDetectorFunc(func(context.Context, project.Snapshot) (project.Model, error) {
				projectsCalled = true
				return project.Model{}, nil
			}),
			manifests: manifestFactAnalyzerFunc(func(context.Context, manifest.Source, manifest.Snapshot) (manifest.Result, error) {
				return manifest.Result{}, nil
			}),
			topology: topologyModelBuilderFunc(func(context.Context, topology.Snapshot) (topology.Model, error) {
				return topology.Model{}, nil
			}),
		}

		if _, err := analyzer.Analyze(ctx, Request{Root: t.TempDir()}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Analyze() error = %v, want context.Canceled", err)
		}
		if projectsCalled {
			t.Fatal("project detection ran after cancellation")
		}
	})

	t.Run("after project detection", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		manifestsCalled := false
		analyzer := LocalTopologyAnalyzer{
			discovery: topologyDiscoveryFunc(func(context.Context, string) (Inventory, error) {
				return emptyInventory(), nil
			}),
			projects: projectBoundaryDetectorFunc(func(context.Context, project.Snapshot) (project.Model, error) {
				cancel()
				return project.Model{}, nil
			}),
			manifests: manifestFactAnalyzerFunc(func(context.Context, manifest.Source, manifest.Snapshot) (manifest.Result, error) {
				manifestsCalled = true
				return manifest.Result{}, nil
			}),
			topology: topologyModelBuilderFunc(func(context.Context, topology.Snapshot) (topology.Model, error) {
				return topology.Model{}, nil
			}),
		}

		if _, err := analyzer.Analyze(ctx, Request{Root: t.TempDir()}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Analyze() error = %v, want context.Canceled", err)
		}
		if manifestsCalled {
			t.Fatal("manifest analysis ran after cancellation")
		}
	})
}

func TestLocalTopologyAnalyzerRejectsInvalidTargetAndCancellation(t *testing.T) {
	t.Parallel()

	analyzer, err := NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzer.Analyze(context.Background(), Request{Root: filepath.Join(t.TempDir(), "missing")}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("Analyze() error = %v, want ErrInvalidTarget", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := analyzer.Analyze(ctx, Request{Root: t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze() error = %v, want context.Canceled", err)
	}
}

func writeTopologyFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func findTopologyDependency(t *testing.T, model topology.Model, name string) topology.Dependency {
	t.Helper()
	for _, dependency := range model.Dependencies {
		if dependency.Name == name {
			return dependency
		}
	}
	t.Fatalf("dependency %q not found", name)
	return topology.Dependency{}
}

type topologyDiscoveryFunc func(context.Context, string) (Inventory, error)

func (function topologyDiscoveryFunc) Discover(ctx context.Context, root string) (Inventory, error) {
	return function(ctx, root)
}

type projectBoundaryDetectorFunc func(context.Context, project.Snapshot) (project.Model, error)

func (function projectBoundaryDetectorFunc) Detect(ctx context.Context, snapshot project.Snapshot) (project.Model, error) {
	return function(ctx, snapshot)
}

type manifestFactAnalyzerFunc func(context.Context, manifest.Source, manifest.Snapshot) (manifest.Result, error)

func (function manifestFactAnalyzerFunc) Analyze(ctx context.Context, source manifest.Source, snapshot manifest.Snapshot) (manifest.Result, error) {
	return function(ctx, source, snapshot)
}

type topologyModelBuilderFunc func(context.Context, topology.Snapshot) (topology.Model, error)

func (function topologyModelBuilderFunc) Build(ctx context.Context, snapshot topology.Snapshot) (topology.Model, error) {
	return function(ctx, snapshot)
}
