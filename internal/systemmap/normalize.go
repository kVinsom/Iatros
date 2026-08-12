package systemmap

import (
	"slices"
	"strings"
)

// Normalized returns a detached model with deterministic ordering and non-nil collections.
func (m Model) Normalized() Model {
	m.Repositories = normalizeSlice(m.Repositories)
	for index := range m.Repositories {
		m.Repositories[index].Evidence = normalizeEvidence(m.Repositories[index].Evidence)
	}
	slices.SortFunc(m.Repositories, compareRepositories)

	m.Services = normalizeSlice(m.Services)
	for index := range m.Services {
		service := &m.Services[index]
		service.EnvironmentIDs = normalizeEnvironmentIDs(service.EnvironmentIDs)
		service.Evidence = normalizeEvidence(service.Evidence)
	}
	slices.SortFunc(m.Services, compareServices)

	m.Libraries = normalizeSlice(m.Libraries)
	for index := range m.Libraries {
		m.Libraries[index].Evidence = normalizeEvidence(m.Libraries[index].Evidence)
	}
	slices.SortFunc(m.Libraries, compareLibraries)

	m.Infrastructure = normalizeSlice(m.Infrastructure)
	for index := range m.Infrastructure {
		infrastructure := &m.Infrastructure[index]
		infrastructure.EnvironmentIDs = normalizeEnvironmentIDs(infrastructure.EnvironmentIDs)
		infrastructure.Evidence = normalizeEvidence(infrastructure.Evidence)
	}
	slices.SortFunc(m.Infrastructure, compareInfrastructure)

	m.Environments = normalizeSlice(m.Environments)
	for index := range m.Environments {
		m.Environments[index].Evidence = normalizeEvidence(m.Environments[index].Evidence)
	}
	slices.SortFunc(m.Environments, compareEnvironments)

	m.Owners = normalizeSlice(m.Owners)
	for index := range m.Owners {
		m.Owners[index].Evidence = normalizeEvidence(m.Owners[index].Evidence)
	}
	slices.SortFunc(m.Owners, compareOwners)

	m.ExternalResources = normalizeSlice(m.ExternalResources)
	for index := range m.ExternalResources {
		m.ExternalResources[index].Evidence = normalizeEvidence(m.ExternalResources[index].Evidence)
	}
	slices.SortFunc(m.ExternalResources, compareExternalResources)

	m.Relationships = normalizeSlice(m.Relationships)
	for index := range m.Relationships {
		relationship := &m.Relationships[index]
		relationship.EnvironmentIDs = normalizeEnvironmentIDs(relationship.EnvironmentIDs)
		relationship.Evidence = normalizeEvidence(relationship.Evidence)
	}
	slices.SortFunc(m.Relationships, compareRelationships)

	m.Diagnostics = normalizeSlice(m.Diagnostics)
	slices.SortFunc(m.Diagnostics, compareDiagnostics)
	return m
}

func normalizeEvidence(evidence []Evidence) []Evidence {
	evidence = normalizeSlice(evidence)
	slices.SortFunc(evidence, compareEvidence)
	return slices.Compact(evidence)
}

func normalizeEnvironmentIDs(environmentIDs []EnvironmentID) []EnvironmentID {
	environmentIDs = normalizeSlice(environmentIDs)
	slices.Sort(environmentIDs)
	return slices.Compact(environmentIDs)
}

func normalizeSlice[S ~[]E, E any](entries S) S {
	normalized := slices.Clone(entries)
	if normalized == nil {
		return make(S, 0)
	}
	return normalized
}

func compareRepositories(left, right Repository) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareServices(left, right Service) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareLibraries(left, right Library) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareInfrastructure(left, right Infrastructure) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareEnvironments(left, right Environment) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareOwners(left, right Owner) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareExternalResources(left, right ExternalResource) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareRelationships(left, right Relationship) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareEvidence(left, right Evidence) int {
	for _, field := range [][2]string{
		{string(left.RepositoryID), string(right.RepositoryID)},
		{left.Path, right.Path},
		{left.Reference, right.Reference},
		{string(left.Kind), string(right.Kind)},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareDiagnostics(left, right Diagnostic) int {
	for _, field := range [][2]string{
		{string(left.RepositoryID), string(right.RepositoryID)},
		{left.Path, right.Path},
		{left.Code, right.Code},
		{left.Message, right.Message},
		{string(left.Level), string(right.Level)},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
}
