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
	identities, isComplete := indexProjectIdentities(
		ctx,
		projects,
		projectRoots,
		identityRootLimit,
	)
	if !isComplete {
		return
	}

	for _, projectRoot := range projectRoots {
		state := projects[projectRoot]
		for _, manifestDocument := range state.manifests {
			if !b.appendManifestDependencies(
				ctx,
				model,
				projectRoot,
				manifestDocument,
				identities,
			) {
				return
			}
		}
	}
}

func indexProjectIdentities(
	ctx context.Context,
	projects map[string]*projectState,
	projectRoots []string,
	maximumRoots int,
) (map[identityKey]*identityTargets, bool) {
	identities := make(map[identityKey]*identityTargets)
	for _, projectRoot := range projectRoots {
		state := projects[projectRoot]
		for _, manifestDocument := range state.manifests {
			if err := ctx.Err(); err != nil {
				return nil, false
			}
			identity := manifestIdentity(manifestDocument)
			if identity.name == "" {
				continue
			}
			targets := identities[identity]
			if targets == nil {
				targets = &identityTargets{roots: make([]string, 0, min(maximumRoots, 4))}
				identities[identity] = targets
			}
			targets.add(projectRoot, maximumRoots)
		}
	}
	return identities, true
}

func (b Builder) appendManifestDependencies(
	ctx context.Context,
	model *Model,
	projectRoot string,
	manifestDocument manifest.Manifest,
	identities map[identityKey]*identityTargets,
) bool {
	ecosystem := ecosystemFor(manifestDocument.Format)
	for _, declaration := range manifestDocument.Dependencies {
		if err := ctx.Err(); err != nil {
			return false
		}
		if len(model.Dependencies) >= b.limits.MaxDependencies {
			model.addIssue(Issue{
				Code: IssueDependencyLimit, Path: manifestDocument.Path,
				Message: "additional dependency declarations were omitted by the configured limit",
			}, b.limits.MaxIssues)
			return false
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
			FromProject:    projectRoot,
			ManifestPath:   manifestDocument.Path,
			Ecosystem:      ecosystem,
			Name:           declaration.Name,
			Constraint:     declaration.Constraint,
			Scope:          string(declaration.Scope),
			Indirect:       declaration.Indirect,
			Optional:       declaration.Optional,
			TargetProjects: make([]string, 0),
		}
		b.resolveDependencyTargets(model, &dependency, targetRoots)
		model.Dependencies = append(model.Dependencies, dependency)
	}
	return true
}

func (b Builder) resolveDependencyTargets(
	model *Model,
	dependency *Dependency,
	targetRoots []string,
) {
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
				Code: IssueDependencyTargetLimit, Path: dependency.ManifestPath,
				Message: "additional ambiguous dependency targets were omitted by the configured limit",
			}, b.limits.MaxIssues)
		}
		dependency.TargetProjects = targetRoots
		model.addIssue(Issue{
			Code: IssueDependencyAmbiguous, Path: dependency.ManifestPath,
			Message: "a dependency identity matches multiple repository projects",
		}, b.limits.MaxIssues)
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

func manifestIdentity(manifestDocument manifest.Manifest) identityKey {
	ecosystem := ecosystemFor(manifestDocument.Format)
	name := manifestDocument.Name
	if manifestDocument.Format == manifest.FormatGoModule {
		name = manifestDocument.Module
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

func normalizeIdentity(ecosystem, identity string) string {
	if strings.IndexFunc(identity, unicode.IsSpace) >= 0 {
		return ""
	}
	switch ecosystem {
	case "node", "php":
		return strings.ToLower(identity)
	case "python":
		return normalizeDelimitedIdentity(identity, func(character rune) bool {
			return character == '-' || character == '_' || character == '.'
		})
	case "rust":
		return normalizeDelimitedIdentity(identity, func(character rune) bool {
			return character == '-' || character == '_'
		})
	default:
		return identity
	}
}

func normalizeDelimitedIdentity(identity string, isDelimiter func(rune) bool) string {
	var normalized strings.Builder
	previousDelimiter := false
	for _, character := range strings.ToLower(identity) {
		if isDelimiter(character) {
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
	for _, field := range [][2]string{
		{left.FromProject, right.FromProject},
		{left.ManifestPath, right.ManifestPath},
		{left.Ecosystem, right.Ecosystem},
		{left.Name, right.Name},
		{left.Scope, right.Scope},
		{left.Constraint, right.Constraint},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
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
