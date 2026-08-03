// Package workflow defines provider-neutral lifecycle result contracts.
package workflow

import (
	"slices"
	"strings"
	"time"

	"github.com/kVinsom/Iatros/internal/schema"
)

// CurrentSchemaVersion identifies the workflow-result schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// ResultID identifies one immutable stage result.
type ResultID string

// RunID correlates stage results produced for one workflow run.
type RunID string

// Stage identifies a stage in the canonical IATROS lifecycle.
type Stage string

const (
	// StageAnalyze identifies evidence collection and interpretation.
	StageAnalyze Stage = "analyze"
	// StagePlan identifies reviewable change planning.
	StagePlan Stage = "plan"
	// StageGenerate identifies candidate artifact generation.
	StageGenerate Stage = "generate"
	// StageValidate identifies deterministic candidate validation.
	StageValidate Stage = "validate"
	// StageDeploy identifies an authorized external effect and its verification.
	StageDeploy Stage = "deploy"
	// StageMonitor identifies operational observation and signal interpretation.
	StageMonitor Stage = "monitor"
	// StageFix identifies diagnosis and remediation-intent creation.
	StageFix Stage = "fix"
)

// Outcome describes the terminal state of a lifecycle stage.
type Outcome string

const (
	// OutcomeCompleted indicates that the complete stage contract was satisfied.
	OutcomeCompleted Outcome = "completed"
	// OutcomePartial indicates a trustworthy result with declared omissions.
	OutcomePartial Outcome = "partial"
	// OutcomeBlocked indicates that a known prerequisite prevented execution.
	OutcomeBlocked Outcome = "blocked"
	// OutcomeUnavailable indicates that a required capability was not installed or entitled.
	OutcomeUnavailable Outcome = "unavailable"
	// OutcomeFailed indicates that no trustworthy complete result could be produced.
	OutcomeFailed Outcome = "failed"
	// OutcomeCancelled indicates that cancellation stopped the stage.
	OutcomeCancelled Outcome = "cancelled"
	// OutcomeDenied indicates that policy or authorization rejected the operation.
	OutcomeDenied Outcome = "denied"
	// OutcomeRolledBack indicates that the attempted change was returned to a verified prior state.
	OutcomeRolledBack Outcome = "rolled_back"
	// OutcomeIndeterminate indicates that the external state could not be proven.
	OutcomeIndeterminate Outcome = "indeterminate"
	// OutcomeSkipped indicates an explicit decision not to execute the stage.
	OutcomeSkipped Outcome = "skipped"
)

// DiagnosticLevel identifies the operational importance of a diagnostic.
type DiagnosticLevel string

const (
	// DiagnosticLevelInfo identifies explanatory information.
	DiagnosticLevelInfo DiagnosticLevel = "info"
	// DiagnosticLevelWarning identifies a recoverable limitation or risk.
	DiagnosticLevelWarning DiagnosticLevel = "warning"
	// DiagnosticLevelError identifies an error that prevented a complete successful result.
	DiagnosticLevelError DiagnosticLevel = "error"
)

// ArtifactReference identifies an immutable stage input or output.
type ArtifactReference struct {
	Kind          string         `json:"kind"`
	ID            string         `json:"id"`
	SchemaVersion schema.Version `json:"schema_version,omitempty"`
	Digest        string         `json:"digest,omitempty"`
}

// Diagnostic explains a result, limitation, policy decision, or failure.
type Diagnostic struct {
	Code    string          `json:"code"`
	Level   DiagnosticLevel `json:"level"`
	Message string          `json:"message"`
}

// Result is the common immutable envelope for a terminal lifecycle-stage result.
// Payload validation remains the responsibility of the capability that owns T.
type Result[T any] struct {
	SchemaVersion schema.Version      `json:"schema_version"`
	ID            ResultID            `json:"id"`
	RunID         RunID               `json:"run_id"`
	Stage         Stage               `json:"stage"`
	Outcome       Outcome             `json:"outcome"`
	StartedAt     time.Time           `json:"started_at"`
	FinishedAt    time.Time           `json:"finished_at"`
	Inputs        []ArtifactReference `json:"inputs"`
	Outputs       []ArtifactReference `json:"outputs"`
	Diagnostics   []Diagnostic        `json:"diagnostics"`
	Data          T                   `json:"data"`
}

// Normalized returns a result with detached envelope collections, UTC timestamps, and deterministic ordering.
func (r Result[T]) Normalized() Result[T] {
	if !r.StartedAt.IsZero() {
		r.StartedAt = r.StartedAt.UTC()
	}
	if !r.FinishedAt.IsZero() {
		r.FinishedAt = r.FinishedAt.UTC()
	}

	r.Inputs = normalizedSlice(r.Inputs)
	slices.SortFunc(r.Inputs, compareArtifactReferences)
	r.Outputs = normalizedSlice(r.Outputs)
	slices.SortFunc(r.Outputs, compareArtifactReferences)
	r.Diagnostics = normalizedSlice(r.Diagnostics)
	slices.SortFunc(r.Diagnostics, compareDiagnostics)

	return r
}

func compareArtifactReferences(left, right ArtifactReference) int {
	if comparison := strings.Compare(left.Kind, right.Kind); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.ID, right.ID); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.SchemaVersion.String(), right.SchemaVersion.String()); comparison != 0 {
		return comparison
	}

	return strings.Compare(left.Digest, right.Digest)
}

func compareDiagnostics(left, right Diagnostic) int {
	if comparison := strings.Compare(left.Code, right.Code); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(string(left.Level), string(right.Level)); comparison != 0 {
		return comparison
	}

	return strings.Compare(left.Message, right.Message)
}

func normalizedSlice[S ~[]E, E any](values S) S {
	result := slices.Clone(values)
	if result == nil {
		return make(S, 0)
	}

	return result
}
