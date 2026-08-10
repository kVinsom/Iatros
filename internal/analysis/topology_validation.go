package analysis

import (
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/repositorypath"
	"github.com/kVinsom/Iatros/internal/topology"
)

// Validate checks the semantic invariants required by the topology report contract.
func (r TopologyReport) Validate() error {
	if r.SchemaVersion != TopologySchemaVersion ||
		r.ReportType != ReportTypeRepositoryTopology ||
		!validScalingProfileName(r.Profile) ||
		r.Target.Kind != TargetKindLocalDirectory ||
		r.Target.Path != TargetRootPath ||
		!validTopologyStatus(r.Status) ||
		!validTopologyData(r) ||
		!validTopologyDiagnostics(r.Status, r.Diagnostics) {
		return ErrInvalidReport
	}
	return nil
}

func validTopologyStatus(status Status) bool {
	return status == StatusCompleted || status == StatusPartial || status == StatusFailed
}

func validTopologyData(report TopologyReport) bool {
	if report.Status == StatusFailed {
		return report.Summary == (TopologySummary{}) && len(report.Projects) == 0 &&
			len(report.Workspaces) == 0 && len(report.Dependencies) == 0 &&
			len(report.NestedRepositories) == 0
	}
	if !validTopologyProjects(report.Projects) || !validTopologyWorkspaces(report.Workspaces) ||
		!validTopologyDependencies(report.Dependencies) ||
		!validSortedRelativePaths(report.NestedRepositories) ||
		!validNestedRepositoryBoundaries(report) ||
		!validTopologyRelationships(report) {
		return false
	}
	return report.Summary == topologySummary(report)
}

func validNestedRepositoryBoundaries(report TopologyReport) bool {
	boundaries := make(map[string]struct{}, len(report.NestedRepositories))
	for _, root := range report.NestedRepositories {
		for ancestor := path.Dir(root); ancestor != "."; ancestor = path.Dir(ancestor) {
			if _, nested := boundaries[ancestor]; nested {
				return false
			}
		}
		boundaries[root] = struct{}{}
	}
	for _, project := range report.Projects {
		for ancestor := project.Root; ancestor != "."; ancestor = path.Dir(ancestor) {
			if _, nested := boundaries[ancestor]; nested {
				return false
			}
		}
	}
	for _, workspace := range report.Workspaces {
		for ancestor := workspace.Root; ancestor != "."; ancestor = path.Dir(ancestor) {
			if _, nested := boundaries[ancestor]; nested {
				return false
			}
		}
	}
	return true
}

func validTopologyProjects(values []TopologyProject) bool {
	for index, value := range values {
		if !validTopologyPath(value.Root) || !validTopologyKind(value.Kind) ||
			!validOptionalTopologyPath(value.PrimaryWorkspaceRoot) ||
			!validSortedTopologyPaths(value.WorkspaceRoots) ||
			(value.PrimaryWorkspaceRoot != "" &&
				(!slices.Contains(value.WorkspaceRoots, value.PrimaryWorkspaceRoot) ||
					!repositorypath.Contains(value.PrimaryWorkspaceRoot, value.Root))) ||
			!validTopologyMarkers(value.Markers) || !validTopologyComponents(value.Components) ||
			(index > 0 && values[index-1].Root >= value.Root) {
			return false
		}
	}
	return true
}

func validTopologyWorkspaces(values []TopologyWorkspace) bool {
	for index, value := range values {
		if !validTopologyPath(value.Root) || !validTopologyMarkers(value.Markers) ||
			!validTopologyComponents(value.Components) ||
			!validSortedTopologyPaths(value.ContainedProjects) ||
			!validSortedTopologyPaths(value.DeclaredProjects) ||
			!validSortedTopologyPaths(value.ExcludedProjects) ||
			!validTopologyDeclarations(value.Root, value.Declarations) ||
			(index > 0 && values[index-1].Root >= value.Root) {
			return false
		}
	}
	return true
}

func validTopologyDependencies(values []TopologyDependency) bool {
	for index, value := range values {
		if !validTopologyPath(value.FromProject) || !validRelativePath(value.ManifestPath) ||
			!validLowerIdentifier(value.Ecosystem, "-_.") || !validText(value.Name) ||
			!validOptionalTopologyText(value.Constraint) || !validTopologyScope(value.Scope) ||
			!validDependencyResolution(value) || !validSortedTopologyPaths(value.TargetProjects) ||
			(index > 0 && compareTopologyDependencies(values[index-1], value) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyMarkers(values []TopologyMarker) bool {
	for index, value := range values {
		if !validLowerIdentifier(value.ID, "-_.") || len(value.Evidence) == 0 ||
			!validSortedRelativePaths(value.Evidence) ||
			(index > 0 && values[index-1].ID >= value.ID) {
			return false
		}
	}
	return true
}

func validTopologyComponents(values []TopologyComponent) bool {
	for index, value := range values {
		if !validRelativePath(value.ManifestPath) ||
			!validLowerIdentifier(value.Format, "-_.") ||
			!validWorkspaceDeclaredState(value) ||
			!validOptionalTopologyText(value.Name) ||
			!validOptionalTopologyText(value.Version) ||
			!validOptionalTopologyText(value.Module) ||
			!validTopologyConstraints(value.Constraints) ||
			(index > 0 && values[index-1].ManifestPath >= value.ManifestPath) {
			return false
		}
	}
	return true
}

func validTopologyConstraints(values []TopologyConstraint) bool {
	for index, value := range values {
		if !validText(value.Name) || !validOptionalTopologyText(value.Value) ||
			!validTopologyConstraintScope(value.Scope) ||
			(index > 0 && compareTopologyConstraints(values[index-1], value) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyDeclarations(
	workspaceRoot string,
	values []TopologyWorkspaceDeclaration,
) bool {
	for index, value := range values {
		if !validRelativePath(value.ManifestPath) ||
			!validTopologyPattern(workspaceRoot, value.Pattern) ||
			!validMemberResolution(value.Resolution) ||
			((value.Pattern == "[outside-root]") !=
				(value.Resolution == string(topology.MemberOutsideRoot))) ||
			!validSortedTopologyPaths(value.ProjectRoots) ||
			!validMemberTargets(value) ||
			(index > 0 && compareTopologyDeclarations(values[index-1], value) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyDiagnostics(status Status, values []Diagnostic) bool {
	if status == StatusCompleted && len(values) != 0 {
		return false
	}
	if status != StatusCompleted && len(values) == 0 {
		return false
	}
	for index, value := range values {
		if !validDiagnosticCode(value.Code) || !validDiagnosticLevel(value.Level) ||
			!validPublicMessage(value.Message) ||
			(index > 0 && compareTopologyDiagnostics(values[index-1], value) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyRelationships(report TopologyReport) bool {
	projects := make(map[string]TopologyProject, len(report.Projects))
	expectedWorkspaceRoots := make(map[string]map[string]struct{}, len(report.Projects))
	projectManifestPaths := make(map[string]map[string]struct{}, len(report.Projects))
	componentsByPath := make(map[string]TopologyComponent)
	for _, value := range report.Projects {
		projects[value.Root] = value
		expectedWorkspaceRoots[value.Root] = make(map[string]struct{})
		projectManifestPaths[value.Root] = make(map[string]struct{}, len(value.Components))
		for _, component := range value.Components {
			if path.Dir(component.ManifestPath) != value.Root {
				return false
			}
			projectManifestPaths[value.Root][component.ManifestPath] = struct{}{}
			if !recordTopologyComponent(componentsByPath, component) {
				return false
			}
		}
	}
	workspaces := make(map[string]TopologyWorkspace, len(report.Workspaces))
	for _, value := range report.Workspaces {
		workspaces[value.Root] = value
	}

	expectedContained := make(map[string]map[string]struct{}, len(workspaces))
	for root := range workspaces {
		expectedContained[root] = make(map[string]struct{})
	}
	for _, value := range report.Projects {
		if value.PrimaryWorkspaceRoot == "" {
			continue
		}
		if _, exists := workspaces[value.PrimaryWorkspaceRoot]; !exists {
			return false
		}
		expectedContained[value.PrimaryWorkspaceRoot][value.Root] = struct{}{}
		expectedWorkspaceRoots[value.Root][value.PrimaryWorkspaceRoot] = struct{}{}
	}

	for _, workspace := range report.Workspaces {
		for _, component := range workspace.Components {
			if path.Dir(component.ManifestPath) != workspace.Root {
				return false
			}
			if !recordTopologyComponent(componentsByPath, component) {
				return false
			}
		}
		if !topologyPathsEqualSet(workspace.ContainedProjects, expectedContained[workspace.Root]) {
			return false
		}

		type manifestMembership struct {
			included map[string]struct{}
			excluded map[string]struct{}
		}
		memberships := make(map[string]*manifestMembership)
		expectedExcluded := make(map[string]struct{})
		for _, declaration := range workspace.Declarations {
			if path.Dir(declaration.ManifestPath) != workspace.Root ||
				!allTopologyProjectsExist(declaration.ProjectRoots, projects) {
				return false
			}
			membership := memberships[declaration.ManifestPath]
			if membership == nil {
				membership = &manifestMembership{
					included: make(map[string]struct{}),
					excluded: make(map[string]struct{}),
				}
				memberships[declaration.ManifestPath] = membership
			}
			for _, projectRoot := range declaration.ProjectRoots {
				if declaration.Exclude {
					membership.excluded[projectRoot] = struct{}{}
					expectedExcluded[projectRoot] = struct{}{}
					continue
				}
				membership.included[projectRoot] = struct{}{}
			}
		}
		expectedDeclared := make(map[string]struct{})
		for _, membership := range memberships {
			for projectRoot := range membership.included {
				if _, excluded := membership.excluded[projectRoot]; !excluded {
					expectedDeclared[projectRoot] = struct{}{}
				}
			}
		}
		if !topologyPathsEqualSet(workspace.DeclaredProjects, expectedDeclared) ||
			!topologyPathsEqualSet(workspace.ExcludedProjects, expectedExcluded) {
			return false
		}
		for projectRoot := range expectedDeclared {
			expectedWorkspaceRoots[projectRoot][workspace.Root] = struct{}{}
		}
	}
	for _, value := range report.Projects {
		if value.PrimaryWorkspaceRoot != nearestTopologyWorkspace(value.Root, workspaces) {
			return false
		}
		if !topologyPathsEqualSet(value.WorkspaceRoots, expectedWorkspaceRoots[value.Root]) {
			return false
		}
	}
	for _, dependency := range report.Dependencies {
		if _, exists := projects[dependency.FromProject]; !exists {
			return false
		}
		if path.Dir(dependency.ManifestPath) != dependency.FromProject ||
			!allTopologyProjectsExist(dependency.TargetProjects, projects) {
			return false
		}
		if _, exists := projectManifestPaths[dependency.FromProject][dependency.ManifestPath]; !exists {
			return false
		}
	}
	return true
}

func validWorkspaceDeclaredState(value TopologyComponent) bool {
	switch value.Format {
	case string(manifest.FormatGoWorkspace):
		return value.WorkspaceDeclared
	case string(manifest.FormatGoModule), string(manifest.FormatPythonProject),
		string(manifest.FormatPHPComposer):
		return !value.WorkspaceDeclared
	default:
		return true
	}
}

func recordTopologyComponent(
	components map[string]TopologyComponent,
	value TopologyComponent,
) bool {
	existing, found := components[value.ManifestPath]
	if !found {
		components[value.ManifestPath] = value
		return true
	}
	return existing.ManifestPath == value.ManifestPath && existing.Format == value.Format &&
		existing.Name == value.Name && existing.Version == value.Version &&
		existing.Module == value.Module &&
		existing.WorkspaceDeclared == value.WorkspaceDeclared &&
		existing.DependenciesTruncated == value.DependenciesTruncated &&
		existing.ConstraintsTruncated == value.ConstraintsTruncated &&
		existing.WorkspaceMembersTruncated == value.WorkspaceMembersTruncated &&
		existing.WorkspaceExcludesTruncated == value.WorkspaceExcludesTruncated &&
		slices.Equal(existing.Constraints, value.Constraints)
}

func nearestTopologyWorkspace(
	projectRoot string,
	workspaces map[string]TopologyWorkspace,
) string {
	for candidate := projectRoot; ; candidate = path.Dir(candidate) {
		if _, exists := workspaces[candidate]; exists {
			return candidate
		}
		if candidate == "." {
			return ""
		}
	}
}

func allTopologyProjectsExist(values []string, known map[string]TopologyProject) bool {
	for _, value := range values {
		if _, exists := known[value]; !exists {
			return false
		}
	}
	return true
}

func topologyPathsEqualSet(values []string, expected map[string]struct{}) bool {
	if len(values) != len(expected) {
		return false
	}
	for _, value := range values {
		if _, exists := expected[value]; !exists {
			return false
		}
	}
	return true
}

func validTopologyKind(value string) bool {
	return value == "code" || value == "infrastructure" || value == "mixed"
}

func validTopologyScope(value string) bool {
	return value == "runtime" || value == "development" || value == "build" || value == "peer"
}

func validTopologyConstraintScope(value string) bool {
	return value == "runtime" || value == "development" || value == "build"
}

func validDependencyResolution(value TopologyDependency) bool {
	switch value.Resolution {
	case string(topology.DependencyInternal):
		return len(value.TargetProjects) == 1 && !value.TargetsTruncated
	case string(topology.DependencyUnresolved):
		return len(value.TargetProjects) == 0 && !value.TargetsTruncated
	case string(topology.DependencyAmbiguous):
		return len(value.TargetProjects) >= 2 ||
			(value.TargetsTruncated && len(value.TargetProjects) >= 1)
	default:
		return false
	}
}

func validMemberResolution(value string) bool {
	switch value {
	case string(topology.MemberMatched), string(topology.MemberUnmatched),
		string(topology.MemberIndeterminate), string(topology.MemberUnsupported),
		string(topology.MemberOutsideRoot):
		return true
	default:
		return false
	}
}

func validMemberTargets(value TopologyWorkspaceDeclaration) bool {
	switch value.Resolution {
	case string(topology.MemberMatched):
		return len(value.ProjectRoots) > 0
	case string(topology.MemberUnmatched), string(topology.MemberIndeterminate),
		string(topology.MemberUnsupported), string(topology.MemberOutsideRoot):
		return len(value.ProjectRoots) == 0 && !value.MatchesTruncated
	default:
		return false
	}
}

func validTopologyPath(value string) bool {
	return value == "." || validRelativePath(value)
}

func validOptionalTopologyPath(value string) bool {
	return value == "" || validTopologyPath(value)
}

func validSortedTopologyPaths(values []string) bool {
	if !strictlySortedStrings(values) {
		return false
	}
	for _, value := range values {
		if !validTopologyPath(value) {
			return false
		}
	}
	return true
}

func validTopologyPattern(workspaceRoot, value string) bool {
	if value == "[outside-root]" {
		return true
	}
	lower := strings.ToLower(value)
	if !validText(value) || strings.Contains(value, "\\") || path.IsAbs(value) ||
		repositorypath.HasWindowsDrivePrefix(value) || strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "file:") {
		return false
	}
	resolved := path.Clean(path.Join(workspaceRoot, value))
	return resolved != ".." && !strings.HasPrefix(resolved, "../")
}

func validOptionalTopologyText(value string) bool {
	return value == "" || validText(value)
}

func compareTopologyConstraints(left, right TopologyConstraint) int {
	for _, values := range [][2]string{
		{left.Name, right.Name}, {left.Scope, right.Scope}, {left.Value, right.Value},
	} {
		if compared := strings.Compare(values[0], values[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareTopologyDeclarations(left, right TopologyWorkspaceDeclaration) int {
	if compared := strings.Compare(left.ManifestPath, right.ManifestPath); compared != 0 {
		return compared
	}
	if left.Exclude != right.Exclude {
		if left.Exclude {
			return 1
		}
		return -1
	}
	return strings.Compare(left.Pattern, right.Pattern)
}

func compareTopologyDependencies(left, right TopologyDependency) int {
	for _, values := range [][2]string{
		{left.FromProject, right.FromProject},
		{left.ManifestPath, right.ManifestPath},
		{left.Ecosystem, right.Ecosystem},
		{left.Name, right.Name},
		{left.Scope, right.Scope},
		{left.Constraint, right.Constraint},
	} {
		if compared := strings.Compare(values[0], values[1]); compared != 0 {
			return compared
		}
	}
	if left.Indirect != right.Indirect {
		if left.Indirect {
			return 1
		}
		return -1
	}
	if left.Optional != right.Optional {
		if left.Optional {
			return 1
		}
		return -1
	}
	return strings.Compare(left.Resolution, right.Resolution)
}

func compareTopologyDiagnostics(left, right Diagnostic) int {
	if compared := strings.Compare(left.Code, right.Code); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Message, right.Message); compared != 0 {
		return compared
	}
	return strings.Compare(left.Level, right.Level)
}
