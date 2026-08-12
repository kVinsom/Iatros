package remoteanalysis

import "github.com/kVinsom/Iatros/internal/schema"

// CurrentSchemaVersion identifies the remote-analysis schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// RepositoryID identifies a repository within one remote system.
type RepositoryID string

// RelationshipID identifies a directed relationship between repositories.
type RelationshipID string

// Provider identifies a supported source-control provider family.
type Provider string

const (
	// ProviderGitHub identifies GitHub and GitHub Enterprise sources.
	ProviderGitHub Provider = "github"
	// ProviderGitLab identifies GitLab.com and self-managed GitLab sources.
	ProviderGitLab Provider = "gitlab"
	// ProviderBitbucket identifies Bitbucket Cloud and Bitbucket Data Center sources.
	ProviderBitbucket Provider = "bitbucket"
)

// Visibility identifies normalized repository visibility.
type Visibility string

const (
	// VisibilityUnknown indicates that visibility was not available to the adapter.
	VisibilityUnknown Visibility = "unknown"
	// VisibilityPublic identifies a publicly readable repository.
	VisibilityPublic Visibility = "public"
	// VisibilityPrivate identifies a privately readable repository.
	VisibilityPrivate Visibility = "private"
	// VisibilityInternal identifies provider-defined organization or instance visibility.
	VisibilityInternal Visibility = "internal"
)

// ReferenceKind identifies the selected Git reference that resolved to Revision.
type ReferenceKind string

const (
	// ReferenceDefaultBranch identifies the repository default branch.
	ReferenceDefaultBranch ReferenceKind = "default_branch"
	// ReferenceBranch identifies an explicitly selected branch.
	ReferenceBranch ReferenceKind = "branch"
	// ReferenceTag identifies an explicitly selected tag.
	ReferenceTag ReferenceKind = "tag"
	// ReferenceCommit identifies a directly selected immutable commit.
	ReferenceCommit ReferenceKind = "commit"
)

// EvidenceKind identifies the source of a remote-analysis fact.
type EvidenceKind string

const (
	// EvidenceProviderMetadata identifies sanitized source-control provider metadata.
	EvidenceProviderMetadata EvidenceKind = "provider_metadata"
	// EvidenceRepositoryFile identifies a declaration inside a remote repository revision.
	EvidenceRepositoryFile EvidenceKind = "repository_file"
)

// DiagnosticLevel identifies remote-analysis diagnostic severity.
type DiagnosticLevel string

const (
	// DiagnosticInfo identifies information that does not make the result partial.
	DiagnosticInfo DiagnosticLevel = "info"
	// DiagnosticWarning identifies omitted or ambiguous analysis that makes the result partial.
	DiagnosticWarning DiagnosticLevel = "warning"
	// DiagnosticError identifies a failed analysis unit that makes the result partial.
	DiagnosticError DiagnosticLevel = "error"
)

// Model is the normalized remote-repository composition of one software system.
type Model struct {
	SchemaVersion schema.Version `json:"schema_version"`
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Repositories  []Repository   `json:"repositories"`
	Relationships []Relationship `json:"relationships"`
	Evidence      []Evidence     `json:"evidence"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
	Partial       bool           `json:"partial"`
}

// Repository describes one remote repository at an immutable revision.
type Repository struct {
	ID            RepositoryID  `json:"id"`
	Provider      Provider      `json:"provider"`
	Host          string        `json:"host"`
	Namespace     string        `json:"namespace"`
	Name          string        `json:"name"`
	Revision      string        `json:"revision"`
	Reference     string        `json:"reference"`
	ReferenceKind ReferenceKind `json:"reference_kind"`
	DefaultBranch string        `json:"default_branch,omitempty"`
	Visibility    Visibility    `json:"visibility"`
	IsFork        bool          `json:"is_fork"`
	IsArchived    bool          `json:"is_archived"`
	Evidence      []Evidence    `json:"evidence"`
}

// Relationship describes one evidenced, directed relationship between repositories.
type Relationship struct {
	ID       RelationshipID `json:"id"`
	SourceID RepositoryID   `json:"source_id"`
	TargetID RepositoryID   `json:"target_id"`
	Kind     string         `json:"kind"`
	Evidence []Evidence     `json:"evidence"`
}

// Evidence identifies sanitized provider metadata or a safe file in an immutable repository revision.
type Evidence struct {
	RepositoryID RepositoryID `json:"repository_id"`
	Kind         EvidenceKind `json:"kind"`
	Path         string       `json:"path,omitempty"`
	Reference    string       `json:"reference,omitempty"`
}

// Diagnostic explains an unsupported, ambiguous, skipped, or failed remote-analysis unit.
type Diagnostic struct {
	Code         string          `json:"code"`
	Level        DiagnosticLevel `json:"level"`
	Provider     Provider        `json:"provider,omitempty"`
	RepositoryID RepositoryID    `json:"repository_id,omitempty"`
	Scope        string          `json:"scope"`
	Message      string          `json:"message"`
}
