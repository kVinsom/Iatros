package readiness_test

import (
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
	"github.com/kVinsom/Iatros/internal/readiness"
)

func TestFindingsFitAnalysisReportContract(t *testing.T) {
	t.Parallel()

	readinessFindings, err := (readiness.Evaluator{}).Evaluate(t.Context(), readiness.Snapshot{
		Directories: []string{"."},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	reportFindings := make([]analysis.Finding, 0, len(readinessFindings))
	for _, readinessFinding := range readinessFindings {
		reportFindings = append(reportFindings, readinessFinding.Normalized())
	}
	report := analysis.Report{
		SchemaVersion: analysis.SchemaVersion,
		Profile:       analysis.ScalingProfileSmall,
		Status:        analysis.StatusCompleted,
		Target: analysis.Target{
			Kind: analysis.TargetKindLocalDirectory,
			Path: analysis.TargetRootPath,
		},
		Summary: analysis.Summary{
			DirectoriesScanned: 1,
			FindingsTotal:      len(reportFindings),
		},
		Ecosystems:  make([]analysis.Ecosystem, 0),
		Findings:    reportFindings,
		Diagnostics: make([]analysis.Diagnostic, 0),
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("translated report validation error = %v; report = %#v", err, report)
	}
}
