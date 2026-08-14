// Package artifactvalidation validates bounded DevOps artifacts without executing them.
package artifactvalidation

import (
	"context"
	"io"

	"github.com/kVinsom/Iatros/internal/schema"
)

// CurrentSchemaVersion identifies the artifact-validation report contract.
const CurrentSchemaVersion schema.Version = "1.0"

// Origin identifies where an artifact came from.
type Origin string

const (
	// OriginExisting identifies an artifact already present in a repository.
	OriginExisting Origin = "existing"
	// OriginGenerated identifies an artifact proposed by a generator.
	OriginGenerated Origin = "generated"
)

// Kind identifies the DevOps role of an artifact.
type Kind string

const (
	KindGeneric       Kind = "generic"
	KindDockerfile    Kind = "dockerfile"
	KindCompose       Kind = "compose"
	KindKubernetes    Kind = "kubernetes"
	KindHelm          Kind = "helm"
	KindTerraform     Kind = "terraform"
	KindPipeline      Kind = "pipeline"
	KindGitOps        Kind = "gitops"
	KindObservability Kind = "observability"
	KindSecurity      Kind = "security"
)

// Format identifies the serialization syntax of an artifact.
type Format string

const (
	FormatText       Format = "text"
	FormatJSON       Format = "json"
	FormatXML        Format = "xml"
	FormatYAML       Format = "yaml"
	FormatHCL        Format = "hcl"
	FormatDockerfile Format = "dockerfile"
)

// Status summarizes the validation report.
type Status string

const (
	StatusPassed             Status = "passed"
	StatusPassedWithWarnings Status = "passed_with_warnings"
	StatusFailed             Status = "failed"
	StatusPartial            Status = "partial"
)

// Outcome summarizes one artifact.
type Outcome string

const (
	OutcomeValid       Outcome = "valid"
	OutcomeWarning     Outcome = "warning"
	OutcomeInvalid     Outcome = "invalid"
	OutcomeUnavailable Outcome = "unavailable"
)

// DiagnosticLevel identifies the importance of a normalized diagnostic.
type DiagnosticLevel string

const (
	DiagnosticInfo    DiagnosticLevel = "info"
	DiagnosticWarning DiagnosticLevel = "warning"
	DiagnosticError   DiagnosticLevel = "error"
)

// Artifact identifies an existing or generated DevOps artifact.
type Artifact struct {
	ID              string `json:"id"`
	RepositoryID    string `json:"repository_id,omitempty"`
	Path            string `json:"path"`
	Origin          Origin `json:"origin"`
	Kind            Kind   `json:"kind"`
	Format          Format `json:"format"`
	Producer        string `json:"producer"`
	ProducerVersion string `json:"producer_version"`
}

// Location identifies a safe source range within an artifact.
type Location struct {
	StartLine   int `json:"start_line,omitempty"`
	StartColumn int `json:"start_column,omitempty"`
	EndLine     int `json:"end_line,omitempty"`
	EndColumn   int `json:"end_column,omitempty"`
}

// Diagnostic is the provider-neutral output of an artifact validator.
type Diagnostic struct {
	Code             string          `json:"code"`
	Level            DiagnosticLevel `json:"level"`
	Message          string          `json:"message"`
	ArtifactID       string          `json:"artifact_id"`
	Path             string          `json:"path"`
	ValidatorID      string          `json:"validator_id"`
	ValidatorVersion string          `json:"validator_version"`
	Location         Location        `json:"location"`
}

// ArtifactResult records content identity and validation outcome without retaining raw content.
type ArtifactResult struct {
	Artifact   Artifact `json:"artifact"`
	Digest     string   `json:"digest,omitempty"`
	BytesRead  int64    `json:"bytes_read"`
	Outcome    Outcome  `json:"outcome"`
	Validators []string `json:"validators"`
}

// Report contains deterministic normalized validation results.
type Report struct {
	SchemaVersion schema.Version   `json:"schema_version"`
	Status        Status           `json:"status"`
	Artifacts     []ArtifactResult `json:"artifacts"`
	Diagnostics   []Diagnostic     `json:"diagnostics"`
}

// Source opens artifact content. The caller retains ownership of Source; the engine closes each returned reader.
type Source interface {
	Open(ctx context.Context, artifact Artifact) (io.ReadCloser, error)
}

// Request binds an artifact descriptor to its content source.
type Request struct {
	Artifact Artifact
	Source   Source
}

// ValidatorDescriptor identifies a deterministic validator implementation.
type ValidatorDescriptor struct {
	ID      string
	Version string
}

// Document provides repeatable read-only access to one bounded artifact.
type Document struct {
	Artifact Artifact
	Digest   string
	Size     int64
	content  []byte
}

// Reader returns a new reader over the bounded content.
func (d Document) Reader() io.Reader {
	return newReadOnlyReader(d.content)
}

// Validator validates an applicable artifact without mutating it or performing external side effects.
type Validator interface {
	Descriptor() ValidatorDescriptor
	AppliesTo(Artifact) bool
	Validate(context.Context, Document, Limits) ([]Diagnostic, error)
}
