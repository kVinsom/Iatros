package changeimpact

import (
	"github.com/kVinsom/Iatros/internal/schema"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

// CurrentSchemaVersion identifies the change-impact schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// ChangeID identifies one changed repository path.
type ChangeID string

// ChangeKind identifies the filesystem operation represented by a change.
type ChangeKind string

const (
	// ChangeAdded identifies a newly added path.
	ChangeAdded ChangeKind = "added"
	// ChangeModified identifies modified content at an existing path.
	ChangeModified ChangeKind = "modified"
	// ChangeDeleted identifies a deleted path.
	ChangeDeleted ChangeKind = "deleted"
	// ChangeRenamed identifies one path moved to another repository-relative path.
	ChangeRenamed ChangeKind = "renamed"
)

// ImpactKind identifies how a target became affected.
type ImpactKind string

const (
	// ImpactDirect identifies a target whose root or evidence contains the changed path.
	ImpactDirect ImpactKind = "direct"
	// ImpactDependent identifies a target reached through a supported dependency relationship.
	ImpactDependent ImpactKind = "dependent"
	// ImpactAssociated identifies an environment attached to an affected service or configuration.
	ImpactAssociated ImpactKind = "associated"
)

// DiagnosticLevel identifies change-impact diagnostic severity.
type DiagnosticLevel string

const (
	// DiagnosticInfo identifies information that does not make the result partial.
	DiagnosticInfo DiagnosticLevel = "info"
	// DiagnosticWarning identifies omitted or ambiguous impact analysis that makes the result partial.
	DiagnosticWarning DiagnosticLevel = "warning"
	// DiagnosticError identifies a failed analysis unit that makes the result partial.
	DiagnosticError DiagnosticLevel = "error"
)

// ChangeSet contains one deterministic set of repository changes.
type ChangeSet struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

// Change describes one changed repository-relative file path.
type Change struct {
	ID           ChangeID               `json:"id"`
	RepositoryID systemmap.RepositoryID `json:"repository_id"`
	Kind         ChangeKind             `json:"kind"`
	Path         string                 `json:"path"`
	PreviousPath string                 `json:"previous_path,omitempty"`
}

// Model contains the bounded services, environments, and configurations affected by a change set.
type Model struct {
	SchemaVersion  schema.Version        `json:"schema_version"`
	ChangeSetID    string                `json:"change_set_id"`
	Services       []ServiceImpact       `json:"services"`
	Environments   []EnvironmentImpact   `json:"environments"`
	Configurations []ConfigurationImpact `json:"configurations"`
	Diagnostics    []Diagnostic          `json:"diagnostics"`
	Partial        bool                  `json:"partial"`
}

// ServiceImpact describes one affected service.
type ServiceImpact struct {
	ServiceID    systemmap.ServiceID    `json:"service_id"`
	RepositoryID systemmap.RepositoryID `json:"repository_id"`
	Kind         ImpactKind             `json:"kind"`
	Causes       []Cause                `json:"causes"`
}

// EnvironmentImpact describes one directly or transitively affected environment.
type EnvironmentImpact struct {
	EnvironmentID systemmap.EnvironmentID `json:"environment_id"`
	Kind          ImpactKind              `json:"kind"`
	Causes        []Cause                 `json:"causes"`
}

// ConfigurationImpact describes one affected infrastructure configuration.
type ConfigurationImpact struct {
	ConfigurationID systemmap.InfrastructureID `json:"configuration_id"`
	RepositoryID    systemmap.RepositoryID     `json:"repository_id"`
	Kind            ImpactKind                 `json:"kind"`
	Causes          []Cause                    `json:"causes"`
}

// Cause preserves the change and dependency path that produced one impact.
type Cause struct {
	ChangeID        ChangeID                   `json:"change_id"`
	Path            string                     `json:"path"`
	RelationshipIDs []systemmap.RelationshipID `json:"relationship_ids"`
}

// Diagnostic explains an unmapped, ambiguous, omitted, or failed impact-analysis unit.
type Diagnostic struct {
	Code         string                 `json:"code"`
	Level        DiagnosticLevel        `json:"level"`
	ChangeID     ChangeID               `json:"change_id,omitempty"`
	RepositoryID systemmap.RepositoryID `json:"repository_id,omitempty"`
	Path         string                 `json:"path"`
	Message      string                 `json:"message"`
}
