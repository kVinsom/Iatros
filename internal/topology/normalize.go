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
	for index, value := range projects {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validDirectory(value.Root) || !validKind(value.Kind) ||
			(value.WorkspaceRoot != "" && !validDirectory(value.WorkspaceRoot)) {
			return ErrInvalidSnapshot
		}
		if err := validateProjectMarkers(ctx, value.Markers, limits.MaxValueBytes); err != nil {
			return err
		}
		if _, exists := seenProjects[value.Root]; exists {
			return ErrInvalidSnapshot
		}
		seenProjects[value.Root] = struct{}{}
	}

	seenWorkspaces := make(map[string]struct{}, len(workspaces))
	for index, value := range workspaces {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validDirectory(value.Root) {
			return ErrInvalidSnapshot
		}
		if err := validateProjectMarkers(ctx, value.Markers, limits.MaxValueBytes); err != nil {
			return err
		}
		if _, exists := seenWorkspaces[value.Root]; exists {
			return ErrInvalidSnapshot
		}
		seenWorkspaces[value.Root] = struct{}{}
	}

	seenManifests := make(map[string]struct{}, len(manifests))
	for index, value := range manifests {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if err := validateManifest(ctx, value, limits.MaxValueBytes); err != nil {
			return err
		}
		if _, exists := seenManifests[value.Path]; exists {
			return ErrInvalidSnapshot
		}
		seenManifests[value.Path] = struct{}{}
	}
	return ctx.Err()
}

func validateManifest(ctx context.Context, value manifest.Manifest, maximum int) error {
	if !validFile(value.Path) || !validIdentifier(string(value.Format)) ||
		!validManifestWorkspaceState(value) ||
		!validOptionalText(value.Name, maximum) || !validOptionalText(value.Version, maximum) ||
		!validOptionalText(value.Module, maximum) {
		return ErrInvalidSnapshot
	}
	for index, dependency := range value.Dependencies {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(dependency.Name, maximum) ||
			!validOptionalText(dependency.Constraint, maximum) || !validScope(dependency.Scope) {
			return ErrInvalidSnapshot
		}
	}
	for index, constraint := range value.Constraints {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(constraint.Name, maximum) ||
			!validOptionalText(constraint.Value, maximum) || !validConstraintScope(constraint.Scope) {
			return ErrInvalidSnapshot
		}
	}
	for index, member := range value.WorkspaceMembers {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(member, maximum) {
			return ErrInvalidSnapshot
		}
	}
	for index, excluded := range value.WorkspaceExcludes {
		if err := periodicContextError(ctx, index); err != nil {
			return err
		}
		if !validText(excluded, maximum) {
			return ErrInvalidSnapshot
		}
	}
	return ctx.Err()
}

func validManifestWorkspaceState(value manifest.Manifest) bool {
	switch value.Format {
	case manifest.FormatGoWorkspace:
		return value.WorkspaceDeclared
	case manifest.FormatGoModule, manifest.FormatPythonProject, manifest.FormatPHPComposer:
		return !value.WorkspaceDeclared
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

func validDirectory(value string) bool {
	return repositorypath.IsValidDirectory(value)
}

func validFile(value string) bool {
	return repositorypath.IsValidFile(value)
}

func validIssuePath(value string) bool {
	return repositorypath.IsValidDirectory(value)
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || !lowerAlphaNumeric(value[0]) ||
		!lowerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
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

func lowerAlphaNumeric(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9')
}

func validIssueCode(value string) bool {
	if value == "" || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func copyMarkers(values []project.Marker) []Marker {
	markers := make([]Marker, 0, len(values))
	for _, value := range values {
		evidence := slices.Clone(value.Evidence)
		slices.Sort(evidence)
		evidence = slices.Compact(evidence)
		markers = append(markers, Marker{
			ID:                value.ID,
			Evidence:          evidence,
			EvidenceTruncated: value.EvidenceTruncated,
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

func componentFromManifest(value manifest.Manifest) Component {
	constraints := make([]Constraint, 0, len(value.Constraints))
	for _, constraint := range value.Constraints {
		constraints = append(constraints, Constraint{
			Name: constraint.Name, Value: constraint.Value, Scope: string(constraint.Scope),
		})
	}
	slices.SortFunc(constraints, compareConstraints)
	constraints = slices.Compact(constraints)
	return Component{
		ManifestPath:               value.Path,
		Format:                     string(value.Format),
		Name:                       value.Name,
		Version:                    value.Version,
		Module:                     value.Module,
		Constraints:                constraints,
		WorkspaceDeclared:          value.WorkspaceDeclared,
		DependenciesTruncated:      value.DependenciesTruncated,
		ConstraintsTruncated:       value.ConstraintsTruncated,
		WorkspaceMembersTruncated:  value.WorkspaceMembersTruncated,
		WorkspaceExcludesTruncated: value.WorkspaceExcludesTruncated,
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
	values := make([]Project, 0, len(states))
	for _, state := range states {
		slices.Sort(state.value.WorkspaceRoots)
		state.value.WorkspaceRoots = slices.Compact(state.value.WorkspaceRoots)
		slices.SortFunc(state.value.Components, compareComponents)
		state.value.Components = slices.CompactFunc(state.value.Components, equalComponents)
		values = append(values, state.value)
	}
	slices.SortFunc(values, func(left, right Project) int {
		return strings.Compare(left.Root, right.Root)
	})
	return values
}

func finalizeWorkspaces(states map[string]*workspaceState) []Workspace {
	values := make([]Workspace, 0, len(states))
	for _, state := range states {
		slices.SortFunc(state.value.Components, compareComponents)
		state.value.Components = slices.CompactFunc(state.value.Components, equalComponents)
		slices.Sort(state.value.ContainedProjects)
		state.value.ContainedProjects = slices.Compact(state.value.ContainedProjects)
		slices.Sort(state.value.DeclaredProjects)
		state.value.DeclaredProjects = slices.Compact(state.value.DeclaredProjects)
		slices.Sort(state.value.ExcludedProjects)
		state.value.ExcludedProjects = slices.Compact(state.value.ExcludedProjects)
		slices.SortFunc(state.value.Declarations, compareDeclarations)
		state.value.Declarations = slices.CompactFunc(state.value.Declarations, equalDeclarations)
		values = append(values, state.value)
	}
	slices.SortFunc(values, func(left, right Workspace) int {
		return strings.Compare(left.Root, right.Root)
	})
	return values
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
