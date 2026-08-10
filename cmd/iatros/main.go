// Command iatros provides the local IATROS command-line interface.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kVinsom/Iatros/internal/analysis"
	"github.com/kVinsom/Iatros/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	services, err := buildServices()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: could not initialize IATROS: %v\n", err)
		return cli.ExitFailure
	}
	return cli.Run(
		ctx,
		version,
		services,
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	)
}

func buildServices() (cli.Services, error) {
	profiles := []analysis.ScalingProfile{
		analysis.SmallScalingProfile(),
		analysis.MonorepoScalingProfile(),
	}
	localAnalyzer, err := analysis.NewProfiledLocalAnalyzer(profiles...)
	if err != nil {
		return cli.Services{}, fmt.Errorf("create local analyzer: %w", err)
	}
	localTopologyAnalyzer, err := analysis.NewProfiledLocalTopologyAnalyzer(profiles...)
	if err != nil {
		return cli.Services{}, fmt.Errorf("create local topology analyzer: %w", err)
	}
	return cli.Services{
		Analysis: localAnalyzer,
		Topology: localTopologyAnalyzer,
	}, nil
}
