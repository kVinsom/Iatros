package artifactvalidation

import (
	"cmp"
	"slices"
	"strings"
)

// Normalized returns a detached report with deterministic ordering and non-nil collections.
func (r Report) Normalized() Report {
	r.Artifacts = cloneSlice(r.Artifacts)
	for index := range r.Artifacts {
		r.Artifacts[index].Validators = normalizeStrings(r.Artifacts[index].Validators)
	}
	slices.SortFunc(r.Artifacts, compareArtifactResults)
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

func normalizeStrings(entries []string) []string {
	entries = cloneSlice(entries)
	slices.Sort(entries)
	return slices.Compact(entries)
}

func compareArtifactResults(left, right ArtifactResult) int {
	return strings.Compare(left.Artifact.ID, right.Artifact.ID)
}

func compareDiagnostics(left, right Diagnostic) int {
	for _, fields := range [][2]string{
		{left.ArtifactID, right.ArtifactID},
		{left.ValidatorID, right.ValidatorID},
		{left.Code, right.Code},
		{string(left.Level), string(right.Level)},
		{left.Message, right.Message},
	} {
		if compared := strings.Compare(fields[0], fields[1]); compared != 0 {
			return compared
		}
	}
	for _, values := range [][2]int{
		{left.Location.StartLine, right.Location.StartLine},
		{left.Location.StartColumn, right.Location.StartColumn},
		{left.Location.EndLine, right.Location.EndLine},
		{left.Location.EndColumn, right.Location.EndColumn},
	} {
		if compared := cmp.Compare(values[0], values[1]); compared != 0 {
			return compared
		}
	}
	return strings.Compare(left.Path, right.Path)
}
