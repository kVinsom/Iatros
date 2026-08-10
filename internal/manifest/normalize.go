package manifest

import (
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

const redactedReference = "[redacted-reference]"

func normalizeManifest(
	parsedManifest Manifest,
	candidate manifestCandidate,
	limits Limits,
) (Manifest, []Issue, error) {
	normalized := detachedManifest(parsedManifest, candidate)
	if !validNormalizedManifest(normalized, limits.MaxValueBytes) {
		return Manifest{}, nil, errInvalidParserResult
	}
	sortAndCompactManifest(&normalized)
	issues := truncateManifestCollections(&normalized, candidate.path, limits)
	return normalized, issues, nil
}

func detachedManifest(parsedManifest Manifest, candidate manifestCandidate) Manifest {
	parsedManifest.Path = candidate.path
	parsedManifest.Format = candidate.format
	parsedManifest.Dependencies = slices.Clone(parsedManifest.Dependencies)
	parsedManifest.Constraints = slices.Clone(parsedManifest.Constraints)
	parsedManifest.WorkspaceMembers = slices.Clone(parsedManifest.WorkspaceMembers)
	parsedManifest.WorkspaceExcludes = slices.Clone(parsedManifest.WorkspaceExcludes)
	parsedManifest.Version = sanitizeReference(parsedManifest.Version)
	for index := range parsedManifest.Dependencies {
		parsedManifest.Dependencies[index].Constraint = sanitizeReference(
			parsedManifest.Dependencies[index].Constraint,
		)
	}
	for index := range parsedManifest.Constraints {
		parsedManifest.Constraints[index].Value = sanitizeReference(
			parsedManifest.Constraints[index].Value,
		)
	}
	return parsedManifest
}

func validNormalizedManifest(candidate Manifest, maximumBytes int) bool {
	if !validWorkspaceDeclaredState(candidate) ||
		!validOptionalManifestText(candidate.Name, maximumBytes) ||
		!validOptionalManifestText(candidate.Version, maximumBytes) ||
		!validOptionalManifestText(candidate.Module, maximumBytes) {
		return false
	}

	for _, dependency := range candidate.Dependencies {
		if !validManifestText(dependency.Name, maximumBytes) ||
			!validOptionalManifestText(dependency.Constraint, maximumBytes) ||
			!validScope(dependency.Scope) {
			return false
		}
	}
	for _, constraint := range candidate.Constraints {
		if !validManifestText(constraint.Name, maximumBytes) ||
			!validOptionalManifestText(constraint.Value, maximumBytes) ||
			!validConstraintScope(constraint.Scope) {
			return false
		}
	}
	for _, workspaceMember := range candidate.WorkspaceMembers {
		if !validManifestText(workspaceMember, maximumBytes) {
			return false
		}
	}
	for _, workspaceExclusion := range candidate.WorkspaceExcludes {
		if !validManifestText(workspaceExclusion, maximumBytes) {
			return false
		}
	}
	return true
}

func sortAndCompactManifest(candidate *Manifest) {
	slices.SortFunc(candidate.Dependencies, compareDependencies)
	candidate.Dependencies = slices.Compact(candidate.Dependencies)
	slices.SortFunc(candidate.Constraints, compareConstraints)
	candidate.Constraints = slices.Compact(candidate.Constraints)
	slices.Sort(candidate.WorkspaceMembers)
	candidate.WorkspaceMembers = slices.Compact(candidate.WorkspaceMembers)
	slices.Sort(candidate.WorkspaceExcludes)
	candidate.WorkspaceExcludes = slices.Compact(candidate.WorkspaceExcludes)
}

func truncateManifestCollections(candidate *Manifest, manifestPath string, limits Limits) []Issue {
	issues := make([]Issue, 0, 4)
	if len(candidate.Dependencies) > limits.MaxDependenciesPerManifest {
		candidate.Dependencies = slices.Clone(candidate.Dependencies[:limits.MaxDependenciesPerManifest])
		candidate.DependenciesTruncated = true
		issues = append(issues, Issue{
			Code:    IssueDependencyLimit,
			Path:    manifestPath,
			Message: "additional direct dependencies were omitted by the configured limit",
		})
	}
	if len(candidate.Constraints) > limits.MaxConstraintsPerManifest {
		candidate.Constraints = slices.Clone(candidate.Constraints[:limits.MaxConstraintsPerManifest])
		candidate.ConstraintsTruncated = true
		issues = append(issues, Issue{
			Code:    IssueConstraintLimit,
			Path:    manifestPath,
			Message: "additional runtime constraints were omitted by the configured limit",
		})
	}
	if len(candidate.WorkspaceMembers) > limits.MaxWorkspaceMembersPerManifest {
		candidate.WorkspaceMembers = slices.Clone(candidate.WorkspaceMembers[:limits.MaxWorkspaceMembersPerManifest])
		candidate.WorkspaceMembersTruncated = true
		issues = append(issues, Issue{
			Code:    IssueWorkspaceLimit,
			Path:    manifestPath,
			Message: "additional workspace members were omitted by the configured limit",
		})
	}
	if len(candidate.WorkspaceExcludes) > limits.MaxWorkspaceExcludesPerManifest {
		candidate.WorkspaceExcludes = slices.Clone(candidate.WorkspaceExcludes[:limits.MaxWorkspaceExcludesPerManifest])
		candidate.WorkspaceExcludesTruncated = true
		issues = append(issues, Issue{
			Code:    IssueWorkspaceExcludeLimit,
			Path:    manifestPath,
			Message: "additional workspace exclusions were omitted by the configured limit",
		})
	}
	return issues
}

func validWorkspaceDeclaredState(candidate Manifest) bool {
	switch candidate.Format {
	case FormatGoWorkspace:
		return candidate.WorkspaceDeclared
	case FormatGoModule, FormatPythonProject, FormatPHPComposer:
		return !candidate.WorkspaceDeclared
	default:
		return true
	}
}

func sanitizeReference(reference string) string {
	target := strings.TrimSpace(reference)
	lower := strings.ToLower(target)
	if strings.Contains(lower, "://") {
		return redactedReference
	}
	if strings.HasPrefix(target, "@") {
		target = strings.TrimSpace(target[1:])
		lower = strings.ToLower(target)
	}
	if looksLikeLocalReference(target) || looksLikeSCPReference(target) {
		return redactedReference
	}
	if scheme, remainder, ok := splitReferenceScheme(lower); ok {
		switch scheme {
		case "catalog", "npm", "workspace":
			if looksLikeLocalReference(strings.TrimSpace(remainder)) {
				return redactedReference
			}
		default:
			return redactedReference
		}
	}
	return reference
}

func looksLikeLocalReference(reference string) bool {
	return path.IsAbs(reference) || looksLikeWindowsAbsolutePath(reference) ||
		strings.HasPrefix(reference, `\\`) ||
		strings.HasPrefix(reference, "./") || strings.HasPrefix(reference, "../") ||
		strings.HasPrefix(reference, "~/") || strings.HasPrefix(reference, `.\`) ||
		strings.HasPrefix(reference, `..\`) || strings.HasPrefix(reference, `~\`)
}

func looksLikeWindowsAbsolutePath(reference string) bool {
	return len(reference) >= 2 &&
		((reference[0] >= 'A' && reference[0] <= 'Z') ||
			(reference[0] >= 'a' && reference[0] <= 'z')) &&
		reference[1] == ':'
}

func looksLikeSCPReference(reference string) bool {
	at := strings.IndexByte(reference, '@')
	if at <= 0 {
		return false
	}
	colon := strings.IndexByte(reference[at+1:], ':')
	return colon > 0 && at+1+colon < len(reference)-1
}

func splitReferenceScheme(reference string) (string, string, bool) {
	colon := strings.IndexByte(reference, ':')
	if colon <= 0 || reference[0] < 'a' || reference[0] > 'z' {
		return "", "", false
	}
	for index := 1; index < colon; index++ {
		character := reference[index]
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '+' && character != '-' && character != '.' {
			return "", "", false
		}
	}
	return reference[:colon], reference[colon+1:], true
}

func validOptionalManifestText(text string, maximumBytes int) bool {
	return text == "" || validManifestText(text, maximumBytes)
}

func validScope(scope Scope) bool {
	switch scope {
	case ScopeRuntime, ScopeDevelopment, ScopeBuild, ScopePeer:
		return true
	default:
		return false
	}
}

func validConstraintScope(scope Scope) bool {
	switch scope {
	case ScopeRuntime, ScopeDevelopment, ScopeBuild:
		return true
	default:
		return false
	}
}

func compareDependencies(left, right Dependency) int {
	if compared := strings.Compare(left.Name, right.Name); compared != 0 {
		return compared
	}
	if compared := strings.Compare(string(left.Scope), string(right.Scope)); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Constraint, right.Constraint); compared != 0 {
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
	return 0
}

func compareConstraints(left, right Constraint) int {
	if compared := strings.Compare(left.Name, right.Name); compared != 0 {
		return compared
	}
	if compared := strings.Compare(string(left.Scope), string(right.Scope)); compared != 0 {
		return compared
	}
	return strings.Compare(left.Value, right.Value)
}
