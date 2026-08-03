package analysis

import (
	"errors"
	"testing"

	"github.com/kVinsom/Iatros/internal/topology"
)

func TestNewTopologyReportMapsStableContract(t *testing.T) {
	t.Parallel()

	model := topology.Model{
		Projects: []topology.Project{
			{
				Root: ".", Kind: "code", PrimaryWorkspaceRoot: ".",
				WorkspaceRoots: []string{"."},
				Markers: []topology.Marker{{
					ID: "node-package", Evidence: []string{"package.json"},
				}},
				Components: []topology.Component{{
					ManifestPath: "package.json", Format: "node_package", Name: "root",
					Constraints: []topology.Constraint{},
				}},
			},
			{
				Root: "packages/api", Kind: "code", PrimaryWorkspaceRoot: ".",
				WorkspaceRoots: []string{"."},
				Markers: []topology.Marker{{
					ID: "node-package", Evidence: []string{"packages/api/package.json"},
				}},
				Components: []topology.Component{{
					ManifestPath: "packages/api/package.json", Format: "node_package",
					Name: "@example/api", Constraints: []topology.Constraint{},
				}},
			},
		},
		Workspaces: []topology.Workspace{{
			Root: ".",
			Components: []topology.Component{{
				ManifestPath: "package.json", Format: "node_package", Name: "root",
				Constraints: []topology.Constraint{},
			}},
			ContainedProjects: []string{".", "packages/api"},
			DeclaredProjects:  []string{"packages/api"},
			Declarations: []topology.WorkspaceDeclaration{{
				ManifestPath: "package.json", Pattern: "packages/*",
				Resolution: topology.MemberMatched, ProjectRoots: []string{"packages/api"},
			}},
		}},
		Dependencies: []topology.Dependency{
			{
				FromProject: "packages/api", ManifestPath: "packages/api/package.json",
				Ecosystem: "node", Name: "react", Scope: "runtime",
				Resolution: topology.DependencyUnresolved, TargetProjects: []string{},
			},
			{
				FromProject: "packages/api", ManifestPath: "packages/api/package.json",
				Ecosystem: "node", Name: "root", Scope: "runtime",
				Resolution: topology.DependencyInternal, TargetProjects: []string{"."},
			},
		},
		NestedRepositories: []string{"tools/external", "vendor/library"},
		Issues: []topology.Issue{{
			Code: "IATROS_MANIFEST_PARSE_FAILED", Path: "packages/broken/package.json",
			Message: "the manifest could not be parsed safely",
		}},
		Partial: true,
	}

	report, err := NewTopologyReport(model)
	if err != nil {
		t.Fatalf("NewTopologyReport() error = %v", err)
	}
	if report.SchemaVersion != TopologySchemaVersion ||
		report.ReportType != ReportTypeRepositoryTopology || report.Status != StatusPartial {
		t.Fatalf("report envelope = %+v", report)
	}
	wantSummary := TopologySummary{
		ProjectsTotal: 2, WorkspacesTotal: 1, ComponentsTotal: 2,
		DependenciesTotal: 2, InternalDependencies: 1, UnresolvedDependencies: 1,
		NestedRepositoriesSkipped: 2,
	}
	if report.Summary != wantSummary {
		t.Fatalf("Summary = %+v, want %+v", report.Summary, wantSummary)
	}
	if len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Message != "packages/broken/package.json: the manifest could not be parsed safely" {
		t.Fatalf("Diagnostics = %+v", report.Diagnostics)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	model.Projects[0].WorkspaceRoots[0] = "changed"
	model.Dependencies[1].TargetProjects[0] = "changed"
	model.NestedRepositories[0] = "changed"
	if report.Projects[0].WorkspaceRoots[0] != "." ||
		report.Dependencies[1].TargetProjects[0] != "." ||
		report.NestedRepositories[0] != "tools/external" {
		t.Fatal("NewTopologyReport() retained caller-owned slices")
	}
}

func TestNewTopologyReportRejectsUnsafeModelIssues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		issue topology.Issue
	}{
		{
			name:  "unix absolute path",
			issue: topology.Issue{Code: "IATROS_TEST", Path: "/private/secret", Message: "unsafe issue"},
		},
		{
			name:  "windows absolute path",
			issue: topology.Issue{Code: "IATROS_TEST", Path: `C:\Users\private`, Message: "unsafe issue"},
		},
		{
			name:  "traversal path",
			issue: topology.Issue{Code: "IATROS_TEST", Path: "../secret", Message: "unsafe issue"},
		},
		{
			name:  "empty path",
			issue: topology.Issue{Code: "IATROS_TEST", Message: "unsafe issue"},
		},
		{
			name:  "unix path in message",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: "open /private/secret: access denied"},
		},
		{
			name:  "windows path in message",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: `open C:\Users\private: access denied`},
		},
		{
			name:  "control character",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: "unsafe\nissue"},
		},
		{
			name:  "UNC path in message",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: `open \\server\share: access denied`},
		},
		{
			name:  "device path in message",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: `open \\?\C:\private: access denied`},
		},
		{
			name:  "embedded drive path in message",
			issue: topology.Issue{Code: "IATROS_TEST", Path: ".", Message: `error=C:\private: access denied`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewTopologyReport(topology.Model{
				Projects: []topology.Project{}, Workspaces: []topology.Workspace{},
				Dependencies: []topology.Dependency{}, Issues: []topology.Issue{test.issue},
				Partial: true,
			})
			if !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("NewTopologyReport() error = %v, want ErrInvalidReport", err)
			}
		})
	}
}

func TestTopologyReportConstructorsAndUpstreamPartialAreValid(t *testing.T) {
	t.Parallel()

	partial, err := NewTopologyReport(topology.Model{
		Projects: []topology.Project{}, Workspaces: []topology.Workspace{},
		Dependencies: []topology.Dependency{}, Issues: []topology.Issue{}, Partial: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if partial.Status != StatusPartial || len(partial.Diagnostics) != 1 ||
		partial.Diagnostics[0].Code != DiagnosticCodeTopologyPartial {
		t.Fatalf("partial report = %+v", partial)
	}

	reports := []TopologyReport{
		partial,
		NewInvalidTopologyTargetReport(),
		NewTopologyAnalysisFailedReport(),
		NewTopologyCanceledReport(),
		NewTopologyUnavailableReport(),
	}
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			t.Fatalf("Validate() error = %v for %+v", err, report)
		}
	}
}

func TestTopologyReportAcceptsRepositoryConfinedParentPattern(t *testing.T) {
	t.Parallel()

	report, err := NewTopologyReport(topology.Model{
		Projects: []topology.Project{{
			Root: "packages/api", Kind: "code",
			WorkspaceRoots: []string{"packages/workspace"},
			Markers: []topology.Marker{{
				ID: "node-package", Evidence: []string{"packages/api/package.json"},
			}},
			Components: []topology.Component{{
				ManifestPath: "packages/api/package.json", Format: "node_package",
				Name: "api", Constraints: []topology.Constraint{},
			}},
		}},
		Workspaces: []topology.Workspace{{
			Root: "packages/workspace", ContainedProjects: []string{},
			DeclaredProjects: []string{"packages/api"}, ExcludedProjects: []string{},
			Declarations: []topology.WorkspaceDeclaration{{
				ManifestPath: "packages/workspace/package.json", Pattern: "../api",
				Resolution: topology.MemberMatched, ProjectRoots: []string{"packages/api"},
			}},
		}},
		Dependencies: []topology.Dependency{}, Issues: []topology.Issue{},
	})
	if err != nil {
		t.Fatalf("NewTopologyReport() error = %v", err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestTopologyReportValidateRejectsUnsafeOrContradictoryData(t *testing.T) {
	t.Parallel()

	base, err := NewTopologyReport(topology.Model{
		Projects: []topology.Project{{
			Root: ".", Kind: "code", WorkspaceRoots: []string{},
			Markers: []topology.Marker{{
				ID: "go-module", Evidence: []string{"go.mod"},
			}},
			Components: []topology.Component{{
				ManifestPath: "go.mod", Format: "go_module", Module: "example.com/app",
				Constraints: []topology.Constraint{},
			}},
		}},
		Workspaces: []topology.Workspace{}, Dependencies: []topology.Dependency{},
		Issues: []topology.Issue{},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*TopologyReport)
	}{
		{name: "schema", mutate: func(report *TopologyReport) { report.SchemaVersion = "9" }},
		{name: "report type", mutate: func(report *TopologyReport) { report.ReportType = "other" }},
		{name: "profile", mutate: func(report *TopologyReport) { report.Profile = "large" }},
		{name: "absolute root", mutate: func(report *TopologyReport) { report.Projects[0].Root = "/private" }},
		{name: "unsafe manifest", mutate: func(report *TopologyReport) {
			report.Projects[0].Components[0].ManifestPath = "../go.mod"
		}},
		{name: "summary mismatch", mutate: func(report *TopologyReport) { report.Summary.ComponentsTotal++ }},
		{name: "unsafe nested repository", mutate: func(report *TopologyReport) {
			report.NestedRepositories = []string{"../outside"}
			report.Summary.NestedRepositoriesSkipped = 1
		}},
		{name: "unsorted nested repositories", mutate: func(report *TopologyReport) {
			report.NestedRepositories = []string{"vendor/library", "tools/external"}
			report.Summary.NestedRepositoriesSkipped = 2
		}},
		{name: "overlapping nested repositories", mutate: func(report *TopologyReport) {
			report.NestedRepositories = []string{"packages", "packages/api"}
			report.Summary.NestedRepositoriesSkipped = 2
		}},
		{name: "nested repository contains project", mutate: func(report *TopologyReport) {
			report.Projects[0].Root = "packages/api"
			report.Projects[0].Markers[0].Evidence = []string{"packages/api/go.mod"}
			report.Projects[0].Components[0].ManifestPath = "packages/api/go.mod"
			report.NestedRepositories = []string{"packages"}
			report.Summary.NestedRepositoriesSkipped = 1
		}},
		{name: "completed diagnostic", mutate: func(report *TopologyReport) {
			report.Diagnostics = []Diagnostic{{Code: "IATROS_TEST", Level: "warning", Message: "Unexpected."}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := base.Normalized()
			test.mutate(&report)
			if err := report.Validate(); !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("Validate() error = %v, want ErrInvalidReport", err)
			}
		})
	}
}

func TestTopologyReportValidateEnforcesReciprocalRelationships(t *testing.T) {
	t.Parallel()

	base, err := NewTopologyReport(topology.Model{
		Projects: []topology.Project{
			{
				Root: ".", Kind: "code", PrimaryWorkspaceRoot: ".",
				WorkspaceRoots: []string{"."}, Markers: []topology.Marker{},
				Components: []topology.Component{},
			},
			{
				Root: "packages/api", Kind: "code", PrimaryWorkspaceRoot: ".",
				WorkspaceRoots: []string{"."}, Markers: []topology.Marker{},
				Components: []topology.Component{},
			},
		},
		Workspaces: []topology.Workspace{{
			Root: ".", Markers: []topology.Marker{}, Components: []topology.Component{},
			ContainedProjects: []string{".", "packages/api"},
			DeclaredProjects:  []string{"packages/api"},
			ExcludedProjects:  []string{},
			Declarations: []topology.WorkspaceDeclaration{{
				ManifestPath: "package.json", Pattern: "packages/*",
				Resolution: topology.MemberMatched, ProjectRoots: []string{"packages/api"},
			}},
		}},
		Dependencies: []topology.Dependency{}, Issues: []topology.Issue{},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*TopologyReport)
	}{
		{
			name: "primary workspace omits project",
			mutate: func(report *TopologyReport) {
				report.Workspaces[0].ContainedProjects = []string{"packages/api"}
			},
		},
		{
			name: "declared projects disagree with declarations",
			mutate: func(report *TopologyReport) {
				report.Workspaces[0].DeclaredProjects = []string{}
			},
		},
		{
			name: "excluded projects disagree with declarations",
			mutate: func(report *TopologyReport) {
				report.Workspaces[0].ExcludedProjects = []string{"packages/api"}
			},
		},
		{
			name: "workspace root is unexplained",
			mutate: func(report *TopologyReport) {
				report.Projects[1].WorkspaceRoots = []string{".", "other"}
				report.Workspaces = append(report.Workspaces, TopologyWorkspace{
					Root: "other", Markers: []TopologyMarker{}, Components: []TopologyComponent{},
					ContainedProjects: []string{}, DeclaredProjects: []string{},
					ExcludedProjects: []string{}, Declarations: []TopologyWorkspaceDeclaration{},
				})
			},
		},
		{
			name: "outside pattern has matched resolution",
			mutate: func(report *TopologyReport) {
				report.Workspaces[0].Declarations[0].Pattern = "[outside-root]"
			},
		},
		{
			name: "outside resolution has ordinary pattern",
			mutate: func(report *TopologyReport) {
				declaration := &report.Workspaces[0].Declarations[0]
				declaration.Resolution = string(topology.MemberOutsideRoot)
				declaration.ProjectRoots = []string{}
				report.Workspaces[0].DeclaredProjects = []string{}
			},
		},
		{
			name: "primary workspace is not nearest",
			mutate: func(report *TopologyReport) {
				report.Workspaces = append(report.Workspaces, TopologyWorkspace{
					Root: "packages", Markers: []TopologyMarker{}, Components: []TopologyComponent{},
					ContainedProjects: []string{}, DeclaredProjects: []string{},
					ExcludedProjects: []string{}, Declarations: []TopologyWorkspaceDeclaration{},
				})
			},
		},
		{
			name: "duplicate component facts disagree",
			mutate: func(report *TopologyReport) {
				report.Projects[0].Components = []TopologyComponent{{
					ManifestPath: "package.json", Format: "node_package", Name: "root",
					Constraints: []TopologyConstraint{},
				}}
				report.Workspaces[0].Components = []TopologyComponent{{
					ManifestPath: "package.json", Format: "node_package", Name: "changed",
					Constraints: []TopologyConstraint{},
				}}
			},
		},
		{
			name: "dependency source component is absent",
			mutate: func(report *TopologyReport) {
				report.Dependencies = []TopologyDependency{{
					FromProject: ".", ManifestPath: "package.json", Ecosystem: "node",
					Name: "external", Scope: "runtime", Resolution: string(topology.DependencyUnresolved),
					TargetProjects: []string{},
				}}
			},
		},
		{
			name: "go module claims workspace declaration",
			mutate: func(report *TopologyReport) {
				report.Projects[0].Components = []TopologyComponent{{
					ManifestPath: "go.mod", Format: "go_module", WorkspaceDeclared: true,
					Constraints: []TopologyConstraint{},
				}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := base.Normalized()
			test.mutate(&report)
			report.Summary = topologySummary(report)
			if err := report.Validate(); !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("Validate() error = %v, want ErrInvalidReport", err)
			}
		})
	}
}

func TestTopologyReportAllowsManifestScopedDeclaredAndExcludedOverlap(t *testing.T) {
	t.Parallel()

	report, err := NewTopologyReport(topology.Model{
		Projects: []topology.Project{{
			Root: "packages/api", Kind: "code", PrimaryWorkspaceRoot: ".",
			WorkspaceRoots: []string{"."}, Markers: []topology.Marker{},
			Components: []topology.Component{},
		}},
		Workspaces: []topology.Workspace{{
			Root: ".", Markers: []topology.Marker{}, Components: []topology.Component{},
			ContainedProjects: []string{"packages/api"},
			DeclaredProjects:  []string{"packages/api"},
			ExcludedProjects:  []string{"packages/api"},
			Declarations: []topology.WorkspaceDeclaration{
				{
					ManifestPath: "Cargo.toml", Pattern: "packages/*", Exclude: true,
					Resolution: topology.MemberMatched, ProjectRoots: []string{"packages/api"},
				},
				{
					ManifestPath: "package.json", Pattern: "packages/*",
					Resolution: topology.MemberMatched, ProjectRoots: []string{"packages/api"},
				},
			},
		}},
		Dependencies: []topology.Dependency{}, Issues: []topology.Issue{},
	})
	if err != nil {
		t.Fatalf("NewTopologyReport() error = %v", err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestTopologyReportNormalizedDeepCopiesCollections(t *testing.T) {
	t.Parallel()

	report := TopologyReport{
		Projects:     []TopologyProject{{Markers: []TopologyMarker{{}}}},
		Workspaces:   []TopologyWorkspace{{Declarations: []TopologyWorkspaceDeclaration{{}}}},
		Dependencies: []TopologyDependency{{}},
	}
	normalized := report.Normalized()
	if normalized.Projects[0].WorkspaceRoots == nil ||
		normalized.Projects[0].Markers[0].Evidence == nil ||
		normalized.Projects[0].Components == nil ||
		normalized.Workspaces[0].ContainedProjects == nil ||
		normalized.Workspaces[0].Declarations[0].ProjectRoots == nil ||
		normalized.Dependencies[0].TargetProjects == nil ||
		normalized.NestedRepositories == nil || normalized.Diagnostics == nil {
		t.Fatal("Normalized() left a nested collection nil")
	}
	if report.Projects[0].WorkspaceRoots != nil || report.Projects[0].Markers[0].Evidence != nil ||
		report.Workspaces[0].Declarations[0].ProjectRoots != nil ||
		report.Dependencies[0].TargetProjects != nil {
		t.Fatal("Normalized() mutated the input report")
	}
}
