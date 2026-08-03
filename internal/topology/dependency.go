package topology

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/kVinsom/Iatros/internal/manifest"
)

type identityKey struct {
	ecosystem string
	name      string
}

type identityTargets struct {
	roots []string
}

func (b Builder) buildDependencies(
	ctx context.Context,
	model *Model,
	projects map[string]*projectState,
) {
	projectRoots := make([]string, 0, len(projects))
	for root := range projects {
		projectRoots = append(projectRoots, root)
	}
	slices.Sort(projectRoots)

	identityRootLimit := b.limits.MaxTargetsPerDependency
	if identityRootLimit < len(projectRoots) {
		identityRootLimit++
	}
	identities := make(map[identityKey]*identityTargets)
	for _, root := range projectRoots {
		state := projects[root]
		for _, value := range state.manifests {
			if err := ctx.Err(); err != nil {
				return
			}
			identity := manifestIdentity(value)
			if identity.name == "" {
				continue
			}
			targets := identities[identity]
			if targets == nil {
				targets = &identityTargets{roots: make([]string, 0, min(identityRootLimit, 4))}
				identities[identity] = targets
			}
			targets.add(root, identityRootLimit)
		}
	}

	for _, root := range projectRoots {
		state := projects[root]
		for _, value := range state.manifests {
			if err := ctx.Err(); err != nil {
				return
			}
			ecosystem := ecosystemFor(value.Format)
			for _, declaration := range value.Dependencies {
				if err := ctx.Err(); err != nil {
					return
				}
				if len(model.Dependencies) >= b.limits.MaxDependencies {
					model.addIssue(Issue{
						Code: IssueDependencyLimit, Path: value.Path,
						Message: "additional dependency declarations were omitted by the configured limit",
					}, b.limits.MaxIssues)
					return
				}
				identity := identities[identityKey{
					ecosystem: ecosystem,
					name:      normalizeIdentity(ecosystem, declaration.Name),
				}]
				var targetRoots []string
				if identity != nil {
					targetRoots = slices.Clone(identity.roots)
				}
				dependency := Dependency{
					FromProject:    root,
					ManifestPath:   value.Path,
					Ecosystem:      ecosystem,
					Name:           declaration.Name,
					Constraint:     declaration.Constraint,
					Scope:          string(declaration.Scope),
					Indirect:       declaration.Indirect,
					Optional:       declaration.Optional,
					TargetProjects: make([]string, 0),
				}
				switch len(targetRoots) {
				case 0:
					dependency.Resolution = DependencyUnresolved
				case 1:
					dependency.Resolution = DependencyInternal
					dependency.TargetProjects = targetRoots
				default:
					dependency.Resolution = DependencyAmbiguous
					if len(targetRoots) > b.limits.MaxTargetsPerDependency {
						dependency.TargetsTruncated = true
						targetRoots = targetRoots[:b.limits.MaxTargetsPerDependency]
						model.addIssue(Issue{
							Code: IssueDependencyTargetLimit, Path: value.Path,
							Message: "additional ambiguous dependency targets were omitted by the configured limit",
						}, b.limits.MaxIssues)
					}
					dependency.TargetProjects = targetRoots
					model.addIssue(Issue{
						Code: IssueDependencyAmbiguous, Path: value.Path,
						Message: "a dependency identity matches multiple repository projects",
					}, b.limits.MaxIssues)
				}
				model.Dependencies = append(model.Dependencies, dependency)
			}
		}
	}
}

func (targets *identityTargets) add(root string, maximum int) {
	index, exists := slices.BinarySearch(targets.roots, root)
	if exists || index >= maximum {
		return
	}
	if len(targets.roots) < maximum {
		targets.roots = append(targets.roots, "")
	} else {
		copy(targets.roots[index+1:], targets.roots[index:len(targets.roots)-1])
		targets.roots[index] = root
		return
	}
	copy(targets.roots[index+1:], targets.roots[index:len(targets.roots)-1])
	targets.roots[index] = root
}

func manifestIdentity(value manifest.Manifest) identityKey {
	ecosystem := ecosystemFor(value.Format)
	name := value.Name
	if value.Format == manifest.FormatGoModule {
		name = value.Module
	}
	return identityKey{ecosystem: ecosystem, name: normalizeIdentity(ecosystem, name)}
}

func ecosystemFor(format manifest.Format) string {
	switch format {
	case manifest.FormatGoModule, manifest.FormatGoWorkspace:
		return "go"
	case manifest.FormatNodePackage:
		return "node"
	case manifest.FormatPythonProject:
		return "python"
	case manifest.FormatRustPackage:
		return "rust"
	case manifest.FormatPHPComposer:
		return "php"
	case manifest.FormatMavenProject:
		return "maven"
	default:
		return string(format)
	}
}

func normalizeIdentity(ecosystem, value string) string {
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return ""
	}
	switch ecosystem {
	case "node", "php":
		return strings.ToLower(value)
	case "python":
		return normalizeDelimitedIdentity(value, func(character rune) bool {
			return character == '-' || character == '_' || character == '.'
		})
	case "rust":
		return normalizeDelimitedIdentity(value, func(character rune) bool {
			return character == '-' || character == '_'
		})
	default:
		return value
	}
}

func normalizeDelimitedIdentity(value string, delimiter func(rune) bool) string {
	var normalized strings.Builder
	previousDelimiter := false
	for _, character := range strings.ToLower(value) {
		if delimiter(character) {
			if !previousDelimiter {
				normalized.WriteByte('-')
			}
			previousDelimiter = true
			continue
		}
		normalized.WriteRune(character)
		previousDelimiter = false
	}
	return normalized.String()
}

func compareDependencies(left, right Dependency) int {
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
	return strings.Compare(string(left.Resolution), string(right.Resolution))
}

func equalDependencies(left, right Dependency) bool {
	return left.FromProject == right.FromProject && left.ManifestPath == right.ManifestPath &&
		left.Ecosystem == right.Ecosystem && left.Name == right.Name &&
		left.Constraint == right.Constraint && left.Scope == right.Scope &&
		left.Indirect == right.Indirect && left.Optional == right.Optional &&
		left.Resolution == right.Resolution && left.TargetsTruncated == right.TargetsTruncated &&
		slices.Equal(left.TargetProjects, right.TargetProjects)
}
