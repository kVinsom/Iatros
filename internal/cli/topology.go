package cli

import (
	"context"
	"errors"

	"github.com/kVinsom/Iatros/internal/analysis"

	"github.com/spf13/cobra"
)

type topologyOptions struct {
	format  string
	profile string
}

var errTopologyUnavailable = errors.New("topology service is not configured")

func newTopologyCommand(analyzer TopologyAnalyzer) *cobra.Command {
	options := topologyOptions{format: formatText, profile: defaultScalingProfile}

	command := &cobra.Command{
		Use:   "topology [path]",
		Short: "Analyze local repository topology.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateReportFormat(options.format); err != nil {
				return err
			}
			profile, err := basicScalingProfile(options.profile)
			if err != nil {
				return err
			}

			report, analyzeErr := analyzeTopology(
				cmd.Context(),
				analyzer,
				analysis.Request{Root: commandTarget(args), Profile: profile},
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
	command.Flags().StringVar(
		&options.profile,
		"profile",
		defaultScalingProfile,
		"scaling profile: small or monorepo",
	)
	return command
}

func analyzeTopology(
	ctx context.Context,
	analyzer TopologyAnalyzer,
	request analysis.Request,
) (analysis.TopologyReport, error) {
	if err := ctx.Err(); err != nil {
		return topologyReportForProfile(analysis.NewTopologyCanceledReport(), request.Profile), err
	}
	if analyzer == nil {
		return topologyReportForProfile(
			analysis.NewTopologyUnavailableReport(),
			request.Profile,
		), errTopologyUnavailable
	}

	model, err := analyzer.Analyze(ctx, request)
	if contextErr := ctx.Err(); contextErr != nil {
		return topologyReportForProfile(
			analysis.NewTopologyCanceledReport(),
			request.Profile,
		), contextErr
	}
	if errors.Is(err, context.Canceled) {
		return topologyReportForProfile(analysis.NewTopologyCanceledReport(), request.Profile), err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return topologyReportForProfile(
			analysis.NewTopologyAnalysisFailedReport(),
			request.Profile,
		), err
	}
	if errors.Is(err, analysis.ErrInvalidTarget) {
		return topologyReportForProfile(
			analysis.NewInvalidTopologyTargetReport(),
			request.Profile,
		), err
	}
	if err != nil {
		return topologyReportForProfile(
			analysis.NewTopologyAnalysisFailedReport(),
			request.Profile,
		), err
	}

	report, reportErr := analysis.NewTopologyReport(model)
	if reportErr != nil {
		return topologyReportForProfile(
			analysis.NewTopologyAnalysisFailedReport(),
			request.Profile,
		), reportErr
	}
	report = topologyReportForProfile(report, request.Profile)
	if err := report.Validate(); err != nil {
		return topologyReportForProfile(
			analysis.NewTopologyAnalysisFailedReport(),
			request.Profile,
		), err
	}
	return report, nil
}

func topologyReportForProfile(
	report analysis.TopologyReport,
	profile analysis.ScalingProfileName,
) analysis.TopologyReport {
	report.Profile = profile
	return report
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
