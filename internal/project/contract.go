package project

import (
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/schema"
)

// CurrentSchemaVersion identifies the project-model schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// ProjectID identifies a project independently from its display name or filesystem location.
type ProjectID string

// EnvironmentID identifies an environment within a project.
type EnvironmentID string

// ServiceID identifies a service within a project.
type ServiceID string

// DependencyID identifies a dependency edge within a project.
type DependencyID string

// ResourceKind identifies the type of resource referenced by a dependency.
type ResourceKind string

const (
	// ResourceKindProject identifies the containing project.
	ResourceKindProject ResourceKind = "project"
	// ResourceKindEnvironment identifies an environment declared by the project.
	ResourceKindEnvironment ResourceKind = "environment"
	// ResourceKindService identifies a service declared by the project.
	ResourceKindService ResourceKind = "service"
	// ResourceKindExternal identifies a resource owned outside the project model.
	ResourceKindExternal ResourceKind = "external"
)

// ConfigurationSource identifies how a configuration entry obtains its value.
type ConfigurationSource string

const (
	// ConfigurationSourceLiteral stores a non-sensitive literal value in the model.
	ConfigurationSourceLiteral ConfigurationSource = "literal"
	// ConfigurationSourceEnvironment reads a value from a named environment variable.
	ConfigurationSourceEnvironment ConfigurationSource = "environment"
	// ConfigurationSourceFile reads a value from a project-relative file.
	ConfigurationSourceFile ConfigurationSource = "file"
	// ConfigurationSourceSecret resolves a sensitive value through a secret reference.
	ConfigurationSourceSecret ConfigurationSource = "secret"
)

// Project is the canonical provider-neutral representation of one operational project.
type Project struct {
	SchemaVersion schema.Version `json:"schema_version"`
	ID            ProjectID      `json:"id"`
	Name          string         `json:"name"`
	Configuration Configuration  `json:"configuration"`
	Environments  []Environment  `json:"environments"`
	Services      []Service      `json:"services"`
	Dependencies  []Dependency   `json:"dependencies"`
}

// Environment describes a deployment or operational context within a project.
type Environment struct {
	ID            EnvironmentID `json:"id"`
	Name          string        `json:"name"`
	Kind          string        `json:"kind"`
	Configuration Configuration `json:"configuration"`
}

// Service describes a provider-neutral deployable or externally operated capability.
type Service struct {
	ID            ServiceID            `json:"id"`
	Name          string               `json:"name"`
	Kind          string               `json:"kind"`
	SourcePath    string               `json:"source_path,omitempty"`
	Configuration Configuration        `json:"configuration"`
	Environments  []ServiceEnvironment `json:"environments"`
}

// ServiceEnvironment binds a service to an environment and records environment-specific overrides.
type ServiceEnvironment struct {
	EnvironmentID EnvironmentID `json:"environment_id"`
	Configuration Configuration `json:"configuration"`
}

// Dependency describes a directed relationship between two project or external resources.
type Dependency struct {
	ID           DependencyID      `json:"id"`
	Source       ResourceReference `json:"source"`
	Target       ResourceReference `json:"target"`
	Kind         string            `json:"kind"`
	Required     bool              `json:"required"`
	Environments []EnvironmentID   `json:"environments"`
}

// ResourceReference identifies one endpoint of a dependency without importing a provider type.
type ResourceReference struct {
	Kind ResourceKind `json:"kind"`
	ID   string       `json:"id"`
}

// Configuration contains deterministic, key-ordered configuration declarations.
type Configuration struct {
	Entries []ConfigurationEntry `json:"entries"`
}

// ConfigurationEntry declares a literal value or a reference to a runtime value source.
type ConfigurationEntry struct {
	Key       string              `json:"key"`
	Source    ConfigurationSource `json:"source"`
	Value     string              `json:"value,omitempty"`
	Reference string              `json:"reference,omitempty"`
	Required  bool                `json:"required"`
	Sensitive bool                `json:"sensitive"`
}

// Normalized returns a detached project value with deterministic collection ordering and non-nil slices.
func (p Project) Normalized() Project {
	p.Configuration = p.Configuration.Normalized()
	p.Environments = normalizeSlice(p.Environments)
	for index := range p.Environments {
		p.Environments[index].Configuration = p.Environments[index].Configuration.Normalized()
	}
	slices.SortFunc(p.Environments, compareEnvironments)

	p.Services = normalizeSlice(p.Services)
	for serviceIndex := range p.Services {
		service := &p.Services[serviceIndex]
		service.Configuration = service.Configuration.Normalized()
		service.Environments = normalizeSlice(service.Environments)
		for environmentIndex := range service.Environments {
			binding := &service.Environments[environmentIndex]
			binding.Configuration = binding.Configuration.Normalized()
		}
		slices.SortFunc(service.Environments, compareServiceEnvironments)
	}
	slices.SortFunc(p.Services, compareServices)

	p.Dependencies = normalizeSlice(p.Dependencies)
	for index := range p.Dependencies {
		p.Dependencies[index].Environments = normalizeSlice(p.Dependencies[index].Environments)
		slices.Sort(p.Dependencies[index].Environments)
	}
	slices.SortFunc(p.Dependencies, compareProjectDependencies)

	return p
}

// Normalized returns a detached configuration with deterministic ordering and a non-nil entry slice.
func (c Configuration) Normalized() Configuration {
	c.Entries = normalizeSlice(c.Entries)
	slices.SortFunc(c.Entries, compareConfigurationEntries)
	return c
}

func compareEnvironments(left, right Environment) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareServiceEnvironments(left, right ServiceEnvironment) int {
	return strings.Compare(string(left.EnvironmentID), string(right.EnvironmentID))
}

func compareServices(left, right Service) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareProjectDependencies(left, right Dependency) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareConfigurationEntries(left, right ConfigurationEntry) int {
	return strings.Compare(left.Key, right.Key)
}

func normalizeSlice[S ~[]E, E any](entries S) S {
	normalized := slices.Clone(entries)
	if normalized == nil {
		return make(S, 0)
	}
	return normalized
}
