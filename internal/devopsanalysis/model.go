package devopsanalysis

import "github.com/kVinsom/Iatros/internal/schema"

// CurrentSchemaVersion identifies the DevOps-analysis model schema understood by this implementation.
const CurrentSchemaVersion schema.Version = "1.0"

// ToolID identifies a detected DevOps tool within one analysis result.
type ToolID string

// Category identifies a normalized DevOps capability area.
type Category string

const (
	// CategoryContainer identifies container build and local composition tooling.
	CategoryContainer Category = "container"
	// CategoryOrchestration identifies workload orchestration and packaging tooling.
	CategoryOrchestration Category = "orchestration"
	// CategoryInfrastructureAsCode identifies declarative infrastructure provisioning tooling.
	CategoryInfrastructureAsCode Category = "infrastructure_as_code"
	// CategoryCICD identifies continuous integration and delivery tooling.
	CategoryCICD Category = "ci_cd"
	// CategoryGitOps identifies Git-driven reconciliation tooling.
	CategoryGitOps Category = "gitops"
	// CategoryObservability identifies monitoring, logging, tracing, and alerting tooling.
	CategoryObservability Category = "observability"
	// CategorySecurity identifies security, policy, and supply-chain tooling.
	CategorySecurity Category = "security"
	// CategorySecrets identifies local declarations for secret-management tooling.
	CategorySecrets Category = "secrets"
)

// Certainty describes whether a fact was read directly or derived deterministically.
type Certainty string

const (
	// CertaintyObserved identifies a fact represented directly in repository content.
	CertaintyObserved Certainty = "observed"
	// CertaintyInferred identifies a deterministic conclusion derived from multiple observations.
	CertaintyInferred Certainty = "inferred"
)

// EvidenceKind identifies the representation supporting a DevOps fact.
type EvidenceKind string

const (
	// EvidenceFilename identifies an exact, allowlisted filename marker.
	EvidenceFilename EvidenceKind = "filename"
	// EvidenceConfiguration identifies a declaration parsed from configuration content.
	EvidenceConfiguration EvidenceKind = "configuration"
	// EvidenceReference identifies a reference correlated across local configuration files.
	EvidenceReference EvidenceKind = "reference"
)

// TerraformBlockKind identifies a supported Terraform or OpenTofu block class.
type TerraformBlockKind string

const (
	// TerraformBlockProvider identifies a required provider declaration.
	TerraformBlockProvider TerraformBlockKind = "provider"
	// TerraformBlockModule identifies a module declaration.
	TerraformBlockModule TerraformBlockKind = "module"
	// TerraformBlockResource identifies a managed resource declaration.
	TerraformBlockResource TerraformBlockKind = "resource"
	// TerraformBlockData identifies a data-source declaration.
	TerraformBlockData TerraformBlockKind = "data"
	// TerraformBlockVariable identifies an input variable declaration without retaining its value.
	TerraformBlockVariable TerraformBlockKind = "variable"
	// TerraformBlockOutput identifies an output declaration without retaining its value.
	TerraformBlockOutput TerraformBlockKind = "output"
)

// Signal identifies an observability signal handled by a configuration.
type Signal string

const (
	// SignalMetrics identifies metric collection, storage, querying, or alerting.
	SignalMetrics Signal = "metrics"
	// SignalLogs identifies log collection, storage, or querying.
	SignalLogs Signal = "logs"
	// SignalTraces identifies distributed tracing.
	SignalTraces Signal = "traces"
	// SignalProfiles identifies continuous profiling.
	SignalProfiles Signal = "profiles"
	// SignalAlerts identifies alert evaluation or routing.
	SignalAlerts Signal = "alerts"
)

// Enforcement identifies how a security control affects delivery.
type Enforcement string

const (
	// EnforcementUnknown indicates that local evidence cannot prove enforcement behavior.
	EnforcementUnknown Enforcement = "unknown"
	// EnforcementAdvisory identifies a control that reports findings without blocking delivery.
	EnforcementAdvisory Enforcement = "advisory"
	// EnforcementBlocking identifies a control configured to block a workflow on failure.
	EnforcementBlocking Enforcement = "blocking"
)

// DiagnosticLevel identifies the severity of a DevOps-analysis diagnostic.
type DiagnosticLevel string

const (
	// DiagnosticInfo identifies information that does not make the model partial.
	DiagnosticInfo DiagnosticLevel = "info"
	// DiagnosticWarning identifies omitted or ambiguous analysis that makes the model partial.
	DiagnosticWarning DiagnosticLevel = "warning"
	// DiagnosticError identifies a failed analysis unit that makes the model partial.
	DiagnosticError DiagnosticLevel = "error"
)

// Model contains normalized facts from one bounded local DevOps configuration analysis.
type Model struct {
	SchemaVersion          schema.Version          `json:"schema_version"`
	Tools                  []Tool                  `json:"tools"`
	ContainerBuilds        []ContainerBuild        `json:"container_builds"`
	ComposeServices        []ComposeService        `json:"compose_services"`
	KubernetesResources    []KubernetesResource    `json:"kubernetes_resources"`
	HelmCharts             []HelmChart             `json:"helm_charts"`
	TerraformBlocks        []TerraformBlock        `json:"terraform_blocks"`
	Pipelines              []Pipeline              `json:"pipelines"`
	GitOpsResources        []GitOpsResource        `json:"gitops_resources"`
	ObservabilityResources []ObservabilityResource `json:"observability_resources"`
	SecurityControls       []SecurityControl       `json:"security_controls"`
	Diagnostics            []Diagnostic            `json:"diagnostics"`
	Partial                bool                    `json:"partial"`
}

// Tool records one evidenced implementation in the DevOps stack.
type Tool struct {
	ID                ToolID     `json:"id"`
	Name              string     `json:"name"`
	Category          Category   `json:"category"`
	VersionConstraint string     `json:"version_constraint,omitempty"`
	Certainty         Certainty  `json:"certainty"`
	Evidence          []Evidence `json:"evidence"`
}

// ContainerBuild describes one container build target without retaining build-argument values.
type ContainerBuild struct {
	ID             string     `json:"id"`
	ToolID         ToolID     `json:"tool_id"`
	Target         string     `json:"target,omitempty"`
	BaseImages     []string   `json:"base_images"`
	BuildArguments []string   `json:"build_arguments"`
	ExposedPorts   []uint16   `json:"exposed_ports"`
	Certainty      Certainty  `json:"certainty"`
	Evidence       []Evidence `json:"evidence"`
}

// ComposeService describes one service declared by a Compose-compatible configuration.
type ComposeService struct {
	ID                   string     `json:"id"`
	ToolID               ToolID     `json:"tool_id"`
	Name                 string     `json:"name"`
	Image                string     `json:"image,omitempty"`
	BuildContext         string     `json:"build_context,omitempty"`
	Profiles             []string   `json:"profiles"`
	DependsOn            []string   `json:"depends_on"`
	EnvironmentVariables []string   `json:"environment_variables"`
	Ports                []uint16   `json:"ports"`
	Certainty            Certainty  `json:"certainty"`
	Evidence             []Evidence `json:"evidence"`
}

// KubernetesResource describes one Kubernetes object or custom resource.
type KubernetesResource struct {
	ID         string     `json:"id"`
	ToolID     ToolID     `json:"tool_id"`
	APIVersion string     `json:"api_version"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	Namespace  string     `json:"namespace,omitempty"`
	Images     []string   `json:"images"`
	Ports      []uint16   `json:"ports"`
	Certainty  Certainty  `json:"certainty"`
	Evidence   []Evidence `json:"evidence"`
}

// HelmChart describes one local Helm chart and its declared chart dependencies.
type HelmChart struct {
	ID           string     `json:"id"`
	ToolID       ToolID     `json:"tool_id"`
	Root         string     `json:"root"`
	Name         string     `json:"name"`
	Version      string     `json:"version,omitempty"`
	AppVersion   string     `json:"app_version,omitempty"`
	Dependencies []string   `json:"dependencies"`
	Certainty    Certainty  `json:"certainty"`
	Evidence     []Evidence `json:"evidence"`
}

// TerraformBlock describes one Terraform or OpenTofu declaration without expression values.
type TerraformBlock struct {
	ID                string             `json:"id"`
	ToolID            ToolID             `json:"tool_id"`
	Kind              TerraformBlockKind `json:"kind"`
	Type              string             `json:"type,omitempty"`
	Name              string             `json:"name"`
	Provider          string             `json:"provider,omitempty"`
	Source            string             `json:"source,omitempty"`
	VersionConstraint string             `json:"version_constraint,omitempty"`
	Certainty         Certainty          `json:"certainty"`
	Evidence          []Evidence         `json:"evidence"`
}

// Pipeline describes one CI/CD workflow and its local job graph.
type Pipeline struct {
	ID        string        `json:"id"`
	ToolID    ToolID        `json:"tool_id"`
	Name      string        `json:"name"`
	Triggers  []string      `json:"triggers"`
	Jobs      []PipelineJob `json:"jobs"`
	Certainty Certainty     `json:"certainty"`
	Evidence  []Evidence    `json:"evidence"`
}

// PipelineJob describes one CI/CD job without retaining scripts, commands, or secret values.
type PipelineJob struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Needs       []string   `json:"needs"`
	Environment string     `json:"environment,omitempty"`
	Evidence    []Evidence `json:"evidence"`
}

// GitOpsResource describes one local reconciliation declaration.
type GitOpsResource struct {
	ID              string     `json:"id"`
	ToolID          ToolID     `json:"tool_id"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	Namespace       string     `json:"namespace,omitempty"`
	SourcePath      string     `json:"source_path,omitempty"`
	TargetNamespace string     `json:"target_namespace,omitempty"`
	Certainty       Certainty  `json:"certainty"`
	Evidence        []Evidence `json:"evidence"`
}

// ObservabilityResource describes one local telemetry or alerting declaration.
type ObservabilityResource struct {
	ID        string     `json:"id"`
	ToolID    ToolID     `json:"tool_id"`
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	Signals   []Signal   `json:"signals"`
	Certainty Certainty  `json:"certainty"`
	Evidence  []Evidence `json:"evidence"`
}

// SecurityControl describes one local security or secret-management configuration.
type SecurityControl struct {
	ID          string      `json:"id"`
	ToolID      ToolID      `json:"tool_id"`
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Scope       string      `json:"scope,omitempty"`
	Enforcement Enforcement `json:"enforcement"`
	Certainty   Certainty   `json:"certainty"`
	Evidence    []Evidence  `json:"evidence"`
}

// Evidence identifies the repository location supporting a fact.
// Positions are one-based and inclusive. Zero positions apply to the complete file.
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
