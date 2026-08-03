package topology

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
)

func TestBuilderAssociatesWorkspacesAndDependencies(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{
			Projects: []project.Project{
				{Root: ".", Kind: project.KindCode},
				{Root: "packages/api", Kind: project.KindCode},
				{Root: "packages/shared", Kind: project.KindCode},
				{Root: "tools/worker", Kind: project.KindCode},
			},
			Workspaces: []project.Workspace{{Root: "."}},
		},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "package.json", Format: manifest.FormatNodePackage, Name: "root",
				WorkspaceMembers: []string{"packages/*", "tools/**"},
			},
			{
				Path: "packages/api/package.json", Format: manifest.FormatNodePackage,
				Name: "@acme/api",
				Dependencies: []manifest.Dependency{
					{Name: "@acme/shared", Constraint: "workspace:*", Scope: manifest.ScopeRuntime},
					{Name: "react", Constraint: "^19", Scope: manifest.ScopeRuntime},
				},
			},
			{
				Path: "packages/shared/package.json", Format: manifest.FormatNodePackage,
				Name: "@acme/shared",
			},
			{
				Path: "tools/worker/package.json", Format: manifest.FormatNodePackage,
				Name: "@acme/worker",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if model.Partial || len(model.Issues) != 0 {
		t.Fatalf("Build() partial model = %+v", model)
	}
	if len(model.Projects) != 4 || len(model.Workspaces) != 1 || len(model.Dependencies) != 2 {
		t.Fatalf("Build() = %+v", model)
	}

	workspace := model.Workspaces[0]
	if workspace.Root != "." || len(workspace.Declarations) != 2 ||
		len(workspace.DeclaredProjects) != 3 || len(workspace.ContainedProjects) != 4 {
		t.Fatalf("workspace = %+v", workspace)
	}
	if workspace.Declarations[0].Resolution != MemberMatched ||
		workspace.Declarations[1].Resolution != MemberMatched {
		t.Fatalf("declarations = %+v", workspace.Declarations)
	}
	for _, value := range model.Projects {
		if value.PrimaryWorkspaceRoot != "." || len(value.WorkspaceRoots) != 1 ||
			value.WorkspaceRoots[0] != "." {
			t.Errorf("project = %+v", value)
		}
	}

	internal := dependencyByName(t, model, "@acme/shared")
	if internal.Resolution != DependencyInternal || len(internal.TargetProjects) != 1 ||
		internal.TargetProjects[0] != "packages/shared" {
		t.Fatalf("internal dependency = %+v", internal)
	}
	unresolved := dependencyByName(t, model, "react")
	if unresolved.Resolution != DependencyUnresolved || len(unresolved.TargetProjects) != 0 {
		t.Fatalf("unresolved dependency = %+v", unresolved)
	}
}

func TestBuilderDerivesNestedAndOverlappingWorkspaces(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: ".", Kind: project.KindCode},
			{Root: "apps", Kind: project.KindCode},
			{Root: "apps/api", Kind: project.KindCode},
			{Root: "libs/shared", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "apps/package.json", Format: manifest.FormatNodePackage,
				WorkspaceMembers: []string{"api", "../libs/*"},
			},
			{
				Path: "package.json", Format: manifest.FormatNodePackage,
				WorkspaceMembers: []string{"apps/**"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Partial || len(model.Workspaces) != 2 {
		t.Fatalf("Build() = %+v", model)
	}
	api := projectByRoot(t, model, "apps/api")
	if api.PrimaryWorkspaceRoot != "apps" || len(api.WorkspaceRoots) != 2 {
		t.Fatalf("api project = %+v", api)
	}
	shared := projectByRoot(t, model, "libs/shared")
	if shared.PrimaryWorkspaceRoot != "." || len(shared.WorkspaceRoots) != 2 ||
		shared.WorkspaceRoots[0] != "." || shared.WorkspaceRoots[1] != "apps" {
		t.Fatalf("shared project = %+v", shared)
	}
}

func TestBuilderNormalizesLocalDependencyIdentities(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "python/app", Kind: project.KindCode},
			{Root: "python/lib", Kind: project.KindCode},
			{Root: "rust/app", Kind: project.KindCode},
			{Root: "rust/lib", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "python/app/pyproject.toml", Format: manifest.FormatPythonProject,
				Name: "python-app", Dependencies: []manifest.Dependency{
					{Name: "My.Lib", Scope: manifest.ScopeRuntime},
				},
			},
			{Path: "python/lib/pyproject.toml", Format: manifest.FormatPythonProject, Name: "my_lib"},
			{
				Path: "rust/app/Cargo.toml", Format: manifest.FormatRustPackage,
				Name: "rust-app", Dependencies: []manifest.Dependency{
					{Name: "shared_lib", Scope: manifest.ScopeRuntime},
				},
			},
			{Path: "rust/lib/Cargo.toml", Format: manifest.FormatRustPackage, Name: "shared-lib"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Partial || len(model.Dependencies) != 2 {
		t.Fatalf("Build() = %+v", model)
	}
	for _, dependency := range model.Dependencies {
		if dependency.Resolution != DependencyInternal || len(dependency.TargetProjects) != 1 {
			t.Errorf("dependency = %+v", dependency)
		}
	}
}

func TestBuilderReportsAmbiguousDependency(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "app", Kind: project.KindCode},
			{Root: "lib-a", Kind: project.KindCode},
			{Root: "lib-b", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "app/package.json", Format: manifest.FormatNodePackage, Name: "app",
				Dependencies: []manifest.Dependency{{Name: "shared", Scope: manifest.ScopeRuntime}},
			},
			{Path: "lib-a/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
			{Path: "lib-b/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dependency := dependencyByName(t, model, "shared")
	if !model.Partial || dependency.Resolution != DependencyAmbiguous ||
		len(dependency.TargetProjects) != 2 || !hasIssue(model, IssueDependencyAmbiguous) {
		t.Fatalf("Build() = %+v", model)
	}
}

func TestBuilderHandlesUnsafeUnsupportedAndMissingMembers(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{{Root: ".", Kind: project.KindCode}}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "package.json", Format: manifest.FormatNodePackage,
			WorkspaceMembers: []string{"../outside", "!packages/private", "missing/*"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Partial || !hasIssue(model, IssueMemberOutsideRoot) ||
		!hasIssue(model, IssueMemberUnsupported) || !hasIssue(model, IssueMemberUnmatched) {
		t.Fatalf("Build() = %+v", model)
	}
	declarations := model.Workspaces[0].Declarations
	foundOutside := false
	for _, declaration := range declarations {
		foundOutside = foundOutside || declaration.Pattern == outsideRootPattern
	}
	if !foundOutside {
		t.Fatalf("declarations = %+v", declarations)
	}
}

func TestBuilderKeepsUnmatchedMemberIndeterminateForPartialInput(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{
			Projects: []project.Project{{Root: ".", Kind: project.KindCode}},
			Partial:  true,
		},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "package.json", Format: manifest.FormatNodePackage,
			WorkspaceMembers: []string{"missing/*"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	declaration := model.Workspaces[0].Declarations[0]
	if !model.Partial || declaration.Resolution != MemberIndeterminate ||
		hasIssue(model, IssueMemberUnmatched) {
		t.Fatalf("Build() = %+v", model)
	}
}

func TestBuilderAppliesWorkspaceExclusionsAfterMatches(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: ".", Kind: project.KindCode},
			{Root: "crates/private", Kind: project.KindCode},
			{Root: "crates/public", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "Cargo.toml", Format: manifest.FormatRustPackage, Name: "workspace",
			WorkspaceMembers:  []string{"crates/**"},
			WorkspaceExcludes: []string{"crates/private"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace := model.Workspaces[0]
	if model.Partial || len(workspace.DeclaredProjects) != 1 ||
		workspace.DeclaredProjects[0] != "crates/public" ||
		len(workspace.ExcludedProjects) != 1 || workspace.ExcludedProjects[0] != "crates/private" {
		t.Fatalf("Build() = %+v", model)
	}
	if len(workspace.Declarations) != 2 || !workspace.Declarations[1].Exclude {
		t.Fatalf("declarations = %+v", workspace.Declarations)
	}
}

func TestBuilderScopesWorkspaceExclusionsToTheirManifest(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: ".", Kind: project.KindCode},
			{Root: "packages/node", Kind: project.KindCode},
			{Root: "packages/rust", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "package.json", Format: manifest.FormatNodePackage, Name: "root",
				WorkspaceMembers: []string{"packages/*"},
			},
			{
				Path: "Cargo.toml", Format: manifest.FormatRustPackage, Name: "root",
				WorkspaceMembers:  []string{"packages/*"},
				WorkspaceExcludes: []string{"packages/node"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace := model.Workspaces[0]
	if !slices.Equal(workspace.DeclaredProjects, []string{"packages/node", "packages/rust"}) ||
		!slices.Equal(workspace.ExcludedProjects, []string{"packages/node"}) {
		t.Fatalf("workspace = %+v", workspace)
	}
	if !slices.Contains(projectByRoot(t, model, "packages/node").WorkspaceRoots, ".") {
		t.Fatalf("node project was removed by another manifest's exclusion: %+v", model)
	}
}

func TestBuilderRejectsMalformedWorkspacePatternWithoutProjects(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "package.json", Format: manifest.FormatNodePackage,
			WorkspaceMembers: []string{"["},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Partial || !hasIssue(model, IssueMemberUnsupported) ||
		model.Workspaces[0].Declarations[0].Resolution != MemberUnsupported {
		t.Fatalf("Build() = %+v", model)
	}
}

func TestWorkspaceMatcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pattern   string
		candidate string
		want      bool
	}{
		{name: "exact", pattern: "packages/api", candidate: "packages/api", want: true},
		{name: "single segment", pattern: "packages/*", candidate: "packages/api", want: true},
		{name: "single segment not nested", pattern: "packages/*", candidate: "packages/a/api"},
		{name: "globstar zero segments", pattern: "packages/**", candidate: "packages", want: true},
		{name: "globstar many segments", pattern: "packages/**/api", candidate: "packages/a/b/api", want: true},
		{name: "multiple globstars", pattern: "**/api/**/test", candidate: "services/api/unit/test", want: true},
		{name: "character class", pattern: "packages/[ab]", candidate: "packages/b", want: true},
		{name: "different suffix", pattern: "**/api", candidate: "services/web"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher, valid := newWorkspaceMatcher(test.pattern)
			if !valid {
				t.Fatalf("newWorkspaceMatcher(%q) rejected a valid pattern", test.pattern)
			}
			if got := matcher.matches(test.candidate); got != test.want {
				t.Fatalf("matches(%q, %q) = %t, want %t", test.pattern, test.candidate, got, test.want)
			}
		})
	}
	if _, valid := newWorkspaceMatcher("["); valid {
		t.Fatal("newWorkspaceMatcher() accepted malformed syntax")
	}
}

func TestBuilderLimitsAreIndependentOfInputOrder(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxProjects = 1
	limits.MaxWorkspaces = 1
	limits.MaxManifests = 1
	limits.MaxNestedRepositories = 1
	builder := mustBuilder(t, limits)
	first := Snapshot{
		Projects: project.Model{
			Projects: []project.Project{
				{Root: "z", Kind: project.KindCode},
				{Root: "a", Kind: project.KindCode},
			},
			Workspaces: []project.Workspace{{Root: "z"}, {Root: "a"}},
		},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{Path: "z/package.json", Format: manifest.FormatNodePackage, Name: "z"},
			{Path: "a/package.json", Format: manifest.FormatNodePackage, Name: "a"},
		}},
	}
	second := Snapshot{
		Projects: project.Model{
			Projects:   slices.Clone(first.Projects.Projects),
			Workspaces: slices.Clone(first.Projects.Workspaces),
		},
		Manifests: manifest.Result{Manifests: slices.Clone(first.Manifests.Manifests)},
	}
	slices.Reverse(second.Projects.Projects)
	slices.Reverse(second.Projects.Workspaces)
	slices.Reverse(second.Manifests.Manifests)

	firstModel, err := builder.Build(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	secondModel, err := builder.Build(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstModel, secondModel) ||
		len(firstModel.Projects) != 1 || firstModel.Projects[0].Root != "a" ||
		len(firstModel.Workspaces) != 1 || firstModel.Workspaces[0].Root != "a" ||
		len(firstModel.Projects[0].Components) != 1 ||
		firstModel.Projects[0].Components[0].ManifestPath != "a/package.json" {
		t.Fatalf("limited models differ:\nfirst:  %+v\nsecond: %+v", firstModel, secondModel)
	}
}

func TestBuilderEnforcesAssociationLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxMatchesPerDeclaration = 1
	limits.MaxDependencies = 1
	builder := mustBuilder(t, limits)
	model, err := builder.Build(context.Background(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: ".", Kind: project.KindCode},
			{Root: "packages/a", Kind: project.KindCode},
			{Root: "packages/b", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "package.json", Format: manifest.FormatNodePackage,
				WorkspaceMembers: []string{"packages/**"},
			},
			{
				Path: "packages/a/package.json", Format: manifest.FormatNodePackage, Name: "a",
				Dependencies: []manifest.Dependency{
					{Name: "one", Scope: manifest.ScopeRuntime},
					{Name: "two", Scope: manifest.ScopeRuntime},
				},
			},
			{Path: "packages/b/package.json", Format: manifest.FormatNodePackage, Name: "b"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Partial || len(model.Dependencies) != 1 ||
		!hasIssue(model, IssueMemberMatchLimit) || !hasIssue(model, IssueDependencyLimit) {
		t.Fatalf("Build() = %+v", model)
	}
}

func TestBuilderEnforcesComponentLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxComponentsPerBoundary = 1
	builder := mustBuilder(t, limits)
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{{Root: ".", Kind: project.KindCode}}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{Path: "composer.json", Format: manifest.FormatPHPComposer},
			{Path: "package.json", Format: manifest.FormatNodePackage},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Projects) != 1 || len(model.Projects[0].Components) != 1 ||
		model.Projects[0].Components[0].ManifestPath != "composer.json" ||
		!hasIssue(model, IssueComponentLimit) {
		t.Fatalf("Build() = %+v, want first component and limit issue", model)
	}
}

func TestBuilderFinalizesExclusionsAtDeclarationLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxWorkspaceDeclarations = 2
	builder := mustBuilder(t, limits)
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: ".", Kind: project.KindCode},
			{Root: "packages/a", Kind: project.KindCode},
			{Root: "packages/b", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "package.json", Format: manifest.FormatNodePackage,
			WorkspaceMembers:  []string{"packages/*"},
			WorkspaceExcludes: []string{"packages/a", "packages/b"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace := model.Workspaces[0]
	if !slices.Equal(workspace.DeclaredProjects, []string{"packages/b"}) ||
		!slices.Equal(workspace.ExcludedProjects, []string{"packages/a"}) ||
		len(workspace.Declarations) != 2 || !hasIssue(model, IssueDeclarationLimit) {
		t.Fatalf("Build() = %+v, want finalized retained declarations", model)
	}
}

func TestBuilderBoundsAmbiguousDependencyTargetsLexically(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxTargetsPerDependency = 2
	builder := mustBuilder(t, limits)
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "app", Kind: project.KindCode},
			{Root: "packages/a", Kind: project.KindCode},
			{Root: "packages/b", Kind: project.KindCode},
			{Root: "packages/c", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "app/package.json", Format: manifest.FormatNodePackage, Name: "app",
				Dependencies: []manifest.Dependency{{Name: "shared", Scope: manifest.ScopeRuntime}},
			},
			{Path: "packages/a/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
			{Path: "packages/b/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
			{Path: "packages/c/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dependency := dependencyByName(t, model, "shared")
	if dependency.Resolution != DependencyAmbiguous || !dependency.TargetsTruncated ||
		!slices.Equal(dependency.TargetProjects, []string{"packages/a", "packages/b"}) ||
		!hasIssue(model, IssueDependencyTargetLimit) ||
		!hasIssue(model, IssueDependencyAmbiguous) {
		t.Fatalf("dependency = %+v, issues = %+v", dependency, model.Issues)
	}
}

func TestBuilderRejectsInvalidSnapshotAndCancellation(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	_, err := builder.Build(context.Background(), Snapshot{Projects: project.Model{
		Projects: []project.Project{{Root: "../outside", Kind: project.KindCode}},
	}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Build() error = %v, want ErrInvalidSnapshot", err)
	}
	_, err = builder.Build(t.Context(), Snapshot{NestedRepositories: []string{"../outside"}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Build(unsafe nested repository) error = %v, want ErrInvalidSnapshot", err)
	}
	_, err = builder.Build(t.Context(), Snapshot{
		Projects:           project.Model{Projects: []project.Project{{Root: "vendor/library", Kind: project.KindCode}}},
		NestedRepositories: []string{"vendor"},
	})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Build(overlapping project) error = %v, want ErrInvalidSnapshot", err)
	}
	_, err = builder.Build(t.Context(), Snapshot{Manifests: manifest.Result{Manifests: []manifest.Manifest{{
		Path: "go.mod", Format: manifest.FormatGoModule, WorkspaceDeclared: true,
	}}}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Build(impossible workspace) error = %v, want ErrInvalidSnapshot", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := builder.Build(ctx, Snapshot{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build() error = %v, want context.Canceled", err)
	}
}

func TestBuilderRetainsAndBoundsNestedRepositories(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxNestedRepositories = 1
	builder := mustBuilder(t, limits)
	model, err := builder.Build(t.Context(), Snapshot{
		NestedRepositories: []string{"vendor/library", "tools/external"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !model.Partial || !slices.Equal(model.NestedRepositories, []string{"tools/external"}) ||
		!hasIssue(model, IssueNestedRepositoryLimit) {
		t.Fatalf("Build() = %+v, want bounded nested repository model", model)
	}
}

func TestBuilderRejectsDuplicateRetainedKeysAtSmallLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxProjects = 1
	limits.MaxWorkspaces = 1
	limits.MaxManifests = 1
	builder := mustBuilder(t, limits)

	tests := []struct {
		name     string
		snapshot Snapshot
	}{
		{
			name: "project root",
			snapshot: Snapshot{Projects: project.Model{Projects: []project.Project{
				{Root: ".", Kind: project.KindCode},
				{Root: ".", Kind: project.KindInfrastructure},
			}}},
		},
		{
			name: "workspace root",
			snapshot: Snapshot{Projects: project.Model{Workspaces: []project.Workspace{
				{Root: "."}, {Root: ".", Markers: []project.Marker{{ID: "go-workspace"}}},
			}}},
		},
		{
			name: "manifest path",
			snapshot: Snapshot{Manifests: manifest.Result{Manifests: []manifest.Manifest{
				{Path: "package.json", Format: manifest.FormatNodePackage},
				{Path: "package.json", Format: manifest.FormatPHPComposer},
			}}},
		},
		{
			name: "nested repository",
			snapshot: Snapshot{NestedRepositories: []string{
				"vendor/library", "vendor/library",
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := builder.Build(t.Context(), test.snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("Build() error = %v, want ErrInvalidSnapshot", err)
			}
			slices.Reverse(test.snapshot.Projects.Projects)
			slices.Reverse(test.snapshot.Projects.Workspaces)
			slices.Reverse(test.snapshot.Manifests.Manifests)
			slices.Reverse(test.snapshot.NestedRepositories)
			if _, err := builder.Build(t.Context(), test.snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("reversed Build() error = %v, want ErrInvalidSnapshot", err)
			}
		})
	}
}

func TestBuilderIgnoresDuplicateKeysOutsideRetainedLimitDeterministically(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxProjects = 1
	builder := mustBuilder(t, limits)
	projects := []project.Project{
		{Root: "z", Kind: project.KindCode},
		{Root: "z", Kind: project.KindInfrastructure},
		{Root: "a", Kind: project.KindCode},
	}
	build := func(values []project.Project) Model {
		model, err := builder.Build(t.Context(), Snapshot{
			Projects: project.Model{Projects: values},
		})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		return model
	}
	first := build(slices.Clone(projects))
	slices.Reverse(projects)
	second := build(projects)
	if !reflect.DeepEqual(first, second) || len(first.Projects) != 1 || first.Projects[0].Root != "a" {
		t.Fatalf("models differ:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestBuilderIsDeterministicAndDoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	first := Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "b", Kind: project.KindCode},
			{Root: "a", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{Path: "b/package.json", Format: manifest.FormatNodePackage, Name: "b"},
			{Path: "a/package.json", Format: manifest.FormatNodePackage, Name: "a"},
		}},
	}
	second := Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "a", Kind: project.KindCode},
			{Root: "b", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{Path: "a/package.json", Format: manifest.FormatNodePackage, Name: "a"},
			{Path: "b/package.json", Format: manifest.FormatNodePackage, Name: "b"},
		}},
	}
	builder := mustBuilder(t, DefaultLimits())
	firstModel, err := builder.Build(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	secondModel, err := builder.Build(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstModel, secondModel) {
		t.Fatalf("models differ:\nfirst:  %+v\nsecond: %+v", firstModel, secondModel)
	}
	if first.Projects.Projects[0].Root != "b" || first.Manifests.Manifests[0].Path != "b/package.json" {
		t.Fatalf("Build() mutated input: %+v", first)
	}
}

func TestBuilderBoundsInheritedIssues(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxIssues = 1
	builder := mustBuilder(t, limits)
	model, err := builder.Build(context.Background(), Snapshot{
		Manifests: manifest.Result{Issues: []manifest.Issue{
			{Code: manifest.IssueParseFailed, Path: "a/package.json", Message: "first issue"},
			{Code: manifest.IssueReadFailed, Path: "b/package.json", Message: "second issue"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Partial || len(model.Issues) != 1 || model.Issues[0].Code != IssueLimit {
		t.Fatalf("Build() = %+v", model)
	}
}

func TestBuilderDeduplicatesInheritedIssuesBeforeApplyingLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxIssues = 2
	builder := mustBuilder(t, limits)
	repeated := manifest.Issue{
		Code: manifest.IssueParseFailed, Path: "a/package.json", Message: "first issue",
	}
	model, err := builder.Build(t.Context(), Snapshot{
		Manifests: manifest.Result{Issues: []manifest.Issue{
			repeated, repeated, repeated,
			{Code: manifest.IssueReadFailed, Path: "b/package.json", Message: "second issue"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Issues) != 2 || hasIssue(model, IssueLimit) {
		t.Fatalf("Issues = %#v, want two distinct inherited issues", model.Issues)
	}
}

func TestBuilderRejectsInheritedReservedIssueLimitMarker(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	_, err := builder.Build(t.Context(), Snapshot{Issues: []Issue{issueLimitMarker()}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Build() error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestBuilderDeduplicatesDependencyIdentityRoots(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{
			{Root: "app", Kind: project.KindCode},
			{Root: "shared", Kind: project.KindCode},
		}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{
			{
				Path: "app/package.json", Format: manifest.FormatNodePackage, Name: "app",
				Dependencies: []manifest.Dependency{{
					Name: "shared", Scope: manifest.ScopeRuntime,
				}},
			},
			{Path: "shared/alternate.json", Format: manifest.FormatNodePackage, Name: "shared"},
			{Path: "shared/package.json", Format: manifest.FormatNodePackage, Name: "shared"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dependency := dependencyByName(t, model, "shared")
	if dependency.Resolution != DependencyInternal ||
		!slices.Equal(dependency.TargetProjects, []string{"shared"}) {
		t.Fatalf("dependency = %#v, want one internal target", dependency)
	}
}

func TestBuilderAcceptsPublicCustomFormatGrammar(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{{Root: ".", Kind: project.KindCode}}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "project.custom", Format: manifest.Format("2custom.v1"), Name: "custom",
		}}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(model.Projects) != 1 || len(model.Projects[0].Components) != 1 ||
		model.Projects[0].Components[0].Format != "2custom.v1" {
		t.Fatalf("Build() = %#v, want retained custom component", model)
	}
}

func TestBuilderRetainsExplicitEmptyWorkspace(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{
		Projects: project.Model{Projects: []project.Project{{Root: ".", Kind: project.KindCode}}},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "Cargo.toml", Format: manifest.FormatRustPackage,
			WorkspaceDeclared: true,
		}}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(model.Workspaces) != 1 || len(model.Workspaces[0].Components) != 1 ||
		!model.Workspaces[0].Components[0].WorkspaceDeclared {
		t.Fatalf("Build() = %#v, want explicit empty workspace", model)
	}
}

func TestBoundedLexicalTopNHonorsCancellationDuringSelection(t *testing.T) {
	t.Parallel()

	values := make([]string, 512)
	for index := range values {
		values[index] = strconv.Itoa(len(values) - index)
	}
	ctx := &cancelAfterTopologyChecksContext{Context: t.Context(), remaining: 1}
	_, err := boundedLexicalTopN(ctx, values, 10, func(value string) string { return value })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("boundedLexicalTopN() error = %v, want context.Canceled", err)
	}
}

func TestManifestValidationHonorsCancellationWithinLargeCollections(t *testing.T) {
	t.Parallel()

	dependencies := make([]manifest.Dependency, 300)
	for index := range dependencies {
		dependencies[index] = manifest.Dependency{
			Name: "dependency-" + strconv.Itoa(index), Scope: manifest.ScopeRuntime,
		}
	}
	ctx := &cancelAfterTopologyChecksContext{Context: t.Context(), remaining: 1}
	err := validateManifest(ctx, manifest.Manifest{
		Path: "package.json", Format: manifest.FormatNodePackage,
		Dependencies: dependencies,
	}, DefaultLimits().MaxValueBytes)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("validateManifest() error = %v, want context.Canceled", err)
	}
}

func BenchmarkBuilder(b *testing.B) {
	const projectCount = 2_000
	projects := make([]project.Project, 0, projectCount)
	for index := range projectCount {
		root := "services/service-" + strconv.Itoa(index)
		projects = append(projects, project.Project{Root: root, Kind: project.KindCode})
	}
	builder, err := NewBuilder(DefaultLimits())
	if err != nil {
		b.Fatal(err)
	}
	snapshot := Snapshot{
		Projects: project.Model{Projects: projects},
		Manifests: manifest.Result{Manifests: []manifest.Manifest{{
			Path: "go.work", Format: manifest.FormatGoWorkspace,
			WorkspaceMembers: []string{"services/**"},
		}}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := builder.Build(context.Background(), snapshot); err != nil {
			b.Fatal(err)
		}
	}
}

func mustBuilder(t *testing.T, limits Limits) Builder {
	t.Helper()
	builder, err := NewBuilder(limits)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	return builder
}

func dependencyByName(t *testing.T, model Model, name string) Dependency {
	t.Helper()
	for _, dependency := range model.Dependencies {
		if dependency.Name == name {
			return dependency
		}
	}
	t.Fatalf("dependency %q not found", name)
	return Dependency{}
}

func projectByRoot(t *testing.T, model Model, root string) Project {
	t.Helper()
	for _, value := range model.Projects {
		if value.Root == root {
			return value
		}
	}
	t.Fatalf("project %q not found", root)
	return Project{}
}

func hasIssue(model Model, code string) bool {
	for _, issue := range model.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type cancelAfterTopologyChecksContext struct {
	context.Context
	remaining int
}

func (ctx *cancelAfterTopologyChecksContext) Err() error {
	if ctx.remaining == 0 {
		return context.Canceled
	}
	ctx.remaining--
	return nil
}
