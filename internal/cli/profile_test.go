package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
	"github.com/kVinsom/Iatros/internal/topology"
)

func TestBasicScalingProfileAcceptsReleasedBasicProfiles(t *testing.T) {
	t.Parallel()

	for _, want := range []analysis.ScalingProfileName{
		analysis.ScalingProfileSmall,
		analysis.ScalingProfileMonorepo,
	} {
		want := want
		t.Run(string(want), func(t *testing.T) {
			t.Parallel()

			got, err := basicScalingProfile(string(want))
			if err != nil {
				t.Fatalf("basicScalingProfile() error = %v", err)
			}
			if got != want {
				t.Fatalf("basicScalingProfile() = %q, want %q", got, want)
			}
		})
	}
}

func TestBasicScalingProfileRejectsUnavailableProfiles(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "large", "enterprise", "MONOREPO"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := basicScalingProfile(value); err == nil {
				t.Fatalf("basicScalingProfile(%q) succeeded", value)
			}
		})
	}
}

func TestAnalyzeSelectsMonorepoProfile(t *testing.T) {
	t.Parallel()

	result := runCLI(
		t,
		analysis.NewLocalStub(),
		"analyze",
		"--profile",
		"monorepo",
		"--format",
		"json",
		t.TempDir(),
	)
	if result.exitCode != ExitNotImplemented || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q", result.exitCode, result.stderr)
	}

	var report analysis.Report
	if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if report.Profile != analysis.ScalingProfileMonorepo {
		t.Fatalf("Profile = %q, want %q", report.Profile, analysis.ScalingProfileMonorepo)
	}
}

func TestTopologySelectsMonorepoProfile(t *testing.T) {
	t.Parallel()

	analyzer, err := analysis.NewProfiledLocalTopologyAnalyzer(
		analysis.SmallScalingProfile(),
		analysis.MonorepoScalingProfile(),
	)
	if err != nil {
		t.Fatal(err)
	}
	result := runTopologyCLI(
		t,
		analyzer,
		"topology",
		"--profile=monorepo",
		"--format=json",
		t.TempDir(),
	)
	if result.exitCode != ExitSuccess || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q", result.exitCode, result.stderr)
	}

	var report analysis.TopologyReport
	if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if report.Profile != analysis.ScalingProfileMonorepo {
		t.Fatalf("Profile = %q, want %q", report.Profile, analysis.ScalingProfileMonorepo)
	}
}

func TestCLIRejectsEnterpriseProfileBeforeCallingServices(t *testing.T) {
	t.Parallel()

	analysisCalled := false
	analysisService := analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
		analysisCalled = true
		return analysis.Report{}, nil
	})
	analysisResult := runCLI(t, analysisService, "analyze", "--profile=enterprise")
	if analysisResult.exitCode != ExitUsage || analysisCalled {
		t.Fatalf("analysis = exit %d, called %t", analysisResult.exitCode, analysisCalled)
	}

	topologyCalled := false
	topologyService := topologyAnalyzerFunc(func(context.Context, analysis.Request) (topology.Model, error) {
		topologyCalled = true
		return topology.Model{}, nil
	})
	topologyResult := runTopologyCLI(t, topologyService, "topology", "--profile=enterprise")
	if topologyResult.exitCode != ExitUsage || topologyCalled {
		t.Fatalf("topology = exit %d, called %t", topologyResult.exitCode, topologyCalled)
	}
}
