package remoteanalysis

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuiltInLimitsAreValidAndMonotonic(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeSystemLimits(), EnterpriseLimits()}
	for index, limits := range profiles {
		if err := limits.Validate(); err != nil {
			t.Fatalf("profile %d Validate() error = %v", index, err)
		}
	}
	for index := 1; index < len(profiles); index++ {
		previous := profiles[index-1]
		current := profiles[index]
		if current.MaxRepositories <= previous.MaxRepositories ||
			current.MaxRelationships <= previous.MaxRelationships ||
			current.MaxProviderPages <= previous.MaxProviderPages ||
			current.MaxProviderRequests <= previous.MaxProviderRequests ||
			current.MaxConcurrentRequests <= previous.MaxConcurrentRequests ||
			current.MaxResponseBytes <= previous.MaxResponseBytes ||
			current.MaxTotalResponseBytes <= previous.MaxTotalResponseBytes ||
			current.MaxEvidencePerFact <= previous.MaxEvidencePerFact ||
			current.MaxDiagnostics <= previous.MaxDiagnostics ||
			current.MaxTextBytes <= previous.MaxTextBytes || current.Timeout <= previous.Timeout {
			t.Fatalf("profile %d does not exceed profile %d", index, index-1)
		}
	}
}

func TestLimitsValidateRejectsInvalidFieldsAndRelationships(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "repositories", mutate: func(limits *Limits) { limits.MaxRepositories = 0 }},
		{name: "relationships", mutate: func(limits *Limits) { limits.MaxRelationships = 0 }},
		{name: "pages", mutate: func(limits *Limits) { limits.MaxProviderPages = 0 }},
		{name: "page size", mutate: func(limits *Limits) { limits.MaxRepositoriesPerPage = 0 }},
		{name: "requests", mutate: func(limits *Limits) { limits.MaxProviderRequests = 0 }},
		{name: "concurrency", mutate: func(limits *Limits) { limits.MaxConcurrentRequests = 0 }},
		{
			name: "concurrency exceeds requests",
			mutate: func(limits *Limits) {
				limits.MaxConcurrentRequests = limits.MaxProviderRequests + 1
			},
		},
		{name: "response bytes", mutate: func(limits *Limits) { limits.MaxResponseBytes = 0 }},
		{
			name: "total response bytes",
			mutate: func(limits *Limits) {
				limits.MaxTotalResponseBytes = limits.MaxResponseBytes - 1
			},
		},
		{name: "evidence", mutate: func(limits *Limits) { limits.MaxEvidencePerFact = 0 }},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 * time.Second }},
		{
			name: "insufficient pages",
			mutate: func(limits *Limits) {
				limits.MaxRepositories = 101
				limits.MaxProviderPages = 1
			},
		},
		{
			name: "insufficient requests",
			mutate: func(limits *Limits) {
				limits.MaxProviderRequests = limits.MaxProviderPages - 1
				limits.MaxConcurrentRequests = 1
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			test.mutate(&limits)
			if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
			}
		})
	}
}

func TestModelValidateWithinRejectsBoundedContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Model, *Limits)
	}{
		{name: "repositories", mutate: func(_ *Model, limits *Limits) { limits.MaxRepositories = 2 }},
		{name: "relationships", mutate: func(_ *Model, limits *Limits) { limits.MaxRelationships = 1 }},
		{
			name: "evidence",
			mutate: func(model *Model, limits *Limits) {
				model.Evidence = append(model.Evidence, model.Evidence[0])
				limits.MaxEvidencePerFact = 1
			},
		},
		{
			name: "diagnostics",
			mutate: func(model *Model, limits *Limits) {
				model.Diagnostics = []Diagnostic{
					{Code: "NOTE_A", Level: DiagnosticInfo, Scope: "system:a", Message: "First note."},
					{Code: "NOTE_B", Level: DiagnosticInfo, Scope: "system:b", Message: "Second note."},
				}
				limits.MaxDiagnostics = 1
			},
		},
		{
			name: "text",
			mutate: func(model *Model, limits *Limits) {
				model.Name = strings.Repeat("a", 9)
				limits.MaxTextBytes = 8
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			limits := DefaultLimits()
			test.mutate(&model, &limits)
			if err := model.ValidateWithin(limits); err == nil {
				t.Fatal("ValidateWithin() error = nil, want rejection")
			}
		})
	}
}
