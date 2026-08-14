package artifactvalidation

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

var (
	// ErrInvalidArtifact indicates an unsafe or malformed artifact descriptor.
	ErrInvalidArtifact = errors.New("artifact descriptor is invalid")
	// ErrInvalidReport indicates a report that violates its normalized contract.
	ErrInvalidReport = errors.New("artifact validation report is invalid")
	// ErrInvalidValidator indicates an unusable or duplicate validator.
	ErrInvalidValidator = errors.New("artifact validator is invalid")
	// ErrInvalidRequest indicates invalid artifact-validation input.
	ErrInvalidRequest = errors.New("artifact validation request is invalid")
)

// Validate checks artifact identity, provenance, path safety, and kind-format compatibility.
func (a Artifact) Validate() error {
	if !validIdentifier(a.ID) ||
		(a.RepositoryID != "" && !validIdentifier(a.RepositoryID)) ||
		!repositorypath.IsValidFile(a.Path) || !validOrigin(a.Origin) || !validKind(a.Kind) ||
		!validFormat(a.Format) || !validNamespacedIdentifier(a.Producer) ||
		!validVersion(a.ProducerVersion) || !validKindFormat(a.Kind, a.Format) {
		return ErrInvalidArtifact
	}
	return nil
}

// Validate checks schema compatibility, normalized ordering, references, and outcome consistency.
func (r Report) Validate() error {
	if !CurrentSchemaVersion.Supports(r.SchemaVersion) || !validStatus(r.Status) || len(r.Artifacts) == 0 {
		return invalidReport("header", "is invalid")
	}
	artifactOutcomes := make(map[string]Outcome, len(r.Artifacts))
	for index, result := range r.Artifacts {
		if err := result.Artifact.Validate(); err != nil || !validOutcome(result.Outcome) ||
			result.BytesRead < 0 || !validResultDigest(result) || !validValidatorIDs(result.Validators) {
			return invalidReport(indexedField("artifacts", index), "is invalid")
		}
		if index > 0 && compareArtifactResults(r.Artifacts[index-1], result) >= 0 {
			return invalidReport("artifacts", "must be strictly ordered by id")
		}
		artifactOutcomes[result.Artifact.ID] = result.Outcome
	}
	for index, diagnostic := range r.Diagnostics {
		if !validDiagnostic(diagnostic) {
			return invalidReport(indexedField("diagnostics", index), "is invalid")
		}
		if _, exists := artifactOutcomes[diagnostic.ArtifactID]; !exists {
			return invalidReport(indexedField("diagnostics", index), "references an unknown artifact")
		}
		if index > 0 && compareDiagnostics(r.Diagnostics[index-1], diagnostic) >= 0 {
			return invalidReport("diagnostics", "must be strictly ordered and unique")
		}
	}
	isPartial := hasUnavailableArtifact(r.Artifacts) || hasPartialDiagnostic(r.Diagnostics)
	if derivedStatus(r.Artifacts, r.Diagnostics, isPartial) != r.Status {
		return invalidReport("status", "does not match artifact outcomes")
	}
	return nil
}

func hasPartialDiagnostic(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "DIAGNOSTICS_TRUNCATED" {
			return true
		}
	}
	return false
}

// ValidateWithin validates a report and verifies every retained collection and text against limits.
func (r Report) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(r.Artifacts) > limits.MaxArtifacts || len(r.Diagnostics) > limits.MaxDiagnostics {
		return invalidReport("collections", "exceed configured limits")
	}
	diagnosticCounts := make(map[string]int, len(r.Artifacts))
	for _, diagnostic := range r.Diagnostics {
		diagnosticCounts[diagnostic.ArtifactID]++
		if diagnosticCounts[diagnostic.ArtifactID] > limits.MaxDiagnosticsPerArtifact ||
			!textsFitLimit(limits.MaxTextBytes, diagnostic.Code, diagnostic.Message,
				diagnostic.ArtifactID, diagnostic.Path, diagnostic.ValidatorID, diagnostic.ValidatorVersion) {
			return invalidReport("diagnostics", "exceed configured limits")
		}
	}
	var totalBytes int64
	for _, result := range r.Artifacts {
		artifact := result.Artifact
		if result.BytesRead < 0 || result.BytesRead > limits.MaxArtifactBytes+1 ||
			result.BytesRead > limits.MaxTotalBytes+1-totalBytes ||
			len(result.Validators) > limits.MaxValidators ||
			!textsFitLimit(limits.MaxTextBytes, artifact.ID, artifact.RepositoryID, artifact.Path,
				artifact.Producer, artifact.ProducerVersion) {
			return invalidReport("artifacts", "exceed configured limits")
		}
		totalBytes += result.BytesRead
	}
	return r.Validate()
}

func validOrigin(origin Origin) bool {
	return origin == OriginExisting || origin == OriginGenerated
}

func validKind(kind Kind) bool {
	switch kind {
	case KindGeneric, KindDockerfile, KindCompose, KindKubernetes, KindHelm, KindTerraform,
		KindPipeline, KindGitOps, KindObservability, KindSecurity:
		return true
	default:
		return false
	}
}

func validFormat(format Format) bool {
	switch format {
	case FormatText, FormatJSON, FormatXML, FormatYAML, FormatHCL, FormatDockerfile:
		return true
	default:
		return false
	}
}

func validKindFormat(kind Kind, format Format) bool {
	switch kind {
	case KindDockerfile:
		return format == FormatDockerfile
	case KindTerraform:
		return format == FormatHCL || format == FormatJSON
	case KindCompose, KindKubernetes, KindHelm, KindPipeline, KindGitOps, KindObservability, KindSecurity:
		return format == FormatYAML || format == FormatJSON
	default:
		return true
	}
}

func validStatus(status Status) bool {
	return status == StatusPassed || status == StatusPassedWithWarnings ||
		status == StatusFailed || status == StatusPartial
}

func validOutcome(outcome Outcome) bool {
	return outcome == OutcomeValid || outcome == OutcomeWarning ||
		outcome == OutcomeInvalid || outcome == OutcomeUnavailable
}

func validResultDigest(result ArtifactResult) bool {
	if result.Digest == "" {
		return result.Outcome == OutcomeUnavailable
	}
	return len(result.Digest) == 71 && strings.HasPrefix(result.Digest, "sha256:") &&
		isLowerHex(result.Digest[len("sha256:"):])
}

func validValidatorIDs(validatorIDs []string) bool {
	for index, validatorID := range validatorIDs {
		if !validNamespacedIdentifier(validatorID) ||
			(index > 0 && validatorIDs[index-1] >= validatorID) {
			return false
		}
	}
	return true
}

func validDiagnostic(diagnostic Diagnostic) bool {
	return validDiagnosticCode(diagnostic.Code) && validDiagnosticLevel(diagnostic.Level) &&
		validText(diagnostic.Message) && validIdentifier(diagnostic.ArtifactID) &&
		repositorypath.IsValidFile(diagnostic.Path) &&
		validNamespacedIdentifier(diagnostic.ValidatorID) && validVersion(diagnostic.ValidatorVersion) &&
		validLocation(diagnostic.Location)
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

func validLocation(location Location) bool {
	if location.StartLine < 0 || location.StartColumn < 0 ||
		location.EndLine < 0 || location.EndColumn < 0 {
		return false
	}
	if location.StartLine == 0 {
		return location.StartColumn == 0 && location.EndLine == 0 && location.EndColumn == 0
	}
	if location.StartColumn == 0 || location.EndLine < location.StartLine {
		return false
	}
	if location.EndLine == 0 {
		return location.EndColumn == 0
	}
	return location.EndColumn > 0 &&
		(location.EndLine != location.StartLine || location.EndColumn >= location.StartColumn)
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

func isLowerHex(text string) bool {
	if text == "" {
		return false
	}
	for _, character := range text {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
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
