package manifest

import (
	"path"
	"slices"
	"strings"
)

const redactedReference = "[redacted-reference]"

func normalizeManifest(
	value Manifest,
	candidate manifestCandidate,
	limits Limits,
) (Manifest, []Issue, error) {
	value.Path = candidate.path
	value.Format = candidate.format
	value.Dependencies = slices.Clone(value.Dependencies)
	value.Constraints = slices.Clone(value.Constraints)
	value.WorkspaceMembers = slices.Clone(value.WorkspaceMembers)
	value.WorkspaceExcludes = slices.Clone(value.WorkspaceExcludes)
	value.Version = sanitizeReference(value.Version)
	if !validWorkspaceDeclaredState(value.Format, value.WorkspaceDeclared) ||
		!validOptionalValue(value.Name, limits.MaxValueBytes) ||
		!validOptionalValue(value.Version, limits.MaxValueBytes) ||
		!validOptionalValue(value.Module, limits.MaxValueBytes) {
		return Manifest{}, nil, errInvalidParserResult
	}

	for index := range value.Dependencies {
		value.Dependencies[index].Constraint = sanitizeReference(value.Dependencies[index].Constraint)
		dependency := value.Dependencies[index]
		if !validValue(dependency.Name, limits.MaxValueBytes) ||
			!validOptionalValue(dependency.Constraint, limits.MaxValueBytes) ||
			!validScope(dependency.Scope) {
			return Manifest{}, nil, errInvalidParserResult
		}
	}
	for index := range value.Constraints {
		value.Constraints[index].Value = sanitizeReference(value.Constraints[index].Value)
		constraint := value.Constraints[index]
		if !validValue(constraint.Name, limits.MaxValueBytes) ||
			!validOptionalValue(constraint.Value, limits.MaxValueBytes) ||
			!validConstraintScope(constraint.Scope) {
			return Manifest{}, nil, errInvalidParserResult
		}
	}
	for _, member := range value.WorkspaceMembers {
		if !validValue(member, limits.MaxValueBytes) {
			return Manifest{}, nil, errInvalidParserResult
		}
	}
	for _, excluded := range value.WorkspaceExcludes {
		if !validValue(excluded, limits.MaxValueBytes) {
			return Manifest{}, nil, errInvalidParserResult
		}
	}

	slices.SortFunc(value.Dependencies, compareDependencies)
	value.Dependencies = slices.Compact(value.Dependencies)
	slices.SortFunc(value.Constraints, compareConstraints)
	value.Constraints = slices.Compact(value.Constraints)
	slices.Sort(value.WorkspaceMembers)
	value.WorkspaceMembers = slices.Compact(value.WorkspaceMembers)
	slices.Sort(value.WorkspaceExcludes)
	value.WorkspaceExcludes = slices.Compact(value.WorkspaceExcludes)

	issues := make([]Issue, 0, 4)
	if len(value.Dependencies) > limits.MaxDependenciesPerManifest {
		value.Dependencies = slices.Clone(value.Dependencies[:limits.MaxDependenciesPerManifest])
		value.DependenciesTruncated = true
		issues = append(issues, Issue{
			Code:    IssueDependencyLimit,
			Path:    candidate.path,
			Message: "additional direct dependencies were omitted by the configured limit",
		})
	}
	if len(value.Constraints) > limits.MaxConstraintsPerManifest {
		value.Constraints = slices.Clone(value.Constraints[:limits.MaxConstraintsPerManifest])
		value.ConstraintsTruncated = true
		issues = append(issues, Issue{
			Code:    IssueConstraintLimit,
			Path:    candidate.path,
			Message: "additional runtime constraints were omitted by the configured limit",
		})
	}
	if len(value.WorkspaceMembers) > limits.MaxWorkspaceMembersPerManifest {
		value.WorkspaceMembers = slices.Clone(value.WorkspaceMembers[:limits.MaxWorkspaceMembersPerManifest])
		value.WorkspaceMembersTruncated = true
		issues = append(issues, Issue{
			Code:    IssueWorkspaceLimit,
			Path:    candidate.path,
			Message: "additional workspace members were omitted by the configured limit",
		})
	}
	if len(value.WorkspaceExcludes) > limits.MaxWorkspaceExcludesPerManifest {
		value.WorkspaceExcludes = slices.Clone(value.WorkspaceExcludes[:limits.MaxWorkspaceExcludesPerManifest])
		value.WorkspaceExcludesTruncated = true
		issues = append(issues, Issue{
			Code:    IssueWorkspaceExcludeLimit,
			Path:    candidate.path,
			Message: "additional workspace exclusions were omitted by the configured limit",
		})
	}
	return value, issues, nil
}

func validWorkspaceDeclaredState(format Format, declared bool) bool {
	switch format {
	case FormatGoWorkspace:
		return declared
	case FormatGoModule, FormatPythonProject, FormatPHPComposer:
		return !declared
	default:
		return true
	}
}

func sanitizeReference(value string) string {
	target := strings.TrimSpace(value)
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
	return value
}

func looksLikeLocalReference(value string) bool {
	return path.IsAbs(value) || looksLikeWindowsPath(value) ||
		strings.HasPrefix(value, `\\`) ||
		strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `.\`) ||
		strings.HasPrefix(value, `..\`) || strings.HasPrefix(value, `~\`)
}

func looksLikeSCPReference(value string) bool {
	at := strings.IndexByte(value, '@')
	if at <= 0 {
		return false
	}
	colon := strings.IndexByte(value[at+1:], ':')
	return colon > 0 && at+1+colon < len(value)-1
}

func splitReferenceScheme(value string) (string, string, bool) {
	colon := strings.IndexByte(value, ':')
	if colon <= 0 || value[0] < 'a' || value[0] > 'z' {
		return "", "", false
	}
	for index := 1; index < colon; index++ {
		character := value[index]
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '+' && character != '-' && character != '.' {
			return "", "", false
		}
	}
	return value[:colon], value[colon+1:], true
}

func validOptionalValue(value string, maxBytes int) bool {
	return value == "" || validValue(value, maxBytes)
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
