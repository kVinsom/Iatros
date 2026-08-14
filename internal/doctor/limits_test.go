package doctor

import "testing"

func TestLimitProfilesIncreaseMonotonically(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeSystemLimits(), EnterpriseLimits()}
	for index, profile := range profiles {
		if err := profile.Validate(); err != nil {
			t.Fatalf("profile %d Validate() error = %v", index, err)
		}
		if index == 0 {
			continue
		}
		previous := profiles[index-1]
		if profile.MaxRules <= previous.MaxRules || profile.MaxInputFacts <= previous.MaxInputFacts ||
			profile.MaxRepositoryFindings <= previous.MaxRepositoryFindings ||
			profile.MaxDiagnostics <= previous.MaxDiagnostics || profile.Timeout <= previous.Timeout {
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
		{name: "rules", mutate: func(limits *Limits) { limits.MaxRules = 0 }},
		{name: "input facts", mutate: func(limits *Limits) { limits.MaxInputFacts = 0 }},
		{name: "repository findings", mutate: func(limits *Limits) { limits.MaxRepositoryFindings = 0 }},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = minimumTextBytes - 1 }},
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
