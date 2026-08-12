package systemmap

import "github.com/kVinsom/Iatros/internal/schema"

// CurrentSchemaVersion identifies the system-map schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

type (
	// RepositoryID identifies a source repository within the system.
	RepositoryID string
	// ServiceID identifies a deployable or externally operated service.
	ServiceID string
	// LibraryID identifies a reusable library or package owned by the system.
	LibraryID string
	// InfrastructureID identifies an infrastructure capability owned by the system.
	InfrastructureID string
	// EnvironmentID identifies an operational environment.
	EnvironmentID string
	// OwnerID identifies a person, team, or organization named by ownership evidence.
	OwnerID string
	// ExternalResourceID identifies a resource outside the system ownership boundary.
	ExternalResourceID string
	// RelationshipID identifies a directed system relationship.
	RelationshipID string
)

// RepositorySource identifies where repository content originated.
type RepositorySource string

const (
	// RepositorySourceLocal identifies a repository supplied from the local filesystem.
	RepositorySourceLocal RepositorySource = "local"
	// RepositorySourceRemote identifies a repository supplied by an authenticated or public remote adapter.
	RepositorySourceRemote RepositorySource = "remote"
)

// OwnerKind identifies the type of an ownership subject.
type OwnerKind string

const (
	// OwnerPerson identifies an individual explicitly named by ownership evidence.
	OwnerPerson OwnerKind = "person"
	// OwnerTeam identifies a team or group.
	OwnerTeam OwnerKind = "team"
	// OwnerOrganization identifies an organization-level owner.
	OwnerOrganization OwnerKind = "organization"
)

// EntityKind identifies a system-map relationship endpoint.
type EntityKind string

const (
	// EntityRepository identifies a repository endpoint.
	EntityRepository EntityKind = "repository"
	// EntityService identifies a service endpoint.
	EntityService EntityKind = "service"
	// EntityLibrary identifies a library endpoint.
	EntityLibrary EntityKind = "library"
	// EntityInfrastructure identifies an infrastructure endpoint.
	EntityInfrastructure EntityKind = "infrastructure"
	// EntityEnvironment identifies an environment endpoint.
	EntityEnvironment EntityKind = "environment"
	// EntityOwner identifies an owner endpoint.
	EntityOwner EntityKind = "owner"
	// EntityExternalResource identifies an external-resource endpoint.
	EntityExternalResource EntityKind = "external_resource"
)

// EvidenceKind identifies the origin of a system-map fact.
type EvidenceKind string

const (
	// EvidenceTarget identifies the selected repository or system root.
	EvidenceTarget EvidenceKind = "target"
	// EvidenceSource identifies source-code evidence.
	EvidenceSource EvidenceKind = "source"
	// EvidenceManifest identifies a package or build manifest.
	EvidenceManifest EvidenceKind = "manifest"
	// EvidenceConfiguration identifies local operational configuration.
	EvidenceConfiguration EvidenceKind = "configuration"
	// EvidenceOwnership identifies an explicit ownership declaration.
	EvidenceOwnership EvidenceKind = "ownership"
	// EvidenceProvider identifies sanitized metadata from a remote provider.
	EvidenceProvider EvidenceKind = "provider"
)

// DiagnosticLevel identifies system-map diagnostic severity.
type DiagnosticLevel string

const (
	// DiagnosticInfo identifies information that does not make the model partial.
	DiagnosticInfo DiagnosticLevel = "info"
	// DiagnosticWarning identifies omitted or ambiguous mapping that makes the model partial.
	DiagnosticWarning DiagnosticLevel = "warning"
	// DiagnosticError identifies a failed mapping unit that makes the model partial.
	DiagnosticError DiagnosticLevel = "error"
)

// Model is the canonical normalized view of one software system.
type Model struct {
	SchemaVersion     schema.Version     `json:"schema_version"`
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Repositories      []Repository       `json:"repositories"`
	Services          []Service          `json:"services"`
	Libraries         []Library          `json:"libraries"`
	Infrastructure    []Infrastructure   `json:"infrastructure"`
	Environments      []Environment      `json:"environments"`
	Owners            []Owner            `json:"owners"`
	ExternalResources []ExternalResource `json:"external_resources"`
	Relationships     []Relationship     `json:"relationships"`
	Diagnostics       []Diagnostic       `json:"diagnostics"`
	Partial           bool               `json:"partial"`
}

// Repository identifies one local or remote source repository in the system.
type Repository struct {
	ID       RepositoryID     `json:"id"`
	Name     string           `json:"name"`
	Source   RepositorySource `json:"source"`
	Provider string           `json:"provider,omitempty"`
	Locator  string           `json:"locator"`
	Revision string           `json:"revision,omitempty"`
	Evidence []Evidence       `json:"evidence"`
}

// Service describes one deployable or externally operated capability.
type Service struct {
	ID             ServiceID       `json:"id"`
	RepositoryID   RepositoryID    `json:"repository_id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Root           string          `json:"root"`
	EnvironmentIDs []EnvironmentID `json:"environment_ids"`
	Evidence       []Evidence      `json:"evidence"`
}

// Library describes one reusable package or library owned by the system.
type Library struct {
	ID           LibraryID    `json:"id"`
	RepositoryID RepositoryID `json:"repository_id"`
	Name         string       `json:"name"`
	Ecosystem    string       `json:"ecosystem"`
	Version      string       `json:"version,omitempty"`
	Root         string       `json:"root"`
	Evidence     []Evidence   `json:"evidence"`
}

// Infrastructure describes one repository-owned infrastructure capability.
type Infrastructure struct {
	ID             InfrastructureID `json:"id"`
	RepositoryID   RepositoryID     `json:"repository_id"`
	Name           string           `json:"name"`
	Kind           string           `json:"kind"`
	Technology     string           `json:"technology"`
	Root           string           `json:"root"`
	EnvironmentIDs []EnvironmentID  `json:"environment_ids"`
	Evidence       []Evidence       `json:"evidence"`
}

// Environment describes an operational context without provider credentials or runtime state.
type Environment struct {
	ID       EnvironmentID `json:"id"`
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Evidence []Evidence    `json:"evidence"`
}

// Owner describes an explicitly evidenced ownership subject.
type Owner struct {
	ID        OwnerID    `json:"id"`
	Kind      OwnerKind  `json:"kind"`
	Name      string     `json:"name"`
	Reference string     `json:"reference,omitempty"`
	Evidence  []Evidence `json:"evidence"`
}

// ExternalResource describes a credential-free dependency outside the system ownership boundary.
type ExternalResource struct {
	ID       ExternalResourceID `json:"id"`
	Name     string             `json:"name"`
	Kind     string             `json:"kind"`
	Provider string             `json:"provider,omitempty"`
	Evidence []Evidence         `json:"evidence"`
}

// EntityReference identifies one relationship endpoint.
type EntityReference struct {
	Kind EntityKind `json:"kind"`
	ID   string     `json:"id"`
}

// Relationship describes one directed, evidenced relationship between system entities.
type Relationship struct {
	ID             RelationshipID  `json:"id"`
	Source         EntityReference `json:"source"`
	Target         EntityReference `json:"target"`
	Kind           string          `json:"kind"`
	EnvironmentIDs []EnvironmentID `json:"environment_ids"`
	Evidence       []Evidence      `json:"evidence"`
}

// Evidence identifies a local repository path or sanitized provider reference supporting a fact.
type Evidence struct {
	RepositoryID RepositoryID `json:"repository_id"`
	Kind         EvidenceKind `json:"kind"`
	Path         string       `json:"path,omitempty"`
	Reference    string       `json:"reference,omitempty"`
}

// Diagnostic explains an unsupported, ambiguous, skipped, or failed mapping unit.
type Diagnostic struct {
	Code         string          `json:"code"`
	Level        DiagnosticLevel `json:"level"`
	RepositoryID RepositoryID    `json:"repository_id,omitempty"`
	Path         string          `json:"path"`
	Message      string          `json:"message"`
}
