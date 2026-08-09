package project

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

// ErrInvalidProject indicates that a project violates the canonical model contract.
var ErrInvalidProject = errors.New("project model is invalid")

// Validate checks the schema, ordering, references, and safety invariants of the project model.
func (p Project) Validate() error {
	if !CurrentSchemaVersion.Supports(p.SchemaVersion) {
		return invalidProject("schema_version", "is unsupported")
	}
	if !validIdentifier(string(p.ID)) {
		return invalidProject("id", "must be a lowercase identifier")
	}
	if !validText(p.Name) {
		return invalidProject("name", "must be non-empty UTF-8 text without surrounding whitespace")
	}
	if err := p.Configuration.validate("configuration"); err != nil {
		return err
	}

	environmentIDs, err := p.validateEnvironments()
	if err != nil {
		return err
	}
	serviceIDs, err := p.validateServices(environmentIDs)
	if err != nil {
		return err
	}
	if err := p.validateDependencies(environmentIDs, serviceIDs); err != nil {
		return err
	}

	return nil
}

func (p Project) validateEnvironments() (map[EnvironmentID]struct{}, error) {
	identifiers := make(map[EnvironmentID]struct{}, len(p.Environments))
	for index, environment := range p.Environments {
		field := fmt.Sprintf("environments[%d]", index)
		if !validIdentifier(string(environment.ID)) {
			return nil, invalidProject(field+".id", "must be a lowercase identifier")
		}
		if index > 0 && p.Environments[index-1].ID >= environment.ID {
			return nil, invalidProject("environments", "must be strictly ordered by id")
		}
		if !validText(environment.Name) || !validIdentifier(environment.Kind) {
			return nil, invalidProject(field, "has an invalid name or kind")
		}
		if err := environment.Configuration.validate(field + ".configuration"); err != nil {
			return nil, err
		}
		identifiers[environment.ID] = struct{}{}
	}

	return identifiers, nil
}

func (p Project) validateServices(
	environments map[EnvironmentID]struct{},
) (map[ServiceID]struct{}, error) {
	identifiers := make(map[ServiceID]struct{}, len(p.Services))
	for index, service := range p.Services {
		field := fmt.Sprintf("services[%d]", index)
		if !validIdentifier(string(service.ID)) {
			return nil, invalidProject(field+".id", "must be a lowercase identifier")
		}
		if index > 0 && p.Services[index-1].ID >= service.ID {
			return nil, invalidProject("services", "must be strictly ordered by id")
		}
		if !validText(service.Name) || !validIdentifier(service.Kind) {
			return nil, invalidProject(field, "has an invalid name or kind")
		}
		if service.SourcePath != "" && !validRelativePath(service.SourcePath) {
			return nil, invalidProject(field+".source_path", "must be a normalized project-relative path")
		}
		if err := service.Configuration.validate(field + ".configuration"); err != nil {
			return nil, err
		}
		for bindingIndex, binding := range service.Environments {
			bindingField := fmt.Sprintf("%s.environments[%d]", field, bindingIndex)
			if bindingIndex > 0 && service.Environments[bindingIndex-1].EnvironmentID >= binding.EnvironmentID {
				return nil, invalidProject(field+".environments", "must be strictly ordered by environment_id")
			}
			if _, exists := environments[binding.EnvironmentID]; !exists {
				return nil, invalidProject(bindingField+".environment_id", "references an unknown environment")
			}
			if err := binding.Configuration.validate(bindingField + ".configuration"); err != nil {
				return nil, err
			}
		}
		identifiers[service.ID] = struct{}{}
	}

	return identifiers, nil
}

func (p Project) validateDependencies(
	environments map[EnvironmentID]struct{},
	services map[ServiceID]struct{},
) error {
	for index, dependency := range p.Dependencies {
		field := fmt.Sprintf("dependencies[%d]", index)
		if !validIdentifier(string(dependency.ID)) {
			return invalidProject(field+".id", "must be a lowercase identifier")
		}
		if index > 0 && p.Dependencies[index-1].ID >= dependency.ID {
			return invalidProject("dependencies", "must be strictly ordered by id")
		}
		if !validIdentifier(dependency.Kind) {
			return invalidProject(field+".kind", "must be a lowercase identifier")
		}
		if err := p.validateReference(field+".source", dependency.Source, environments, services); err != nil {
			return err
		}
		if err := p.validateReference(field+".target", dependency.Target, environments, services); err != nil {
			return err
		}
		if dependency.Source == dependency.Target {
			return invalidProject(field, "cannot depend on the same resource reference")
		}
		for environmentIndex, environmentID := range dependency.Environments {
			if environmentIndex > 0 && dependency.Environments[environmentIndex-1] >= environmentID {
				return invalidProject(field+".environments", "must be strictly ordered")
			}
			if _, exists := environments[environmentID]; !exists {
				return invalidProject(field+".environments", "references an unknown environment")
			}
		}
	}

	return nil
}

func (p Project) validateReference(
	field string,
	reference ResourceReference,
	environments map[EnvironmentID]struct{},
	services map[ServiceID]struct{},
) error {
	switch reference.Kind {
	case ResourceKindProject:
		if !validIdentifier(reference.ID) {
			return invalidProject(field+".id", "must be a lowercase identifier")
		}
		if reference.ID != string(p.ID) {
			return invalidProject(field, "references an unknown project")
		}
	case ResourceKindEnvironment:
		if !validIdentifier(reference.ID) {
			return invalidProject(field+".id", "must be a lowercase identifier")
		}
		if _, exists := environments[EnvironmentID(reference.ID)]; !exists {
			return invalidProject(field, "references an unknown environment")
		}
	case ResourceKindService:
		if !validIdentifier(reference.ID) {
			return invalidProject(field+".id", "must be a lowercase identifier")
		}
		if _, exists := services[ServiceID(reference.ID)]; !exists {
			return invalidProject(field, "references an unknown service")
		}
	case ResourceKindExternal:
		if !validText(reference.ID) {
			return invalidProject(field+".id", "must be a non-empty external identifier")
		}
		return nil
	default:
		return invalidProject(field+".kind", "is unsupported")
	}

	return nil
}

func (c Configuration) validate(field string) error {
	for index, entry := range c.Entries {
		entryField := fmt.Sprintf("%s.entries[%d]", field, index)
		if !validConfigurationKey(entry.Key) {
			return invalidProject(entryField+".key", "is invalid")
		}
		if index > 0 && c.Entries[index-1].Key >= entry.Key {
			return invalidProject(field+".entries", "must be strictly ordered by key")
		}

		switch entry.Source {
		case ConfigurationSourceLiteral:
			if entry.Reference != "" || entry.Sensitive || !validConfigurationLiteral(entry.Value) {
				return invalidProject(entryField, "contains an unsafe literal configuration value")
			}
		case ConfigurationSourceEnvironment:
			if entry.Value != "" || !validConfigurationKey(entry.Reference) {
				return invalidProject(entryField, "contains an invalid environment reference")
			}
		case ConfigurationSourceFile:
			if entry.Value != "" || !validRelativePath(entry.Reference) {
				return invalidProject(entryField, "contains an invalid file reference")
			}
		case ConfigurationSourceSecret:
			if entry.Value != "" || !entry.Sensitive || !validText(entry.Reference) {
				return invalidProject(entryField, "must contain only a sensitive secret reference")
			}
		default:
			return invalidProject(entryField+".source", "is unsupported")
		}
	}

	return nil
}

func invalidProject(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidProject, field, reason)
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
		if (character != '-' && character != '_' && character != '.') || previousSeparator {
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

func validConfigurationKey(key string) bool {
	if key == "" || !alphaNumeric(key[0]) || !alphaNumeric(key[len(key)-1]) {
		return false
	}
	for index := range len(key) {
		character := key[index]
		if !alphaNumeric(character) && character != '-' && character != '_' && character != '.' {
			return false
		}
	}

	return true
}

func alphaNumeric(character byte) bool {
	return lowerAlphaNumeric(character) || (character >= 'A' && character <= 'Z')
}

func validText(text string) bool {
	if text == "" || !utf8.ValidString(text) || strings.TrimSpace(text) != text {
		return false
	}
	for _, character := range text {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}

	return true
}

func validConfigurationLiteral(literal string) bool {
	return utf8.ValidString(literal) && !strings.ContainsRune(literal, '\x00')
}

func validRelativePath(candidatePath string) bool {
	return repositorypath.IsValidDirectory(candidatePath)
}
