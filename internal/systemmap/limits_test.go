package systemmap

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
		if current.MaxRepositories < previous.MaxRepositories ||
			current.MaxServices < previous.MaxServices ||
			current.MaxLibraries < previous.MaxLibraries ||
			current.MaxInfrastructure < previous.MaxInfrastructure ||
			current.MaxEnvironments < previous.MaxEnvironments ||
			current.MaxOwners < previous.MaxOwners ||
			current.MaxExternalResources < previous.MaxExternalResources ||
			current.MaxRelationships < previous.MaxRelationships ||
			current.MaxEnvironmentsPerEntity < previous.MaxEnvironmentsPerEntity ||
			current.MaxEvidencePerFact < previous.MaxEvidencePerFact ||
			current.MaxDiagnostics < previous.MaxDiagnostics ||
			current.MaxTextBytes < previous.MaxTextBytes || current.Timeout < previous.Timeout {
			t.Fatalf("profile %d is smaller than profile %d", index, index-1)
		}
	}
}

func TestLimitsValidateRejectsNonPositiveFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "repositories", mutate: func(limits *Limits) { limits.MaxRepositories = 0 }},
		{name: "services", mutate: func(limits *Limits) { limits.MaxServices = 0 }},
		{name: "libraries", mutate: func(limits *Limits) { limits.MaxLibraries = 0 }},
		{name: "infrastructure", mutate: func(limits *Limits) { limits.MaxInfrastructure = 0 }},
		{name: "environments", mutate: func(limits *Limits) { limits.MaxEnvironments = 0 }},
		{name: "owners", mutate: func(limits *Limits) { limits.MaxOwners = 0 }},
		{name: "external resources", mutate: func(limits *Limits) { limits.MaxExternalResources = 0 }},
		{name: "relationships", mutate: func(limits *Limits) { limits.MaxRelationships = 0 }},
		{name: "entity environments", mutate: func(limits *Limits) { limits.MaxEnvironmentsPerEntity = 0 }},
		{name: "evidence", mutate: func(limits *Limits) { limits.MaxEvidencePerFact = 0 }},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 * time.Second }},
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

func TestModelValidateWithinRejectsEveryBoundedDimension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Model, *Limits)
	}{
		{name: "repositories", mutate: func(_ *Model, limits *Limits) { limits.MaxRepositories = 1 }},
		{
			name: "services",
			mutate: func(model *Model, limits *Limits) {
				model.Services = append(model.Services, model.Services[0])
				limits.MaxServices = 1
			},
		},
		{
			name: "libraries",
			mutate: func(model *Model, limits *Limits) {
				model.Libraries = append(model.Libraries, model.Libraries[0])
				limits.MaxLibraries = 1
			},
		},
		{
			name: "infrastructure",
			mutate: func(model *Model, limits *Limits) {
				model.Infrastructure = append(model.Infrastructure, model.Infrastructure[0])
				limits.MaxInfrastructure = 1
			},
		},
		{
			name: "environments",
			mutate: func(model *Model, limits *Limits) {
				model.Environments = append(model.Environments, model.Environments[0])
				limits.MaxEnvironments = 1
			},
		},
		{
			name: "owners",
			mutate: func(model *Model, limits *Limits) {
				model.Owners = append(model.Owners, model.Owners[0])
				limits.MaxOwners = 1
			},
		},
		{
			name: "external resources",
			mutate: func(model *Model, limits *Limits) {
				model.ExternalResources = append(model.ExternalResources, model.ExternalResources[0])
				limits.MaxExternalResources = 1
			},
		},
		{name: "relationships", mutate: func(_ *Model, limits *Limits) { limits.MaxRelationships = 2 }},
		{
			name: "environments per entity",
			mutate: func(model *Model, limits *Limits) {
				model.Services[0].EnvironmentIDs = []EnvironmentID{"production", "staging"}
				model.Environments = append(model.Environments, Environment{
					ID: "staging", Name: "Staging", Kind: "staging",
					Evidence: []Evidence{{
						RepositoryID: "api-repo", Kind: EvidenceConfiguration, Path: "deploy/staging.yaml",
					}},
				})
				*model = model.Normalized()
				limits.MaxEnvironmentsPerEntity = 1
			},
		},
		{
			name: "evidence per fact",
			mutate: func(model *Model, limits *Limits) {
				model.Services[0].Evidence = append(
					model.Services[0].Evidence,
					model.Services[0].Evidence[0],
				)
				limits.MaxEvidencePerFact = 1
			},
		},
		{
			name: "diagnostics",
			mutate: func(model *Model, limits *Limits) {
				model.Diagnostics = []Diagnostic{
					{Code: "NOTE_A", Level: DiagnosticInfo, Path: ".", Message: "First note."},
					{Code: "NOTE_B", Level: DiagnosticInfo, Path: ".", Message: "Second note."},
				}
				limits.MaxDiagnostics = 1
			},
		},
		{
			name: "text bytes",
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
