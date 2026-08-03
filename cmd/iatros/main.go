// Command iatros provides the local IATROS command-line interface.
package main

import (
	"context"
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

	services := cli.Services{}
	localAnalyzer, err := analysis.NewLocalAnalyzer()
	if err == nil {
		services.Analysis = localAnalyzer
	}
	localTopologyAnalyzer, err := analysis.NewLocalTopologyAnalyzer()
	if err == nil {
		services.Topology = localTopologyAnalyzer
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
