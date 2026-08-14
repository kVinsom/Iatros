package analysis

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/repositorypath"
)

// Validate checks the semantic invariants required by the stable report contract.
func (r Report) Validate() error {
	if r.SchemaVersion != SchemaVersion ||
		!validScalingProfileName(r.Profile) ||
		r.Target.Kind != TargetKindLocalDirectory ||
		r.Target.Path != TargetRootPath ||
		!validStatus(r.Status) ||
		!validSummary(r.Summary, len(r.Ecosystems), len(r.Findings)) ||
		!validResultData(r.Status, r.Summary, r.Ecosystems, r.Findings) ||
		!validDiagnostics(r.Status, r.Diagnostics) {
		return ErrInvalidReport
	}

	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusNotImplemented, StatusCompleted, StatusPartial, StatusFailed:
		return true
	default:
		return false
	}
}

func validSummary(summary Summary, ecosystems, findings int) bool {
	return summary.DirectoriesScanned >= 0 &&
		summary.FilesScanned >= 0 &&
		summary.NestedRepositoriesSkipped >= 0 &&
		summary.EcosystemsDetected == ecosystems &&
		summary.FindingsTotal == findings
}

func validResultData(
	status Status,
	summary Summary,
	ecosystems []Ecosystem,
	findings []Finding,
) bool {
	if (status == StatusNotImplemented || status == StatusFailed) &&
		(summary != (Summary{}) || len(ecosystems) != 0 || len(findings) != 0) {
		return false
	}

	for index, ecosystem := range ecosystems {
		if !validLowerIdentifier(ecosystem.ID, ".-_") ||
			!validLowerIdentifier(ecosystem.Category, "_") ||
			len(ecosystem.Evidence) == 0 ||
			!validSortedRelativePaths(ecosystem.Evidence) ||
			(index > 0 && ecosystems[index-1].ID >= ecosystem.ID) {
			return false
		}
	}

	for index, finding := range findings {
		if !validFinding(finding) ||
			(index > 0 && compareFindings(findings[index-1], finding) >= 0) {
			return false
		}
	}

	return true
}

func validFinding(finding Finding) bool {
	if finding.Validate() != nil || !validPublicMessage(finding.Title) ||
		!validPublicMessage(finding.Description) ||
		!validPublicMessage(finding.Risk.Summary) ||
		!validPublicMessage(finding.Recommendation.Summary) {
		return false
	}
	for _, observation := range finding.Evidence {
		if !validPublicMessage(observation.Description) ||
			(observation.Reference != "" && !validPublicMessage(observation.Reference)) {
			return false
		}
	}
	for _, action := range finding.Recommendation.Actions {
		if !validPublicMessage(action) {
			return false
		}
	}
	if finding.Exclusion != nil && (!validPublicMessage(finding.Exclusion.Reason) ||
		!validPublicMessage(finding.Exclusion.RequestedBy) ||
		!validPublicMessage(finding.Exclusion.ApprovedBy)) {
		return false
	}
	return true
}

func validDiagnostics(status Status, diagnostics []Diagnostic) bool {
	if status != StatusCompleted && len(diagnostics) == 0 {
		return false
	}

	for _, diagnostic := range diagnostics {
		if !validDiagnosticCode(diagnostic.Code) ||
			!validDiagnosticLevel(diagnostic.Level) ||
			!validPublicMessage(diagnostic.Message) {
			return false
		}
	}
	if status == StatusNotImplemented {
		return len(diagnostics) == 1 && diagnostics[0] == notImplementedDiagnostic()
	}

	return true
}

func validLowerIdentifier(identifier, separators string) bool {
	if !validText(identifier) ||
		!lowerAlphaNumeric(identifier[0]) || !lowerAlphaNumeric(identifier[len(identifier)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(identifier) {
		character := identifier[index]
		if lowerAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if !strings.ContainsRune(separators, rune(character)) || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func validDiagnosticCode(code string) bool {
	if !validText(code) ||
		!upperAlphaNumeric(code[0]) || !upperAlphaNumeric(code[len(code)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(code) {
		character := code[index]
		if upperAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if character != '_' || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func validModelIssue(code, issuePath, message string) bool {
	return validDiagnosticCode(code) &&
		(issuePath == "." || validRelativePath(issuePath)) &&
		validPublicMessage(message)
}

func validPublicMessage(message string) bool {
	return validText(message) && !messageContainsSensitiveLocation(message)
}

func messageContainsSensitiveLocation(message string) bool {
	for token := range strings.FieldsSeq(message) {
		candidate := strings.Trim(token, "()[]{}<>,;:'\"")
		if unsafeMessageLocation(candidate) {
			return true
		}
		for fragment := range strings.SplitSeq(candidate, "=") {
			if fragment != candidate && unsafeMessageLocation(
				strings.Trim(fragment, "()[]{}<>,;:'\""),
			) {
				return true
			}
		}
	}
	return false
}

func unsafeMessageLocation(candidate string) bool {
	lower := strings.ToLower(candidate)
	return path.IsAbs(candidate) || looksLikeWindowsPath(candidate) ||
		strings.ContainsRune(candidate, '\\') || strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "file:")
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func validDiagnosticLevel(level string) bool {
	return level == "info" || level == "warning" || level == "error"
}

func validSortedRelativePaths(relativePaths []string) bool {
	if !strictlySortedStrings(relativePaths) {
		return false
	}
	for _, relativePath := range relativePaths {
		if !validRelativePath(relativePath) {
			return false
		}
	}
	return true
}

func validRelativePath(relativePath string) bool {
	return repositorypath.IsValidFile(relativePath)
}

func looksLikeWindowsPath(candidate string) bool {
	return len(candidate) >= 2 &&
		((candidate[0] >= 'A' && candidate[0] <= 'Z') ||
			(candidate[0] >= 'a' && candidate[0] <= 'z')) &&
		candidate[1] == ':'
}

func strictlySortedStrings(entries []string) bool {
	for index := 1; index < len(entries); index++ {
		if entries[index-1] >= entries[index] {
			return false
		}
	}
	return true
}

func compareFindings(left, right Finding) int {
	if comparison := severityRank(left.Severity) - severityRank(right.Severity); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.ID, right.ID)
}

func severityRank(severity finding.Severity) int {
	switch severity {
	case finding.SeverityCritical:
		return 0
	case finding.SeverityHigh:
		return 1
	case finding.SeverityMedium:
		return 2
	case finding.SeverityLow:
		return 3
	default:
		return 4
	}
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
