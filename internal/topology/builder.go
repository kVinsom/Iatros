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
	// IssueNestedRepositoryLimit identifies omitted nested repository boundaries.
	IssueNestedRepositoryLimit = "IATROS_TOPOLOGY_NESTED_REPOSITORY_LIMIT"
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
	nestedRepositories, err := limitedNestedRepositories(
		buildCtx,
		snapshot.NestedRepositories,
		b.limits.MaxNestedRepositories,
	)
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
	if !validNestedRepositories(nestedRepositories, projects, workspaces) {
		return model, ErrInvalidSnapshot
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
	if len(snapshot.NestedRepositories) > len(nestedRepositories) {
		model.addIssue(Issue{
			Code: IssueNestedRepositoryLimit, Path: ".",
			Message: "additional nested repository boundaries were omitted by the configured topology limit",
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
	model.NestedRepositories = nestedRepositories
	slices.SortFunc(model.Dependencies, compareDependencies)
	model.Dependencies = slices.CompactFunc(model.Dependencies, equalDependencies)
	slices.SortFunc(model.Issues, compareIssues)
	return model, nil
}

type projectState struct {
	project   Project
	manifests []manifest.Manifest
}

type workspaceState struct {
	workspace Workspace
}

func (b Builder) buildProjects(
	ctx context.Context,
	model *Model,
	boundaries []project.Boundary,
	manifests []manifest.Manifest,
) map[string]*projectState {
	states := make(map[string]*projectState, len(boundaries))
	for _, boundary := range boundaries {
		states[boundary.Root] = &projectState{project: Project{
			Root:           boundary.Root,
			Kind:           string(boundary.Kind),
			WorkspaceRoots: make([]string, 0),
			Markers:        copyMarkers(boundary.Markers),
			Components:     make([]Component, 0),
		}}
	}

	for _, manifestDocument := range manifests {
		if err := ctx.Err(); err != nil {
			return states
		}
		if manifestDocument.Format == manifest.FormatGoWorkspace {
			continue
		}
		root := path.Dir(manifestDocument.Path)
		state, exists := states[root]
		if !exists {
			model.addIssue(Issue{
				Code: IssueManifestUnassigned, Path: manifestDocument.Path,
				Message: "the manifest has no retained project boundary",
			}, b.limits.MaxIssues)
			continue
		}
		if len(state.project.Components) >= b.limits.MaxComponentsPerBoundary {
			model.addIssue(Issue{
				Code: IssueComponentLimit, Path: manifestDocument.Path,
				Message: "additional project components were omitted by the configured limit",
			}, b.limits.MaxIssues)
			continue
		}
		state.project.Components = append(
			state.project.Components,
			componentFromManifest(manifestDocument),
		)
		state.manifests = append(state.manifests, manifestDocument)
		if manifestDocument.DependenciesTruncated || manifestDocument.ConstraintsTruncated ||
			manifestDocument.WorkspaceMembersTruncated ||
			manifestDocument.WorkspaceExcludesTruncated {
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
	for _, manifestDocument := range manifests {
		if manifestDeclaresWorkspace(manifestDocument) {
			rootSet[path.Dir(manifestDocument.Path)] = struct{}{}
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
		states[root] = &workspaceState{workspace: Workspace{
			Root:              root,
			Markers:           copyMarkers(markersByRoot[root]),
			Components:        make([]Component, 0),
			ContainedProjects: make([]string, 0),
			DeclaredProjects:  make([]string, 0),
			ExcludedProjects:  make([]string, 0),
			Declarations:      make([]WorkspaceDeclaration, 0),
		}}
	}

	for _, manifestDocument := range manifests {
		if err := ctx.Err(); err != nil {
			return states
		}
		if !manifestDeclaresWorkspace(manifestDocument) {
			continue
		}
		state, exists := states[path.Dir(manifestDocument.Path)]
		if !exists {
			continue
		}
		if len(state.workspace.Components) >= b.limits.MaxComponentsPerBoundary {
			model.addIssue(Issue{
				Code: IssueComponentLimit, Path: manifestDocument.Path,
				Message: "additional workspace components were omitted by the configured limit",
			}, b.limits.MaxIssues)
			continue
		}
		state.workspace.Components = append(
			state.workspace.Components,
			componentFromManifest(manifestDocument),
		)
		if manifestDocument.DependenciesTruncated || manifestDocument.ConstraintsTruncated ||
			manifestDocument.WorkspaceMembersTruncated ||
			manifestDocument.WorkspaceExcludesTruncated {
			model.Partial = true
		}
	}
	return states
}

func manifestDeclaresWorkspace(manifestDocument manifest.Manifest) bool {
	return manifestDocument.Format == manifest.FormatGoWorkspace ||
		manifestDocument.WorkspaceDeclared || len(manifestDocument.WorkspaceMembers) != 0 ||
		len(manifestDocument.WorkspaceExcludes) != 0
}

func limitedProjects(
	ctx context.Context,
	projects []project.Boundary,
	maximum int,
) ([]project.Boundary, error) {
	return boundedLexicalTopN(ctx, projects, maximum, func(boundary project.Boundary) string {
		return boundary.Root
	})
}

func limitedWorkspaces(
	ctx context.Context,
	workspaces []project.Workspace,
	maximum int,
) ([]project.Workspace, error) {
	return boundedLexicalTopN(ctx, workspaces, maximum, func(workspace project.Workspace) string {
		return workspace.Root
	})
}

func limitedManifests(
	ctx context.Context,
	manifests []manifest.Manifest,
	maximum int,
) ([]manifest.Manifest, error) {
	return boundedLexicalTopN(ctx, manifests, maximum, func(manifestDocument manifest.Manifest) string {
		return manifestDocument.Path
	})
}

func limitedNestedRepositories(
	ctx context.Context,
	nestedRepositories []string,
	maximum int,
) ([]string, error) {
	return boundedLexicalTopN(ctx, nestedRepositories, maximum, func(repositoryRoot string) string {
		return repositoryRoot
	})
}

func validNestedRepositories(
	nestedRepositories []string,
	projects []project.Boundary,
	workspaces []project.Workspace,
) bool {
	boundaries := make(map[string]struct{}, len(nestedRepositories))
	for _, repositoryRoot := range nestedRepositories {
		if !validFile(repositoryRoot) {
			return false
		}
		for ancestor := path.Dir(repositoryRoot); ancestor != "."; ancestor = path.Dir(ancestor) {
			if _, nested := boundaries[ancestor]; nested {
				return false
			}
		}
		boundaries[repositoryRoot] = struct{}{}
	}
	for _, projectBoundary := range projects {
		if pathInsideNestedRepository(projectBoundary.Root, boundaries) {
			return false
		}
	}
	for _, workspace := range workspaces {
		if pathInsideNestedRepository(workspace.Root, boundaries) {
			return false
		}
	}
	return true
}

func pathInsideNestedRepository(repositoryPath string, boundaries map[string]struct{}) bool {
	for candidate := repositoryPath; candidate != "."; candidate = path.Dir(candidate) {
		if _, nested := boundaries[candidate]; nested {
			return true
		}
	}
	return false
}

func boundedLexicalTopN[T any](
	ctx context.Context,
	entries []T,
	maximum int,
	key func(T) string,
) ([]T, error) {
	compare := func(left, right T) int {
		return strings.Compare(key(left), key(right))
	}
	if len(entries) <= maximum {
		selected := make([]T, 0, len(entries))
		for index, entry := range entries {
			if index%128 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			selected = append(selected, entry)
		}
		slices.SortFunc(selected, compare)
		for index := 1; index < len(selected); index++ {
			if key(selected[index-1]) == key(selected[index]) {
				return nil, ErrInvalidSnapshot
			}
		}
		return selected, ctx.Err()
	}

	selected := make([]T, 0, min(len(entries), maximum))
	selectedKeys := make(map[string]bool, cap(selected))
	for index, entry := range entries {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		entryKey := key(entry)
		if _, retained := selectedKeys[entryKey]; retained {
			selectedKeys[entryKey] = true
			continue
		}
		if len(selected) < maximum {
			selected = append(selected, entry)
			selectedKeys[entryKey] = false
			siftUpLargest(selected, len(selected)-1, compare)
			continue
		}
		if strings.Compare(entryKey, key(selected[0])) >= 0 {
			continue
		}
		delete(selectedKeys, key(selected[0]))
		selected[0] = entry
		selectedKeys[entryKey] = false
		siftDownLargest(selected, 0, compare)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, duplicate := range selectedKeys {
		if duplicate {
			return nil, ErrInvalidSnapshot
		}
	}
	slices.SortFunc(selected, compare)
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
			siftUpLargest(selected, len(selected)-1, compareIssues)
			return nil
		}
		if compareIssues(issue, selected[0]) >= 0 {
			truncated = true
			return nil
		}
		delete(seen, selected[0])
		selected[0] = issue
		seen[issue] = struct{}{}
		siftDownLargest(selected, 0, compareIssues)
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

func siftUpLargest[T any](entries []T, index int, compare func(T, T) int) {
	for index > 0 {
		parent := (index - 1) / 2
		if compare(entries[parent], entries[index]) >= 0 {
			return
		}
		entries[parent], entries[index] = entries[index], entries[parent]
		index = parent
	}
}

func siftDownLargest[T any](entries []T, index int, compare func(T, T) int) {
	for {
		left := index*2 + 1
		if left >= len(entries) {
			return
		}
		largest := left
		right := left + 1
		if right < len(entries) && compare(entries[right], entries[left]) > 0 {
			largest = right
		}
		if compare(entries[index], entries[largest]) >= 0 {
			return
		}
		entries[index], entries[largest] = entries[largest], entries[index]
		index = largest
	}
}

func emptyModel(partial bool) Model {
	return Model{
		Projects:           make([]Project, 0),
		Workspaces:         make([]Workspace, 0),
		Dependencies:       make([]Dependency, 0),
		NestedRepositories: make([]string, 0),
		Issues:             make([]Issue, 0),
		Partial:            partial,
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
