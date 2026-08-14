package doctor

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/finding"
)

var (
	// ErrInvalidSnapshot indicates malformed or unsupported Doctor input.
	ErrInvalidSnapshot = errors.New("doctor snapshot is invalid")
	// ErrInvalidReport indicates a Doctor report that violates its normalized contract.
	ErrInvalidReport = errors.New("doctor report is invalid")
	// ErrInvalidRule indicates an unusable or duplicate Doctor rule.
	ErrInvalidRule = errors.New("doctor rule is invalid")
	// ErrRuleBudgetExceeded indicates that a rule found more results than the current audit can retain.
	ErrRuleBudgetExceeded = errors.New("doctor rule finding budget exceeded")
)

// Validate checks schema compatibility, category coverage, findings, diagnostics, and derived status.
func (r Report) Validate() error {
	if !CurrentSchemaVersion.Supports(r.SchemaVersion) || !validIdentifier(r.SnapshotID) ||
		!validStatus(r.Status) || len(r.Coverage) != 6 {
		return invalidReport("header", "is invalid")
	}
	for index, coverage := range r.Coverage {
		if !validCoverage(coverage) || categoryRank(coverage.Category) != index {
			return invalidReport(indexedField("coverage", index), "is invalid")
		}
	}
	if err := r.Findings.Validate(); err != nil {
		return invalidReport("findings", "are invalid")
	}
	for index, diagnostic := range r.Diagnostics {
		if !validDiagnostic(diagnostic) ||
			(index > 0 && compareDiagnostics(r.Diagnostics[index-1], diagnostic) >= 0) {
			return invalidReport(indexedField("diagnostics", index), "is invalid")
		}
	}
	if reportStatus(r.Coverage, r.Findings.Findings) != r.Status {
		return invalidReport("status", "does not match coverage and active findings")
	}
	return nil
}

// ValidateWithin validates a report and verifies retained diagnostics and findings against limits.
func (r Report) ValidateWithin(limits Limits, findingLimits finding.Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(r.Diagnostics) > limits.MaxDiagnostics {
		return invalidReport("diagnostics", "exceed configured limits")
	}
	for _, diagnostic := range r.Diagnostics {
		if !textsFitLimit(limits.MaxTextBytes, diagnostic.Code, diagnostic.Message, diagnostic.RuleID) {
			return invalidReport("diagnostics", "exceed configured text limits")
		}
	}
	if err := r.Findings.ValidateWithin(findingLimits); err != nil {
		return invalidReport("findings", "exceed configured limits")
	}
	return r.Validate()
}

func validCoverage(coverage Coverage) bool {
	if !validCategory(coverage.Category) || !validCoverageStatus(coverage.Status) ||
		coverage.RulesEvaluated < 0 || coverage.RulesSkipped < 0 ||
		coverage.RulesEvaluated+coverage.RulesSkipped == 0 {
		return false
	}
	if coverage.Status == CoverageEvaluated {
		return coverage.RulesEvaluated > 0 && coverage.RulesSkipped == 0 && coverage.Reason == ""
	}
	return coverage.RulesSkipped > 0 && validText(coverage.Reason)
}

func validCategory(category Category) bool {
	return categoryRank(category) < 6
}

func validCoverageStatus(status CoverageStatus) bool {
	return status == CoverageEvaluated || status == CoveragePartial || status == CoverageNotEvaluated
}

func validStatus(status Status) bool {
	return status == StatusReady || status == StatusAttentionRequired ||
		status == StatusNotReady || status == StatusPartial
}

func validDiagnostic(diagnostic Diagnostic) bool {
	return validDiagnosticCode(diagnostic.Code) && validDiagnosticLevel(diagnostic.Level) &&
		validText(diagnostic.Message) && validCategory(diagnostic.Category) &&
		(diagnostic.RuleID == "" || validNamespacedIdentifier(diagnostic.RuleID))
}

func validDiagnosticCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	for _, character := range code {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func validDiagnosticLevel(level DiagnosticLevel) bool {
	return level == DiagnosticInfo || level == DiagnosticWarning || level == DiagnosticError
}

func validRuleDescriptor(descriptor RuleDescriptor) bool {
	const knownInputs = InputCodeAnalysis | InputDevOpsAnalysis | InputSystemMap | InputArtifactValidation
	return validNamespacedIdentifier(descriptor.ID) && validVersion(descriptor.Version) &&
		validCategory(descriptor.Category) && descriptor.RequiredInputs != 0 &&
		descriptor.RequiredInputs&^knownInputs == 0
}

func validIdentifier(identifier string) bool {
	if identifier == "" || len(identifier) > 256 || !lowerAlphaNumeric(identifier[0]) ||
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

func validNamespacedIdentifier(identifier string) bool {
	return validIdentifier(identifier) && strings.ContainsRune(identifier, '.')
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
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

func textsFitLimit(limit int, texts ...string) bool {
	for _, text := range texts {
		if len(text) > limit {
			return false
		}
	}
	return true
}

func indexedField(field string, index int) string {
	return fmt.Sprintf("%s[%d]", field, index)
}

func invalidReport(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidReport, field, reason)
}

func reportStatus(coverage []Coverage, findings []finding.Finding) Status {
	for _, categoryCoverage := range coverage {
		if categoryCoverage.Status != CoverageEvaluated {
			return StatusPartial
		}
	}
	status := StatusReady
	for _, auditFinding := range findings {
		if auditFinding.Disposition == finding.DispositionExcluded {
			continue
		}
		if auditFinding.Severity == finding.SeverityHigh ||
			auditFinding.Severity == finding.SeverityCritical {
			return StatusNotReady
		}
		status = StatusAttentionRequired
	}
	return status
}
