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
	profile   ScalingProfileName
}

// NewLocalAnalyzer creates the local analyzer with the conservative default profile.
func NewLocalAnalyzer() (LocalAnalyzer, error) {
	return NewLocalAnalyzerWithProfile(SmallScalingProfile())
}

// NewLocalAnalyzerWithProfile creates a local analyzer with one validated scaling profile.
func NewLocalAnalyzerWithProfile(profile ScalingProfile) (LocalAnalyzer, error) {
	if err := profile.Validate(); err != nil {
		return LocalAnalyzer{}, err
	}
	discovery, err := NewLocalDiscovery(profile.Discovery)
	if err != nil {
		return LocalAnalyzer{}, err
	}
	detector, err := detection.NewMarkerDetector(profile.Detection)
	if err != nil {
		return LocalAnalyzer{}, err
	}

	return newLocalAnalyzerWithProfile(
		discovery,
		detector,
		readiness.Evaluator{},
		profile.Name,
	), nil
}

func newLocalAnalyzer(
	discovery inventoryDiscoverer,
	detector technologyDetector,
	evaluator readinessEvaluator,
) LocalAnalyzer {
	return newLocalAnalyzerWithProfile(
		discovery,
		detector,
		evaluator,
		ScalingProfileSmall,
	)
}

func newLocalAnalyzerWithProfile(
	discovery inventoryDiscoverer,
	detector technologyDetector,
	evaluator readinessEvaluator,
	profile ScalingProfileName,
) LocalAnalyzer {
	return LocalAnalyzer{
		discovery: discovery,
		detector:  detector,
		evaluator: evaluator,
		profile:   profile,
	}
}

// Analyze returns a deterministic report built exclusively from local repository metadata.
func (a LocalAnalyzer) Analyze(ctx context.Context, request Request) (Report, error) {
	profile := normalizedScalingProfileName(a.profile)
	requestedProfile := request.Profile
	if requestedProfile == "" {
		requestedProfile = profile
	}
	if requestedProfile != profile {
		return failedProfileReport(
			requestedProfile,
			DiagnosticCodeScalingProfileUnavailable,
			"The requested scaling profile is unavailable.",
		), ErrScalingProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return reportWithProfile(NewCanceledReport(), profile), err
	}
	if a.discovery == nil || a.detector == nil || a.evaluator == nil {
		return reportWithProfile(NewAnalysisFailedReport(), profile), errLocalAnalyzerUnavailable
	}

	inventory, err := a.discovery.Discover(ctx, request.Root)
	if err != nil {
		return reportForAnalysisError(ctx, profile, err)
	}
	if !validDiscoveryIssues(inventory.Issues) {
		return reportWithProfile(NewAnalysisFailedReport(), profile), ErrInvalidReport
	}
	if err := ctx.Err(); err != nil {
		return reportWithProfile(NewCanceledReport(), profile), err
	}

	technologies, err := a.detector.Detect(ctx, inventory.Files)
	if err != nil {
		return reportForAnalysisError(ctx, profile, err)
	}
	if err := ctx.Err(); err != nil {
		return reportWithProfile(NewCanceledReport(), profile), err
	}

	findings, err := a.evaluator.Evaluate(ctx, readinessSnapshot(inventory, technologies))
	if err != nil {
		return reportForAnalysisError(ctx, profile, err)
	}
	if err := ctx.Err(); err != nil {
		return reportWithProfile(NewCanceledReport(), profile), err
	}

	report := completedLocalReport(inventory, technologies, findings)
	report.Profile = profile
	if err := report.Validate(); err != nil {
		return reportWithProfile(NewAnalysisFailedReport(), profile), err
	}
	return report, nil
}

func reportForAnalysisError(
	ctx context.Context,
	profile ScalingProfileName,
	err error,
) (Report, error) {
	if contextErr := ctx.Err(); contextErr != nil {
		return reportWithProfile(NewCanceledReport(), profile), contextErr
	}
	switch {
	case errors.Is(err, ErrInvalidTarget):
		return reportWithProfile(NewInvalidTargetReport(), profile), ErrInvalidTarget
	case errors.Is(err, context.Canceled):
		return reportWithProfile(NewCanceledReport(), profile), err
	default:
		return reportWithProfile(NewAnalysisFailedReport(), profile), err
	}
}

func reportWithProfile(report Report, profile ScalingProfileName) Report {
	report.Profile = normalizedScalingProfileName(profile)
	return report
}

func failedProfileReport(profile ScalingProfileName, code, message string) Report {
	return reportWithProfile(NewFailedReport(Diagnostic{
		Code:    code,
		Level:   "error",
		Message: message,
	}), profile)
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
	report.Summary.NestedRepositoriesSkipped = len(inventory.NestedRepositories)
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
	for _, readinessFinding := range findings {
		reportValues = append(reportValues, readinessFinding.Normalized())
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
