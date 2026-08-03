package manifest

import (
	"context"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func validateTOML(data []byte, maximumDepth int) error {
	var value map[string]any
	if err := toml.Unmarshal(data, &value); err != nil {
		return err
	}
	return validateTOMLDepth(value, 1, maximumDepth)
}

func validateTOMLDepth(value any, depth, maximumDepth int) error {
	if depth > maximumDepth {
		return errInvalidManifest
	}
	switch current := value.(type) {
	case map[string]any:
		for _, nested := range current {
			if err := validateTOMLDepth(nested, depth+1, maximumDepth); err != nil {
				return err
			}
		}
	case []any:
		for _, nested := range current {
			if err := validateTOMLDepth(nested, depth+1, maximumDepth); err != nil {
				return err
			}
		}
	case []map[string]any:
		for _, nested := range current {
			if err := validateTOMLDepth(nested, depth+1, maximumDepth); err != nil {
				return err
			}
		}
	}
	return nil
}

type pythonParser struct{}

func (pythonParser) Format() Format { return FormatPythonProject }

func (pythonParser) Filenames() []string { return []string{"pyproject.toml"} }

type pythonDocument struct {
	Project struct {
		Name                 string              `toml:"name"`
		Version              string              `toml:"version"`
		RequiresPython       string              `toml:"requires-python"`
		Dependencies         []string            `toml:"dependencies"`
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
	} `toml:"project"`
	BuildSystem struct {
		Requires []string `toml:"requires"`
	} `toml:"build-system"`
	Tool struct {
		Poetry struct {
			Name            string                       `toml:"name"`
			Version         string                       `toml:"version"`
			Dependencies    map[string]any               `toml:"dependencies"`
			DevDependencies map[string]any               `toml:"dev-dependencies"`
			Group           map[string]pythonPoetryGroup `toml:"group"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

type pythonPoetryGroup struct {
	Dependencies map[string]any `toml:"dependencies"`
}

func (pythonParser) Parse(ctx context.Context, document Document, limits Limits) (Manifest, error) {
	data, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateTOML(data, limits.MaxNestingDepth); err != nil {
		return Manifest{}, err
	}
	var value pythonDocument
	if err := toml.Unmarshal(data, &value); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Name:    firstNonEmpty(value.Project.Name, value.Tool.Poetry.Name),
		Version: firstNonEmpty(value.Project.Version, value.Tool.Poetry.Version),
	}
	if value.Project.RequiresPython != "" {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "python", Value: value.Project.RequiresPython, Scope: ScopeRuntime,
		})
	}
	manifest.Dependencies, err = appendRequirementList(
		manifest.Dependencies, value.Project.Dependencies, ScopeRuntime, false,
	)
	if err != nil {
		return Manifest{}, err
	}
	for _, declarations := range value.Project.OptionalDependencies {
		manifest.Dependencies, err = appendRequirementList(
			manifest.Dependencies, declarations, ScopeRuntime, true,
		)
		if err != nil {
			return Manifest{}, err
		}
	}
	manifest.Dependencies, err = appendRequirementList(
		manifest.Dependencies, value.BuildSystem.Requires, ScopeBuild, false,
	)
	if err != nil {
		return Manifest{}, err
	}
	if err := appendPoetryDependencies(&manifest, value.Tool.Poetry.Dependencies, ScopeRuntime); err != nil {
		return Manifest{}, err
	}
	if err := appendPoetryDependencies(&manifest, value.Tool.Poetry.DevDependencies, ScopeDevelopment); err != nil {
		return Manifest{}, err
	}
	for _, group := range value.Tool.Poetry.Group {
		if err := appendPoetryDependencies(&manifest, group.Dependencies, ScopeDevelopment); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func appendPoetryDependencies(manifest *Manifest, values map[string]any, scope Scope) error {
	for name, value := range values {
		specifications, err := tomlDependencyValues(value, name)
		if err != nil {
			return err
		}
		for _, specification := range specifications {
			if name == "python" {
				manifest.Constraints = append(manifest.Constraints, Constraint{
					Name: "python", Value: specification.constraint, Scope: scope,
				})
				continue
			}
			manifest.Dependencies = append(manifest.Dependencies, Dependency{
				Name: name, Constraint: specification.constraint, Scope: scope,
				Optional: specification.optional,
			})
		}
	}
	return nil
}

type rustParser struct{}

func (rustParser) Format() Format { return FormatRustPackage }

func (rustParser) Filenames() []string { return []string{"Cargo.toml"} }

type rustWorkspace struct {
	Members        []string `toml:"members"`
	DefaultMembers []string `toml:"default-members"`
	Exclude        []string `toml:"exclude"`
}

type rustDocument struct {
	Package struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		RustVersion string `toml:"rust-version"`
	} `toml:"package"`
	Workspace         *rustWorkspace        `toml:"workspace"`
	Dependencies      map[string]any        `toml:"dependencies"`
	DevDependencies   map[string]any        `toml:"dev-dependencies"`
	BuildDependencies map[string]any        `toml:"build-dependencies"`
	Target            map[string]rustTarget `toml:"target"`
}

type rustTarget struct {
	Dependencies      map[string]any `toml:"dependencies"`
	DevDependencies   map[string]any `toml:"dev-dependencies"`
	BuildDependencies map[string]any `toml:"build-dependencies"`
}

func (rustParser) Parse(ctx context.Context, document Document, limits Limits) (Manifest, error) {
	data, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateTOML(data, limits.MaxNestingDepth); err != nil {
		return Manifest{}, err
	}
	var value rustDocument
	if err := toml.Unmarshal(data, &value); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{Name: value.Package.Name, Version: value.Package.Version}
	if value.Workspace != nil {
		manifest.WorkspaceDeclared = true
		manifest.WorkspaceMembers = value.Workspace.Members
		manifest.WorkspaceExcludes = value.Workspace.Exclude
	}
	if value.Package.RustVersion != "" {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "rust", Value: value.Package.RustVersion, Scope: ScopeBuild,
		})
	}
	if err := appendRustDependencies(&manifest, value.Dependencies, ScopeRuntime); err != nil {
		return Manifest{}, err
	}
	if err := appendRustDependencies(&manifest, value.DevDependencies, ScopeDevelopment); err != nil {
		return Manifest{}, err
	}
	if err := appendRustDependencies(&manifest, value.BuildDependencies, ScopeBuild); err != nil {
		return Manifest{}, err
	}
	for _, target := range value.Target {
		if err := appendRustDependencies(&manifest, target.Dependencies, ScopeRuntime); err != nil {
			return Manifest{}, err
		}
		if err := appendRustDependencies(&manifest, target.DevDependencies, ScopeDevelopment); err != nil {
			return Manifest{}, err
		}
		if err := appendRustDependencies(&manifest, target.BuildDependencies, ScopeBuild); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func appendRustDependencies(manifest *Manifest, values map[string]any, scope Scope) error {
	for alias, value := range values {
		specifications, err := tomlDependencyValues(value, alias)
		if err != nil {
			return err
		}
		for _, specification := range specifications {
			manifest.Dependencies = append(manifest.Dependencies, Dependency{
				Name: specification.name, Constraint: specification.constraint, Scope: scope,
				Optional: specification.optional,
			})
		}
	}
	return nil
}

type tomlDependency struct {
	name       string
	constraint string
	optional   bool
}

func tomlDependencyValues(value any, defaultName string) ([]tomlDependency, error) {
	switch current := value.(type) {
	case string:
		return []tomlDependency{{name: defaultName, constraint: current}}, nil
	case map[string]any:
		specification, err := tomlDependencyTable(current, defaultName)
		return []tomlDependency{specification}, err
	case []any:
		return tomlDependencyList(current, defaultName)
	case []map[string]any:
		values := make([]any, len(current))
		for index := range current {
			values[index] = current[index]
		}
		return tomlDependencyList(values, defaultName)
	default:
		return nil, errInvalidManifest
	}
}

func tomlDependencyList(values []any, defaultName string) ([]tomlDependency, error) {
	if len(values) == 0 {
		return nil, errInvalidManifest
	}
	dependencies := make([]tomlDependency, 0, len(values))
	for _, value := range values {
		nested, err := tomlDependencyValues(value, defaultName)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, nested...)
	}
	return dependencies, nil
}

func tomlDependencyTable(values map[string]any, defaultName string) (tomlDependency, error) {
	constraint, err := optionalString(values, "version")
	if err != nil {
		return tomlDependency{}, err
	}
	name, err := optionalString(values, "package")
	if err != nil {
		return tomlDependency{}, err
	}
	if name == "" {
		name = defaultName
	}
	optional, err := optionalBool(values, "optional")
	return tomlDependency{name: name, constraint: constraint, optional: optional}, err
}

func optionalString(values map[string]any, key string) (string, error) {
	value, exists := values[key]
	if !exists {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", errInvalidManifest
	}
	return text, nil
}

func optionalBool(values map[string]any, key string) (bool, error) {
	value, exists := values[key]
	if !exists {
		return false, nil
	}
	flag, ok := value.(bool)
	if !ok {
		return false, errInvalidManifest
	}
	return flag, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
