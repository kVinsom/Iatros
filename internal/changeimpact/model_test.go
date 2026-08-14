package changeimpact

import (
	"errors"
	"slices"
	"testing"

	"github.com/kVinsom/Iatros/internal/systemmap"
)

func TestChangeSetNormalizedDetachesAndSortsInput(t *testing.T) {
	t.Parallel()

	input := ChangeSet{ID: "changes", Changes: []Change{
		{ID: "web", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/web/main.go"},
		{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/main.go"},
	}}
	normalized := input.Normalized()
	if normalized.Changes[0].ID != "api" || normalized.Changes[1].ID != "web" {
		t.Fatalf("Normalized() changes = %#v, want id ordering", normalized.Changes)
	}
	normalized.Changes[0].Path = "changed.go"
	if input.Changes[1].Path == "changed.go" {
		t.Fatal("Normalized() retained an alias to the input changes")
	}
}

func TestChangeSetValidateRejectsInvalidOperationsAndPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change Change
	}{
		{name: "unknown repository", change: Change{ID: "change", RepositoryID: "other", Kind: ChangeModified, Path: "main.go"}},
		{name: "unknown operation", change: Change{ID: "change", RepositoryID: "app-repo", Kind: "copied", Path: "main.go"}},
		{name: "absolute path", change: Change{ID: "change", RepositoryID: "app-repo", Kind: ChangeModified, Path: "/main.go"}},
		{name: "parent path", change: Change{ID: "change", RepositoryID: "app-repo", Kind: ChangeModified, Path: "../main.go"}},
		{name: "previous path on modification", change: Change{ID: "change", RepositoryID: "app-repo", Kind: ChangeModified, Path: "main.go", PreviousPath: "old.go"}},
		{name: "rename without previous path", change: Change{ID: "change", RepositoryID: "app-repo", Kind: ChangeRenamed, Path: "main.go"}},
		{name: "rename to same path", change: Change{ID: "change", RepositoryID: "app-repo", Kind: ChangeRenamed, Path: "main.go", PreviousPath: "main.go"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			changeSet := ChangeSet{ID: "changes", Changes: []Change{test.change}}
			if err := changeSet.Validate(testSystem()); !errors.Is(err, ErrInvalidChangeSet) {
				t.Fatalf("Validate() error = %v, want ErrInvalidChangeSet", err)
			}
		})
	}
}

func TestModelNormalizedDetachesNestedCausesAndRemovesDuplicates(t *testing.T) {
	t.Parallel()

	cause := Cause{
		ChangeID: "shared", Path: "libs/shared/client.go",
		RelationshipIDs: []systemmap.RelationshipID{"api-depends-shared"},
	}
	input := Model{
		SchemaVersion: CurrentSchemaVersion,
		ChangeSetID:   "changes",
		Services: []ServiceImpact{{
			ServiceID: "api", RepositoryID: "app-repo", Kind: ImpactDependent,
			Causes: []Cause{cause, cause},
		}},
	}
	normalized := input.Normalized()
	if len(normalized.Services[0].Causes) != 1 || normalized.Environments == nil ||
		normalized.Configurations == nil || normalized.Diagnostics == nil {
		t.Fatalf("Normalized() = %#v, want unique causes and non-nil collections", normalized)
	}
	normalized.Services[0].Causes[0].RelationshipIDs[0] = "changed"
	if input.Services[0].Causes[0].RelationshipIDs[0] == "changed" {
		t.Fatal("Normalized() retained a nested relationship slice alias")
	}
}

func TestModelValidateRejectsDisconnectedAndUnsupportedCauseChains(t *testing.T) {
	t.Parallel()

	changes := ChangeSet{ID: "shared-change", Changes: []Change{{
		ID: "shared", RepositoryID: "app-repo", Kind: ChangeModified, Path: "libs/shared/client.go",
	}}}.Normalized()
	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	model, err := analyzer.Analyze(t.Context(), testSystem(), changes)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	model.Services[1].Causes[0].RelationshipIDs = []systemmap.RelationshipID{"api-depends-shared"}
	model = model.Normalized()
	if err := model.Validate(testSystem(), changes); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("disconnected Validate() error = %v, want ErrInvalidModel", err)
	}

	system := testSystem()
	system.Relationships = append(system.Relationships, systemmap.Relationship{
		ID: "web-related-cluster", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "web"},
		Target: systemmap.EntityReference{Kind: systemmap.EntityInfrastructure, ID: "prod-cluster"}, Kind: "related_to",
		Evidence: []systemmap.Evidence{{
			RepositoryID: "infra-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/prod/main.tf",
		}},
	})
	system = system.Normalized()
	infraChanges := ChangeSet{ID: "infra-change", Changes: []Change{{
		ID: "terraform", RepositoryID: "infra-repo", Kind: ChangeModified, Path: "infra/prod/main.tf",
	}}}.Normalized()
	unsupported := Model{
		SchemaVersion: CurrentSchemaVersion,
		ChangeSetID:   infraChanges.ID,
		Services: []ServiceImpact{{
			ServiceID: "web", RepositoryID: "app-repo", Kind: ImpactDependent,
			Causes: []Cause{{
				ChangeID: "terraform", Path: "infra/prod/main.tf",
				RelationshipIDs: []systemmap.RelationshipID{"web-related-cluster"},
			}},
		}},
		Environments:   []EnvironmentImpact{},
		Configurations: []ConfigurationImpact{},
		Diagnostics:    []Diagnostic{},
	}.Normalized()
	if err := unsupported.Validate(system, infraChanges); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("unsupported Validate() error = %v, want ErrInvalidModel", err)
	}
}

func TestModelValidateRejectsImpactKindWithoutSupportingCause(t *testing.T) {
	t.Parallel()

	changes := ChangeSet{ID: "shared-change", Changes: []Change{{
		ID: "shared", RepositoryID: "app-repo", Kind: ChangeModified, Path: "libs/shared/client.go",
	}}}.Normalized()
	model := Model{
		SchemaVersion: CurrentSchemaVersion,
		ChangeSetID:   changes.ID,
		Services: []ServiceImpact{{
			ServiceID: "api", RepositoryID: "app-repo", Kind: ImpactDirect,
			Causes: []Cause{{
				ChangeID: "shared", Path: "libs/shared/client.go",
				RelationshipIDs: []systemmap.RelationshipID{"api-depends-shared"},
			}},
		}},
		Environments:   []EnvironmentImpact{},
		Configurations: []ConfigurationImpact{},
		Diagnostics:    []Diagnostic{},
	}.Normalized()
	if err := model.Validate(testSystem(), changes); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("Validate() error = %v, want ErrInvalidModel", err)
	}
}

func TestAnalyzerOutputCollectionsRemainSorted(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits())
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	result, err := analyzer.Analyze(t.Context(), testSystem(), ChangeSet{ID: "changes", Changes: []Change{
		{ID: "web", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/web/main.go"},
		{ID: "api", RepositoryID: "app-repo", Kind: ChangeModified, Path: "services/api/main.go"},
	}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !slices.IsSortedFunc(result.Services, compareServiceImpacts) ||
		!slices.IsSortedFunc(result.Environments, compareEnvironmentImpacts) {
		t.Fatalf("result = %#v, want deterministic sorted impacts", result)
	}
}
