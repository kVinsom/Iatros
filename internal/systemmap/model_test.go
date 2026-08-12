package systemmap

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestModelNormalizedProducesDetachedValidModel(t *testing.T) {
	t.Parallel()

	input := validModel()
	input.Repositories[0], input.Repositories[1] = input.Repositories[1], input.Repositories[0]
	input.Services[0].EnvironmentIDs = []EnvironmentID{"production", "production"}
	input.Services[0].Evidence = append(input.Services[0].Evidence, input.Services[0].Evidence[0])
	input.Diagnostics = nil

	normalized := input.Normalized()
	if err := normalized.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
	if normalized.Repositories == nil || normalized.Diagnostics == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if normalized.Repositories[0].ID != "api-repo" {
		t.Fatalf("first repository = %q, want api-repo", normalized.Repositories[0].ID)
	}
	if got := len(normalized.Services[0].EnvironmentIDs); got != 1 {
		t.Fatalf("environment ids = %d, want 1", got)
	}
	if got := len(normalized.Services[0].Evidence); got != 1 {
		t.Fatalf("service evidence = %d, want 1", got)
	}

	input.Repositories[0].Name = "Changed"
	input.Services[0].EnvironmentIDs[0] = "changed"
	input.Services[0].Evidence[0].Path = "changed.go"
	if normalized.Repositories[1].Name == "Changed" ||
		normalized.Services[0].EnvironmentIDs[0] == "changed" ||
		normalized.Services[0].Evidence[0].Path == "changed.go" {
		t.Fatal("Normalized() retained mutable input storage")
	}
}

func TestModelJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validModel()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Model
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelValidateRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Model)
	}{
		{
			name: "unsupported schema",
			mutate: func(model *Model) {
				model.SchemaVersion = "2.0"
			},
		},
		{
			name: "invalid system id",
			mutate: func(model *Model) {
				model.ID = "System A"
			},
		},
		{
			name: "no repositories",
			mutate: func(model *Model) {
				model.Repositories = nil
			},
		},
		{
			name: "repositories out of order",
			mutate: func(model *Model) {
				model.Repositories[0], model.Repositories[1] = model.Repositories[1], model.Repositories[0]
			},
		},
		{
			name: "local repository with provider",
			mutate: func(model *Model) {
				model.Repositories[0].Provider = "github"
			},
		},
		{
			name: "duplicate repository source location",
			mutate: func(model *Model) {
				duplicate := model.Repositories[0]
				duplicate.ID = "infra-repo"
				duplicate.Name = "Duplicate API Repository"
				duplicate.Evidence = []Evidence{{
					RepositoryID: "infra-repo", Kind: EvidenceTarget, Path: ".",
				}}
				model.Repositories[1] = duplicate
			},
		},
		{
			name: "remote repository with credential locator",
			mutate: func(model *Model) {
				model.Repositories[1].Locator = "user@github.com/acme/infra"
			},
		},
		{
			name: "remote repository with uppercase locator",
			mutate: func(model *Model) {
				model.Repositories[1].Locator = "github.com/Acme/checkout-infra"
			},
		},
		{
			name: "remote repository without immutable revision",
			mutate: func(model *Model) {
				model.Repositories[1].Revision = ""
			},
		},
		{
			name: "remote repository with unsafe revision",
			mutate: func(model *Model) {
				model.Repositories[1].Revision = "https://github.com/acme/checkout-infra/commit/abc123"
			},
		},
		{
			name: "repository evidence references another repository",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0].RepositoryID = "infra-repo"
			},
		},
		{
			name: "local repository without target evidence",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0] = Evidence{
					RepositoryID: "api-repo", Kind: EvidenceSource, Path: "main.go",
				}
			},
		},
		{
			name: "remote repository without provider evidence",
			mutate: func(model *Model) {
				model.Repositories[1].Evidence[0] = Evidence{
					RepositoryID: "infra-repo", Kind: EvidenceConfiguration, Path: "catalog.yaml",
				}
			},
		},
		{
			name: "provider evidence contains credential URL",
			mutate: func(model *Model) {
				model.Repositories[1].Evidence[0].Reference =
					"https://secret@github.com/acme/checkout-infra"
			},
		},
		{
			name: "provider evidence has path",
			mutate: func(model *Model) {
				model.Repositories[1].Evidence[0].Path = "catalog.yaml"
			},
		},
		{
			name: "service references unknown repository",
			mutate: func(model *Model) {
				model.Services[0].RepositoryID = "missing"
			},
		},
		{
			name: "service references unknown environment",
			mutate: func(model *Model) {
				model.Services[0].EnvironmentIDs = []EnvironmentID{"missing"}
			},
		},
		{
			name: "library references unknown repository",
			mutate: func(model *Model) {
				model.Libraries[0].RepositoryID = "missing"
			},
		},
		{
			name: "infrastructure references unknown environment",
			mutate: func(model *Model) {
				model.Infrastructure[0].EnvironmentIDs = []EnvironmentID{"missing"}
			},
		},
		{
			name: "environment kind is invalid",
			mutate: func(model *Model) {
				model.Environments[0].Kind = "Production"
			},
		},
		{
			name: "duplicate environment reference",
			mutate: func(model *Model) {
				model.Services[0].EnvironmentIDs = []EnvironmentID{"production", "production"}
			},
		},
		{
			name: "invalid owner kind",
			mutate: func(model *Model) {
				model.Owners[0].Kind = "department"
			},
		},
		{
			name: "external resource provider is invalid",
			mutate: func(model *Model) {
				model.ExternalResources[0].Provider = "Payment Provider"
			},
		},
		{
			name: "unknown relationship endpoint",
			mutate: func(model *Model) {
				model.Relationships[0].Target.ID = "missing"
			},
		},
		{
			name: "unknown relationship endpoint kind",
			mutate: func(model *Model) {
				model.Relationships[0].Target.Kind = "unknown"
			},
		},
		{
			name: "relationship without evidence",
			mutate: func(model *Model) {
				model.Relationships[0].Evidence = nil
			},
		},
		{
			name: "self relationship",
			mutate: func(model *Model) {
				model.Relationships[0].Target = model.Relationships[0].Source
			},
		},
		{
			name: "relationships out of order",
			mutate: func(model *Model) {
				model.Relationships[0], model.Relationships[1] =
					model.Relationships[1], model.Relationships[0]
			},
		},
		{
			name: "complete model with warning",
			mutate: func(model *Model) {
				model.Diagnostics = []Diagnostic{{
					Code: "LIMIT_REACHED", Level: DiagnosticWarning, Path: ".", Message: "Limit reached.",
				}}
			},
		},
		{
			name: "partial model without diagnostic",
			mutate: func(model *Model) {
				model.Partial = true
			},
		},
		{
			name: "partial model with only information",
			mutate: func(model *Model) {
				model.Partial = true
				model.Diagnostics = []Diagnostic{{
					Code: "NOTE", Level: DiagnosticInfo, Path: ".", Message: "Mapping completed.",
				}}
			},
		},
		{
			name: "diagnostic references unknown repository",
			mutate: func(model *Model) {
				model.Partial = true
				model.Diagnostics = []Diagnostic{{
					Code:         "READ_FAILED",
					Level:        DiagnosticError,
					RepositoryID: "missing",
					Path:         ".",
					Message:      "Repository could not be read.",
				}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			test.mutate(&model)
			if err := model.Validate(); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("Validate() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateAcceptsPartialResultWithWarning(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Partial = true
	model.Diagnostics = []Diagnostic{{
		Code:         "READ_FAILED",
		Level:        DiagnosticWarning,
		RepositoryID: "api-repo",
		Path:         "optional/catalog.yaml",
		Message:      "The optional catalog could not be read.",
	}}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelValidateWithinRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxRepositories = 0
	if err := validModel().ValidateWithin(limits); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("ValidateWithin() error = %v, want ErrInvalidLimits", err)
	}
}

func TestModelSupportsAndNormalizesEveryEntityKind(t *testing.T) {
	t.Parallel()

	model := validModel()

	service := model.Services[0]
	service.ID = "worker"
	service.Name = "Checkout Worker"
	model.Services = append(model.Services, service)

	library := model.Libraries[0]
	library.ID = "shared-model"
	library.Name = "Shared Model"
	model.Libraries = append(model.Libraries, library)

	component := model.Infrastructure[0]
	component.ID = "production-network"
	component.Name = "Production Network"
	model.Infrastructure = append(model.Infrastructure, component)

	environment := model.Environments[0]
	environment.ID = "staging"
	environment.Name = "Staging"
	environment.Kind = "staging"
	model.Environments = append(model.Environments, environment)

	owner := model.Owners[0]
	owner.ID = "service-team"
	owner.Name = "Service Team"
	model.Owners = append(model.Owners, owner)

	resource := model.ExternalResources[0]
	resource.ID = "tax-api"
	resource.Name = "Tax API"
	model.ExternalResources = append(model.ExternalResources, resource)

	model.Relationships = append(model.Relationships,
		Relationship{
			ID:     "repository-contains-library",
			Source: EntityReference{Kind: EntityRepository, ID: "api-repo"},
			Target: EntityReference{Kind: EntityLibrary, ID: "checkout-client"},
			Kind:   "contains",
			Evidence: []Evidence{{
				RepositoryID: "api-repo", Kind: EvidenceManifest, Path: "go.mod",
			}},
		},
		Relationship{
			ID:     "service-available-in-environment",
			Source: EntityReference{Kind: EntityService, ID: "checkout-api"},
			Target: EntityReference{Kind: EntityEnvironment, ID: "production"},
			Kind:   "available_in", EnvironmentIDs: []EnvironmentID{"production"},
			Evidence: []Evidence{{
				RepositoryID: "infra-repo",
				Kind:         EvidenceConfiguration,
				Path:         "clusters/production/kustomization.yaml",
			}},
		},
	)
	model.Diagnostics = []Diagnostic{
		{Code: "NOTE_B", Level: DiagnosticInfo, Path: ".", Message: "Second note."},
		{Code: "NOTE_A", Level: DiagnosticInfo, Path: ".", Message: "First note."},
		{
			Code: "NOTE_C", Level: DiagnosticInfo, RepositoryID: "api-repo",
			Path: ".", Message: "Repository note.",
		},
	}

	normalized := model.Normalized()
	if err := normalized.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
	if normalized.Services[1].ID != "worker" || normalized.Libraries[1].ID != "shared-model" ||
		normalized.Infrastructure[1].ID != "production-network" ||
		normalized.Environments[1].ID != "staging" || normalized.Owners[1].ID != "service-team" ||
		normalized.ExternalResources[1].ID != "tax-api" ||
		normalized.Diagnostics[0].Code != "NOTE_A" {
		t.Fatal("Normalized() did not order every entity collection")
	}
}

func validModel() Model {
	return Model{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "checkout-system",
		Name:          "Checkout System",
		Repositories: []Repository{
			{
				ID: "api-repo", Name: "API Repository", Source: RepositorySourceLocal, Locator: ".",
				Evidence: []Evidence{{RepositoryID: "api-repo", Kind: EvidenceTarget, Path: "."}},
			},
			{
				ID:       "infra-repo",
				Name:     "Infrastructure Repository",
				Source:   RepositorySourceRemote,
				Provider: "github",
				Locator:  "github.com/acme/checkout-infra",
				Revision: "abc123",
				Evidence: []Evidence{{
					RepositoryID: "infra-repo",
					Kind:         EvidenceProvider,
					Reference:    "github repository acme/checkout-infra at abc123",
				}},
			},
		},
		Services: []Service{{
			ID: "checkout-api", RepositoryID: "api-repo", Name: "Checkout API", Kind: "api", Root: ".",
			EnvironmentIDs: []EnvironmentID{"production"},
			Evidence:       []Evidence{{RepositoryID: "api-repo", Kind: EvidenceSource, Path: "cmd/api/main.go"}},
		}},
		Libraries: []Library{{
			ID: "checkout-client", RepositoryID: "api-repo", Name: "Checkout Client",
			Ecosystem: "go", Version: "v1.2.0", Root: "pkg/client",
			Evidence: []Evidence{{RepositoryID: "api-repo", Kind: EvidenceManifest, Path: "go.mod"}},
		}},
		Infrastructure: []Infrastructure{{
			ID: "production-cluster", RepositoryID: "infra-repo", Name: "Production Cluster",
			Kind: "cluster", Technology: "kubernetes", Root: "clusters/production",
			EnvironmentIDs: []EnvironmentID{"production"},
			Evidence: []Evidence{{
				RepositoryID: "infra-repo",
				Kind:         EvidenceConfiguration,
				Path:         "clusters/production/kustomization.yaml",
			}},
		}},
		Environments: []Environment{{
			ID: "production", Name: "Production", Kind: "production",
			Evidence: []Evidence{{
				RepositoryID: "infra-repo",
				Kind:         EvidenceConfiguration,
				Path:         "clusters/production/kustomization.yaml",
			}},
		}},
		Owners: []Owner{{
			ID: "platform-team", Kind: OwnerTeam, Name: "Platform Team", Reference: "@acme/platform",
			Evidence: []Evidence{{
				RepositoryID: "api-repo", Kind: EvidenceOwnership, Path: ".github/CODEOWNERS",
			}},
		}},
		ExternalResources: []ExternalResource{{
			ID: "payment-api", Name: "Payment API", Kind: "api", Provider: "payments",
			Evidence: []Evidence{{
				RepositoryID: "api-repo", Kind: EvidenceConfiguration, Path: "deploy/production.yaml",
			}},
		}},
		Relationships: []Relationship{
			{
				ID:     "calls-payment",
				Source: EntityReference{Kind: EntityService, ID: "checkout-api"},
				Target: EntityReference{Kind: EntityExternalResource, ID: "payment-api"},
				Kind:   "calls", EnvironmentIDs: []EnvironmentID{"production"},
				Evidence: []Evidence{{
					RepositoryID: "api-repo", Kind: EvidenceConfiguration, Path: "deploy/production.yaml",
				}},
			},
			{
				ID:     "owns-api",
				Source: EntityReference{Kind: EntityOwner, ID: "platform-team"},
				Target: EntityReference{Kind: EntityService, ID: "checkout-api"},
				Kind:   "owns",
				Evidence: []Evidence{{
					RepositoryID: "api-repo", Kind: EvidenceOwnership, Path: ".github/CODEOWNERS",
				}},
			},
			{
				ID:     "runs-on-cluster",
				Source: EntityReference{Kind: EntityService, ID: "checkout-api"},
				Target: EntityReference{Kind: EntityInfrastructure, ID: "production-cluster"},
				Kind:   "runs_on", EnvironmentIDs: []EnvironmentID{"production"},
				Evidence: []Evidence{{
					RepositoryID: "infra-repo",
					Kind:         EvidenceConfiguration,
					Path:         "clusters/production/kustomization.yaml",
				}},
			},
		},
		Diagnostics: []Diagnostic{},
	}
}
