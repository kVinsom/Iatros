package manifest

import (
	"context"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func validateTOML(content []byte, maximumDepth int) error {
	var documentTree map[string]any
	if err := toml.Unmarshal(content, &documentTree); err != nil {
		return err
	}
	return validateTOMLDepth(documentTree, 1, maximumDepth)
}

func validateTOMLDepth(node any, depth, maximumDepth int) error {
	if depth > maximumDepth {
		return errInvalidManifest
	}
	switch current := node.(type) {
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
	content, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateTOML(content, limits.MaxNestingDepth); err != nil {
		return Manifest{}, err
	}
	var parsedDocument pythonDocument
	if err := toml.Unmarshal(content, &parsedDocument); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Name: firstNonEmpty(
			parsedDocument.Project.Name,
			parsedDocument.Tool.Poetry.Name,
		),
		Version: firstNonEmpty(
			parsedDocument.Project.Version,
			parsedDocument.Tool.Poetry.Version,
		),
	}
	if parsedDocument.Project.RequiresPython != "" {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "python", Value: parsedDocument.Project.RequiresPython, Scope: ScopeRuntime,
		})
	}
	manifest.Dependencies, err = appendRequirementList(
		manifest.Dependencies,
		parsedDocument.Project.Dependencies,
		dependencyDeclarationOptions{scope: ScopeRuntime},
	)
	if err != nil {
		return Manifest{}, err
	}
	for _, declarations := range parsedDocument.Project.OptionalDependencies {
		manifest.Dependencies, err = appendRequirementList(
			manifest.Dependencies,
			declarations,
			dependencyDeclarationOptions{scope: ScopeRuntime, isOptional: true},
		)
		if err != nil {
			return Manifest{}, err
		}
	}
	manifest.Dependencies, err = appendRequirementList(
		manifest.Dependencies,
		parsedDocument.BuildSystem.Requires,
		dependencyDeclarationOptions{scope: ScopeBuild},
	)
	if err != nil {
		return Manifest{}, err
	}
	if err := appendPoetryDependencies(
		&manifest,
		parsedDocument.Tool.Poetry.Dependencies,
		ScopeRuntime,
	); err != nil {
		return Manifest{}, err
	}
	if err := appendPoetryDependencies(
		&manifest,
		parsedDocument.Tool.Poetry.DevDependencies,
		ScopeDevelopment,
	); err != nil {
		return Manifest{}, err
	}
	for _, group := range parsedDocument.Tool.Poetry.Group {
		if err := appendPoetryDependencies(&manifest, group.Dependencies, ScopeDevelopment); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func appendPoetryDependencies(
	manifest *Manifest,
	declarations map[string]any,
	scope Scope,
) error {
	for name, rawSpecification := range declarations {
		specifications, err := tomlDependencyValues(rawSpecification, name)
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
	content, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateTOML(content, limits.MaxNestingDepth); err != nil {
		return Manifest{}, err
	}
	var parsedDocument rustDocument
	if err := toml.Unmarshal(content, &parsedDocument); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{Name: parsedDocument.Package.Name, Version: parsedDocument.Package.Version}
	if parsedDocument.Workspace != nil {
		manifest.WorkspaceDeclared = true
		manifest.WorkspaceMembers = parsedDocument.Workspace.Members
		manifest.WorkspaceExcludes = parsedDocument.Workspace.Exclude
	}
	if parsedDocument.Package.RustVersion != "" {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "rust", Value: parsedDocument.Package.RustVersion, Scope: ScopeBuild,
		})
	}
	if err := appendRustDependencies(&manifest, parsedDocument.Dependencies, ScopeRuntime); err != nil {
		return Manifest{}, err
	}
	if err := appendRustDependencies(
		&manifest,
		parsedDocument.DevDependencies,
		ScopeDevelopment,
	); err != nil {
		return Manifest{}, err
	}
	if err := appendRustDependencies(
		&manifest,
		parsedDocument.BuildDependencies,
		ScopeBuild,
	); err != nil {
		return Manifest{}, err
	}
	for _, target := range parsedDocument.Target {
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

func appendRustDependencies(
	manifest *Manifest,
	declarations map[string]any,
	scope Scope,
) error {
	for alias, rawSpecification := range declarations {
		specifications, err := tomlDependencyValues(rawSpecification, alias)
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

func tomlDependencyValues(rawSpecification any, defaultName string) ([]tomlDependency, error) {
	switch current := rawSpecification.(type) {
	case string:
		return []tomlDependency{{name: defaultName, constraint: current}}, nil
	case map[string]any:
		specification, err := tomlDependencyTable(current, defaultName)
		return []tomlDependency{specification}, err
	case []any:
		return tomlDependencyList(current, defaultName)
	case []map[string]any:
		rawSpecifications := make([]any, len(current))
		for index := range current {
			rawSpecifications[index] = current[index]
		}
		return tomlDependencyList(rawSpecifications, defaultName)
	default:
		return nil, errInvalidManifest
	}
}

func tomlDependencyList(rawSpecifications []any, defaultName string) ([]tomlDependency, error) {
	if len(rawSpecifications) == 0 {
		return nil, errInvalidManifest
	}
	dependencies := make([]tomlDependency, 0, len(rawSpecifications))
	for _, rawSpecification := range rawSpecifications {
		nested, err := tomlDependencyValues(rawSpecification, defaultName)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, nested...)
	}
	return dependencies, nil
}

func tomlDependencyTable(fields map[string]any, defaultName string) (tomlDependency, error) {
	constraint, err := optionalString(fields, "version")
	if err != nil {
		return tomlDependency{}, err
	}
	name, err := optionalString(fields, "package")
	if err != nil {
		return tomlDependency{}, err
	}
	if name == "" {
		name = defaultName
	}
	optional, err := optionalBool(fields, "optional")
	return tomlDependency{name: name, constraint: constraint, optional: optional}, err
}

func optionalString(fields map[string]any, key string) (string, error) {
	rawField, exists := fields[key]
	if !exists {
		return "", nil
	}
	text, ok := rawField.(string)
	if !ok {
		return "", errInvalidManifest
	}
	return text, nil
}

func optionalBool(fields map[string]any, key string) (bool, error) {
	rawField, exists := fields[key]
	if !exists {
		return false, nil
	}
	flag, ok := rawField.(bool)
	if !ok {
		return false, errInvalidManifest
	}
	return flag, nil
}

func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}
