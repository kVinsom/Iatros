// Package doctor audits normalized IATROS facts for operational readiness.
package doctor

import (
	"context"

	"github.com/kVinsom/Iatros/internal/artifactvalidation"
	"github.com/kVinsom/Iatros/internal/codeanalysis"
	"github.com/kVinsom/Iatros/internal/devopsanalysis"
	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/schema"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

// CurrentSchemaVersion identifies the Doctor report contract.
const CurrentSchemaVersion schema.Version = "1.0"

// Category identifies an operational audit domain.
type Category string

const (
	CategoryProduction  Category = "production"
	CategoryReliability Category = "reliability"
	CategoryPerformance Category = "performance"
	CategorySecurity    Category = "security"
	CategoryDeployment  Category = "deployment"
	CategoryCost        Category = "cost"
)

// CoverageStatus describes whether a category was evaluated from complete evidence.
type CoverageStatus string

const (
	CoverageEvaluated    CoverageStatus = "evaluated"
	CoveragePartial      CoverageStatus = "partial"
	CoverageNotEvaluated CoverageStatus = "not_evaluated"
)

// Status summarizes system readiness without treating missing evidence as success.
type Status string

const (
	StatusReady             Status = "ready"
	StatusAttentionRequired Status = "attention_required"
	StatusNotReady          Status = "not_ready"
	StatusPartial           Status = "partial"
)

// DiagnosticLevel identifies the importance of a Doctor execution diagnostic.
type DiagnosticLevel string

const (
	DiagnosticInfo    DiagnosticLevel = "info"
	DiagnosticWarning DiagnosticLevel = "warning"
	DiagnosticError   DiagnosticLevel = "error"
)

// InputSet identifies normalized evidence required by a rule.
type InputSet uint8

const (
	InputCodeAnalysis InputSet = 1 << iota
	InputDevOpsAnalysis
	InputSystemMap
	InputArtifactValidation
)

// Snapshot contains normalized facts consumed by Doctor rules.
// Nil inputs are explicitly unavailable and never interpreted as empty successful analysis.
type Snapshot struct {
	ID                 string
	CodeAnalysis       *codeanalysis.Model
	DevOpsAnalysis     *devopsanalysis.Model
	SystemMap          *systemmap.Model
	ValidationReports  []artifactvalidation.Report
	RepositoryFindings []finding.Finding
}

// RuleDescriptor declares stable rule identity, category, and evidence requirements.
type RuleDescriptor struct {
	ID               string
	Version          string
	Category         Category
	RequiredInputs   InputSet
	RequiresComplete bool
}

// RuleBudget bounds the findings one rule may produce for the current audit.
type RuleBudget struct {
	MaxFindings           int
	MaxSubjectsPerFinding int
	MaxEvidencePerFinding int
	MaxActionsPerFinding  int
	MaxTextBytes          int
}

// Rule evaluates one operational concern from normalized facts.
type Rule interface {
	Descriptor() RuleDescriptor
	Evaluate(ctx context.Context, snapshot Snapshot, budget RuleBudget) ([]finding.Finding, error)
}

// Coverage records rule execution for one audit category.
type Coverage struct {
	Category       Category       `json:"category"`
	Status         CoverageStatus `json:"status"`
	RulesEvaluated int            `json:"rules_evaluated"`
	RulesSkipped   int            `json:"rules_skipped"`
	Reason         string         `json:"reason,omitempty"`
}

// Diagnostic reports unavailable evidence or rule execution failure.
type Diagnostic struct {
	Code     string          `json:"code"`
	Level    DiagnosticLevel `json:"level"`
	Message  string          `json:"message"`
	Category Category        `json:"category"`
	RuleID   string          `json:"rule_id,omitempty"`
}

// Report contains coverage, unified findings, and operational diagnostics.
type Report struct {
	SchemaVersion schema.Version `json:"schema_version"`
	SnapshotID    string         `json:"snapshot_id"`
	Status        Status         `json:"status"`
	Coverage      []Coverage     `json:"coverage"`
	Findings      finding.Model  `json:"findings"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}
