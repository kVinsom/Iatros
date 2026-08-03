// Package analysis defines provider-neutral local repository analysis contracts.
package analysis

import "errors"

const (
	// SchemaVersion identifies the current machine-readable report schema.
	SchemaVersion = "0.3"
	// TargetKindLocalDirectory identifies a repository rooted in a local directory.
	TargetKindLocalDirectory = "local_directory"
	// TargetRootPath is the privacy-safe path used for the selected analysis root.
	TargetRootPath = "."
)

const (
	// DiagnosticCodeAnalysisNotImplemented identifies the placeholder analysis outcome.
	DiagnosticCodeAnalysisNotImplemented = "IATROS_ANALYSIS_NOT_IMPLEMENTED"
	// DiagnosticCodeAnalysisCanceled identifies analysis stopped by cancellation.
	DiagnosticCodeAnalysisCanceled = "IATROS_ANALYSIS_CANCELLED"
	// DiagnosticCodeAnalysisFailed identifies an unexpected analysis failure.
	DiagnosticCodeAnalysisFailed = "IATROS_ANALYSIS_FAILED"
	// DiagnosticCodeAnalysisUnavailable identifies a missing analysis implementation.
	DiagnosticCodeAnalysisUnavailable = "IATROS_ANALYSIS_UNAVAILABLE"
	// DiagnosticCodeTargetInvalid identifies an unsupported or inaccessible analysis root.
	DiagnosticCodeTargetInvalid = "IATROS_TARGET_INVALID"
	// DiagnosticCodeScalingProfileUnavailable identifies a profile disabled by composition.
	DiagnosticCodeScalingProfileUnavailable = "IATROS_SCALING_PROFILE_UNAVAILABLE"
)

// ErrInvalidReport indicates that an analyzer returned a report outside the stable contract.
var ErrInvalidReport = errors.New("analysis report is invalid")

// Status describes whether analysis completed and whether its result is trustworthy.
type Status string

const (
	// StatusNotImplemented indicates that no repository analysis was performed.
	StatusNotImplemented Status = "not_implemented"
	// StatusCompleted indicates that analysis finished within all configured limits.
	StatusCompleted Status = "completed"
	// StatusPartial indicates that analysis produced a trustworthy but incomplete result.
	StatusPartial Status = "partial"
	// StatusFailed indicates that no trustworthy analysis result could be produced.
	StatusFailed Status = "failed"
)

// Report is the versioned result envelope shared by all output formats.
type Report struct {
	SchemaVersion string             `json:"schema_version"`
	Profile       ScalingProfileName `json:"profile"`
	Status        Status             `json:"status"`
	Target        Target             `json:"target"`
	Summary       Summary            `json:"summary"`
	Ecosystems    []Ecosystem        `json:"ecosystems"`
	Findings      []Finding          `json:"findings"`
	Diagnostics   []Diagnostic       `json:"diagnostics"`
}

// Target identifies the analyzed resource without exposing its absolute local path.
type Target struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Summary contains deterministic counts for the performed scan.
type Summary struct {
	DirectoriesScanned        int `json:"directories_scanned"`
	FilesScanned              int `json:"files_scanned"`
	NestedRepositoriesSkipped int `json:"nested_repositories_skipped"`
	EcosystemsDetected        int `json:"ecosystems_detected"`
	FindingsTotal             int `json:"findings_total"`
}

// Ecosystem records a detected project ecosystem and its local evidence.
type Ecosystem struct {
	ID                string   `json:"id"`
	Category          string   `json:"category"`
	Evidence          []string `json:"evidence"`
	EvidenceTruncated bool     `json:"evidence_truncated"`
}

// Finding describes an evidence-based repository-readiness observation.
type Finding struct {
	Code        string   `json:"code"`
	Severity    string   `json:"severity"`
	Message     string   `json:"message"`
	Evidence    []string `json:"evidence"`
	Remediation string   `json:"remediation"`
}

// Diagnostic explains an analysis outcome or limitation.
type Diagnostic struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// NewNotImplementedReport returns the approved placeholder result without fabricated scan data.
func NewNotImplementedReport() Report {
	report := newReport(StatusNotImplemented)
	report.Diagnostics = append(report.Diagnostics, notImplementedDiagnostic())

	return report
}

// NewInvalidTargetReport creates the canonical report for a rejected analysis root.
func NewInvalidTargetReport() Report {
	return NewFailedReport(Diagnostic{
		Code:  DiagnosticCodeTargetInvalid,
		Level: "error",
		Message: "The selected target is missing, inaccessible, not a directory, " +
			"or an unsupported link.",
	})
}

// NewFailedReport creates a failed report containing the supplied diagnostic.
func NewFailedReport(diagnostic Diagnostic) Report {
	report := newReport(StatusFailed)
	report.Diagnostics = append(report.Diagnostics, diagnostic)

	return report
}

// NewAnalysisFailedReport creates the canonical report for an unexpected analysis failure.
func NewAnalysisFailedReport() Report {
	return NewFailedReport(Diagnostic{
		Code:    DiagnosticCodeAnalysisFailed,
		Level:   "error",
		Message: "Local repository analysis failed.",
	})
}

// NewCanceledReport creates the canonical report for a canceled analysis.
func NewCanceledReport() Report {
	return NewFailedReport(Diagnostic{
		Code:    DiagnosticCodeAnalysisCanceled,
		Level:   "error",
		Message: "Local repository analysis was cancelled.",
	})
}

// Normalized ensures collection fields encode as arrays instead of null values.
func (r Report) Normalized() Report {
	if r.Ecosystems == nil {
		r.Ecosystems = make([]Ecosystem, 0)
	}
	if r.Findings == nil {
		r.Findings = make([]Finding, 0)
	}
	if r.Diagnostics == nil {
		r.Diagnostics = make([]Diagnostic, 0)
	}

	return r
}

func notImplementedDiagnostic() Diagnostic {
	return Diagnostic{
		Code:    DiagnosticCodeAnalysisNotImplemented,
		Level:   "info",
		Message: "Local repository analysis is not implemented yet.",
	}
}

func newReport(status Status) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Profile:       ScalingProfileSmall,
		Status:        status,
		Target: Target{
			Kind: TargetKindLocalDirectory,
			Path: TargetRootPath,
		},
		Ecosystems:  make([]Ecosystem, 0),
		Findings:    make([]Finding, 0),
		Diagnostics: make([]Diagnostic, 0),
	}
}
