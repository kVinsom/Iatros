package topology

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
	"github.com/kVinsom/Iatros/internal/repositorypath"
)

func validateInputs(
	ctx context.Context,
	projects []project.Boundary,
	workspaces []project.Workspace,
	manifests []manifest.Manifest,
	limits Limits,
) error {
	seenProjects := make(map[string]struct{}, len(projects))
	for index, projectBoundary := range projects {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validDirectory(projectBoundary.Root) || !validKind(projectBoundary.Kind) ||
			(projectBoundary.WorkspaceRoot != "" &&
				!validDirectory(projectBoundary.WorkspaceRoot)) {
			return ErrInvalidSnapshot
		}
		if err := validateProjectMarkers(
			ctx,
			projectBoundary.Markers,
			limits.MaxValueBytes,
		); err != nil {
			return err
		}
		if _, exists := seenProjects[projectBoundary.Root]; exists {
			return ErrInvalidSnapshot
		}
		seenProjects[projectBoundary.Root] = struct{}{}
	}

	seenWorkspaces := make(map[string]struct{}, len(workspaces))
	for index, workspace := range workspaces {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validDirectory(workspace.Root) {
			return ErrInvalidSnapshot
		}
		if err := validateProjectMarkers(ctx, workspace.Markers, limits.MaxValueBytes); err != nil {
			return err
		}
		if _, exists := seenWorkspaces[workspace.Root]; exists {
			return ErrInvalidSnapshot
		}
		seenWorkspaces[workspace.Root] = struct{}{}
	}

	seenManifests := make(map[string]struct{}, len(manifests))
	for index, manifestDocument := range manifests {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if err := validateManifest(ctx, manifestDocument, limits.MaxValueBytes); err != nil {
			return err
		}
		if _, exists := seenManifests[manifestDocument.Path]; exists {
			return ErrInvalidSnapshot
		}
		seenManifests[manifestDocument.Path] = struct{}{}
	}
	return ctx.Err()
}

func validateManifest(
	ctx context.Context,
	manifestDocument manifest.Manifest,
	maximumBytes int,
) error {
	if !validFile(manifestDocument.Path) ||
		!validIdentifier(string(manifestDocument.Format)) ||
		!validManifestWorkspaceState(manifestDocument) ||
		!validOptionalText(manifestDocument.Name, maximumBytes) ||
		!validOptionalText(manifestDocument.Version, maximumBytes) ||
		!validOptionalText(manifestDocument.Module, maximumBytes) {
		return ErrInvalidSnapshot
	}
	for index, dependency := range manifestDocument.Dependencies {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(dependency.Name, maximumBytes) ||
			!validOptionalText(dependency.Constraint, maximumBytes) ||
			!validScope(dependency.Scope) {
			return ErrInvalidSnapshot
		}
	}
	for index, constraint := range manifestDocument.Constraints {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(constraint.Name, maximumBytes) ||
			!validOptionalText(constraint.Value, maximumBytes) ||
			!validConstraintScope(constraint.Scope) {
			return ErrInvalidSnapshot
		}
	}
	for index, workspaceMember := range manifestDocument.WorkspaceMembers {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(workspaceMember, maximumBytes) {
			return ErrInvalidSnapshot
		}
	}
	for index, workspaceExclusion := range manifestDocument.WorkspaceExcludes {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(workspaceExclusion, maximumBytes) {
			return ErrInvalidSnapshot
		}
	}
	return ctx.Err()
}

func validManifestWorkspaceState(manifestDocument manifest.Manifest) bool {
	switch manifestDocument.Format {
	case manifest.FormatGoWorkspace:
		return manifestDocument.WorkspaceDeclared
	case manifest.FormatGoModule, manifest.FormatPythonProject, manifest.FormatPHPComposer:
		return !manifestDocument.WorkspaceDeclared
	default:
		return true
	}
}

func validateProjectMarkers(
	ctx context.Context,
	markers []project.Marker,
	maximum int,
) error {
	for index, marker := range markers {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validIdentifier(marker.ID) {
			return ErrInvalidSnapshot
		}
		for evidenceIndex, evidence := range marker.Evidence {
			if err := periodicContextError(ctx, evidenceIndex); err != nil {
				return err
			}
			if !validFile(evidence) || len(evidence) > maximum {
				return ErrInvalidSnapshot
			}
		}
	}
	return ctx.Err()
}

func periodicContextError(ctx context.Context, index int) error {
	if index%128 != 0 {
		return nil
	}
	return ctx.Err()
}

func validKind(kind project.Kind) bool {
	switch kind {
	case project.KindCode, project.KindInfrastructure, project.KindMixed:
		return true
	default:
		return false
	}
}

func validScope(scope manifest.Scope) bool {
	switch scope {
	case manifest.ScopeRuntime, manifest.ScopeDevelopment, manifest.ScopeBuild, manifest.ScopePeer:
		return true
	default:
		return false
	}
}

func validConstraintScope(scope manifest.Scope) bool {
	switch scope {
	case manifest.ScopeRuntime, manifest.ScopeDevelopment, manifest.ScopeBuild:
		return true
	default:
		return false
	}
}

func validDirectory(directoryPath string) bool {
	return repositorypath.IsValidDirectory(directoryPath)
}

func validFile(filePath string) bool {
	return repositorypath.IsValidFile(filePath)
}

func validIssuePath(issuePath string) bool {
	return repositorypath.IsValidDirectory(issuePath)
}

func validOptionalText(text string, maximumBytes int) bool {
	return text == "" || validText(text, maximumBytes)
}

func validText(text string, maximumBytes int) bool {
	if text == "" || len(text) > maximumBytes || !utf8.ValidString(text) ||
		strings.TrimSpace(text) != text {
		return false
	}
	for _, character := range text {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func validIdentifier(identifier string) bool {
	if identifier == "" || !lowerAlphaNumeric(identifier[0]) ||
		!lowerAlphaNumeric(identifier[len(identifier)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(identifier) {
		character := identifier[index]
		if lowerAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if character != '_' && character != '-' && character != '.' || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func validIssueCode(code string) bool {
	if code == "" || code[0] < 'A' || code[0] > 'Z' {
		return false
	}
	for _, character := range code {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func copyMarkers(sourceMarkers []project.Marker) []Marker {
	markers := make([]Marker, 0, len(sourceMarkers))
	for _, sourceMarker := range sourceMarkers {
		evidence := slices.Clone(sourceMarker.Evidence)
		slices.Sort(evidence)
		evidence = slices.Compact(evidence)
		markers = append(markers, Marker{
			ID:                sourceMarker.ID,
			Evidence:          evidence,
			EvidenceTruncated: sourceMarker.EvidenceTruncated,
		})
	}
	slices.SortFunc(markers, func(left, right Marker) int {
		return strings.Compare(left.ID, right.ID)
	})
	return slices.CompactFunc(markers, func(left, right Marker) bool {
		return left.ID == right.ID && left.EvidenceTruncated == right.EvidenceTruncated &&
			slices.Equal(left.Evidence, right.Evidence)
	})
}

func componentFromManifest(manifestDocument manifest.Manifest) Component {
	constraints := make([]Constraint, 0, len(manifestDocument.Constraints))
	for _, constraint := range manifestDocument.Constraints {
		constraints = append(constraints, Constraint{
			Name: constraint.Name, Value: constraint.Value, Scope: string(constraint.Scope),
		})
	}
	slices.SortFunc(constraints, compareConstraints)
	constraints = slices.Compact(constraints)
	return Component{
		ManifestPath:               manifestDocument.Path,
		Format:                     string(manifestDocument.Format),
		Name:                       manifestDocument.Name,
		Version:                    manifestDocument.Version,
		Module:                     manifestDocument.Module,
		Constraints:                constraints,
		WorkspaceDeclared:          manifestDocument.WorkspaceDeclared,
		DependenciesTruncated:      manifestDocument.DependenciesTruncated,
		ConstraintsTruncated:       manifestDocument.ConstraintsTruncated,
		WorkspaceMembersTruncated:  manifestDocument.WorkspaceMembersTruncated,
		WorkspaceExcludesTruncated: manifestDocument.WorkspaceExcludesTruncated,
	}
}

func compareConstraints(left, right Constraint) int {
	if compared := strings.Compare(left.Name, right.Name); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Scope, right.Scope); compared != 0 {
		return compared
	}
	return strings.Compare(left.Value, right.Value)
}

func compareComponents(left, right Component) int {
	return strings.Compare(left.ManifestPath, right.ManifestPath)
}

func finalizeProjects(states map[string]*projectState) []Project {
	projects := make([]Project, 0, len(states))
	for _, state := range states {
		slices.Sort(state.project.WorkspaceRoots)
		state.project.WorkspaceRoots = slices.Compact(state.project.WorkspaceRoots)
		slices.SortFunc(state.project.Components, compareComponents)
		state.project.Components = slices.CompactFunc(state.project.Components, equalComponents)
		projects = append(projects, state.project)
	}
	slices.SortFunc(projects, func(left, right Project) int {
		return strings.Compare(left.Root, right.Root)
	})
	return projects
}

func finalizeWorkspaces(states map[string]*workspaceState) []Workspace {
	workspaces := make([]Workspace, 0, len(states))
	for _, state := range states {
		slices.SortFunc(state.workspace.Components, compareComponents)
		state.workspace.Components = slices.CompactFunc(state.workspace.Components, equalComponents)
		slices.Sort(state.workspace.ContainedProjects)
		state.workspace.ContainedProjects = slices.Compact(state.workspace.ContainedProjects)
		slices.Sort(state.workspace.DeclaredProjects)
		state.workspace.DeclaredProjects = slices.Compact(state.workspace.DeclaredProjects)
		slices.Sort(state.workspace.ExcludedProjects)
		state.workspace.ExcludedProjects = slices.Compact(state.workspace.ExcludedProjects)
		slices.SortFunc(state.workspace.Declarations, compareDeclarations)
		state.workspace.Declarations = slices.CompactFunc(state.workspace.Declarations, equalDeclarations)
		workspaces = append(workspaces, state.workspace)
	}
	slices.SortFunc(workspaces, func(left, right Workspace) int {
		return strings.Compare(left.Root, right.Root)
	})
	return workspaces
}

func compareDeclarations(left, right WorkspaceDeclaration) int {
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

func equalComponents(left, right Component) bool {
	return left.ManifestPath == right.ManifestPath && left.Format == right.Format &&
		left.Name == right.Name && left.Version == right.Version && left.Module == right.Module &&
		left.WorkspaceDeclared == right.WorkspaceDeclared &&
		left.DependenciesTruncated == right.DependenciesTruncated &&
		left.ConstraintsTruncated == right.ConstraintsTruncated &&
		left.WorkspaceMembersTruncated == right.WorkspaceMembersTruncated &&
		left.WorkspaceExcludesTruncated == right.WorkspaceExcludesTruncated &&
		slices.Equal(left.Constraints, right.Constraints)
}

func equalDeclarations(left, right WorkspaceDeclaration) bool {
	return left.ManifestPath == right.ManifestPath && left.Pattern == right.Pattern &&
		left.Exclude == right.Exclude && left.Resolution == right.Resolution &&
		left.MatchesTruncated == right.MatchesTruncated &&
		slices.Equal(left.ProjectRoots, right.ProjectRoots)
}
