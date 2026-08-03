package cli

import (
	"context"
	"errors"

	"github.com/kVinsom/Iatros/internal/analysis"

	"github.com/spf13/cobra"
)

type analyzeOptions struct {
	format string
}

var errAnalysisUnavailable = errors.New("analysis service is not configured")

func newAnalyzeCommand(analyzer Analyzer) *cobra.Command {
	options := analyzeOptions{format: formatText}

	command := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a local repository directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateReportFormat(options.format); err != nil {
				return err
			}

			report, analyzeErr := analyze(cmd.Context(), analyzer, commandTarget(args))
			if err := writeReport(cmd.OutOrStdout(), options.format, report); err != nil {
				return newExitError(ExitFailure, "could not write analysis report")
			}

			return analysisExitError(report.Status, analyzeErr)
		},
	}

	command.Flags().StringVar(
		&options.format,
		"format",
		formatText,
		"report format: text or json",
	)

	return command
}

func analyze(ctx context.Context, analyzer Analyzer, root string) (analysis.Report, error) {
	if err := ctx.Err(); err != nil {
		return analysis.NewCanceledReport(), err
	}

	if analyzer == nil {
		return failedReport(
			analysis.DiagnosticCodeAnalysisUnavailable,
			"Local repository analysis is unavailable.",
		), errAnalysisUnavailable
	}

	report, err := analyzer.Analyze(ctx, analysis.Request{Root: root})
	if contextErr := ctx.Err(); contextErr != nil {
		return analysis.NewCanceledReport(), contextErr
	}
	if errors.Is(err, context.Canceled) {
		return analysis.NewCanceledReport(), err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return analysis.NewAnalysisFailedReport(), err
	}
	if report.Validate() != nil || !outcomeMatchesReport(report, err) {
		return analysis.NewAnalysisFailedReport(), analysis.ErrInvalidReport
	}

	return report, err
}

func analysisExitError(status analysis.Status, err error) error {
	switch {
	case errors.Is(err, analysis.ErrInvalidTarget):
		return newReportedExitError(ExitInvalidTarget, "invalid analysis target")
	case errors.Is(err, analysis.ErrNotImplemented), status == analysis.StatusNotImplemented:
		return newReportedExitError(ExitNotImplemented, "analysis is not implemented")
	case err != nil, status == analysis.StatusFailed:
		return newReportedExitError(ExitFailure, "analysis failed")
	default:
		return nil
	}
}

func outcomeMatchesReport(report analysis.Report, err error) bool {
	notImplemented := errors.Is(err, analysis.ErrNotImplemented)
	invalidTarget := errors.Is(err, analysis.ErrInvalidTarget)
	if notImplemented && invalidTarget {
		return false
	}

	switch {
	case notImplemented:
		return report.Status == analysis.StatusNotImplemented
	case invalidTarget:
		return report.Status == analysis.StatusFailed &&
			hasSingleDiagnostic(report, analysis.DiagnosticCodeTargetInvalid)
	case err != nil:
		return report.Status == analysis.StatusFailed
	default:
		return true
	}
}

func hasSingleDiagnostic(report analysis.Report, code string) bool {
	return len(report.Diagnostics) == 1 && report.Diagnostics[0].Code == code
}

func failedReport(code, message string) analysis.Report {
	return analysis.NewFailedReport(analysis.Diagnostic{
		Code:    code,
		Level:   "error",
		Message: message,
	})
}
