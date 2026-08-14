package changeimpact

import (
	"errors"
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
		if current.MaxChanges <= previous.MaxChanges || current.MaxServices <= previous.MaxServices ||
			current.MaxEnvironments <= previous.MaxEnvironments ||
			current.MaxConfigurations <= previous.MaxConfigurations ||
			current.MaxDirectMatches <= previous.MaxDirectMatches ||
			current.MaxTraversalDepth <= previous.MaxTraversalDepth ||
			current.MaxRelationshipTraversals <= previous.MaxRelationshipTraversals ||
			current.MaxCausesPerImpact <= previous.MaxCausesPerImpact ||
			current.MaxRelationshipsPerCause <= previous.MaxRelationshipsPerCause ||
			current.MaxDiagnostics <= previous.MaxDiagnostics ||
			current.MaxTextBytes <= previous.MaxTextBytes || current.Timeout <= previous.Timeout {
			t.Fatalf("profile %d does not exceed profile %d", index, index-1)
		}
	}
}

func TestLimitsValidateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "changes", mutate: func(limits *Limits) { limits.MaxChanges = 0 }},
		{name: "services", mutate: func(limits *Limits) { limits.MaxServices = 0 }},
		{name: "environments", mutate: func(limits *Limits) { limits.MaxEnvironments = 0 }},
		{name: "configurations", mutate: func(limits *Limits) { limits.MaxConfigurations = 0 }},
		{name: "direct matches", mutate: func(limits *Limits) { limits.MaxDirectMatches = 0 }},
		{name: "depth", mutate: func(limits *Limits) { limits.MaxTraversalDepth = 0 }},
		{name: "traversals", mutate: func(limits *Limits) { limits.MaxRelationshipTraversals = 0 }},
		{name: "causes", mutate: func(limits *Limits) { limits.MaxCausesPerImpact = 0 }},
		{name: "relationships per cause", mutate: func(limits *Limits) { limits.MaxRelationshipsPerCause = 0 }},
		{
			name:   "relationships exceed depth",
			mutate: func(limits *Limits) { limits.MaxRelationshipsPerCause = limits.MaxTraversalDepth + 1 },
		},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "limiting diagnostics", mutate: func(limits *Limits) {
			limits.MaxDiagnostics = minimumLimitingDiagnostics - 1
		}},
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

func TestAnalyzerReportsConfiguredLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*Limits)
		changes    ChangeSet
		diagnostic string
	}{
		{
			name:   "direct matches",
			mutate: func(limits *Limits) { limits.MaxDirectMatches = 1 },
			changes: ChangeSet{ID: "direct", Changes: []Change{
				{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/a.go"},
				{ID: "web", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/web/a.go"},
			}},
			diagnostic: diagnosticDirectMatchLimit,
		},
		{
			name:   "traversal work",
			mutate: func(limits *Limits) { limits.MaxRelationshipTraversals = 1 },
			changes: ChangeSet{ID: "work", Changes: []Change{{
				ID: "shared", RepositoryID: "app-repo", Kind: ChangeModified, Path: "libs/shared/client.go",
			}}},
			diagnostic: diagnosticTraversalLimit,
		},
		{
			name: "depth",
			mutate: func(limits *Limits) {
				limits.MaxTraversalDepth = 1
				limits.MaxRelationshipsPerCause = 1
			},
			changes: ChangeSet{ID: "depth", Changes: []Change{{
				ID: "shared", RepositoryID: "app-repo", Kind: ChangeModified, Path: "libs/shared/client.go",
			}}},
			diagnostic: diagnosticDepthLimit,
		},
		{
			name:   "causes",
			mutate: func(limits *Limits) { limits.MaxCausesPerImpact = 1 },
			changes: ChangeSet{ID: "causes", Changes: []Change{
				{ID: "api-a", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/a.go"},
				{ID: "api-b", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/b.go"},
			}},
			diagnostic: diagnosticCauseLimit,
		},
		{
			name:   "services",
			mutate: func(limits *Limits) { limits.MaxServices = 1 },
			changes: ChangeSet{ID: "services", Changes: []Change{
				{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/a.go"},
				{ID: "web", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/web/a.go"},
			}},
			diagnostic: diagnosticServiceLimit,
		},
		{
			name:   "environments",
			mutate: func(limits *Limits) { limits.MaxEnvironments = 1 },
			changes: ChangeSet{ID: "environments", Changes: []Change{
				{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/a.go"},
				{ID: "web", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/web/a.go"},
			}},
			diagnostic: diagnosticEnvironmentLimit,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			test.mutate(&limits)
			analyzer, err := NewAnalyzer(limits)
			if err != nil {
				t.Fatalf("NewAnalyzer() error = %v", err)
			}
			result, err := analyzer.Analyze(t.Context(), testSystem(), test.changes)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if !result.Partial || !hasDiagnostic(result.Diagnostics, test.diagnostic) {
				t.Fatalf("result = %#v, want partial %s diagnostic", result, test.diagnostic)
			}
		})
	}
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
