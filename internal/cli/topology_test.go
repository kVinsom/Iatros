package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
	"github.com/kVinsom/Iatros/internal/topology"
)

func TestTopologyLocalRepositoryJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyCLIFixture(t, root, "package.json", `{
  "name": "root",
  "workspaces": ["packages/*"]
}`)
	writeTopologyCLIFixture(t, root, "packages/api/package.json", `{
  "name": "@example/api",
  "dependencies": {"@example/shared": "workspace:*", "react": "^19"}
}`)
	writeTopologyCLIFixture(t, root, "packages/shared/package.json", `{
  "name": "@example/shared"
}`)
	analyzer, err := analysis.NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatal(err)
	}

	result := runTopologyCLI(t, analyzer, "topology", "--format=json", root)
	report := decodeTopologyReport(t, result.stdout)
	if result.exitCode != ExitSuccess || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q", result.exitCode, result.stderr)
	}
	if report.SchemaVersion != analysis.TopologySchemaVersion ||
		report.ReportType != analysis.ReportTypeRepositoryTopology ||
		report.Status != analysis.StatusCompleted {
		t.Fatalf("report envelope = %+v", report)
	}
	if report.Summary.ProjectsTotal != 3 || report.Summary.WorkspacesTotal != 1 ||
		report.Summary.ComponentsTotal != 3 || report.Summary.DependenciesTotal != 2 ||
		report.Summary.InternalDependencies != 1 || report.Summary.UnresolvedDependencies != 1 {
		t.Fatalf("Summary = %+v", report.Summary)
	}
	if report.Projects == nil || report.Workspaces == nil || report.Dependencies == nil ||
		report.Diagnostics == nil || report.Projects[0].Components == nil ||
		report.Workspaces[0].Declarations == nil {
		t.Fatal("JSON collections must be arrays, not null")
	}
	if strings.Contains(result.stdout, ": null") {
		t.Fatal("topology JSON contains a null field")
	}
	if strings.Contains(result.stdout, filepath.ToSlash(root)) {
		t.Fatal("topology report exposed the absolute analysis root")
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	second := runTopologyCLI(t, analyzer, "topology", root, "--format=json")
	if second.exitCode != ExitSuccess || second.stdout != result.stdout || second.stderr != "" {
		t.Fatalf("topology JSON is not deterministic:\nfirst:\n%s\nsecond:\n%s", result.stdout, second.stdout)
	}
}

func TestTopologyPartialReportExitsSuccessfully(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTopologyCLIFixture(t, root, "package.json", `{"name":"first","name":"second"}`)
	analyzer, err := analysis.NewLocalTopologyAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	result := runTopologyCLI(t, analyzer, "topology", "--format", "json", root)
	report := decodeTopologyReport(t, result.stdout)
	if result.exitCode != ExitSuccess || report.Status != analysis.StatusPartial ||
		len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != "IATROS_MANIFEST_PARSE_FAILED" || result.stderr != "" {
		t.Fatalf("Run() = exit %d, report %+v, stderr %q", result.exitCode, report, result.stderr)
	}
}

func TestTopologyJSONNormalizesNilCollections(t *testing.T) {
	t.Parallel()

	analyzer := topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		return topology.Model{
			Projects: []topology.Project{{Root: ".", Kind: "code"}},
		}, nil
	})
	result := runTopologyCLI(t, analyzer, "topology", "--format=json")

	if result.exitCode != ExitSuccess || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q", result.exitCode, result.stderr)
	}
	if strings.Contains(result.stdout, ": null") {
		t.Fatalf("topology JSON contains a null collection:\n%s", result.stdout)
	}
}

func TestTopologyTextContainsCompleteModel(t *testing.T) {
	t.Parallel()

	model := topology.Model{
		Projects: []topology.Project{{
			Root: ".", Kind: "code", PrimaryWorkspaceRoot: ".", WorkspaceRoots: []string{"."},
			Markers: []topology.Marker{{ID: "go-module", Evidence: []string{"go.mod"}}},
			Components: []topology.Component{{
				ManifestPath: "go.mod", Format: "go_module", Module: "example.com/app",
				Constraints: []topology.Constraint{{Name: "go", Value: "1.25", Scope: "runtime"}},
			}},
		}},
		Workspaces: []topology.Workspace{{
			Root: ".", Markers: []topology.Marker{},
			Components: []topology.Component{{
				ManifestPath: "go.work", Format: "go_workspace", WorkspaceDeclared: true,
				Constraints: []topology.Constraint{},
			}},
			ContainedProjects: []string{"."}, DeclaredProjects: []string{"."},
			ExcludedProjects: []string{},
			Declarations: []topology.WorkspaceDeclaration{{
				ManifestPath: "go.work", Pattern: ".", Resolution: topology.MemberMatched,
				ProjectRoots: []string{"."},
			}},
		}},
		Dependencies: []topology.Dependency{{
			FromProject: ".", ManifestPath: "go.mod", Ecosystem: "go",
			Name: "example.com/lib", Constraint: "v1.0.0", Scope: "runtime",
			Resolution: topology.DependencyUnresolved, TargetProjects: []string{},
		}},
		Issues: []topology.Issue{},
	}
	result := runTopologyCLI(t, topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		return model, nil
	}), "topology")
	if result.exitCode != ExitSuccess || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q", result.exitCode, result.stderr)
	}
	for _, expected := range []string{
		"IATROS Repository Topology", "Schema version: 0.1",
		"Report type: repository_topology", "Status: completed",
		"Target kind: local_directory", "Target: .",
		"Projects total: 1", "Root: .", "ID: go-module", "Manifest path: go.mod",
		"Module: example.com/app", "Workspace declared: false", "Name: go", "Value: 1.25",
		"Workspaces total: 1", "Contained projects:", "Declared projects:",
		"Declarations:", "Pattern: .", "Workspace declared: true",
		"Dependencies total: 1", "Name: example.com/lib", "Resolution: unresolved",
		"Targets truncated: false",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("text report does not contain %q:\n%s", expected, result.stdout)
		}
	}
}

func TestTopologyFailuresProduceSafeReports(t *testing.T) {
	t.Parallel()

	privateRoot := filepath.Join(t.TempDir(), "private-topology-target")
	malformed := topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		return topology.Model{
			Projects: []topology.Project{{Root: "/private/secret", Kind: "code"}},
		}, nil
	})
	malformedIssue := topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		return topology.Model{
			Projects: []topology.Project{}, Workspaces: []topology.Workspace{},
			Dependencies: []topology.Dependency{}, Partial: true,
			Issues: []topology.Issue{{
				Code: "IATROS_TEST", Path: "/private/issue-secret", Message: "unsafe issue",
			}},
		}, nil
	})

	tests := []struct {
		name      string
		analyzer  TopologyAnalyzer
		args      []string
		wantExit  int
		wantCode  string
		forbidden string
	}{
		{
			name: "invalid target", analyzer: topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
				return topology.Model{}, analysis.ErrInvalidTarget
			}),
			args:     []string{"topology", "--format=json", privateRoot},
			wantExit: ExitInvalidTarget, wantCode: analysis.DiagnosticCodeTargetInvalid,
			forbidden: "private-topology-target",
		},
		{
			name: "unavailable", analyzer: nil, args: []string{"topology", "--format=json"},
			wantExit: ExitFailure, wantCode: analysis.DiagnosticCodeTopologyUnavailable,
		},
		{
			name: "malformed model", analyzer: malformed, args: []string{"topology", "--format=json"},
			wantExit: ExitFailure, wantCode: analysis.DiagnosticCodeTopologyFailed,
			forbidden: "secret",
		},
		{
			name: "malformed issue", analyzer: malformedIssue, args: []string{"topology", "--format=json"},
			wantExit: ExitFailure, wantCode: analysis.DiagnosticCodeTopologyFailed,
			forbidden: "issue-secret",
		},
		{
			name: "internal deadline", analyzer: topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
				return topology.Model{}, context.DeadlineExceeded
			}),
			args:     []string{"topology", "--format=json"},
			wantExit: ExitFailure, wantCode: analysis.DiagnosticCodeTopologyFailed,
		},
		{
			name: "analyzer cancellation", analyzer: topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
				return topology.Model{}, context.Canceled
			}),
			args:     []string{"topology", "--format=json"},
			wantExit: ExitFailure, wantCode: analysis.DiagnosticCodeTopologyCanceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runTopologyCLI(t, test.analyzer, test.args...)
			report := decodeTopologyReport(t, result.stdout)
			if result.exitCode != test.wantExit || report.Status != analysis.StatusFailed ||
				len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != test.wantCode ||
				result.stderr != "" {
				t.Fatalf("Run() = exit %d, report %+v, stderr %q", result.exitCode, report, result.stderr)
			}
			if test.forbidden != "" && strings.Contains(result.stdout, test.forbidden) {
				t.Fatalf("report exposed %q", test.forbidden)
			}
		})
	}
}

func TestTopologyCancellationAndOutputFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	analyzer := topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		called = true
		return topology.Model{}, nil
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(
		ctx,
		"test-version",
		Services{Topology: analyzer},
		[]string{"topology", "--format=json"},
		&stdout,
		&stderr,
	)
	report := decodeTopologyReport(t, stdout.String())
	if exitCode != ExitFailure || called || len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != analysis.DiagnosticCodeTopologyCanceled || stderr.Len() != 0 {
		t.Fatalf("Run() = exit %d, called %t, report %+v, stderr %q", exitCode, called, report, stderr.String())
	}

	stageCtx, cancelStage := context.WithCancel(t.Context())
	stdout.Reset()
	stderr.Reset()
	exitCode = Run(
		stageCtx,
		"test-version",
		Services{Topology: topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
			cancelStage()
			return topology.Model{}, nil
		})},
		[]string{"topology", "--format=json"},
		&stdout,
		&stderr,
	)
	report = decodeTopologyReport(t, stdout.String())
	if exitCode != ExitFailure || len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != analysis.DiagnosticCodeTopologyCanceled || stderr.Len() != 0 {
		t.Fatalf("stage cancellation = exit %d, report %+v, stderr %q", exitCode, report, stderr.String())
	}

	stderr.Reset()
	exitCode = Run(
		t.Context(),
		"test-version",
		Services{Topology: topologyAnalyzerFunc(emptyTopologyModel)},
		[]string{"topology"},
		failingWriter{err: errors.New("write failed")},
		&stderr,
	)
	if exitCode != ExitFailure || !strings.Contains(stderr.String(), "could not write topology report") {
		t.Fatalf("output failure = exit %d, stderr %q", exitCode, stderr.String())
	}
}

func runTopologyCLI(t *testing.T, analyzer TopologyAnalyzer, args ...string) commandResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(
		t.Context(),
		"test-version",
		Services{Topology: analyzer},
		args,
		&stdout,
		&stderr,
	)
	return commandResult{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}

func decodeTopologyReport(t *testing.T, output string) analysis.TopologyReport {
	t.Helper()
	var report analysis.TopologyReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output)
	}
	return report
}

func writeTopologyCLIFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func emptyTopologyModel(context.Context, analysis.Request) (topology.Model, error) {
	return topology.Model{
		Projects: []topology.Project{}, Workspaces: []topology.Workspace{},
		Dependencies: []topology.Dependency{}, Issues: []topology.Issue{},
	}, nil
}

type topologyAnalyzerFunc func(context.Context, analysis.Request) (topology.Model, error)

func (function topologyAnalyzerFunc) Analyze(
	ctx context.Context,
	request analysis.Request,
) (topology.Model, error) {
	return function(ctx, request)
}
