package changeimpact

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kVinsom/Iatros/internal/systemmap"
)

func TestAnalyzerMapsDirectAndDependentImpact(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	changes := ChangeSet{ID: "change-42", Changes: []Change{
		{ID: "infra", RepositoryID: "infra-repo", Kind: ChangeModified, Path: "infra/prod/main.tf"},
		{ID: "library", RepositoryID: "app-repo", Kind: ChangeModified, Path: "libs/shared/client.go"},
		{ID: "unmapped", RepositoryID: "app-repo", Kind: ChangeAdded, Path: "docs/design.md"},
	}}

	result, err := analyzer.Analyze(t.Context(), testSystem(), changes)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Partial {
		t.Fatal("Analyze() returned a partial result")
	}
	if got := serviceIDs(result.Services); !slices.Equal(got, []systemmap.ServiceID{"api", "web"}) {
		t.Fatalf("service ids = %v, want api and web", got)
	}
	if result.Services[0].Kind != ImpactDependent || result.Services[1].Kind != ImpactDependent {
		t.Fatalf("service impact kinds = %q, %q, want dependent", result.Services[0].Kind, result.Services[1].Kind)
	}
	if got := configurationIDs(result.Configurations); !slices.Equal(
		got,
		[]systemmap.InfrastructureID{"prod-cluster"},
	) {
		t.Fatalf("configuration ids = %v, want prod-cluster", got)
	}
	if result.Configurations[0].Kind != ImpactDirect {
		t.Fatalf("configuration kind = %q, want direct", result.Configurations[0].Kind)
	}
	if got := environmentIDs(result.Environments); !slices.Equal(
		got,
		[]systemmap.EnvironmentID{"production", "staging"},
	) {
		t.Fatalf("environment ids = %v, want production and staging", got)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != diagnosticChangeUnmapped {
		t.Fatalf("diagnostics = %#v, want one unmapped-change diagnostic", result.Diagnostics)
	}

	webImpact := result.Services[1]
	if !hasRelationshipChain(webImpact.Causes, []systemmap.RelationshipID{
		"api-depends-shared", "web-calls-api",
	}) {
		t.Fatalf("web causes = %#v, want dependency chain", webImpact.Causes)
	}
}

func TestAnalyzerMarksDirectServiceAndEnvironmentImpact(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	changes := ChangeSet{ID: "direct-change", Changes: []Change{
		{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/main.go"},
		{
			ID: "environment", RepositoryID: "infra-repo", Kind: ChangeModified,
			Path: "environments/production.yaml",
		},
	}}

	result, err := analyzer.Analyze(t.Context(), testSystem(), changes)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Services[0].ServiceID != "api" || result.Services[0].Kind != ImpactDirect {
		t.Fatalf("api impact = %#v, want direct", result.Services[0])
	}
	if result.Environments[0].EnvironmentID != "production" ||
		result.Environments[0].Kind != ImpactDirect {
		t.Fatalf("production impact = %#v, want direct", result.Environments[0])
	}
	if result.Services[1].ServiceID != "web" || result.Services[1].Kind != ImpactDependent {
		t.Fatalf("web impact = %#v, want dependent", result.Services[1])
	}
}

func TestAnalyzerMapsBothSidesOfRename(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	changes := ChangeSet{ID: "rename", Changes: []Change{{
		ID: "move", RepositoryID: "app-repo", Kind: ChangeRenamed,
		PreviousPath: "services/api/main.go", Path: "services/web/main.go",
	}}}

	result, err := analyzer.Analyze(t.Context(), testSystem(), changes)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(result.Services) != 2 || result.Services[0].Kind != ImpactDirect ||
		result.Services[1].Kind != ImpactDirect {
		t.Fatalf("service impacts = %#v, want two direct impacts", result.Services)
	}
	if !hasCausePath(result.Services[0].Causes, "services/api/main.go") ||
		!hasCausePath(result.Services[1].Causes, "services/web/main.go") {
		t.Fatalf("service causes = %#v, want old and new rename paths", result.Services)
	}
}

func TestAnalyzerDoesNotPropagateUnknownOrOwnershipRelationships(t *testing.T) {
	t.Parallel()

	system := testSystem()
	system.Relationships = append(system.Relationships,
		systemmap.Relationship{
			ID: "owner-owns-api", Source: systemmap.EntityReference{Kind: systemmap.EntityOwner, ID: "team"},
			Target: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "api"}, Kind: "owns",
			Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceOwnership, Path: "CODEOWNERS"}},
		},
		systemmap.Relationship{
			ID: "web-related-cluster", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "web"},
			Target: systemmap.EntityReference{Kind: systemmap.EntityInfrastructure, ID: "prod-cluster"}, Kind: "related_to",
			Evidence: []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/prod/main.tf"}},
		},
	)
	system = system.Normalized()
	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	result, err := analyzer.Analyze(t.Context(), system, ChangeSet{ID: "infra", Changes: []Change{{
		ID: "terraform", RepositoryID: "infra-repo", Kind: ChangeModified, Path: "infra/prod/main.tf",
	}}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(result.Services) != 2 || result.Services[0].ServiceID != "api" ||
		result.Services[1].ServiceID != "web" {
		t.Fatalf("services = %#v, unsupported edges changed propagation", result.Services)
	}
}

func TestAnalyzerReportsPartialSystemMap(t *testing.T) {
	t.Parallel()

	system := testSystem()
	system.Partial = true
	system.Diagnostics = []systemmap.Diagnostic{{
		Code: "SOURCE_PARTIAL", Level: systemmap.DiagnosticWarning,
		RepositoryID: "app-repo", Path: ".", Message: "A source was omitted.",
	}}
	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	result, err := analyzer.Analyze(t.Context(), system, ChangeSet{ID: "empty", Changes: []Change{}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !result.Partial || len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != diagnosticSystemMapPartial {
		t.Fatalf("result = %#v, want partial system-map diagnostic", result)
	}
}

func TestAnalyzerHonorsCancellation(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := analyzer.Analyze(ctx, testSystem(), ChangeSet{ID: "cancelled", Changes: []Change{}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze() error = %v, want context.Canceled", err)
	}
	if result.ChangeSetID != "cancelled" || result.Services == nil {
		t.Fatalf("result = %#v, want normalized empty model", result)
	}
}

func BenchmarkAnalyzer(b *testing.B) {
	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		b.Fatalf("NewAnalyzer() error = %v", err)
	}
	changes := ChangeSet{ID: "benchmark", Changes: make([]Change, 100)}
	for index := range changes.Changes {
		changes.Changes[index] = Change{
			ID: ChangeID(fmt.Sprintf("change-%03d", index)), RepositoryID: "app-repo",
			Kind: ChangeModified, Path: fmt.Sprintf("libs/shared/file-%03d.go", index),
		}
	}
	system := testSystem()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := analyzer.Analyze(context.Background(), system, changes); err != nil {
			b.Fatalf("Analyze() error = %v", err)
		}
	}
}

func serviceIDs(impacts []ServiceImpact) []systemmap.ServiceID {
	ids := make([]systemmap.ServiceID, 0, len(impacts))
	for _, impact := range impacts {
		ids = append(ids, impact.ServiceID)
	}
	return ids
}

func environmentIDs(impacts []EnvironmentImpact) []systemmap.EnvironmentID {
	ids := make([]systemmap.EnvironmentID, 0, len(impacts))
	for _, impact := range impacts {
		ids = append(ids, impact.EnvironmentID)
	}
	return ids
}

func configurationIDs(impacts []ConfigurationImpact) []systemmap.InfrastructureID {
	ids := make([]systemmap.InfrastructureID, 0, len(impacts))
	for _, impact := range impacts {
		ids = append(ids, impact.ConfigurationID)
	}
	return ids
}

func hasRelationshipChain(causes []Cause, chain []systemmap.RelationshipID) bool {
	for _, cause := range causes {
		if slices.Equal(cause.RelationshipIDs, chain) {
			return true
		}
	}
	return false
}

func hasCausePath(causes []Cause, changedPath string) bool {
	for _, cause := range causes {
		if cause.Path == changedPath {
			return true
		}
	}
	return false
}

func testSystem() systemmap.Model {
	return systemmap.Model{
		SchemaVersion: systemmap.CurrentSchemaVersion,
		ID:            "checkout",
		Name:          "Checkout",
		Repositories: []systemmap.Repository{
			{
				ID: "app-repo", Name: "Application", Source: systemmap.RepositorySourceLocal, Locator: ".",
				Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceTarget, Path: "."}},
			},
			{
				ID: "infra-repo", Name: "Infrastructure", Source: systemmap.RepositorySourceLocal, Locator: "infra-repo",
				Evidence: []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceTarget, Path: "."}},
			},
		},
		Services: []systemmap.Service{
			{
				ID: "api", RepositoryID: "app-repo", Name: "API", Kind: "api", Root: "services/api",
				EnvironmentIDs: []systemmap.EnvironmentID{"production"},
				Evidence:       []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceSource, Path: "services/api/main.go"}},
			},
			{
				ID: "web", RepositoryID: "app-repo", Name: "Web", Kind: "web", Root: "services/web",
				EnvironmentIDs: []systemmap.EnvironmentID{"staging"},
				Evidence:       []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceSource, Path: "services/web/main.go"}},
			},
		},
		Libraries: []systemmap.Library{{
			ID: "shared", RepositoryID: "app-repo", Name: "Shared", Ecosystem: "go", Root: "libs/shared",
			Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceManifest, Path: "go.mod"}},
		}},
		Infrastructure: []systemmap.Infrastructure{{
			ID: "prod-cluster", RepositoryID: "infra-repo", Name: "Production Cluster",
			Kind: "cluster", Technology: "terraform", Root: "infra/prod",
			EnvironmentIDs: []systemmap.EnvironmentID{"production"},
			Evidence:       []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/prod/main.tf"}},
		}},
		Environments: []systemmap.Environment{
			{
				ID: "production", Name: "Production", Kind: "production",
				Evidence: []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "environments/production.yaml"}},
			},
			{
				ID: "staging", Name: "Staging", Kind: "staging",
				Evidence: []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "environments/staging.yaml"}},
			},
		},
		Owners: []systemmap.Owner{{
			ID: "team", Kind: systemmap.OwnerTeam, Name: "Checkout Team",
			Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceOwnership, Path: "CODEOWNERS"}},
		}},
		ExternalResources: []systemmap.ExternalResource{},
		Relationships: []systemmap.Relationship{
			{
				ID: "api-depends-shared", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "api"},
				Target: systemmap.EntityReference{Kind: systemmap.EntityLibrary, ID: "shared"}, Kind: "depends_on",
				Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceManifest, Path: "go.mod"}},
			},
			{
				ID: "api-runs-cluster", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "api"},
				Target: systemmap.EntityReference{Kind: systemmap.EntityInfrastructure, ID: "prod-cluster"}, Kind: "runs_on",
				EnvironmentIDs: []systemmap.EnvironmentID{"production"},
				Evidence:       []systemmap.Evidence{{RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/prod/main.tf"}},
			},
			{
				ID: "web-calls-api", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "web"},
				Target: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "api"}, Kind: "calls",
				Evidence: []systemmap.Evidence{{RepositoryID: "app-repo", Kind: systemmap.EvidenceSource, Path: "services/web/client.go"}},
			},
		},
		Diagnostics: []systemmap.Diagnostic{},
	}.Normalized()
}
