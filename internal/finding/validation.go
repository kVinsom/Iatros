package finding

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
	"github.com/kVinsom/Iatros/internal/security"
)

var (
	// ErrInvalidFinding indicates a malformed, unsafe, or incomplete finding.
	ErrInvalidFinding = errors.New("finding is invalid")
	// ErrInvalidExclusion indicates a malformed, unbounded, or unauditable exclusion.
	ErrInvalidExclusion = errors.New("finding exclusion is invalid")
	// ErrInvalidModel indicates a finding model that violates its normalized contract.
	ErrInvalidModel = errors.New("finding model is invalid")
)

// Validate checks every required finding dimension and controlled-exclusion invariant.
func (f Finding) Validate() error {
	if !validNamespacedIdentifier(f.ID) {
		return invalidFinding("id", "is invalid")
	}
	if !validNamespacedIdentifier(f.RuleID) {
		return invalidFinding("rule_id", "is invalid")
	}
	if !validText(f.Title) || !validText(f.Description) {
		return invalidFinding("description", "is invalid")
	}
	if !validSeverity(f.Severity) || !validConfidence(f.Confidence) {
		return invalidFinding("assessment", "is invalid")
	}
	if len(f.Subjects) == 0 || !validSubjects(f.Subjects) {
		return invalidFinding("subjects", "are invalid")
	}
	if len(f.Evidence) == 0 || !validEvidenceList(f.Evidence) {
		return invalidFinding("evidence", "is invalid")
	}
	if !validProvenance(f.Provenance) {
		return invalidFinding("provenance", "is invalid")
	}
	if !validRisk(f.Risk) {
		return invalidFinding("risk", "is invalid")
	}
	if !validRecommendation(f.Recommendation) {
		return invalidFinding("recommendation", "is invalid")
	}
	if !validDisposition(f.Disposition, f.Exclusion) {
		return invalidFinding("disposition", "is invalid")
	}
	return nil
}

// ValidateWithin checks a finding and verifies its retained data against configured limits.
func (f Finding) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(f.Subjects) > limits.MaxSubjectsPerFinding ||
		len(f.Evidence) > limits.MaxEvidencePerFinding ||
		len(f.Recommendation.Actions) > limits.MaxActionsPerFinding {
		return invalidFinding("collections", "exceed configured limits")
	}
	if !findingTextsFitLimit(f, limits.MaxTextBytes) {
		return invalidFinding("text", "exceeds the configured limit")
	}
	return f.Validate()
}

// Validate checks target, scope, audit actors, reason, and bounded lifetime.
func (e Exclusion) Validate() error {
	if !validNamespacedIdentifier(e.ID) {
		return invalidExclusion("id", "is invalid")
	}
	hasFindingTarget := e.FindingID != ""
	hasRuleTarget := e.RuleID != ""
	if hasFindingTarget == hasRuleTarget {
		return invalidExclusion("target", "must select exactly one finding or rule")
	}
	if hasFindingTarget && !validNamespacedIdentifier(e.FindingID) {
		return invalidExclusion("finding_id", "is invalid")
	}
	if hasRuleTarget && (!validNamespacedIdentifier(e.RuleID) || e.Scope.isEmpty()) {
		return invalidExclusion("rule_id", "requires a valid bounded scope")
	}
	if !e.Scope.isValid() {
		return invalidExclusion("scope", "is invalid")
	}
	if !validText(e.Reason) || !validText(e.RequestedBy) || !validText(e.ApprovedBy) {
		return invalidExclusion("audit", "is invalid")
	}
	if !validTimestamp(e.CreatedAt) || !validTimestamp(e.ExpiresAt) ||
		!e.ExpiresAt.After(e.CreatedAt) {
		return invalidExclusion("lifetime", "is invalid")
	}
	return nil
}

// ValidateWithin checks an exclusion and verifies its text against configured limits.
func (e Exclusion) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if !textsFitLimit(limits.MaxTextBytes,
		e.ID, e.FindingID, e.RuleID, e.Scope.RepositoryID, e.Scope.EnvironmentID,
		e.Scope.SubjectKind, e.Scope.SubjectID, e.Scope.PathPrefix, e.Reason,
		e.RequestedBy, e.ApprovedBy,
	) {
		return invalidExclusion("text", "exceeds the configured limit")
	}
	return e.Validate()
}

// Validate checks normalized ordering, uniqueness, references, and evaluated exclusion state.
func (m Model) Validate() error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) || !validTimestamp(m.EvaluatedAt) {
		return invalidModel("header", "is invalid")
	}
	for index, exclusion := range m.Exclusions {
		if err := exclusion.Validate(); err != nil {
			return invalidModel(indexedField("exclusions", index), "is invalid")
		}
		if index > 0 && compareExclusions(m.Exclusions[index-1], exclusion) >= 0 {
			return invalidModel("exclusions", "must be strictly ordered by id")
		}
	}
	exclusionIndex := newExclusionIndex(m.Exclusions)
	seenFindingIDs := make(map[string]struct{}, len(m.Findings))
	for index, finding := range m.Findings {
		if err := finding.Validate(); err != nil {
			return invalidModel(indexedField("findings", index), "is invalid")
		}
		if _, exists := seenFindingIDs[finding.ID]; exists {
			return invalidModel("findings", "contain duplicate ids")
		}
		seenFindingIDs[finding.ID] = struct{}{}
		if index > 0 && compareFindings(m.Findings[index-1], finding) >= 0 {
			return invalidModel("findings", "must be strictly ordered")
		}
		selectedExclusion := exclusionIndex.selectFor(finding, m.EvaluatedAt)
		if finding.Disposition == DispositionActive && selectedExclusion != nil {
			return invalidModel(indexedField("findings", index), "did not apply an active exclusion")
		}
		if finding.Disposition == DispositionExcluded &&
			(selectedExclusion == nil ||
				!validAppliedExclusion(finding, *selectedExclusion, m.EvaluatedAt)) {
			return invalidModel(indexedField("findings", index), "has an invalid exclusion")
		}
	}
	return nil
}

// ValidateWithin validates the model and every retained record against configured limits.
func (m Model) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Findings) > limits.MaxFindings || len(m.Exclusions) > limits.MaxExclusions {
		return invalidModel("collections", "exceed configured limits")
	}
	for _, finding := range m.Findings {
		if err := finding.ValidateWithin(limits); err != nil {
			return invalidModel("findings", "contain an invalid record")
		}
	}
	for _, exclusion := range m.Exclusions {
		if err := exclusion.ValidateWithin(limits); err != nil {
			return invalidModel("exclusions", "contain an invalid record")
		}
	}
	return m.Validate()
}

func validSubjects(subjects []Subject) bool {
	for index, subject := range subjects {
		if !validIdentifier(subject.Kind) || !validDottedIdentifier(subject.ID) ||
			(subject.RepositoryID != "" && !validDottedIdentifier(subject.RepositoryID)) ||
			(subject.EnvironmentID != "" && !validDottedIdentifier(subject.EnvironmentID)) ||
			(index > 0 && compareSubjects(subjects[index-1], subject) >= 0) {
			return false
		}
	}
	return true
}

func validEvidenceList(evidence []Evidence) bool {
	for index, observation := range evidence {
		if !validEvidence(observation) ||
			(index > 0 && compareEvidence(evidence[index-1], observation) >= 0) {
			return false
		}
	}
	return true
}

func validEvidence(evidence Evidence) bool {
	if !validEvidenceKind(evidence.Kind) || !validText(evidence.Description) ||
		!validEvidencePosition(evidence) {
		return false
	}
	switch evidence.Kind {
	case EvidenceRepositoryFile:
		return validDottedIdentifier(evidence.RepositoryID) &&
			repositorypath.IsValidFile(evidence.Path) && evidence.Reference == ""
	case EvidenceObservation:
		return evidence.RepositoryID == "" && evidence.Path == "" && evidence.Reference == "" &&
			evidence.StartLine == 0
	case EvidenceRuntime, EvidenceExternal:
		return security.IsSafeProviderReference(evidence.Reference) &&
			evidence.RepositoryID == "" && evidence.Path == "" &&
			evidence.StartLine == 0
	default:
		return false
	}
}

func validEvidencePosition(evidence Evidence) bool {
	positions := [4]int{evidence.StartLine, evidence.StartColumn, evidence.EndLine, evidence.EndColumn}
	allZero := true
	for _, position := range positions {
		if position < 0 {
			return false
		}
		allZero = allZero && position == 0
	}
	if allZero {
		return true
	}
	if evidence.StartLine <= 0 || evidence.StartColumn <= 0 || evidence.EndLine <= 0 ||
		evidence.EndColumn <= 0 || evidence.EndLine < evidence.StartLine {
		return false
	}
	return evidence.EndLine != evidence.StartLine || evidence.EndColumn >= evidence.StartColumn
}

func validEvidenceKind(kind EvidenceKind) bool {
	return kind == EvidenceObservation || kind == EvidenceRepositoryFile ||
		kind == EvidenceRuntime || kind == EvidenceExternal
}

func validProvenance(provenance Provenance) bool {
	return validNamespacedIdentifier(provenance.Producer) &&
		validVersion(provenance.ProducerVersion) && validVersion(provenance.RuleVersion) &&
		validIdentifier(provenance.Source)
}

func validVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return true
}

func validRisk(risk Risk) bool {
	return (risk.Level == RiskLow || risk.Level == RiskMedium || risk.Level == RiskHigh ||
		risk.Level == RiskCritical) &&
		(risk.Likelihood == LikelihoodUnlikely || risk.Likelihood == LikelihoodPossible ||
			risk.Likelihood == LikelihoodLikely || risk.Likelihood == LikelihoodAlmostCertain) &&
		validText(risk.Summary)
}

func validRecommendation(recommendation Recommendation) bool {
	if !validText(recommendation.Summary) || len(recommendation.Actions) == 0 {
		return false
	}
	for index, action := range recommendation.Actions {
		if !validText(action) || (index > 0 && recommendation.Actions[index-1] >= action) {
			return false
		}
	}
	return true
}

func validDisposition(disposition Disposition, exclusion *AppliedExclusion) bool {
	switch disposition {
	case DispositionActive:
		return exclusion == nil
	case DispositionExcluded:
		return exclusion != nil && validAppliedExclusionRecord(*exclusion)
	default:
		return false
	}
}

func validAppliedExclusionRecord(exclusion AppliedExclusion) bool {
	return validNamespacedIdentifier(exclusion.ID) && validText(exclusion.Reason) &&
		validText(exclusion.RequestedBy) && validText(exclusion.ApprovedBy) &&
		validTimestamp(exclusion.AppliedAt) && validTimestamp(exclusion.ExpiresAt) &&
		exclusion.ExpiresAt.After(exclusion.AppliedAt)
}

func validAppliedExclusion(
	finding Finding,
	record Exclusion,
	evaluatedAt time.Time,
) bool {
	if finding.Exclusion.ID != record.ID || !record.matches(finding) ||
		!record.isActiveAt(evaluatedAt) {
		return false
	}
	return finding.Exclusion.Reason == record.Reason &&
		finding.Exclusion.RequestedBy == record.RequestedBy &&
		finding.Exclusion.ApprovedBy == record.ApprovedBy &&
		finding.Exclusion.AppliedAt.Equal(evaluatedAt) &&
		finding.Exclusion.ExpiresAt.Equal(record.ExpiresAt)
}

func (s ExclusionScope) isEmpty() bool {
	return s == (ExclusionScope{})
}

func (s ExclusionScope) isValid() bool {
	if (s.SubjectKind == "") != (s.SubjectID == "") {
		return false
	}
	return (s.RepositoryID == "" || validDottedIdentifier(s.RepositoryID)) &&
		(s.EnvironmentID == "" || validDottedIdentifier(s.EnvironmentID)) &&
		(s.SubjectKind == "" || validIdentifier(s.SubjectKind)) &&
		(s.SubjectID == "" || validDottedIdentifier(s.SubjectID)) &&
		(s.PathPrefix == "" || s.PathPrefix == "." || repositorypath.IsValidDirectory(s.PathPrefix))
}

func validSeverity(severity Severity) bool {
	return severity == SeverityInformational || severity == SeverityLow ||
		severity == SeverityMedium || severity == SeverityHigh || severity == SeverityCritical
}

func validConfidence(confidence Confidence) bool {
	return confidence == ConfidenceLow || confidence == ConfidenceMedium ||
		confidence == ConfidenceHigh || confidence == ConfidenceConfirmed
}

func validTimestamp(timestamp time.Time) bool {
	if timestamp.IsZero() {
		return false
	}
	_, offset := timestamp.Zone()
	return offset == 0
}

func validDottedIdentifier(identifier string) bool {
	segments := 0
	for segment := range strings.SplitSeq(identifier, ".") {
		if !validIdentifier(segment) {
			return false
		}
		segments++
	}
	return segments > 0
}

func validNamespacedIdentifier(identifier string) bool {
	segments := 0
	for segment := range strings.SplitSeq(identifier, ".") {
		if !validIdentifier(segment) {
			return false
		}
		segments++
	}
	return segments >= 2
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
		if (character != '-' && character != '_') || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
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

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func findingTextsFitLimit(finding Finding, maximumBytes int) bool {
	if !textsFitLimit(maximumBytes, finding.ID, finding.RuleID, finding.Title,
		finding.Description, string(finding.Severity), string(finding.Confidence),
		finding.Provenance.Producer, finding.Provenance.ProducerVersion,
		finding.Provenance.RuleVersion, finding.Provenance.Source,
		string(finding.Risk.Level), string(finding.Risk.Likelihood), finding.Risk.Summary,
		finding.Recommendation.Summary, string(finding.Disposition)) {
		return false
	}
	for _, subject := range finding.Subjects {
		if !textsFitLimit(maximumBytes, subject.Kind, subject.ID, subject.RepositoryID,
			subject.EnvironmentID) {
			return false
		}
	}
	for _, evidence := range finding.Evidence {
		if !textsFitLimit(maximumBytes, string(evidence.Kind), evidence.Description,
			evidence.RepositoryID, evidence.Path, evidence.Reference) {
			return false
		}
	}
	for _, action := range finding.Recommendation.Actions {
		if len(action) > maximumBytes {
			return false
		}
	}
	if finding.Exclusion != nil && !textsFitLimit(maximumBytes, finding.Exclusion.ID,
		finding.Exclusion.Reason, finding.Exclusion.RequestedBy, finding.Exclusion.ApprovedBy) {
		return false
	}
	return true
}

func textsFitLimit(maximumBytes int, fields ...string) bool {
	for _, field := range fields {
		if len(field) > maximumBytes {
			return false
		}
	}
	return true
}

func invalidFinding(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidFinding, field, reason)
}

func invalidExclusion(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidExclusion, field, reason)
}

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}
