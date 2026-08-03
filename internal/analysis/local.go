package analysis

import (
	"context"
	"errors"
	"slices"

	"github.com/kVinsom/Iatros/internal/detection"
	"github.com/kVinsom/Iatros/internal/readiness"
)

var errLocalAnalyzerUnavailable = errors.New("local analyzer is unavailable")

type inventoryDiscoverer interface {
	Discover(context.Context, string) (Inventory, error)
}

type technologyDetector interface {
	Detect(context.Context, []string) ([]detection.Technology, error)
}

type readinessEvaluator interface {
	Evaluate(context.Context, readiness.Snapshot) ([]readiness.Finding, error)
}

// LocalAnalyzer orchestrates bounded local discovery, detection, and readiness evaluation.
type LocalAnalyzer struct {
	discovery inventoryDiscoverer
	detector  technologyDetector
	evaluator readinessEvaluator
}

// NewLocalAnalyzer creates the local analyzer with the conservative default profile.
func NewLocalAnalyzer() (LocalAnalyzer, error) {
	discovery, err := NewLocalDiscovery(DefaultDiscoveryLimits())
	if err != nil {
		return LocalAnalyzer{}, err
	}
	detector, err := detection.NewMarkerDetector(detection.DefaultLimits())
	if err != nil {
		return LocalAnalyzer{}, err
	}

	return newLocalAnalyzer(discovery, detector, readiness.Evaluator{}), nil
}

func newLocalAnalyzer(
	discovery inventoryDiscoverer,
	detector technologyDetector,
	evaluator readinessEvaluator,
) LocalAnalyzer {
	return LocalAnalyzer{
		discovery: discovery,
		detector:  detector,
		evaluator: evaluator,
	}
}

// Analyze returns a deterministic report built exclusively from local repository metadata.
func (a LocalAnalyzer) Analyze(ctx context.Context, request Request) (Report, error) {
	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}
	if a.discovery == nil || a.detector == nil || a.evaluator == nil {
		return NewAnalysisFailedReport(), errLocalAnalyzerUnavailable
	}

	inventory, err := a.discovery.Discover(ctx, request.Root)
	if err != nil {
		return reportForAnalysisError(ctx, err)
	}
	if !validDiscoveryIssues(inventory.Issues) {
		return NewAnalysisFailedReport(), ErrInvalidReport
	}
	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}

	technologies, err := a.detector.Detect(ctx, inventory.Files)
	if err != nil {
		return reportForAnalysisError(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}

	findings, err := a.evaluator.Evaluate(ctx, readinessSnapshot(inventory, technologies))
	if err != nil {
		return reportForAnalysisError(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return NewCanceledReport(), err
	}

	report := completedLocalReport(inventory, technologies, findings)
	if err := report.Validate(); err != nil {
		return NewAnalysisFailedReport(), err
	}
	return report, nil
}

func reportForAnalysisError(ctx context.Context, err error) (Report, error) {
	if contextErr := ctx.Err(); contextErr != nil {
		return NewCanceledReport(), contextErr
	}
	switch {
	case errors.Is(err, ErrInvalidTarget):
		return NewInvalidTargetReport(), ErrInvalidTarget
	case errors.Is(err, context.Canceled):
		return NewCanceledReport(), err
	default:
		return NewAnalysisFailedReport(), err
	}
}

func readinessSnapshot(
	inventory Inventory,
	technologies []detection.Technology,
) readiness.Snapshot {
	readinessTechnologies := make([]readiness.Technology, 0, len(technologies))
	for _, technology := range technologies {
		readinessTechnologies = append(readinessTechnologies, readiness.Technology{
			ID:       technology.ID,
			Category: readiness.TechnologyCategory(technology.Category),
		})
	}

	return readiness.Snapshot{
		Directories:  inventory.Directories,
		Files:        inventory.Files,
		Technologies: readinessTechnologies,
		Partial:      inventory.Partial,
	}
}

func completedLocalReport(
	inventory Inventory,
	technologies []detection.Technology,
	readinessFindings []readiness.Finding,
) Report {
	status := StatusCompleted
	if inventory.Partial {
		status = StatusPartial
	}

	report := newReport(status)
	report.Summary.DirectoriesScanned = len(inventory.Directories)
	report.Summary.FilesScanned = len(inventory.Files)
	report.Ecosystems = reportEcosystems(technologies)
	report.Findings = reportFindings(readinessFindings)
	report.Diagnostics = reportDiagnostics(inventory.Issues)
	report.Summary.EcosystemsDetected = len(report.Ecosystems)
	report.Summary.FindingsTotal = len(report.Findings)
	return report
}

func reportEcosystems(technologies []detection.Technology) []Ecosystem {
	ecosystems := make([]Ecosystem, 0, len(technologies))
	for _, technology := range technologies {
		ecosystems = append(ecosystems, Ecosystem{
			ID:                technology.ID,
			Category:          string(technology.Category),
			Evidence:          slices.Clone(technology.Evidence),
			EvidenceTruncated: technology.EvidenceTruncated,
		})
	}
	return ecosystems
}

func reportFindings(findings []readiness.Finding) []Finding {
	reportValues := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		reportValues = append(reportValues, Finding{
			Code:        finding.Code,
			Severity:    string(finding.Severity),
			Message:     finding.Message,
			Evidence:    slices.Clone(finding.Evidence),
			Remediation: finding.Remediation,
		})
	}
	return reportValues
}

func reportDiagnostics(issues []DiscoveryIssue) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		message := issue.Message
		if issue.Path != TargetRootPath {
			message = issue.Path + ": " + message
		}
		diagnostics = append(diagnostics, Diagnostic{
			Code:    issue.Code,
			Level:   "warning",
			Message: message,
		})
	}
	return diagnostics
}

func validDiscoveryIssues(issues []DiscoveryIssue) bool {
	for _, issue := range issues {
		if !validModelIssue(issue.Code, issue.Path, issue.Message) {
			return false
		}
	}
	return true
}
