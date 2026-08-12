package remoteanalysis

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

	m.Relationships = normalizeSlice(m.Relationships)
	for index := range m.Relationships {
		m.Relationships[index].Evidence = normalizeEvidence(m.Relationships[index].Evidence)
	}
	slices.SortFunc(m.Relationships, compareRelationships)

	m.Evidence = normalizeEvidence(m.Evidence)
	m.Diagnostics = normalizeSlice(m.Diagnostics)
	slices.SortFunc(m.Diagnostics, compareDiagnostics)
	return m
}

func normalizeEvidence(evidence []Evidence) []Evidence {
	evidence = normalizeSlice(evidence)
	slices.SortFunc(evidence, compareEvidence)
	return slices.Compact(evidence)
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
		{string(left.Provider), string(right.Provider)},
		{string(left.RepositoryID), string(right.RepositoryID)},
		{left.Scope, right.Scope},
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
