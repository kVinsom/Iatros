package topology

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/repositorypath"
)

func (b Builder) associateWorkspaces(
	ctx context.Context,
	model *Model,
	projects map[string]*projectState,
	workspaces map[string]*workspaceState,
	manifests []manifest.Manifest,
) error {
	workspaceRoots := make(map[string]struct{}, len(workspaces))
	for root := range workspaces {
		workspaceRoots[root] = struct{}{}
	}
	for root, state := range projects {
		if err := ctx.Err(); err != nil {
			return err
		}
		primary := nearestWorkspace(root, workspaceRoots)
		state.value.PrimaryWorkspaceRoot = primary
		if primary != "" {
			state.value.WorkspaceRoots = append(state.value.WorkspaceRoots, primary)
			workspaces[primary].value.ContainedProjects = append(
				workspaces[primary].value.ContainedProjects, root,
			)
		}
	}

	projectRoots := make([]string, 0, len(projects))
	for root := range projects {
		projectRoots = append(projectRoots, root)
	}
	slices.Sort(projectRoots)

	membershipIncomplete := model.Partial
	declarationCount := 0
	for _, value := range manifests {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(value.WorkspaceMembers) == 0 && len(value.WorkspaceExcludes) == 0 {
			continue
		}
		workspaceRoot := path.Dir(value.Path)
		state, exists := workspaces[workspaceRoot]
		if !exists {
			continue
		}
		declaredStart := len(state.value.DeclaredProjects)
		excludedStart := len(state.value.ExcludedProjects)
		for _, declarationSet := range []struct {
			values  []string
			exclude bool
		}{
			{values: value.WorkspaceMembers},
			{values: value.WorkspaceExcludes, exclude: true},
		} {
			for _, member := range declarationSet.values {
				if declarationCount >= b.limits.MaxWorkspaceDeclarations {
					model.addIssue(Issue{
						Code: IssueDeclarationLimit, Path: value.Path,
						Message: "additional workspace declarations were omitted by the configured limit",
					}, b.limits.MaxIssues)
					applyManifestExclusions(
						&state.value,
						declaredStart,
						excludedStart,
					)
					associateDeclaredMemberships(projects, workspaces)
					return nil
				}
				declarationCount++

				declaration, issue, err := b.resolveWorkspaceMember(
					ctx,
					workspaceRoot, value.Path, member, declarationSet.exclude,
					projectRoots, membershipIncomplete,
				)
				if err != nil {
					return err
				}
				state.value.Declarations = append(state.value.Declarations, declaration)
				if issue != nil {
					model.addIssue(*issue, b.limits.MaxIssues)
				}
				if declaration.Exclude {
					state.value.ExcludedProjects = append(
						state.value.ExcludedProjects, declaration.ProjectRoots...,
					)
					continue
				}
				state.value.DeclaredProjects = append(
					state.value.DeclaredProjects, declaration.ProjectRoots...,
				)
			}
		}
		applyManifestExclusions(&state.value, declaredStart, excludedStart)
	}
	associateDeclaredMemberships(projects, workspaces)
	return nil
}

func (b Builder) resolveWorkspaceMember(
	ctx context.Context,
	workspaceRoot string,
	manifestPath string,
	member string,
	exclude bool,
	projectRoots []string,
	partial bool,
) (WorkspaceDeclaration, *Issue, error) {
	declaration := WorkspaceDeclaration{
		ManifestPath: manifestPath,
		Pattern:      member,
		Exclude:      exclude,
		ProjectRoots: make([]string, 0),
	}
	pattern, resolution := repositoryPattern(workspaceRoot, member)
	if resolution != "" {
		declaration.Resolution = resolution
		if resolution == MemberOutsideRoot {
			declaration.Pattern = outsideRootPattern
			return declaration, &Issue{
				Code: IssueMemberOutsideRoot, Path: manifestPath,
				Message: "a workspace declaration outside the selected repository was ignored",
			}, nil
		}
		return declaration, &Issue{
			Code: IssueMemberUnsupported, Path: manifestPath,
			Message: "a workspace declaration uses unsupported pattern semantics",
		}, nil
	}

	matcher, valid := newWorkspaceMatcher(pattern)
	if !valid {
		declaration.Resolution = MemberUnsupported
		return declaration, &Issue{
			Code: IssueMemberUnsupported, Path: manifestPath,
			Message: "a workspace declaration uses unsupported pattern semantics",
		}, nil
	}
	for _, projectRoot := range projectRoots {
		if err := ctx.Err(); err != nil {
			return WorkspaceDeclaration{}, nil, err
		}
		if !matcher.matches(projectRoot) {
			continue
		}
		if len(declaration.ProjectRoots) >= b.limits.MaxMatchesPerDeclaration {
			declaration.MatchesTruncated = true
			declaration.Resolution = MemberMatched
			return declaration, &Issue{
				Code: IssueMemberMatchLimit, Path: manifestPath,
				Message: "additional workspace project matches were omitted by the configured limit",
			}, nil
		}
		declaration.ProjectRoots = append(declaration.ProjectRoots, projectRoot)
	}
	if len(declaration.ProjectRoots) != 0 {
		declaration.Resolution = MemberMatched
		return declaration, nil, nil
	}
	if partial {
		declaration.Resolution = MemberIndeterminate
		return declaration, nil, nil
	}
	declaration.Resolution = MemberUnmatched
	return declaration, &Issue{
		Code: IssueMemberUnmatched, Path: manifestPath,
		Message: "a workspace declaration did not match a known project boundary",
	}, nil
}

func applyManifestExclusions(
	workspace *Workspace,
	declaredStart int,
	excludedStart int,
) {
	if excludedStart == len(workspace.ExcludedProjects) {
		return
	}
	excluded := make(map[string]struct{}, len(workspace.ExcludedProjects)-excludedStart)
	for _, projectRoot := range workspace.ExcludedProjects[excludedStart:] {
		excluded[projectRoot] = struct{}{}
	}
	retained := workspace.DeclaredProjects[:declaredStart]
	for _, projectRoot := range workspace.DeclaredProjects[declaredStart:] {
		if _, removed := excluded[projectRoot]; !removed {
			retained = append(retained, projectRoot)
		}
	}
	workspace.DeclaredProjects = retained
}

func associateDeclaredMemberships(
	projects map[string]*projectState,
	workspaces map[string]*workspaceState,
) {
	for workspaceRoot, state := range workspaces {
		for _, projectRoot := range state.value.DeclaredProjects {
			projects[projectRoot].value.WorkspaceRoots = append(
				projects[projectRoot].value.WorkspaceRoots, workspaceRoot,
			)
		}
	}
}

func repositoryPattern(workspaceRoot, member string) (string, MemberResolution) {
	lower := strings.ToLower(member)
	if strings.Contains(member, "\\") || strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "file:") || path.IsAbs(member) ||
		repositorypath.HasWindowsDrivePrefix(member) {
		return "", MemberOutsideRoot
	}
	if strings.HasPrefix(member, "!") || strings.ContainsAny(member, "{}") ||
		containsExtendedGlob(member) {
		return "", MemberUnsupported
	}
	pattern := path.Clean(path.Join(workspaceRoot, member))
	if pattern == ".." || strings.HasPrefix(pattern, "../") {
		return "", MemberOutsideRoot
	}
	return pattern, ""
}

func containsExtendedGlob(value string) bool {
	for _, prefix := range []string{"@(", "+(", "?(", "*(", "!("} {
		if strings.Contains(value, prefix) {
			return true
		}
	}
	return false
}

type workspaceMatcher struct {
	segments []string
}

func newWorkspaceMatcher(pattern string) (workspaceMatcher, bool) {
	segments := strings.Split(pattern, "/")
	for _, segment := range segments {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return workspaceMatcher{}, false
		}
	}
	return workspaceMatcher{segments: segments}, true
}

func (m workspaceMatcher) matches(candidatePath string) bool {
	candidate := strings.Split(candidatePath, "/")
	patternIndex := 0
	candidateIndex := 0
	globstarIndex := -1
	globstarCandidate := 0

	for candidateIndex < len(candidate) {
		if patternIndex < len(m.segments) && m.segments[patternIndex] != "**" {
			matched, _ := path.Match(m.segments[patternIndex], candidate[candidateIndex])
			if matched {
				patternIndex++
				candidateIndex++
				continue
			}
		}
		if patternIndex < len(m.segments) && m.segments[patternIndex] == "**" {
			globstarIndex = patternIndex
			globstarCandidate = candidateIndex
			patternIndex++
			continue
		}
		if globstarIndex < 0 {
			return false
		}
		globstarCandidate++
		candidateIndex = globstarCandidate
		patternIndex = globstarIndex + 1
	}
	for patternIndex < len(m.segments) && m.segments[patternIndex] == "**" {
		patternIndex++
	}
	return patternIndex == len(m.segments)
}

func nearestWorkspace(root string, workspaces map[string]struct{}) string {
	for candidate := root; ; candidate = path.Dir(candidate) {
		if _, exists := workspaces[candidate]; exists {
			return candidate
		}
		if candidate == "." {
			return ""
		}
	}
}
