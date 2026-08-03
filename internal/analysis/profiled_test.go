package analysis

import (
	"errors"
	"testing"
)

func TestProfiledLocalAnalyzerSelectsEveryEnabledProfile(t *testing.T) {
	t.Parallel()

	profiles := []ScalingProfile{
		SmallScalingProfile(),
		MonorepoScalingProfile(),
		EnterpriseScalingProfile(),
	}
	analyzer, err := NewProfiledLocalAnalyzer(profiles...)
	if err != nil {
		t.Fatalf("NewProfiledLocalAnalyzer() error = %v", err)
	}

	for _, profile := range profiles {
		profile := profile
		t.Run(string(profile.Name), func(t *testing.T) {
			t.Parallel()

			report, err := analyzer.Analyze(t.Context(), Request{
				Root:    t.TempDir(),
				Profile: profile.Name,
			})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if report.Profile != profile.Name {
				t.Fatalf("Profile = %q, want %q", report.Profile, profile.Name)
			}
			if err := report.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestProfiledLocalTopologyAnalyzerSelectsEveryEnabledProfile(t *testing.T) {
	t.Parallel()

	profiles := []ScalingProfile{
		SmallScalingProfile(),
		MonorepoScalingProfile(),
		EnterpriseScalingProfile(),
	}
	analyzer, err := NewProfiledLocalTopologyAnalyzer(profiles...)
	if err != nil {
		t.Fatalf("NewProfiledLocalTopologyAnalyzer() error = %v", err)
	}

	for _, profile := range profiles {
		profile := profile
		t.Run(string(profile.Name), func(t *testing.T) {
			t.Parallel()

			model, err := analyzer.Analyze(t.Context(), Request{
				Root:    t.TempDir(),
				Profile: profile.Name,
			})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if model.Projects == nil || model.Workspaces == nil || model.Dependencies == nil ||
				model.Issues == nil {
				t.Fatal("Analyze() returned nil topology collections")
			}
		})
	}
}

func TestProfiledLocalAnalyzerReportsDisabledEnterpriseProfile(t *testing.T) {
	t.Parallel()

	analyzer, err := NewProfiledLocalAnalyzer(
		SmallScalingProfile(),
		MonorepoScalingProfile(),
	)
	if err != nil {
		t.Fatalf("NewProfiledLocalAnalyzer() error = %v", err)
	}
	report, err := analyzer.Analyze(t.Context(), Request{
		Root:    t.TempDir(),
		Profile: ScalingProfileEnterprise,
	})
	if !errors.Is(err, ErrScalingProfileUnavailable) {
		t.Fatalf("Analyze() error = %v, want ErrScalingProfileUnavailable", err)
	}
	if report.Profile != ScalingProfileEnterprise || report.Status != StatusFailed ||
		len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != DiagnosticCodeScalingProfileUnavailable {
		t.Fatalf("report = %+v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProfiledConstructorsRejectEmptyAndDuplicateProfileSets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "empty analysis", run: func() error {
			_, err := NewProfiledLocalAnalyzer()
			return err
		}},
		{name: "duplicate analysis", run: func() error {
			profile := SmallScalingProfile()
			_, err := NewProfiledLocalAnalyzer(profile, profile)
			return err
		}},
		{name: "empty topology", run: func() error {
			_, err := NewProfiledLocalTopologyAnalyzer()
			return err
		}},
		{name: "duplicate topology", run: func() error {
			profile := SmallScalingProfile()
			_, err := NewProfiledLocalTopologyAnalyzer(profile, profile)
			return err
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.run(); !errors.Is(err, ErrInvalidScalingProfile) {
				t.Fatalf("constructor error = %v, want ErrInvalidScalingProfile", err)
			}
		})
	}
}

func TestConfiguredAnalyzersRejectMismatchedProfile(t *testing.T) {
	t.Parallel()

	localAnalyzer, err := NewLocalAnalyzerWithProfile(MonorepoScalingProfile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localAnalyzer.Analyze(t.Context(), Request{
		Root: t.TempDir(), Profile: ScalingProfileSmall,
	}); !errors.Is(err, ErrScalingProfileUnavailable) {
		t.Fatalf("local Analyze() error = %v, want ErrScalingProfileUnavailable", err)
	}

	topologyAnalyzer, err := NewLocalTopologyAnalyzerWithProfile(MonorepoScalingProfile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := topologyAnalyzer.Analyze(t.Context(), Request{
		Root: t.TempDir(), Profile: ScalingProfileSmall,
	}); !errors.Is(err, ErrScalingProfileUnavailable) {
		t.Fatalf("topology Analyze() error = %v, want ErrScalingProfileUnavailable", err)
	}
}
