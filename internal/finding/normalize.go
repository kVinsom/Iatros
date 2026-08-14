package finding

import (
	"slices"
	"strings"
)

// Normalized returns a detached finding with deterministic nested collections.
func (f Finding) Normalized() Finding {
	f.Subjects = cloneSlice(f.Subjects)
	slices.SortFunc(f.Subjects, compareSubjects)
	f.Subjects = slices.CompactFunc(f.Subjects, func(left, right Subject) bool {
		return compareSubjects(left, right) == 0
	})
	f.Evidence = cloneSlice(f.Evidence)
	slices.SortFunc(f.Evidence, compareEvidence)
	f.Evidence = slices.CompactFunc(f.Evidence, func(left, right Evidence) bool {
		return compareEvidence(left, right) == 0
	})
	f.Recommendation.Actions = cloneSlice(f.Recommendation.Actions)
	slices.Sort(f.Recommendation.Actions)
	f.Recommendation.Actions = slices.Compact(f.Recommendation.Actions)
	if f.Exclusion != nil {
		cloned := *f.Exclusion
		cloned.AppliedAt = cloned.AppliedAt.UTC()
		cloned.ExpiresAt = cloned.ExpiresAt.UTC()
		f.Exclusion = &cloned
	}
	return f
}

// Normalized returns a detached exclusion record.
func (e Exclusion) Normalized() Exclusion {
	e.CreatedAt = e.CreatedAt.UTC()
	e.ExpiresAt = e.ExpiresAt.UTC()
	return e
}

// Normalized returns a detached model with deterministic, non-nil collections.
func (m Model) Normalized() Model {
	m.EvaluatedAt = m.EvaluatedAt.UTC()
	m.Findings = cloneSlice(m.Findings)
	for index := range m.Findings {
		m.Findings[index] = m.Findings[index].Normalized()
	}
	slices.SortFunc(m.Findings, compareFindings)

	m.Exclusions = cloneSlice(m.Exclusions)
	for index := range m.Exclusions {
		m.Exclusions[index] = m.Exclusions[index].Normalized()
	}
	slices.SortFunc(m.Exclusions, compareExclusions)
	return m
}

func cloneSlice[S ~[]E, E any](entries S) S {
	cloned := slices.Clone(entries)
	if cloned == nil {
		return make(S, 0)
	}
	return cloned
}

func compareFindings(left, right Finding) int {
	if compared := severityRank(left.Severity) - severityRank(right.Severity); compared != 0 {
		return compared
	}
	return strings.Compare(left.ID, right.ID)
}

func compareSubjects(left, right Subject) int {
	for _, fields := range [][2]string{
		{left.Kind, right.Kind},
		{left.ID, right.ID},
		{left.RepositoryID, right.RepositoryID},
		{left.EnvironmentID, right.EnvironmentID},
	} {
		if compared := strings.Compare(fields[0], fields[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareEvidence(left, right Evidence) int {
	for _, fields := range [][2]string{
		{string(left.Kind), string(right.Kind)},
		{left.RepositoryID, right.RepositoryID},
		{left.Path, right.Path},
		{left.Reference, right.Reference},
		{left.Description, right.Description},
	} {
		if compared := strings.Compare(fields[0], fields[1]); compared != 0 {
			return compared
		}
	}
	for _, values := range [][2]int{
		{left.StartLine, right.StartLine},
		{left.StartColumn, right.StartColumn},
		{left.EndLine, right.EndLine},
		{left.EndColumn, right.EndColumn},
	} {
		if values[0] != values[1] {
			return values[0] - values[1]
		}
	}
	return 0
}

func compareExclusions(left, right Exclusion) int {
	return strings.Compare(left.ID, right.ID)
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 4
	}
}
