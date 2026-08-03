package cli

import (
	"context"
	"errors"

	"github.com/kVinsom/Iatros/internal/analysis"

	"github.com/spf13/cobra"
)

type topologyOptions struct {
	format string
}

var errTopologyUnavailable = errors.New("topology service is not configured")

func newTopologyCommand(analyzer TopologyAnalyzer) *cobra.Command {
	options := topologyOptions{format: formatText}

	command := &cobra.Command{
		Use:   "topology [path]",
		Short: "Analyze local repository topology.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateReportFormat(options.format); err != nil {
				return err
			}

			report, analyzeErr := analyzeTopology(
				cmd.Context(),
				analyzer,
				commandTarget(args),
			)
			if err := writeTopologyReport(cmd.OutOrStdout(), options.format, report); err != nil {
				return newExitError(ExitFailure, "could not write topology report")
			}
			return topologyExitError(report.Status, analyzeErr)
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

func analyzeTopology(
	ctx context.Context,
	analyzer TopologyAnalyzer,
	root string,
) (analysis.TopologyReport, error) {
	if err := ctx.Err(); err != nil {
		return analysis.NewTopologyCanceledReport(), err
	}
	if analyzer == nil {
		return analysis.NewTopologyUnavailableReport(), errTopologyUnavailable
	}

	model, err := analyzer.Analyze(ctx, analysis.Request{Root: root})
	if contextErr := ctx.Err(); contextErr != nil {
		return analysis.NewTopologyCanceledReport(), contextErr
	}
	if errors.Is(err, context.Canceled) {
		return analysis.NewTopologyCanceledReport(), err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return analysis.NewTopologyAnalysisFailedReport(), err
	}
	if errors.Is(err, analysis.ErrInvalidTarget) {
		return analysis.NewInvalidTopologyTargetReport(), err
	}
	if err != nil {
		return analysis.NewTopologyAnalysisFailedReport(), err
	}

	report, reportErr := analysis.NewTopologyReport(model)
	if reportErr != nil {
		return analysis.NewTopologyAnalysisFailedReport(), reportErr
	}
	return report, nil
}

func topologyExitError(status analysis.Status, err error) error {
	switch {
	case errors.Is(err, analysis.ErrInvalidTarget):
		return newReportedExitError(ExitInvalidTarget, "invalid topology target")
	case err != nil, status == analysis.StatusFailed:
		return newReportedExitError(ExitFailure, "topology analysis failed")
	default:
		return nil
	}
}
