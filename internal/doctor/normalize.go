package doctor

import (
	"slices"
	"strings"
)

// Normalized returns a detached report with deterministic ordering and non-nil collections.
func (r Report) Normalized() Report {
	r.Coverage = cloneSlice(r.Coverage)
	slices.SortFunc(r.Coverage, func(left, right Coverage) int {
		return categoryRank(left.Category) - categoryRank(right.Category)
	})
	r.Findings = r.Findings.Normalized()
	r.Diagnostics = cloneSlice(r.Diagnostics)
	slices.SortFunc(r.Diagnostics, compareDiagnostics)
	return r
}

func cloneSlice[S ~[]E, E any](entries S) S {
	cloned := slices.Clone(entries)
	if cloned == nil {
		return make(S, 0)
	}
	return cloned
}

func compareDiagnostics(left, right Diagnostic) int {
	if compared := categoryRank(left.Category) - categoryRank(right.Category); compared != 0 {
		return compared
	}
	for _, fields := range [][2]string{
		{left.RuleID, right.RuleID},
		{left.Code, right.Code},
		{string(left.Level), string(right.Level)},
		{left.Message, right.Message},
	} {
		if compared := strings.Compare(fields[0], fields[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func categoryRank(category Category) int {
	switch category {
	case CategoryProduction:
		return 0
	case CategoryReliability:
		return 1
	case CategoryPerformance:
		return 2
	case CategorySecurity:
		return 3
	case CategoryDeployment:
		return 4
	case CategoryCost:
		return 5
	default:
		return 6
	}
}
