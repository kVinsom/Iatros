package analysis

import (
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/topology"
)

const (
	// TopologySchemaVersion identifies the current repository-topology report schema.
	TopologySchemaVersion = "0.3"
	// ReportTypeRepositoryTopology identifies repository-topology reports.
	ReportTypeRepositoryTopology = "repository_topology"
)

const (
	// DiagnosticCodeTopologyCanceled identifies topology analysis stopped by cancellation.
	DiagnosticCodeTopologyCanceled = "IATROS_TOPOLOGY_CANCELLED"
	// DiagnosticCodeTopologyFailed identifies an unexpected topology analysis failure.
	DiagnosticCodeTopologyFailed = "IATROS_TOPOLOGY_FAILED"
	// DiagnosticCodeTopologyPartial identifies incomplete upstream evidence without a more specific issue.
	DiagnosticCodeTopologyPartial = "IATROS_TOPOLOGY_PARTIAL"
	// DiagnosticCodeTopologyUnavailable identifies a missing topology implementation.
	DiagnosticCodeTopologyUnavailable = "IATROS_TOPOLOGY_UNAVAILABLE"
)

// TopologyReport is the versioned repository-topology envelope shared by all output formats.
type TopologyReport struct {
	SchemaVersion      string               `json:"schema_version"`
	ReportType         string               `json:"report_type"`
	Profile            ScalingProfileName   `json:"profile"`
	Status             Status               `json:"status"`
	Target             Target               `json:"target"`
	Summary            TopologySummary      `json:"summary"`
	Projects           []TopologyProject    `json:"projects"`
	Workspaces         []TopologyWorkspace  `json:"workspaces"`
	Dependencies       []TopologyDependency `json:"dependencies"`
	NestedRepositories []string             `json:"nested_repositories"`
	Diagnostics        []Diagnostic         `json:"diagnostics"`
}

// TopologySummary contains deterministic counts for one topology report.
type TopologySummary struct {
	ProjectsTotal             int `json:"projects_total"`
	WorkspacesTotal           int `json:"workspaces_total"`
	ComponentsTotal           int `json:"components_total"`
	DependenciesTotal         int `json:"dependencies_total"`
	InternalDependencies      int `json:"internal_dependencies"`
	UnresolvedDependencies    int `json:"unresolved_dependencies"`
	AmbiguousDependencies     int `json:"ambiguous_dependencies"`
	NestedRepositoriesSkipped int `json:"nested_repositories_skipped"`
}

// TopologyMarker records one project or workspace boundary signal.
type TopologyMarker struct {
	ID                string   `json:"id"`
	Evidence          []string `json:"evidence"`
	EvidenceTruncated bool     `json:"evidence_truncated"`
}

// TopologyConstraint records a runtime, platform, or toolchain requirement.
type TopologyConstraint struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

// TopologyComponent identifies one parsed manifest associated with a project or workspace.
type TopologyComponent struct {
	ManifestPath               string               `json:"manifest_path"`
	Format                     string               `json:"format"`
	Name                       string               `json:"name"`
	Version                    string               `json:"version"`
	Module                     string               `json:"module"`
	Constraints                []TopologyConstraint `json:"constraints"`
	WorkspaceDeclared          bool                 `json:"workspace_declared"`
	DependenciesTruncated      bool                 `json:"dependencies_truncated"`
	ConstraintsTruncated       bool                 `json:"constraints_truncated"`
	WorkspaceMembersTruncated  bool                 `json:"workspace_members_truncated"`
	WorkspaceExcludesTruncated bool                 `json:"workspace_excludes_truncated"`
}

// TopologyProject is one code, infrastructure, or mixed repository boundary.
type TopologyProject struct {
	Root                 string              `json:"root"`
	Kind                 string              `json:"kind"`
	PrimaryWorkspaceRoot string              `json:"primary_workspace_root"`
	WorkspaceRoots       []string            `json:"workspace_roots"`
	Markers              []TopologyMarker    `json:"markers"`
	Components           []TopologyComponent `json:"components"`
}

// TopologyWorkspaceDeclaration records one member declaration and its known project matches.
type TopologyWorkspaceDeclaration struct {
	ManifestPath     string   `json:"manifest_path"`
	Pattern          string   `json:"pattern"`
	Exclude          bool     `json:"exclude"`
	Resolution       string   `json:"resolution"`
	ProjectRoots     []string `json:"project_roots"`
	MatchesTruncated bool     `json:"matches_truncated"`
}

// TopologyWorkspace coordinates contained or explicitly declared projects.
type TopologyWorkspace struct {
	Root              string                         `json:"root"`
	Markers           []TopologyMarker               `json:"markers"`
	Components        []TopologyComponent            `json:"components"`
	ContainedProjects []string                       `json:"contained_projects"`
	DeclaredProjects  []string                       `json:"declared_projects"`
	ExcludedProjects  []string                       `json:"excluded_projects"`
	Declarations      []TopologyWorkspaceDeclaration `json:"declarations"`
}

// TopologyDependency records one direct declaration and any repository-local targets.
type TopologyDependency struct {
	FromProject      string   `json:"from_project"`
	ManifestPath     string   `json:"manifest_path"`
	Ecosystem        string   `json:"ecosystem"`
	Name             string   `json:"name"`
	Constraint       string   `json:"constraint"`
	Scope            string   `json:"scope"`
	Indirect         bool     `json:"indirect"`
	Optional         bool     `json:"optional"`
	Resolution       string   `json:"resolution"`
	TargetProjects   []string `json:"target_projects"`
	TargetsTruncated bool     `json:"targets_truncated"`
}

// NewTopologyReport maps a provider-neutral topology model into the stable report contract.
func NewTopologyReport(model topology.Model) (TopologyReport, error) {
	if !validTopologyModelIssues(model.Issues) {
		return TopologyReport{}, ErrInvalidReport
	}
	report := newTopologyReport(StatusCompleted)
	report.Projects = copyTopologyProjects(model.Projects)
	report.Workspaces = copyTopologyWorkspaces(model.Workspaces)
	report.Dependencies = copyTopologyDependencies(model.Dependencies)
	report.NestedRepositories = copyTopologyPaths(model.NestedRepositories)
	report.Diagnostics = topologyDiagnostics(model.Issues)
	if model.Partial || len(report.Diagnostics) > 0 {
		report.Status = StatusPartial
	}
	if report.Status == StatusPartial && len(report.Diagnostics) == 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{
			Code:    DiagnosticCodeTopologyPartial,
			Level:   "warning",
			Message: "Repository topology is incomplete because upstream analysis was partial.",
		})
	}
	report.Summary = topologySummary(report)

	if err := report.Validate(); err != nil {
		return TopologyReport{}, err
	}
	return report, nil
}

// NewInvalidTopologyTargetReport creates the canonical report for a rejected topology root.
func NewInvalidTopologyTargetReport() TopologyReport {
	return NewFailedTopologyReport(Diagnostic{
		Code:  DiagnosticCodeTargetInvalid,
		Level: "error",
		Message: "The selected target is missing, inaccessible, not a directory, " +
			"or an unsupported link.",
	})
}

// NewFailedTopologyReport creates a failed topology report containing one diagnostic.
func NewFailedTopologyReport(diagnostic Diagnostic) TopologyReport {
	report := newTopologyReport(StatusFailed)
	report.Diagnostics = append(report.Diagnostics, diagnostic)
	return report
}

// NewTopologyAnalysisFailedReport creates the canonical unexpected-failure report.
func NewTopologyAnalysisFailedReport() TopologyReport {
	return NewFailedTopologyReport(Diagnostic{
		Code:    DiagnosticCodeTopologyFailed,
		Level:   "error",
		Message: "Local repository topology analysis failed.",
	})
}

// NewTopologyCanceledReport creates the canonical canceled topology report.
func NewTopologyCanceledReport() TopologyReport {
	return NewFailedTopologyReport(Diagnostic{
		Code:    DiagnosticCodeTopologyCanceled,
		Level:   "error",
		Message: "Local repository topology analysis was cancelled.",
	})
}

// NewTopologyUnavailableReport creates the canonical unavailable topology report.
func NewTopologyUnavailableReport() TopologyReport {
	return NewFailedTopologyReport(Diagnostic{
		Code:    DiagnosticCodeTopologyUnavailable,
		Level:   "error",
		Message: "Local repository topology analysis is unavailable.",
	})
}

// Normalized ensures every collection encodes as an array instead of null.
func (r TopologyReport) Normalized() TopologyReport {
	if r.Projects == nil {
		r.Projects = make([]TopologyProject, 0)
	} else {
		r.Projects = slices.Clone(r.Projects)
	}
	for index := range r.Projects {
		r.Projects[index] = normalizeTopologyProject(r.Projects[index])
	}
	if r.Workspaces == nil {
		r.Workspaces = make([]TopologyWorkspace, 0)
	} else {
		r.Workspaces = slices.Clone(r.Workspaces)
	}
	for index := range r.Workspaces {
		r.Workspaces[index] = normalizeTopologyWorkspace(r.Workspaces[index])
	}
	if r.Dependencies == nil {
		r.Dependencies = make([]TopologyDependency, 0)
	} else {
		r.Dependencies = slices.Clone(r.Dependencies)
	}
	for index := range r.Dependencies {
		if r.Dependencies[index].TargetProjects == nil {
			r.Dependencies[index].TargetProjects = make([]string, 0)
		} else {
			r.Dependencies[index].TargetProjects = slices.Clone(
				r.Dependencies[index].TargetProjects,
			)
		}
	}
	if r.NestedRepositories == nil {
		r.NestedRepositories = make([]string, 0)
	} else {
		r.NestedRepositories = slices.Clone(r.NestedRepositories)
	}
	if r.Diagnostics == nil {
		r.Diagnostics = make([]Diagnostic, 0)
	} else {
		r.Diagnostics = slices.Clone(r.Diagnostics)
	}
	return r
}

func newTopologyReport(status Status) TopologyReport {
	return TopologyReport{
		SchemaVersion: TopologySchemaVersion,
		ReportType:    ReportTypeRepositoryTopology,
		Profile:       ScalingProfileSmall,
		Status:        status,
		Target: Target{
			Kind: TargetKindLocalDirectory,
			Path: TargetRootPath,
		},
		Projects:           make([]TopologyProject, 0),
		Workspaces:         make([]TopologyWorkspace, 0),
		Dependencies:       make([]TopologyDependency, 0),
		NestedRepositories: make([]string, 0),
		Diagnostics:        make([]Diagnostic, 0),
	}
}

func copyTopologyProjects(values []topology.Project) []TopologyProject {
	projects := make([]TopologyProject, 0, len(values))
	for _, value := range values {
		projects = append(projects, TopologyProject{
			Root:                 value.Root,
			Kind:                 value.Kind,
			PrimaryWorkspaceRoot: value.PrimaryWorkspaceRoot,
			WorkspaceRoots:       slices.Clone(value.WorkspaceRoots),
			Markers:              copyTopologyMarkers(value.Markers),
			Components:           copyTopologyComponents(value.Components),
		})
	}
	return projects
}

func copyTopologyWorkspaces(values []topology.Workspace) []TopologyWorkspace {
	workspaces := make([]TopologyWorkspace, 0, len(values))
	for _, value := range values {
		declarations := make([]TopologyWorkspaceDeclaration, 0, len(value.Declarations))
		for _, declaration := range value.Declarations {
			declarations = append(declarations, TopologyWorkspaceDeclaration{
				ManifestPath:     declaration.ManifestPath,
				Pattern:          declaration.Pattern,
				Exclude:          declaration.Exclude,
				Resolution:       string(declaration.Resolution),
				ProjectRoots:     slices.Clone(declaration.ProjectRoots),
				MatchesTruncated: declaration.MatchesTruncated,
			})
		}
		workspaces = append(workspaces, TopologyWorkspace{
			Root:              value.Root,
			Markers:           copyTopologyMarkers(value.Markers),
			Components:        copyTopologyComponents(value.Components),
			ContainedProjects: slices.Clone(value.ContainedProjects),
			DeclaredProjects:  slices.Clone(value.DeclaredProjects),
			ExcludedProjects:  slices.Clone(value.ExcludedProjects),
			Declarations:      declarations,
		})
	}
	return workspaces
}

func copyTopologyDependencies(values []topology.Dependency) []TopologyDependency {
	dependencies := make([]TopologyDependency, 0, len(values))
	for _, value := range values {
		dependencies = append(dependencies, TopologyDependency{
			FromProject:      value.FromProject,
			ManifestPath:     value.ManifestPath,
			Ecosystem:        value.Ecosystem,
			Name:             value.Name,
			Constraint:       value.Constraint,
			Scope:            value.Scope,
			Indirect:         value.Indirect,
			Optional:         value.Optional,
			Resolution:       string(value.Resolution),
			TargetProjects:   slices.Clone(value.TargetProjects),
			TargetsTruncated: value.TargetsTruncated,
		})
	}
	return dependencies
}

func copyTopologyPaths(values []string) []string {
	if values == nil {
		return make([]string, 0)
	}
	return slices.Clone(values)
}

func copyTopologyMarkers(values []topology.Marker) []TopologyMarker {
	markers := make([]TopologyMarker, 0, len(values))
	for _, value := range values {
		markers = append(markers, TopologyMarker{
			ID:                value.ID,
			Evidence:          slices.Clone(value.Evidence),
			EvidenceTruncated: value.EvidenceTruncated,
		})
	}
	return markers
}

func copyTopologyComponents(values []topology.Component) []TopologyComponent {
	components := make([]TopologyComponent, 0, len(values))
	for _, value := range values {
		constraints := make([]TopologyConstraint, 0, len(value.Constraints))
		for _, constraint := range value.Constraints {
			constraints = append(constraints, TopologyConstraint{
				Name: constraint.Name, Value: constraint.Value, Scope: constraint.Scope,
			})
		}
		components = append(components, TopologyComponent{
			ManifestPath:               value.ManifestPath,
			Format:                     value.Format,
			Name:                       value.Name,
			Version:                    value.Version,
			Module:                     value.Module,
			Constraints:                constraints,
			WorkspaceDeclared:          value.WorkspaceDeclared,
			DependenciesTruncated:      value.DependenciesTruncated,
			ConstraintsTruncated:       value.ConstraintsTruncated,
			WorkspaceMembersTruncated:  value.WorkspaceMembersTruncated,
			WorkspaceExcludesTruncated: value.WorkspaceExcludesTruncated,
		})
	}
	return components
}

func topologyDiagnostics(issues []topology.Issue) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		message := issue.Message
		if issue.Path != "." {
			message = issue.Path + ": " + message
		}
		diagnostics = append(diagnostics, Diagnostic{
			Code: issue.Code, Level: "warning", Message: message,
		})
	}
	slices.SortFunc(diagnostics, func(left, right Diagnostic) int {
		if compared := strings.Compare(left.Code, right.Code); compared != 0 {
			return compared
		}
		return strings.Compare(left.Message, right.Message)
	})
	return slices.CompactFunc(diagnostics, func(left, right Diagnostic) bool {
		return left == right
	})
}

func validTopologyModelIssues(issues []topology.Issue) bool {
	for _, issue := range issues {
		if !validModelIssue(issue.Code, issue.Path, issue.Message) {
			return false
		}
	}
	return true
}

func topologySummary(report TopologyReport) TopologySummary {
	type componentKey struct {
		format string
		path   string
	}
	componentKeys := make(map[componentKey]struct{})
	for _, project := range report.Projects {
		for _, component := range project.Components {
			componentKeys[componentKey{format: component.Format, path: component.ManifestPath}] = struct{}{}
		}
	}
	for _, workspace := range report.Workspaces {
		for _, component := range workspace.Components {
			componentKeys[componentKey{format: component.Format, path: component.ManifestPath}] = struct{}{}
		}
	}

	summary := TopologySummary{
		ProjectsTotal:             len(report.Projects),
		WorkspacesTotal:           len(report.Workspaces),
		ComponentsTotal:           len(componentKeys),
		DependenciesTotal:         len(report.Dependencies),
		NestedRepositoriesSkipped: len(report.NestedRepositories),
	}
	for _, dependency := range report.Dependencies {
		switch dependency.Resolution {
		case string(topology.DependencyInternal):
			summary.InternalDependencies++
		case string(topology.DependencyUnresolved):
			summary.UnresolvedDependencies++
		case string(topology.DependencyAmbiguous):
			summary.AmbiguousDependencies++
		}
	}
	return summary
}

func normalizeTopologyProject(value TopologyProject) TopologyProject {
	if value.WorkspaceRoots == nil {
		value.WorkspaceRoots = make([]string, 0)
	} else {
		value.WorkspaceRoots = slices.Clone(value.WorkspaceRoots)
	}
	value.Markers = normalizeTopologyMarkers(value.Markers)
	value.Components = normalizeTopologyComponents(value.Components)
	return value
}

func normalizeTopologyWorkspace(value TopologyWorkspace) TopologyWorkspace {
	value.Markers = normalizeTopologyMarkers(value.Markers)
	value.Components = normalizeTopologyComponents(value.Components)
	if value.ContainedProjects == nil {
		value.ContainedProjects = make([]string, 0)
	} else {
		value.ContainedProjects = slices.Clone(value.ContainedProjects)
	}
	if value.DeclaredProjects == nil {
		value.DeclaredProjects = make([]string, 0)
	} else {
		value.DeclaredProjects = slices.Clone(value.DeclaredProjects)
	}
	if value.ExcludedProjects == nil {
		value.ExcludedProjects = make([]string, 0)
	} else {
		value.ExcludedProjects = slices.Clone(value.ExcludedProjects)
	}
	if value.Declarations == nil {
		value.Declarations = make([]TopologyWorkspaceDeclaration, 0)
	} else {
		value.Declarations = slices.Clone(value.Declarations)
	}
	for index := range value.Declarations {
		if value.Declarations[index].ProjectRoots == nil {
			value.Declarations[index].ProjectRoots = make([]string, 0)
		} else {
			value.Declarations[index].ProjectRoots = slices.Clone(
				value.Declarations[index].ProjectRoots,
			)
		}
	}
	return value
}

func normalizeTopologyMarkers(values []TopologyMarker) []TopologyMarker {
	if values == nil {
		return make([]TopologyMarker, 0)
	}
	values = slices.Clone(values)
	for index := range values {
		if values[index].Evidence == nil {
			values[index].Evidence = make([]string, 0)
		} else {
			values[index].Evidence = slices.Clone(values[index].Evidence)
		}
	}
	return values
}

func normalizeTopologyComponents(values []TopologyComponent) []TopologyComponent {
	if values == nil {
		return make([]TopologyComponent, 0)
	}
	values = slices.Clone(values)
	for index := range values {
		if values[index].Constraints == nil {
			values[index].Constraints = make([]TopologyConstraint, 0)
		} else {
			values[index].Constraints = slices.Clone(values[index].Constraints)
		}
	}
	return values
}
