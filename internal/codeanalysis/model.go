package codeanalysis

import "github.com/kVinsom/Iatros/internal/schema"

// CurrentSchemaVersion identifies the code-analysis model schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// ServiceID identifies a detected service within one code-analysis result.
type ServiceID string

// Protocol identifies a normalized application or transport protocol.
type Protocol string

const (
	// ProtocolHTTP identifies unencrypted HTTP.
	ProtocolHTTP Protocol = "http"
	// ProtocolHTTPS identifies HTTP protected by TLS.
	ProtocolHTTPS Protocol = "https"
	// ProtocolGRPC identifies gRPC.
	ProtocolGRPC Protocol = "grpc"
	// ProtocolGraphQL identifies GraphQL API operations.
	ProtocolGraphQL Protocol = "graphql"
	// ProtocolWebSocket identifies WebSocket connections.
	ProtocolWebSocket Protocol = "websocket"
	// ProtocolTCP identifies a raw TCP listener.
	ProtocolTCP Protocol = "tcp"
	// ProtocolUDP identifies a raw UDP listener.
	ProtocolUDP Protocol = "udp"
	// ProtocolAMQP identifies the Advanced Message Queuing Protocol.
	ProtocolAMQP Protocol = "amqp"
	// ProtocolKafka identifies the Apache Kafka client protocol.
	ProtocolKafka Protocol = "kafka"
	// ProtocolNATS identifies the NATS client protocol.
	ProtocolNATS Protocol = "nats"
)

// PortReferenceKind identifies the source of a non-literal port.
type PortReferenceKind string

const (
	// PortReferenceEnvironment identifies an environment-variable port reference.
	PortReferenceEnvironment PortReferenceKind = "environment_variable"
	// PortReferenceConfiguration identifies a configuration-key port reference.
	PortReferenceConfiguration PortReferenceKind = "configuration"
)

// Certainty describes whether a fact was read directly or derived from multiple observations.
type Certainty string

const (
	// CertaintyObserved identifies a fact represented directly in repository content.
	CertaintyObserved Certainty = "observed"
	// CertaintyInferred identifies a deterministic conclusion derived from multiple observations.
	CertaintyInferred Certainty = "inferred"
)

// EvidenceKind identifies the repository representation supporting a fact.
type EvidenceKind string

const (
	// EvidenceSource identifies a source-code declaration or expression.
	EvidenceSource EvidenceKind = "source"
	// EvidenceImport identifies a source-code import or equivalent dependency declaration.
	EvidenceImport EvidenceKind = "import"
	// EvidenceManifest identifies a package or build manifest declaration.
	EvidenceManifest EvidenceKind = "manifest"
	// EvidenceConfiguration identifies a local configuration declaration.
	EvidenceConfiguration EvidenceKind = "configuration"
	// EvidenceInfrastructure identifies a local infrastructure declaration.
	EvidenceInfrastructure EvidenceKind = "infrastructure"
)

// ResourceKind classifies an external runtime dependency.
type ResourceKind string

const (
	// ResourceDatabase identifies a persistent database dependency.
	ResourceDatabase ResourceKind = "database"
	// ResourceCache identifies a cache or ephemeral key-value dependency.
	ResourceCache ResourceKind = "cache"
	// ResourceMessageBroker identifies a message broker or event-stream dependency.
	ResourceMessageBroker ResourceKind = "message_broker"
)

// DiagnosticLevel identifies the severity of a code-analysis diagnostic.
type DiagnosticLevel string

const (
	// DiagnosticInfo identifies an informational diagnostic that does not make the result partial.
	DiagnosticInfo DiagnosticLevel = "info"
	// DiagnosticWarning identifies omitted or ambiguous analysis that makes the result partial.
	DiagnosticWarning DiagnosticLevel = "warning"
	// DiagnosticError identifies a failed analysis unit that makes the result partial.
	DiagnosticError DiagnosticLevel = "error"
)

// Model contains normalized facts from one bounded static repository analysis.
type Model struct {
	SchemaVersion        schema.Version        `json:"schema_version"`
	Services             []Service             `json:"services"`
	Frameworks           []Framework           `json:"frameworks"`
	PortBindings         []PortBinding         `json:"port_bindings"`
	APIEndpoints         []APIEndpoint         `json:"api_endpoints"`
	EnvironmentVariables []EnvironmentVariable `json:"environment_variables"`
	ResourceDependencies []ResourceDependency  `json:"resource_dependencies"`
	Diagnostics          []Diagnostic          `json:"diagnostics"`
	Partial              bool                  `json:"partial"`
}

// Service describes one statically detected application, worker, job, or other executable capability.
type Service struct {
	ID          ServiceID  `json:"id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Root        string     `json:"root"`
	Entrypoints []string   `json:"entrypoints"`
	Certainty   Certainty  `json:"certainty"`
	Evidence    []Evidence `json:"evidence"`
}

// Framework describes one framework associated with a detected service.
type Framework struct {
	ServiceID         ServiceID  `json:"service_id"`
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Language          string     `json:"language"`
	VersionConstraint string     `json:"version_constraint,omitempty"`
	Certainty         Certainty  `json:"certainty"`
	Evidence          []Evidence `json:"evidence"`
}

// PortBinding describes a literal or named runtime port used by a service.
// Exactly one of Port and Reference is populated.
type PortBinding struct {
	ServiceID     ServiceID         `json:"service_id"`
	Name          string            `json:"name,omitempty"`
	Port          uint16            `json:"port,omitempty"`
	Reference     string            `json:"reference,omitempty"`
	ReferenceKind PortReferenceKind `json:"reference_kind,omitempty"`
	Protocol      Protocol          `json:"protocol"`
	Certainty     Certainty         `json:"certainty"`
	Evidence      []Evidence        `json:"evidence"`
}

// APIEndpoint describes one statically declared API operation.
type APIEndpoint struct {
	ServiceID ServiceID  `json:"service_id"`
	Protocol  Protocol   `json:"protocol"`
	Method    string     `json:"method,omitempty"`
	Path      string     `json:"path"`
	Operation string     `json:"operation,omitempty"`
	Certainty Certainty  `json:"certainty"`
	Evidence  []Evidence `json:"evidence"`
}

// EnvironmentVariable records usage metadata without retaining a resolved or default value.
type EnvironmentVariable struct {
	ServiceID   ServiceID  `json:"service_id"`
	Name        string     `json:"name"`
	IsRequired  bool       `json:"is_required"`
	HasDefault  bool       `json:"has_default"`
	IsSensitive bool       `json:"is_sensitive"`
	Certainty   Certainty  `json:"certainty"`
	Evidence    []Evidence `json:"evidence"`
}

// ResourceDependency records a service dependency without retaining credentials or connection strings.
type ResourceDependency struct {
	ServiceID  ServiceID    `json:"service_id"`
	Kind       ResourceKind `json:"kind"`
	Technology string       `json:"technology"`
	Name       string       `json:"name,omitempty"`
	Certainty  Certainty    `json:"certainty"`
	Evidence   []Evidence   `json:"evidence"`
}

// Evidence identifies the repository location and representation supporting a fact.
// Positions are one-based and inclusive. Zero positions apply evidence to the complete file.
type Evidence struct {
	Kind        EvidenceKind `json:"kind"`
	Path        string       `json:"path"`
	StartLine   int          `json:"start_line,omitempty"`
	StartColumn int          `json:"start_column,omitempty"`
	EndLine     int          `json:"end_line,omitempty"`
	EndColumn   int          `json:"end_column,omitempty"`
}

// Diagnostic explains an unsupported, ambiguous, skipped, or failed analysis unit.
type Diagnostic struct {
	Code    string          `json:"code"`
	Level   DiagnosticLevel `json:"level"`
	Path    string          `json:"path"`
	Message string          `json:"message"`
}
