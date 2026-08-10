package codeanalysis

import (
	"errors"
	"testing"
)

func TestBuiltInLimitsAreValidAndMonotonic(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeRepositoryLimits(), EnterpriseLimits()}
	for _, limits := range profiles {
		if err := limits.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	}
	for index := 1; index < len(profiles); index++ {
		smaller := profiles[index-1]
		larger := profiles[index]
		if larger.MaxFiles <= smaller.MaxFiles ||
			larger.MaxFileBytes <= smaller.MaxFileBytes ||
			larger.MaxTotalBytes <= smaller.MaxTotalBytes ||
			larger.MaxSyntaxNodesPerFile <= smaller.MaxSyntaxNodesPerFile ||
			larger.MaxServices <= smaller.MaxServices ||
			larger.MaxAPIEndpoints <= smaller.MaxAPIEndpoints ||
			larger.MaxEvidencePerFact <= smaller.MaxEvidencePerFact ||
			larger.Timeout <= smaller.Timeout {
			t.Fatalf("larger limits do not exceed the preceding profile: %+v", larger)
		}
	}
}

func TestLimitsValidateRejectsInvalidBudgets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "files", mutate: func(limits *Limits) { limits.MaxFiles = 0 }},
		{name: "file bytes", mutate: func(limits *Limits) { limits.MaxFileBytes = 0 }},
		{name: "total bytes", mutate: func(limits *Limits) {
			limits.MaxTotalBytes = limits.MaxFileBytes - 1
		}},
		{name: "line bytes", mutate: func(limits *Limits) {
			limits.MaxLineBytes = int(limits.MaxFileBytes) + 1
		}},
		{name: "nesting", mutate: func(limits *Limits) { limits.MaxNestingDepth = 0 }},
		{name: "syntax nodes", mutate: func(limits *Limits) { limits.MaxSyntaxNodesPerFile = 0 }},
		{name: "services", mutate: func(limits *Limits) { limits.MaxServices = 0 }},
		{name: "frameworks", mutate: func(limits *Limits) { limits.MaxFrameworks = 0 }},
		{name: "ports", mutate: func(limits *Limits) { limits.MaxPortBindings = 0 }},
		{name: "endpoints", mutate: func(limits *Limits) { limits.MaxAPIEndpoints = 0 }},
		{name: "environment variables", mutate: func(limits *Limits) {
			limits.MaxEnvironmentVariables = 0
		}},
		{name: "resources", mutate: func(limits *Limits) { limits.MaxResourceDependencies = 0 }},
		{name: "evidence", mutate: func(limits *Limits) { limits.MaxEvidencePerFact = 0 }},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			testCase.mutate(&limits)
			if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
			}
		})
	}
}
