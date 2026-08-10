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

func validTopologyProjects(projects []TopologyProject) bool {
	for index, project := range projects {
		if !validTopologyPath(project.Root) || !validTopologyKind(project.Kind) ||
			!validOptionalTopologyPath(project.PrimaryWorkspaceRoot) ||
			!validSortedTopologyPaths(project.WorkspaceRoots) ||
			(project.PrimaryWorkspaceRoot != "" &&
				(!slices.Contains(project.WorkspaceRoots, project.PrimaryWorkspaceRoot) ||
					!topologyPathContains(project.PrimaryWorkspaceRoot, project.Root))) ||
			!validTopologyMarkers(project.Markers) ||
			!validTopologyComponents(project.Components) ||
			(index > 0 && projects[index-1].Root >= project.Root) {
			return false
		}
	}
	return true
}

func validTopologyWorkspaces(workspaces []TopologyWorkspace) bool {
	for index, workspace := range workspaces {
		if !validTopologyPath(workspace.Root) || !validTopologyMarkers(workspace.Markers) ||
			!validTopologyComponents(workspace.Components) ||
			!validSortedTopologyPaths(workspace.ContainedProjects) ||
			!validSortedTopologyPaths(workspace.DeclaredProjects) ||
			!validSortedTopologyPaths(workspace.ExcludedProjects) ||
			!validTopologyDeclarations(workspace.Root, workspace.Declarations) ||
			(index > 0 && workspaces[index-1].Root >= workspace.Root) {
			return false
		}
	}
	return true
}

func validTopologyDependencies(dependencies []TopologyDependency) bool {
	for index, dependency := range dependencies {
		if !validTopologyPath(dependency.FromProject) ||
			!validRelativePath(dependency.ManifestPath) ||
			!validLowerIdentifier(dependency.Ecosystem, "-_.") ||
			!validText(dependency.Name) ||
			!validOptionalTopologyText(dependency.Constraint) ||
			!validTopologyScope(dependency.Scope) ||
			!validDependencyResolution(dependency) ||
			!validSortedTopologyPaths(dependency.TargetProjects) ||
			(index > 0 && compareTopologyDependencies(dependencies[index-1], dependency) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyMarkers(markers []TopologyMarker) bool {
	for index, marker := range markers {
		if !validLowerIdentifier(marker.ID, "-_.") || len(marker.Evidence) == 0 ||
			!validSortedRelativePaths(marker.Evidence) ||
			(index > 0 && markers[index-1].ID >= marker.ID) {
			return false
		}
	}
	return true
}

func validTopologyComponents(components []TopologyComponent) bool {
	for index, component := range components {
		if !validRelativePath(component.ManifestPath) ||
			!validLowerIdentifier(component.Format, "-_.") ||
			!validWorkspaceDeclaredState(component) ||
			!validOptionalTopologyText(component.Name) ||
			!validOptionalTopologyText(component.Version) ||
			!validOptionalTopologyText(component.Module) ||
			!validTopologyConstraints(component.Constraints) ||
			(index > 0 && components[index-1].ManifestPath >= component.ManifestPath) {
			return false
		}
	}
	return true
}

func validTopologyConstraints(constraints []TopologyConstraint) bool {
	for index, constraint := range constraints {
		if !validText(constraint.Name) || !validOptionalTopologyText(constraint.Value) ||
			!validTopologyConstraintScope(constraint.Scope) ||
			(index > 0 && compareTopologyConstraints(constraints[index-1], constraint) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyDeclarations(
	workspaceRoot string,
	declarations []TopologyWorkspaceDeclaration,
) bool {
	for index, declaration := range declarations {
		if !validRelativePath(declaration.ManifestPath) ||
			!validTopologyPattern(workspaceRoot, declaration.Pattern) ||
			!validMemberResolution(declaration.Resolution) ||
			((declaration.Pattern == "[outside-root]") !=
				(declaration.Resolution == string(topology.MemberOutsideRoot))) ||
			!validSortedTopologyPaths(declaration.ProjectRoots) ||
			!validMemberTargets(declaration) ||
			(index > 0 && compareTopologyDeclarations(declarations[index-1], declaration) >= 0) {
			return false
		}
	}
	return true
}

func validTopologyDiagnostics(status Status, diagnostics []Diagnostic) bool {
	if status == StatusCompleted && len(diagnostics) != 0 {
		return false
	}
	if status != StatusCompleted && len(diagnostics) == 0 {
		return false
	}
	for index, diagnostic := range diagnostics {
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!validPublicMessage(diagnostic.Message) ||
			(index > 0 && compareTopologyDiagnostics(diagnostics[index-1], diagnostic) >= 0) {
			return false
		}
	}
	return true
}

type topologyRelationshipIndex struct {
	projects               map[string]TopologyProject
	workspaces             map[string]TopologyWorkspace
	expectedWorkspaceRoots map[string]map[string]struct{}
	expectedContained      map[string]map[string]struct{}
	projectManifestPaths   map[string]map[string]struct{}
	componentsByPath       map[string]TopologyComponent
}

type manifestMembership struct {
	included map[string]struct{}
	excluded map[string]struct{}
}

func validTopologyRelationships(report TopologyReport) bool {
	relationships, isValid := newTopologyRelationshipIndex(report)
	if !isValid {
		return false
	}
	return relationships.validWorkspaces(report.Workspaces) &&
		relationships.validProjects(report.Projects) &&
		relationships.validDependencies(report.Dependencies)
}

func newTopologyRelationshipIndex(report TopologyReport) (*topologyRelationshipIndex, bool) {
	relationships := &topologyRelationshipIndex{
		projects:               make(map[string]TopologyProject, len(report.Projects)),
		workspaces:             make(map[string]TopologyWorkspace, len(report.Workspaces)),
		expectedWorkspaceRoots: make(map[string]map[string]struct{}, len(report.Projects)),
		expectedContained:      make(map[string]map[string]struct{}, len(report.Workspaces)),
		projectManifestPaths:   make(map[string]map[string]struct{}, len(report.Projects)),
		componentsByPath:       make(map[string]TopologyComponent),
	}
	for _, project := range report.Projects {
		relationships.projects[project.Root] = project
		relationships.expectedWorkspaceRoots[project.Root] = make(map[string]struct{})
		relationships.projectManifestPaths[project.Root] = make(
			map[string]struct{},
			len(project.Components),
		)
		for _, component := range project.Components {
			if path.Dir(component.ManifestPath) != project.Root ||
				!recordTopologyComponent(relationships.componentsByPath, component) {
				return nil, false
			}
			relationships.projectManifestPaths[project.Root][component.ManifestPath] = struct{}{}
		}
	}
	for _, workspace := range report.Workspaces {
		relationships.workspaces[workspace.Root] = workspace
		relationships.expectedContained[workspace.Root] = make(map[string]struct{})
	}
	for _, project := range report.Projects {
		if project.PrimaryWorkspaceRoot == "" {
			continue
		}
		if _, exists := relationships.workspaces[project.PrimaryWorkspaceRoot]; !exists {
			return nil, false
		}
		relationships.expectedContained[project.PrimaryWorkspaceRoot][project.Root] = struct{}{}
		relationships.expectedWorkspaceRoots[project.Root][project.PrimaryWorkspaceRoot] = struct{}{}
	}
	return relationships, true
}

func (relationships *topologyRelationshipIndex) validWorkspaces(
	workspaces []TopologyWorkspace,
) bool {
	for _, workspace := range workspaces {
		if !relationships.validWorkspaceComponents(workspace) ||
			!topologyPathsEqualSet(
				workspace.ContainedProjects,
				relationships.expectedContained[workspace.Root],
			) {
			return false
		}
		expectedDeclared, expectedExcluded, isValid := relationships.workspaceMemberships(workspace)
		if !isValid || !topologyPathsEqualSet(workspace.DeclaredProjects, expectedDeclared) ||
			!topologyPathsEqualSet(workspace.ExcludedProjects, expectedExcluded) {
			return false
		}
		for projectRoot := range expectedDeclared {
			relationships.expectedWorkspaceRoots[projectRoot][workspace.Root] = struct{}{}
		}
	}
	return true
}

func (relationships *topologyRelationshipIndex) validWorkspaceComponents(
	workspace TopologyWorkspace,
) bool {
	for _, component := range workspace.Components {
		if path.Dir(component.ManifestPath) != workspace.Root ||
			!recordTopologyComponent(relationships.componentsByPath, component) {
			return false
		}
	}
	return true
}

func (relationships *topologyRelationshipIndex) workspaceMemberships(
	workspace TopologyWorkspace,
) (map[string]struct{}, map[string]struct{}, bool) {
	memberships := make(map[string]*manifestMembership)
	expectedExcluded := make(map[string]struct{})
	for _, declaration := range workspace.Declarations {
		if path.Dir(declaration.ManifestPath) != workspace.Root ||
			!allTopologyProjectsExist(declaration.ProjectRoots, relationships.projects) {
			return nil, nil, false
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
			if _, isExcluded := membership.excluded[projectRoot]; !isExcluded {
				expectedDeclared[projectRoot] = struct{}{}
			}
		}
	}
	return expectedDeclared, expectedExcluded, true
}

func (relationships *topologyRelationshipIndex) validProjects(projects []TopologyProject) bool {
	for _, project := range projects {
		if project.PrimaryWorkspaceRoot != nearestTopologyWorkspace(
			project.Root,
			relationships.workspaces,
		) || !topologyPathsEqualSet(
			project.WorkspaceRoots,
			relationships.expectedWorkspaceRoots[project.Root],
		) {
			return false
		}
	}
	return true
}

func (relationships *topologyRelationshipIndex) validDependencies(
	dependencies []TopologyDependency,
) bool {
	for _, dependency := range dependencies {
		if _, exists := relationships.projects[dependency.FromProject]; !exists {
			return false
		}
		if path.Dir(dependency.ManifestPath) != dependency.FromProject ||
			!allTopologyProjectsExist(dependency.TargetProjects, relationships.projects) {
			return false
		}
		if _, exists := relationships.projectManifestPaths[dependency.FromProject][dependency.ManifestPath]; !exists {
			return false
		}
	}
	return true
}

func validWorkspaceDeclaredState(component TopologyComponent) bool {
	switch component.Format {
	case string(manifest.FormatGoWorkspace):
		return component.WorkspaceDeclared
	case string(manifest.FormatGoModule), string(manifest.FormatPythonProject),
		string(manifest.FormatPHPComposer):
		return !component.WorkspaceDeclared
	default:
		return true
	}
}

func recordTopologyComponent(
	components map[string]TopologyComponent,
	component TopologyComponent,
) bool {
	existing, found := components[component.ManifestPath]
	if !found {
		components[component.ManifestPath] = component
		return true
	}
	return existing.ManifestPath == component.ManifestPath && existing.Format == component.Format &&
		existing.Name == component.Name && existing.Version == component.Version &&
		existing.Module == component.Module &&
		existing.WorkspaceDeclared == component.WorkspaceDeclared &&
		existing.DependenciesTruncated == component.DependenciesTruncated &&
		existing.ConstraintsTruncated == component.ConstraintsTruncated &&
		existing.WorkspaceMembersTruncated == component.WorkspaceMembersTruncated &&
		existing.WorkspaceExcludesTruncated == component.WorkspaceExcludesTruncated &&
		slices.Equal(existing.Constraints, component.Constraints)
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

func allTopologyProjectsExist(projectRoots []string, projects map[string]TopologyProject) bool {
	for _, projectRoot := range projectRoots {
		if _, exists := projects[projectRoot]; !exists {
			return false
		}
	}
	return true
}

func topologyPathsEqualSet(paths []string, expected map[string]struct{}) bool {
	if len(paths) != len(expected) {
		return false
	}
	for _, candidatePath := range paths {
		if _, exists := expected[candidatePath]; !exists {
			return false
		}
	}
	return true
}

func topologyPathContains(root, candidate string) bool {
	return root == "." || candidate == root || strings.HasPrefix(candidate, root+"/")
}

func validTopologyKind(kind string) bool {
	return kind == "code" || kind == "infrastructure" || kind == "mixed"
}

func validTopologyScope(scope string) bool {
	return scope == "runtime" || scope == "development" || scope == "build" || scope == "peer"
}

func validTopologyConstraintScope(scope string) bool {
	return scope == "runtime" || scope == "development" || scope == "build"
}

func validDependencyResolution(dependency TopologyDependency) bool {
	switch dependency.Resolution {
	case string(topology.DependencyInternal):
		return len(dependency.TargetProjects) == 1 && !dependency.TargetsTruncated
	case string(topology.DependencyUnresolved):
		return len(dependency.TargetProjects) == 0 && !dependency.TargetsTruncated
	case string(topology.DependencyAmbiguous):
		return len(dependency.TargetProjects) >= 2 ||
			(dependency.TargetsTruncated && len(dependency.TargetProjects) >= 1)
	default:
		return false
	}
}

func validMemberResolution(resolution string) bool {
	switch resolution {
	case string(topology.MemberMatched), string(topology.MemberUnmatched),
		string(topology.MemberIndeterminate), string(topology.MemberUnsupported),
		string(topology.MemberOutsideRoot):
		return true
	default:
		return false
	}
}

func validMemberTargets(declaration TopologyWorkspaceDeclaration) bool {
	switch declaration.Resolution {
	case string(topology.MemberMatched):
		return len(declaration.ProjectRoots) > 0
	case string(topology.MemberUnmatched), string(topology.MemberIndeterminate),
		string(topology.MemberUnsupported), string(topology.MemberOutsideRoot):
		return len(declaration.ProjectRoots) == 0 && !declaration.MatchesTruncated
	default:
		return false
	}
}

func validTopologyPath(candidatePath string) bool {
	return candidatePath == "." || validRelativePath(candidatePath)
}

func validOptionalTopologyPath(candidatePath string) bool {
	return candidatePath == "" || validTopologyPath(candidatePath)
}

func validSortedTopologyPaths(paths []string) bool {
	if !strictlySortedStrings(paths) {
		return false
	}
	for _, candidatePath := range paths {
		if !validTopologyPath(candidatePath) {
			return false
		}
	}
	return true
}

func validTopologyPattern(workspaceRoot, pattern string) bool {
	if pattern == "[outside-root]" {
		return true
	}
	lower := strings.ToLower(pattern)
	if !validText(pattern) || strings.Contains(pattern, "\\") || path.IsAbs(pattern) ||
		looksLikeWindowsPath(pattern) || strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "file:") {
		return false
	}
	resolved := path.Clean(path.Join(workspaceRoot, pattern))
	return resolved != ".." && !strings.HasPrefix(resolved, "../")
}

func validOptionalTopologyText(text string) bool {
	return text == "" || validText(text)
}

func compareTopologyConstraints(left, right TopologyConstraint) int {
	return compareTopologyTextFields([][2]string{
		{left.Name, right.Name}, {left.Scope, right.Scope}, {left.Value, right.Value},
	})
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
	if compared := compareTopologyTextFields([][2]string{
		{left.FromProject, right.FromProject},
		{left.ManifestPath, right.ManifestPath},
		{left.Ecosystem, right.Ecosystem},
		{left.Name, right.Name},
		{left.Scope, right.Scope},
		{left.Constraint, right.Constraint},
	}); compared != 0 {
		return compared
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

func compareTopologyTextFields(fields [][2]string) int {
	for _, field := range fields {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
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
