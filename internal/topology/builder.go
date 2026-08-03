package topology

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
)

const (
	// IssueProjectLimit identifies omitted project boundaries.
	IssueProjectLimit = "IATROS_TOPOLOGY_PROJECT_LIMIT"
	// IssueWorkspaceLimit identifies omitted workspace boundaries.
	IssueWorkspaceLimit = "IATROS_TOPOLOGY_WORKSPACE_LIMIT"
	// IssueManifestLimit identifies omitted parsed manifests.
	IssueManifestLimit = "IATROS_TOPOLOGY_MANIFEST_LIMIT"
	// IssueComponentLimit identifies omitted components at one boundary.
	IssueComponentLimit = "IATROS_TOPOLOGY_COMPONENT_LIMIT"
	// IssueDeclarationLimit identifies omitted workspace declarations.
	IssueDeclarationLimit = "IATROS_TOPOLOGY_DECLARATION_LIMIT"
	// IssueMemberMatchLimit identifies omitted project matches for one declaration.
	IssueMemberMatchLimit = "IATROS_TOPOLOGY_MEMBER_MATCH_LIMIT"
	// IssueDependencyLimit identifies omitted dependency declarations.
	IssueDependencyLimit = "IATROS_TOPOLOGY_DEPENDENCY_LIMIT"
	// IssueDependencyTargetLimit identifies omitted ambiguous dependency targets.
	IssueDependencyTargetLimit = "IATROS_TOPOLOGY_DEPENDENCY_TARGET_LIMIT"
	// IssueManifestUnassigned identifies a manifest without a retained project boundary.
	IssueManifestUnassigned = "IATROS_TOPOLOGY_MANIFEST_UNASSIGNED"
	// IssueMemberUnsupported identifies unsupported workspace pattern semantics.
	IssueMemberUnsupported = "IATROS_TOPOLOGY_MEMBER_UNSUPPORTED"
	// IssueMemberOutsideRoot identifies a workspace member that could escape the repository.
	IssueMemberOutsideRoot = "IATROS_TOPOLOGY_MEMBER_OUTSIDE_ROOT"
	// IssueMemberUnmatched identifies a declaration with no known project match.
	IssueMemberUnmatched = "IATROS_TOPOLOGY_MEMBER_UNMATCHED"
	// IssueDependencyAmbiguous identifies multiple repository projects with one dependency identity.
	IssueDependencyAmbiguous = "IATROS_TOPOLOGY_DEPENDENCY_AMBIGUOUS"
	// IssueLimit identifies additional omitted topology issues.
	IssueLimit = "IATROS_TOPOLOGY_ISSUE_LIMIT"
)

const outsideRootPattern = "[outside-root]"

// Builder associates normalized project and manifest snapshots.
type Builder struct {
	limits Limits
}

// NewBuilder creates a topology builder with validated injectable limits.
func NewBuilder(limits Limits) (Builder, error) {
	if err := limits.Validate(); err != nil {
		return Builder{}, err
	}
	return Builder{limits: limits}, nil
}

// Build returns deterministic topology without reading the filesystem or resolving external state.
func (b Builder) Build(ctx context.Context, snapshot Snapshot) (Model, error) {
	partial := snapshot.Projects.Partial || snapshot.Manifests.Partial || len(snapshot.Issues) != 0
	model := emptyModel(partial)
	if err := b.limits.Validate(); err != nil {
		return model, err
	}
	if err := ctx.Err(); err != nil {
		return model, err
	}
	buildCtx, cancel := context.WithTimeout(ctx, b.limits.Timeout)
	defer cancel()

	projects, err := limitedProjects(buildCtx, snapshot.Projects.Projects, b.limits.MaxProjects)
	if err != nil {
		return model, err
	}
	workspaces, err := limitedWorkspaces(buildCtx, snapshot.Projects.Workspaces, b.limits.MaxWorkspaces)
	if err != nil {
		return model, err
	}
	manifests, err := limitedManifests(buildCtx, snapshot.Manifests.Manifests, b.limits.MaxManifests)
	if err != nil {
		return model, err
	}
	if err := validateInputs(
		buildCtx,
		projects,
		workspaces,
		manifests,
		b.limits,
	); err != nil {
		return model, err
	}
	inheritedIssues, issuesTruncated, err := limitedSnapshotIssues(
		buildCtx,
		snapshot.Manifests.Issues,
		snapshot.Issues,
		b.limits,
	)
	if err != nil {
		return model, err
	}
	if len(snapshot.Projects.Projects) > len(projects) {
		model.addIssue(Issue{
			Code: IssueProjectLimit, Path: ".",
			Message: "additional projects were omitted by the configured topology limit",
		}, b.limits.MaxIssues)
	}
	if len(snapshot.Projects.Workspaces) > len(workspaces) {
		model.addIssue(Issue{
			Code: IssueWorkspaceLimit, Path: ".",
			Message: "additional workspace boundaries were omitted by the configured topology limit",
		}, b.limits.MaxIssues)
	}
	if len(snapshot.Manifests.Manifests) > len(manifests) {
		model.addIssue(Issue{
			Code: IssueManifestLimit, Path: ".",
			Message: "additional manifests were omitted by the configured topology limit",
		}, b.limits.MaxIssues)
	}
	for _, issue := range inheritedIssues {
		model.addIssue(issue, b.limits.MaxIssues)
	}
	if issuesTruncated {
		model.addIssue(issueLimitMarker(), b.limits.MaxIssues)
	}

	projectStates := b.buildProjects(buildCtx, &model, projects, manifests)
	if err := buildCtx.Err(); err != nil {
		return emptyModel(partial), err
	}
	workspaceStates := b.buildWorkspaces(buildCtx, &model, workspaces, manifests)
	if err := buildCtx.Err(); err != nil {
		return emptyModel(partial), err
	}

	if err := b.associateWorkspaces(
		buildCtx,
		&model,
		projectStates,
		workspaceStates,
		manifests,
	); err != nil {
		return emptyModel(partial), err
	}
	b.buildDependencies(buildCtx, &model, projectStates)
	if err := buildCtx.Err(); err != nil {
		return emptyModel(partial), err
	}

	model.Projects = finalizeProjects(projectStates)
	model.Workspaces = finalizeWorkspaces(workspaceStates)
	slices.SortFunc(model.Dependencies, compareDependencies)
	model.Dependencies = slices.CompactFunc(model.Dependencies, equalDependencies)
	slices.SortFunc(model.Issues, compareIssues)
	return model, nil
}

type projectState struct {
	value     Project
	manifests []manifest.Manifest
}

type workspaceState struct {
	value Workspace
}

func (b Builder) buildProjects(
	ctx context.Context,
	model *Model,
	boundaries []project.Project,
	manifests []manifest.Manifest,
) map[string]*projectState {
	states := make(map[string]*projectState, len(boundaries))
	for _, boundary := range boundaries {
		states[boundary.Root] = &projectState{value: Project{
			Root:           boundary.Root,
			Kind:           string(boundary.Kind),
			WorkspaceRoots: make([]string, 0),
			Markers:        copyMarkers(boundary.Markers),
			Components:     make([]Component, 0),
		}}
	}

	for _, value := range manifests {
		if err := ctx.Err(); err != nil {
			return states
		}
		if value.Format == manifest.FormatGoWorkspace {
			continue
		}
		root := path.Dir(value.Path)
		state, exists := states[root]
		if !exists {
			model.addIssue(Issue{
				Code: IssueManifestUnassigned, Path: value.Path,
				Message: "the manifest has no retained project boundary",
			}, b.limits.MaxIssues)
			continue
		}
		if len(state.value.Components) >= b.limits.MaxComponentsPerBoundary {
			model.addIssue(Issue{
				Code: IssueComponentLimit, Path: value.Path,
				Message: "additional project components were omitted by the configured limit",
			}, b.limits.MaxIssues)
			continue
		}
		state.value.Components = append(state.value.Components, componentFromManifest(value))
		state.manifests = append(state.manifests, value)
		if value.DependenciesTruncated || value.ConstraintsTruncated ||
			value.WorkspaceMembersTruncated || value.WorkspaceExcludesTruncated {
			model.Partial = true
		}
	}
	return states
}

func (b Builder) buildWorkspaces(
	ctx context.Context,
	model *Model,
	boundaries []project.Workspace,
	manifests []manifest.Manifest,
) map[string]*workspaceState {
	markersByRoot := make(map[string][]project.Marker, len(boundaries))
	rootSet := make(map[string]struct{}, len(boundaries)+len(manifests))
	for _, boundary := range boundaries {
		rootSet[boundary.Root] = struct{}{}
		markersByRoot[boundary.Root] = boundary.Markers
	}
	for _, value := range manifests {
		if manifestDeclaresWorkspace(value) {
			rootSet[path.Dir(value.Path)] = struct{}{}
		}
	}

	roots := make([]string, 0, len(rootSet))
	for root := range rootSet {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	if len(roots) > b.limits.MaxWorkspaces {
		model.addIssue(Issue{
			Code: IssueWorkspaceLimit, Path: roots[b.limits.MaxWorkspaces],
			Message: "additional derived workspaces were omitted by the configured topology limit",
		}, b.limits.MaxIssues)
		roots = roots[:b.limits.MaxWorkspaces]
	}

	states := make(map[string]*workspaceState, len(roots))
	for _, root := range roots {
		states[root] = &workspaceState{value: Workspace{
			Root:              root,
			Markers:           copyMarkers(markersByRoot[root]),
			Components:        make([]Component, 0),
			ContainedProjects: make([]string, 0),
			DeclaredProjects:  make([]string, 0),
			ExcludedProjects:  make([]string, 0),
			Declarations:      make([]WorkspaceDeclaration, 0),
		}}
	}

	for _, value := range manifests {
		if err := ctx.Err(); err != nil {
			return states
		}
		if !manifestDeclaresWorkspace(value) {
			continue
		}
		state, exists := states[path.Dir(value.Path)]
		if !exists {
			continue
		}
		if len(state.value.Components) >= b.limits.MaxComponentsPerBoundary {
			model.addIssue(Issue{
				Code: IssueComponentLimit, Path: value.Path,
				Message: "additional workspace components were omitted by the configured limit",
			}, b.limits.MaxIssues)
			continue
		}
		state.value.Components = append(state.value.Components, componentFromManifest(value))
		if value.DependenciesTruncated || value.ConstraintsTruncated ||
			value.WorkspaceMembersTruncated || value.WorkspaceExcludesTruncated {
			model.Partial = true
		}
	}
	return states
}

func manifestDeclaresWorkspace(value manifest.Manifest) bool {
	return value.Format == manifest.FormatGoWorkspace || value.WorkspaceDeclared ||
		len(value.WorkspaceMembers) != 0 || len(value.WorkspaceExcludes) != 0
}

func limitedProjects(
	ctx context.Context,
	values []project.Project,
	maximum int,
) ([]project.Project, error) {
	return boundedLexicalTopN(ctx, values, maximum, func(value project.Project) string {
		return value.Root
	})
}

func limitedWorkspaces(
	ctx context.Context,
	values []project.Workspace,
	maximum int,
) ([]project.Workspace, error) {
	return boundedLexicalTopN(ctx, values, maximum, func(value project.Workspace) string {
		return value.Root
	})
}

func limitedManifests(
	ctx context.Context,
	values []manifest.Manifest,
	maximum int,
) ([]manifest.Manifest, error) {
	return boundedLexicalTopN(ctx, values, maximum, func(value manifest.Manifest) string {
		return value.Path
	})
}

func boundedLexicalTopN[T any](
	ctx context.Context,
	values []T,
	maximum int,
	key func(T) string,
) ([]T, error) {
	if len(values) <= maximum {
		selected := make([]T, 0, len(values))
		for index, value := range values {
			if index%128 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			selected = append(selected, value)
		}
		slices.SortFunc(selected, func(left, right T) int {
			return strings.Compare(key(left), key(right))
		})
		for index := 1; index < len(selected); index++ {
			if key(selected[index-1]) == key(selected[index]) {
				return nil, ErrInvalidSnapshot
			}
		}
		return selected, ctx.Err()
	}

	selected := make([]T, 0, min(len(values), maximum))
	selectedKeys := make(map[string]bool, cap(selected))
	for index, value := range values {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		valueKey := key(value)
		if _, retained := selectedKeys[valueKey]; retained {
			selectedKeys[valueKey] = true
			continue
		}
		if len(selected) < maximum {
			selected = append(selected, value)
			selectedKeys[valueKey] = false
			siftUpLargest(selected, len(selected)-1, key)
			continue
		}
		if strings.Compare(valueKey, key(selected[0])) >= 0 {
			continue
		}
		delete(selectedKeys, key(selected[0]))
		selected[0] = value
		selectedKeys[valueKey] = false
		siftDownLargest(selected, 0, key)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, duplicate := range selectedKeys {
		if duplicate {
			return nil, ErrInvalidSnapshot
		}
	}
	slices.SortFunc(selected, func(left, right T) int {
		return strings.Compare(key(left), key(right))
	})
	return selected, nil
}

func limitedSnapshotIssues(
	ctx context.Context,
	manifestIssues []manifest.Issue,
	inheritedIssues []Issue,
	limits Limits,
) ([]Issue, bool, error) {
	capacity := min(len(manifestIssues), limits.MaxIssues)
	capacity += min(len(inheritedIssues), limits.MaxIssues-capacity)
	selected := make([]Issue, 0, capacity)
	seen := make(map[Issue]struct{}, cap(selected))
	truncated := false
	processed := 0

	add := func(issue Issue) error {
		if processed%128 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		processed++
		if !validIssueCode(issue.Code) || !validIssuePath(issue.Path) ||
			!validText(issue.Message, limits.MaxValueBytes) {
			return ErrInvalidSnapshot
		}
		if issue == issueLimitMarker() {
			return ErrInvalidSnapshot
		}
		if _, exists := seen[issue]; exists {
			return nil
		}
		if len(selected) < limits.MaxIssues {
			selected = append(selected, issue)
			seen[issue] = struct{}{}
			siftUpLargestIssue(selected, len(selected)-1)
			return nil
		}
		if compareIssues(issue, selected[0]) >= 0 {
			truncated = true
			return nil
		}
		delete(seen, selected[0])
		selected[0] = issue
		seen[issue] = struct{}{}
		siftDownLargestIssue(selected, 0)
		truncated = true
		return nil
	}

	for _, issue := range manifestIssues {
		if err := add(Issue(issue)); err != nil {
			return nil, false, err
		}
	}
	for _, issue := range inheritedIssues {
		if err := add(issue); err != nil {
			return nil, false, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	slices.SortFunc(selected, compareIssues)
	return selected, truncated, nil
}

func siftUpLargestIssue(values []Issue, index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if compareIssues(values[parent], values[index]) >= 0 {
			return
		}
		values[parent], values[index] = values[index], values[parent]
		index = parent
	}
}

func siftDownLargestIssue(values []Issue, index int) {
	for {
		left := index*2 + 1
		if left >= len(values) {
			return
		}
		largest := left
		right := left + 1
		if right < len(values) && compareIssues(values[right], values[left]) > 0 {
			largest = right
		}
		if compareIssues(values[index], values[largest]) >= 0 {
			return
		}
		values[index], values[largest] = values[largest], values[index]
		index = largest
	}
}

func siftUpLargest[T any](values []T, index int, key func(T) string) {
	for index > 0 {
		parent := (index - 1) / 2
		if strings.Compare(key(values[parent]), key(values[index])) >= 0 {
			return
		}
		values[parent], values[index] = values[index], values[parent]
		index = parent
	}
}

func siftDownLargest[T any](values []T, index int, key func(T) string) {
	for {
		left := index*2 + 1
		if left >= len(values) {
			return
		}
		largest := left
		right := left + 1
		if right < len(values) && strings.Compare(key(values[right]), key(values[left])) > 0 {
			largest = right
		}
		if strings.Compare(key(values[index]), key(values[largest])) >= 0 {
			return
		}
		values[index], values[largest] = values[largest], values[index]
		index = largest
	}
}

func emptyModel(partial bool) Model {
	return Model{
		Projects:     make([]Project, 0),
		Workspaces:   make([]Workspace, 0),
		Dependencies: make([]Dependency, 0),
		Issues:       make([]Issue, 0),
		Partial:      partial,
	}
}

func (m *Model) addIssue(issue Issue, maximum int) {
	m.Partial = true
	if len(m.Issues) == maximum && m.Issues[maximum-1] == issueLimitMarker() {
		return
	}
	for _, existing := range m.Issues {
		if existing == issue || existing == issueLimitMarker() {
			return
		}
	}
	if len(m.Issues) < maximum {
		m.Issues = append(m.Issues, issue)
		return
	}
	m.Issues[maximum-1] = issueLimitMarker()
}

func issueLimitMarker() Issue {
	return Issue{
		Code: IssueLimit, Path: ".",
		Message: "additional topology issues were omitted by the configured limit",
	}
}

func compareIssues(left, right Issue) int {
	if compared := strings.Compare(left.Path, right.Path); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Code, right.Code); compared != 0 {
		return compared
	}
	return strings.Compare(left.Message, right.Message)
}
