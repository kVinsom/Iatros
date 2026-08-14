package changeimpact

import (
	"slices"
	"strings"
)

// Normalized returns a detached change set with deterministic ordering and a non-nil collection.
func (s ChangeSet) Normalized() ChangeSet {
	s.Changes = cloneSlice(s.Changes)
	slices.SortFunc(s.Changes, compareChanges)
	return s
}

// Normalized returns a detached model with deterministic ordering and non-nil collections.
func (m Model) Normalized() Model {
	m.Services = cloneSlice(m.Services)
	for index := range m.Services {
		m.Services[index].Causes = normalizeCauses(m.Services[index].Causes)
	}
	slices.SortFunc(m.Services, compareServiceImpacts)

	m.Environments = cloneSlice(m.Environments)
	for index := range m.Environments {
		m.Environments[index].Causes = normalizeCauses(m.Environments[index].Causes)
	}
	slices.SortFunc(m.Environments, compareEnvironmentImpacts)

	m.Configurations = cloneSlice(m.Configurations)
	for index := range m.Configurations {
		m.Configurations[index].Causes = normalizeCauses(m.Configurations[index].Causes)
	}
	slices.SortFunc(m.Configurations, compareConfigurationImpacts)

	m.Diagnostics = cloneSlice(m.Diagnostics)
	slices.SortFunc(m.Diagnostics, compareDiagnostics)
	return m
}

func normalizeCauses(causes []Cause) []Cause {
	causes = cloneSlice(causes)
	for index := range causes {
		causes[index].RelationshipIDs = cloneSlice(causes[index].RelationshipIDs)
	}
	slices.SortFunc(causes, compareCauses)
	return slices.CompactFunc(causes, func(left, right Cause) bool {
		return compareCauses(left, right) == 0
	})
}

func cloneSlice[S ~[]E, E any](entries S) S {
	cloned := slices.Clone(entries)
	if cloned == nil {
		return make(S, 0)
	}
	return cloned
}

func compareChanges(left, right Change) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareServiceImpacts(left, right ServiceImpact) int {
	return strings.Compare(string(left.ServiceID), string(right.ServiceID))
}

func compareEnvironmentImpacts(left, right EnvironmentImpact) int {
	return strings.Compare(string(left.EnvironmentID), string(right.EnvironmentID))
}

func compareConfigurationImpacts(left, right ConfigurationImpact) int {
	return strings.Compare(string(left.ConfigurationID), string(right.ConfigurationID))
}

func compareCauses(left, right Cause) int {
	if compared := strings.Compare(string(left.ChangeID), string(right.ChangeID)); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Path, right.Path); compared != 0 {
		return compared
	}
	for index := range min(len(left.RelationshipIDs), len(right.RelationshipIDs)) {
		if compared := strings.Compare(
			string(left.RelationshipIDs[index]),
			string(right.RelationshipIDs[index]),
		); compared != 0 {
			return compared
		}
	}
	return len(left.RelationshipIDs) - len(right.RelationshipIDs)
}

func compareDiagnostics(left, right Diagnostic) int {
	for _, field := range [][2]string{
		{string(left.RepositoryID), string(right.RepositoryID)},
		{left.Path, right.Path},
		{string(left.ChangeID), string(right.ChangeID)},
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
