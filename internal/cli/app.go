// Package cli implements the Cobra adapter for the IATROS command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kVinsom/Iatros/internal/analysis"

	"github.com/spf13/cobra"
)

const defaultVersion = "dev"

// Analyzer is the analysis behavior consumed by the CLI adapter.
type Analyzer interface {
	Analyze(context.Context, analysis.Request) (analysis.Report, error)
}

// Run executes a fresh command tree and maps its outcome to a process exit code.
func Run(
	ctx context.Context,
	version string,
	analyzer Analyzer,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	stdoutTracker := newWriteErrorTracker(stdout)
	stderrTracker := newWriteErrorTracker(stderr)
	root := newRootCommand(version, analyzer)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	root.SetOut(stdoutTracker)
	root.SetErr(stderrTracker)

	command, err := root.ExecuteContextC(ctx)
	if err == nil {
		if stdoutTracker.Err() != nil || stderrTracker.Err() != nil {
			_, _ = fmt.Fprintln(stderrTracker, "Error: could not write command output")
			return ExitFailure
		}
		return ExitSuccess
	}

	var commandErr *exitError
	if errors.As(err, &commandErr) {
		if !commandErr.reportWritten {
			_, _ = fmt.Fprintf(stderrTracker, "Error: %s\n", commandErr.message)
			if stderrTracker.Err() != nil {
				return ExitFailure
			}
		}
		return commandErr.code
	}
	if stdoutTracker.Err() != nil || stderrTracker.Err() != nil {
		if stderrTracker.Err() == nil {
			_, _ = fmt.Fprintln(stderrTracker, "Error: could not write command output")
		}
		return ExitFailure
	}

	if command == nil {
		command = root
	}

	_, _ = fmt.Fprintf(stderrTracker, "Error: %s\n\n%s", err, command.UsageString())
	if stderrTracker.Err() != nil {
		return ExitFailure
	}
	return ExitUsage
}

func newRootCommand(version string, analyzer Analyzer) *cobra.Command {
	version = strings.TrimSpace(version)
	if version == "" {
		version = defaultVersion
	}

	root := &cobra.Command{
		Use:           "iatros",
		Short:         "Analyze and improve software delivery workflows.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.Version = version
	root.SetVersionTemplate("iatros {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		newAnalyzeCommand(analyzer),
		newVersionCommand(version),
	)

	return root
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the IATROS version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "iatros %s\n", version)
			if err != nil {
				return newExitError(ExitFailure, "could not write version output")
			}
			return nil
		},
	}
}
