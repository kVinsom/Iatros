package artifactvalidation

import "testing"

func TestLimitProfilesIncreaseMonotonically(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeRepositoryLimits(), EnterpriseLimits()}
	for index, profile := range profiles {
		if err := profile.Validate(); err != nil {
			t.Fatalf("profile %d Validate() error = %v", index, err)
		}
		if index == 0 {
			continue
		}
		previous := profiles[index-1]
		if profile.MaxArtifacts <= previous.MaxArtifacts ||
			profile.MaxArtifactBytes <= previous.MaxArtifactBytes ||
			profile.MaxTotalBytes <= previous.MaxTotalBytes ||
			profile.MaxSyntaxNodes <= previous.MaxSyntaxNodes ||
			profile.Timeout <= previous.Timeout {
			t.Fatalf("profile %d does not increase material budgets", index)
		}
	}
}

func TestLimitsRejectInvalidBudgets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "artifacts", mutate: func(limits *Limits) { limits.MaxArtifacts = 0 }},
		{name: "artifact bytes", mutate: func(limits *Limits) { limits.MaxArtifactBytes = 0 }},
		{name: "total bytes", mutate: func(limits *Limits) { limits.MaxTotalBytes = 1 }},
		{name: "validators", mutate: func(limits *Limits) { limits.MaxValidators = 0 }},
		{name: "per artifact diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnosticsPerArtifact = 0 }},
		{name: "global diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 1 }},
		{name: "syntax nodes", mutate: func(limits *Limits) { limits.MaxSyntaxNodes = 0 }},
		{name: "depth", mutate: func(limits *Limits) { limits.MaxNestingDepth = 0 }},
		{name: "aliases", mutate: func(limits *Limits) { limits.MaxAliases = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limits := DefaultLimits()
			test.mutate(&limits)
			if err := limits.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid limits")
			}
		})
	}
}
