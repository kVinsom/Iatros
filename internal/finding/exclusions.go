package finding

import (
	"context"
	"path"
	"slices"
	"time"
)

// ApplyExclusions validates and deterministically applies active exclusion records.
func ApplyExclusions(
	ctx context.Context,
	findings []Finding,
	exclusions []Exclusion,
	evaluatedAt time.Time,
	limits Limits,
) (Model, error) {
	empty := emptyModel(evaluatedAt.UTC())
	if err := limits.Validate(); err != nil {
		return empty, err
	}
	if !validTimestamp(evaluatedAt) {
		return empty, invalidModel("evaluated_at", "is invalid")
	}
	evaluatedAt = evaluatedAt.UTC()
	if len(findings) > limits.MaxFindings || len(exclusions) > limits.MaxExclusions {
		return empty, invalidModel("collections", "exceed configured limits")
	}

	evaluationContext, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	if err := evaluationContext.Err(); err != nil {
		return empty, err
	}

	normalizedExclusions := cloneSlice(exclusions)
	for index := range normalizedExclusions {
		normalizedExclusions[index] = normalizedExclusions[index].Normalized()
	}
	slices.SortFunc(normalizedExclusions, compareExclusions)
	for index, exclusion := range normalizedExclusions {
		if err := evaluationContext.Err(); err != nil {
			return empty, err
		}
		if err := exclusion.ValidateWithin(limits); err != nil {
			return empty, err
		}
		if index > 0 && normalizedExclusions[index-1].ID == exclusion.ID {
			return empty, invalidModel("exclusions", "contain duplicate ids")
		}
	}
	exclusionIndex := newExclusionIndex(normalizedExclusions)

	normalizedFindings := cloneSlice(findings)
	for index := range normalizedFindings {
		if err := evaluationContext.Err(); err != nil {
			return empty, err
		}
		normalizedFindings[index] = normalizedFindings[index].Normalized()
		if normalizedFindings[index].Disposition != DispositionActive ||
			normalizedFindings[index].Exclusion != nil {
			return empty, invalidFinding("disposition", "must be active before exclusion evaluation")
		}
		if err := normalizedFindings[index].ValidateWithin(limits); err != nil {
			return empty, err
		}
		if exclusion := exclusionIndex.selectFor(normalizedFindings[index], evaluatedAt); exclusion != nil {
			normalizedFindings[index].Disposition = DispositionExcluded
			normalizedFindings[index].Exclusion = &AppliedExclusion{
				ID:          exclusion.ID,
				Reason:      exclusion.Reason,
				RequestedBy: exclusion.RequestedBy,
				ApprovedBy:  exclusion.ApprovedBy,
				AppliedAt:   evaluatedAt,
				ExpiresAt:   exclusion.ExpiresAt,
			}
		}
	}

	model := Model{
		SchemaVersion: CurrentSchemaVersion,
		EvaluatedAt:   evaluatedAt,
		Findings:      normalizedFindings,
		Exclusions:    normalizedExclusions,
	}.Normalized()
	if err := model.ValidateWithin(limits); err != nil {
		return empty, err
	}
	return model, nil
}

type exclusionIndex struct {
	byFindingID map[string][]*Exclusion
	byRuleID    map[string][]*Exclusion
}

func newExclusionIndex(exclusions []Exclusion) exclusionIndex {
	index := exclusionIndex{
		byFindingID: make(map[string][]*Exclusion),
		byRuleID:    make(map[string][]*Exclusion),
	}
	for position := range exclusions {
		exclusion := &exclusions[position]
		if exclusion.FindingID != "" {
			index.byFindingID[exclusion.FindingID] = append(
				index.byFindingID[exclusion.FindingID],
				exclusion,
			)
			continue
		}
		index.byRuleID[exclusion.RuleID] = append(index.byRuleID[exclusion.RuleID], exclusion)
	}
	return index
}

func (i exclusionIndex) selectFor(finding Finding, evaluatedAt time.Time) *Exclusion {
	if selected := selectBestExclusion(
		finding,
		i.byFindingID[finding.ID],
		evaluatedAt,
	); selected != nil {
		return selected
	}
	return selectBestExclusion(finding, i.byRuleID[finding.RuleID], evaluatedAt)
}

func selectBestExclusion(
	finding Finding,
	exclusions []*Exclusion,
	evaluatedAt time.Time,
) *Exclusion {
	var selected *Exclusion
	selectedRank := -1
	for _, candidate := range exclusions {
		if !candidate.isActiveAt(evaluatedAt) || !candidate.matches(finding) {
			continue
		}
		rank := candidate.Scope.specificity()
		if selected == nil || rank > selectedRank {
			selected = candidate
			selectedRank = rank
		}
	}
	return selected
}

func (e Exclusion) isActiveAt(evaluatedAt time.Time) bool {
	return !evaluatedAt.Before(e.CreatedAt) && evaluatedAt.Before(e.ExpiresAt)
}

func (e Exclusion) matches(finding Finding) bool {
	if e.FindingID != "" && e.FindingID != finding.ID {
		return false
	}
	if e.RuleID != "" && e.RuleID != finding.RuleID {
		return false
	}
	return e.Scope.matches(finding)
}

func (s ExclusionScope) specificity() int {
	rank := 0
	for _, selector := range []string{
		s.RepositoryID, s.EnvironmentID, s.SubjectKind, s.SubjectID, s.PathPrefix,
	} {
		if selector != "" {
			rank++
		}
	}
	return rank
}

func (s ExclusionScope) matches(finding Finding) bool {
	if s.RepositoryID != "" && !hasRepository(finding, s.RepositoryID) {
		return false
	}
	if s.EnvironmentID != "" && !hasEnvironment(finding.Subjects, s.EnvironmentID) {
		return false
	}
	if s.SubjectKind != "" && !hasSubject(finding.Subjects, s.SubjectKind, s.SubjectID) {
		return false
	}
	return s.PathPrefix == "" || hasPathPrefix(finding.Evidence, s.PathPrefix)
}

func hasRepository(finding Finding, repositoryID string) bool {
	for _, subject := range finding.Subjects {
		if subject.RepositoryID == repositoryID {
			return true
		}
	}
	for _, evidence := range finding.Evidence {
		if evidence.RepositoryID == repositoryID {
			return true
		}
	}
	return false
}

func hasEnvironment(subjects []Subject, environmentID string) bool {
	for _, subject := range subjects {
		if subject.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

func hasSubject(subjects []Subject, kind, id string) bool {
	for _, subject := range subjects {
		if subject.Kind == kind && subject.ID == id {
			return true
		}
	}
	return false
}

func hasPathPrefix(evidence []Evidence, prefix string) bool {
	for _, observation := range evidence {
		if observation.Kind != EvidenceRepositoryFile {
			continue
		}
		if prefix == "." || observation.Path == prefix || path.Dir(observation.Path) == prefix ||
			isDescendantPath(observation.Path, prefix) {
			return true
		}
	}
	return false
}

func isDescendantPath(candidate, prefix string) bool {
	for directory := path.Dir(candidate); directory != "."; directory = path.Dir(directory) {
		if directory == prefix {
			return true
		}
	}
	return false
}

func emptyModel(evaluatedAt time.Time) Model {
	return Model{
		SchemaVersion: CurrentSchemaVersion,
		EvaluatedAt:   evaluatedAt,
		Findings:      make([]Finding, 0),
		Exclusions:    make([]Exclusion, 0),
	}
}
