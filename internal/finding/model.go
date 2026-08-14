package finding

import (
	"time"

	"github.com/kVinsom/Iatros/internal/schema"
)

// CurrentSchemaVersion identifies the finding schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// Severity identifies the urgency and potential consequence of a finding.
type Severity string

const (
	// SeverityInformational identifies context that does not currently require remediation.
	SeverityInformational Severity = "informational"
	// SeverityLow identifies a low-consequence problem.
	SeverityLow Severity = "low"
	// SeverityMedium identifies a problem that should be planned for remediation.
	SeverityMedium Severity = "medium"
	// SeverityHigh identifies a serious problem that should be addressed promptly.
	SeverityHigh Severity = "high"
	// SeverityCritical identifies an immediate or potentially catastrophic problem.
	SeverityCritical Severity = "critical"
)

// Confidence identifies how strongly the available evidence supports a finding.
type Confidence string

const (
	// ConfidenceLow identifies a tentative conclusion.
	ConfidenceLow Confidence = "low"
	// ConfidenceMedium identifies a conclusion supported by incomplete or indirect evidence.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceHigh identifies a conclusion supported by strong evidence.
	ConfidenceHigh Confidence = "high"
	// ConfidenceConfirmed identifies a conclusion established by authoritative evidence.
	ConfidenceConfirmed Confidence = "confirmed"
)

// EvidenceKind identifies the representation that supports a finding.
type EvidenceKind string

const (
	// EvidenceObservation identifies a bounded analyzer observation without a file location.
	EvidenceObservation EvidenceKind = "observation"
	// EvidenceRepositoryFile identifies evidence in a repository-relative file.
	EvidenceRepositoryFile EvidenceKind = "repository_file"
	// EvidenceRuntime identifies evidence observed from a running system.
	EvidenceRuntime EvidenceKind = "runtime"
	// EvidenceExternal identifies evidence returned by an external provider.
	EvidenceExternal EvidenceKind = "external"
)

// RiskLevel identifies the assessed business or operational risk.
type RiskLevel string

const (
	// RiskLow identifies limited adverse impact.
	RiskLow RiskLevel = "low"
	// RiskMedium identifies meaningful but contained adverse impact.
	RiskMedium RiskLevel = "medium"
	// RiskHigh identifies serious adverse impact.
	RiskHigh RiskLevel = "high"
	// RiskCritical identifies potentially catastrophic adverse impact.
	RiskCritical RiskLevel = "critical"
)

// Likelihood identifies the assessed probability of the described risk.
type Likelihood string

const (
	// LikelihoodUnlikely identifies a risk that is not expected under normal conditions.
	LikelihoodUnlikely Likelihood = "unlikely"
	// LikelihoodPossible identifies a risk that can reasonably occur.
	LikelihoodPossible Likelihood = "possible"
	// LikelihoodLikely identifies a risk that is expected to occur.
	LikelihoodLikely Likelihood = "likely"
	// LikelihoodAlmostCertain identifies a risk expected under most relevant conditions.
	LikelihoodAlmostCertain Likelihood = "almost_certain"
)

// Disposition identifies whether a finding is actionable or controlled by an exclusion.
type Disposition string

const (
	// DispositionActive identifies a finding that remains actionable.
	DispositionActive Disposition = "active"
	// DispositionExcluded identifies a finding retained under an approved, time-bounded exclusion.
	DispositionExcluded Disposition = "excluded"
)

// Finding contains the required normalized explanation of one detected problem.
type Finding struct {
	ID             string            `json:"id"`
	RuleID         string            `json:"rule_id"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Severity       Severity          `json:"severity"`
	Confidence     Confidence        `json:"confidence"`
	Subjects       []Subject         `json:"subjects"`
	Evidence       []Evidence        `json:"evidence"`
	Provenance     Provenance        `json:"provenance"`
	Risk           Risk              `json:"risk"`
	Recommendation Recommendation    `json:"recommendation"`
	Disposition    Disposition       `json:"disposition"`
	Exclusion      *AppliedExclusion `json:"exclusion,omitempty"`
}

// Subject identifies a normalized system object affected by a finding.
type Subject struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	RepositoryID  string `json:"repository_id,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
}

// Evidence identifies a bounded observation supporting a finding.
type Evidence struct {
	Kind         EvidenceKind `json:"kind"`
	Description  string       `json:"description"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Path         string       `json:"path,omitempty"`
	Reference    string       `json:"reference,omitempty"`
	StartLine    int          `json:"start_line,omitempty"`
	StartColumn  int          `json:"start_column,omitempty"`
	EndLine      int          `json:"end_line,omitempty"`
	EndColumn    int          `json:"end_column,omitempty"`
}

// Provenance identifies the producer and rule contract responsible for a finding.
type Provenance struct {
	Producer        string `json:"producer"`
	ProducerVersion string `json:"producer_version"`
	RuleVersion     string `json:"rule_version"`
	Source          string `json:"source"`
}

// Risk describes the potential impact and likelihood represented by a finding.
type Risk struct {
	Level      RiskLevel  `json:"level"`
	Likelihood Likelihood `json:"likelihood"`
	Summary    string     `json:"summary"`
}

// Recommendation describes the desired outcome and concrete remediation actions.
type Recommendation struct {
	Summary string   `json:"summary"`
	Actions []string `json:"actions"`
}

// AppliedExclusion preserves the audit-relevant decision attached to an excluded finding.
type AppliedExclusion struct {
	ID          string    `json:"id"`
	Reason      string    `json:"reason"`
	RequestedBy string    `json:"requested_by"`
	ApprovedBy  string    `json:"approved_by"`
	AppliedAt   time.Time `json:"applied_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Exclusion is an immutable approval record used to control one finding or a scoped rule.
type Exclusion struct {
	ID          string         `json:"id"`
	FindingID   string         `json:"finding_id,omitempty"`
	RuleID      string         `json:"rule_id,omitempty"`
	Scope       ExclusionScope `json:"scope"`
	Reason      string         `json:"reason"`
	RequestedBy string         `json:"requested_by"`
	ApprovedBy  string         `json:"approved_by"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// ExclusionScope narrows an exclusion to matching finding subjects or repository evidence.
type ExclusionScope struct {
	RepositoryID  string `json:"repository_id,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
	SubjectKind   string `json:"subject_kind,omitempty"`
	SubjectID     string `json:"subject_id,omitempty"`
	PathPrefix    string `json:"path_prefix,omitempty"`
}

// Model contains evaluated findings and the exclusion records considered for them.
type Model struct {
	SchemaVersion schema.Version `json:"schema_version"`
	EvaluatedAt   time.Time      `json:"evaluated_at"`
	Findings      []Finding      `json:"findings"`
	Exclusions    []Exclusion    `json:"exclusions"`
}
