package analysis

import (
	"path"
	"strings"
	"unicode/utf8"

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
	if !validFindingCode(finding.Code) ||
		!validSeverity(finding.Severity) ||
		!validPublicMessage(finding.Message) ||
		len(finding.Evidence) == 0 ||
		!strictlySortedStrings(finding.Evidence) ||
		!validPublicMessage(finding.Remediation) {
		return false
	}
	for _, evidence := range finding.Evidence {
		if !validPublicMessage(evidence) {
			return false
		}
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

func validFindingCode(value string) bool {
	segments := 0
	for segment := range strings.SplitSeq(value, ".") {
		if !validLowerIdentifier(segment, "-_") {
			return false
		}
		segments++
	}
	return segments >= 2
}

func validLowerIdentifier(value, separators string) bool {
	if !validText(value) ||
		!lowerAlphaNumeric(value[0]) || !lowerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
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

func validDiagnosticCode(value string) bool {
	if !validText(value) ||
		!upperAlphaNumeric(value[0]) || !upperAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
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

func validPublicMessage(value string) bool {
	return validText(value) && !messageContainsSensitiveLocation(value)
}

func messageContainsSensitiveLocation(message string) bool {
	for token := range strings.FieldsSeq(message) {
		candidate := strings.Trim(token, "()[]{}<>,;:'\"")
		if unsafeMessageLocation(candidate) {
			return true
		}
		for value := range strings.SplitSeq(candidate, "=") {
			if value != candidate && unsafeMessageLocation(
				strings.Trim(value, "()[]{}<>,;:'\""),
			) {
				return true
			}
		}
	}
	return false
}

func unsafeMessageLocation(value string) bool {
	lower := strings.ToLower(value)
	return path.IsAbs(value) || repositorypath.HasWindowsDrivePrefix(value) ||
		strings.ContainsRune(value, '\\') || strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "file:")
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func validSeverity(severity string) bool {
	return severity == "info" || severity == "warning" || severity == "critical"
}

func validDiagnosticLevel(level string) bool {
	return level == "info" || level == "warning" || level == "error"
}

func validSortedRelativePaths(values []string) bool {
	if !strictlySortedStrings(values) {
		return false
	}
	for _, value := range values {
		if !validRelativePath(value) {
			return false
		}
	}
	return true
}

func validRelativePath(value string) bool {
	return repositorypath.IsValidFile(value)
}

func strictlySortedStrings(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] >= values[index] {
			return false
		}
	}
	return true
}

func compareFindings(left, right Finding) int {
	if comparison := severityRank(left.Severity) - severityRank(right.Severity); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Code, right.Code); comparison != 0 {
		return comparison
	}
	return compareStringSlices(left.Evidence, right.Evidence)
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func compareStringSlices(left, right []string) int {
	for index := 0; index < min(len(left), len(right)); index++ {
		if comparison := strings.Compare(left[index], right[index]); comparison != 0 {
			return comparison
		}
	}
	return len(left) - len(right)
}

func validText(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}
