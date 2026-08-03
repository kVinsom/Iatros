package cli

import (
	"context"
	"errors"

	"github.com/kVinsom/Iatros/internal/analysis"

	"github.com/spf13/cobra"
)

type analyzeOptions struct {
	format  string
	profile string
}

var errAnalysisUnavailable = errors.New("analysis service is not configured")

func newAnalyzeCommand(analyzer Analyzer) *cobra.Command {
	options := analyzeOptions{format: formatText, profile: defaultScalingProfile}

	command := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a local repository directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateReportFormat(options.format); err != nil {
				return err
			}
			profile, err := basicScalingProfile(options.profile)
			if err != nil {
				return err
			}

			report, analyzeErr := analyze(cmd.Context(), analyzer, analysis.Request{
				Root:    commandTarget(args),
				Profile: profile,
			})
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
	command.Flags().StringVar(
		&options.profile,
		"profile",
		defaultScalingProfile,
		"scaling profile: small or monorepo",
	)

	return command
}

func analyze(
	ctx context.Context,
	analyzer Analyzer,
	request analysis.Request,
) (analysis.Report, error) {
	if err := ctx.Err(); err != nil {
		return reportForProfile(analysis.NewCanceledReport(), request.Profile), err
	}

	if analyzer == nil {
		return failedReportForProfile(
			request.Profile,
			analysis.DiagnosticCodeAnalysisUnavailable,
			"Local repository analysis is unavailable.",
		), errAnalysisUnavailable
	}

	report, err := analyzer.Analyze(ctx, request)
	if contextErr := ctx.Err(); contextErr != nil {
		return reportForProfile(analysis.NewCanceledReport(), request.Profile), contextErr
	}
	if errors.Is(err, context.Canceled) {
		return reportForProfile(analysis.NewCanceledReport(), request.Profile), err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return reportForProfile(analysis.NewAnalysisFailedReport(), request.Profile), err
	}
	if report.Validate() != nil || report.Profile != request.Profile ||
		!outcomeMatchesReport(report, err) {
		return reportForProfile(
			analysis.NewAnalysisFailedReport(),
			request.Profile,
		), analysis.ErrInvalidReport
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

func failedReportForProfile(
	profile analysis.ScalingProfileName,
	code string,
	message string,
) analysis.Report {
	return reportForProfile(failedReport(code, message), profile)
}

func reportForProfile(
	report analysis.Report,
	profile analysis.ScalingProfileName,
) analysis.Report {
	report.Profile = profile
	return report
}
