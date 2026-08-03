package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidResult indicates that a workflow result violates the common envelope contract.
var ErrInvalidResult = errors.New("workflow result is invalid")

// Validate checks the schema, lifecycle, identity, time, ordering, and diagnostic invariants.
func (r Result[T]) Validate() error {
	if !CurrentSchemaVersion.Supports(r.SchemaVersion) {
		return invalidResult("schema_version", "is unsupported")
	}
	if !validIdentifier(string(r.ID)) {
		return invalidResult("id", "must be a lowercase identifier")
	}
	if !validIdentifier(string(r.RunID)) {
		return invalidResult("run_id", "must be a lowercase identifier")
	}
	if !validStage(r.Stage) {
		return invalidResult("stage", "is unsupported")
	}
	if !validOutcome(r.Outcome) {
		return invalidResult("outcome", "is unsupported")
	}
	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() ||
		r.StartedAt.Location() != time.UTC || r.FinishedAt.Location() != time.UTC ||
		r.FinishedAt.Before(r.StartedAt) {
		return invalidResult("timestamps", "must be non-zero UTC values in chronological order")
	}
	if err := validateArtifactReferences("inputs", r.Inputs); err != nil {
		return err
	}
	if err := validateArtifactReferences("outputs", r.Outputs); err != nil {
		return err
	}
	if err := validateDiagnostics(r.Outcome, r.Diagnostics); err != nil {
		return err
	}

	return nil
}

func validateArtifactReferences(field string, references []ArtifactReference) error {
	for index, reference := range references {
		itemField := fmt.Sprintf("%s[%d]", field, index)
		if !validIdentifier(reference.Kind) || !validText(reference.ID) {
			return invalidResult(itemField, "has an invalid kind or id")
		}
		if reference.SchemaVersion != "" && reference.SchemaVersion.Validate() != nil {
			return invalidResult(itemField+".schema_version", "is invalid")
		}
		if reference.Digest != "" && !validDigest(reference.Digest) {
			return invalidResult(itemField+".digest", "is invalid")
		}
		if index > 0 && compareArtifactReferences(references[index-1], reference) >= 0 {
			return invalidResult(field, "must be strictly ordered")
		}
	}

	return nil
}

func validateDiagnostics(outcome Outcome, diagnostics []Diagnostic) error {
	if outcome != OutcomeCompleted && len(diagnostics) == 0 {
		return invalidResult("diagnostics", "must explain a non-completed outcome")
	}

	for index, diagnostic := range diagnostics {
		field := fmt.Sprintf("diagnostics[%d]", index)
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!validText(diagnostic.Message) {
			return invalidResult(field, "is invalid")
		}
		if outcome == OutcomeCompleted && diagnostic.Level == DiagnosticLevelError {
			return invalidResult(field, "cannot report an error for a completed outcome")
		}
		if index > 0 && compareDiagnostics(diagnostics[index-1], diagnostic) >= 0 {
			return invalidResult("diagnostics", "must be strictly ordered")
		}
	}

	return nil
}

func validStage(stage Stage) bool {
	switch stage {
	case StageAnalyze, StagePlan, StageGenerate, StageValidate, StageDeploy, StageMonitor, StageFix:
		return true
	default:
		return false
	}
}

func validOutcome(outcome Outcome) bool {
	switch outcome {
	case OutcomeCompleted, OutcomePartial, OutcomeBlocked, OutcomeUnavailable, OutcomeFailed,
		OutcomeCancelled, OutcomeDenied, OutcomeRolledBack, OutcomeIndeterminate, OutcomeSkipped:
		return true
	default:
		return false
	}
}

func validDiagnosticLevel(level DiagnosticLevel) bool {
	return level == DiagnosticLevelInfo || level == DiagnosticLevelWarning || level == DiagnosticLevelError
}

func validDiagnosticCode(value string) bool {
	if value == "" || !upperAlphaNumeric(value[0]) || !upperAlphaNumeric(value[len(value)-1]) {
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

func validDigest(value string) bool {
	algorithm, encoded, found := strings.Cut(value, ":")
	if !found || !validIdentifier(algorithm) || len(encoded) < 16 || len(encoded)%2 != 0 {
		return false
	}
	for index := range len(encoded) {
		character := encoded[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}

	return true
}

func validIdentifier(value string) bool {
	if value == "" || !lowerAlphaNumeric(value[0]) || !lowerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
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

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
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

func invalidResult(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidResult, field, reason)
}
