package remoteanalysis

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestModelSupportsGitHubGitLabBitbucketAndPolyrepositories(t *testing.T) {
	t.Parallel()

	model := validModel()
	if err := model.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}

	providers := map[Provider]bool{}
	for _, repository := range model.Repositories {
		providers[repository.Provider] = true
	}
	for _, provider := range []Provider{ProviderGitHub, ProviderGitLab, ProviderBitbucket} {
		if !providers[provider] {
			t.Fatalf("provider %q was not represented", provider)
		}
	}
	if len(model.Relationships) != 2 {
		t.Fatalf("relationships = %d, want 2", len(model.Relationships))
	}
}

func TestModelNormalizedProducesDetachedValidModel(t *testing.T) {
	t.Parallel()

	input := validModel()
	input.Repositories[0], input.Repositories[2] = input.Repositories[2], input.Repositories[0]
	input.Repositories[0].Evidence = append(
		input.Repositories[0].Evidence,
		input.Repositories[0].Evidence[0],
	)
	input.Relationships[0], input.Relationships[1] = input.Relationships[1], input.Relationships[0]
	input.Diagnostics = []Diagnostic{
		{
			Code: "NOTE_B", Level: DiagnosticInfo, Provider: ProviderGitLab,
			RepositoryID: "gitlab-web", Scope: "repository:platform/apps/checkout-web",
			Message: "Repository note.",
		},
		{Code: "NOTE_A", Level: DiagnosticInfo, Scope: "system:checkout", Message: "System note."},
	}

	normalized := input.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.Repositories == nil || normalized.Relationships == nil ||
		normalized.Evidence == nil || normalized.Diagnostics == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if normalized.Repositories[0].ID != "bitbucket-ops" ||
		normalized.Relationships[0].ID != "api-depends-web" {
		t.Fatal("Normalized() did not order identity collections")
	}
	if normalized.Diagnostics[0].Code != "NOTE_A" {
		t.Fatal("Normalized() did not order diagnostics")
	}
	if len(normalized.Repositories[2].Evidence) != 1 {
		t.Fatal("Normalized() did not compact repository evidence")
	}

	input.Repositories[0].Name = "Changed"
	input.Repositories[0].Evidence[0].Reference = "changed"
	input.Evidence[0].Reference = "changed"
	if normalized.Repositories[2].Name == "Changed" ||
		normalized.Repositories[2].Evidence[0].Reference == "changed" ||
		normalized.Evidence[0].Reference == "changed" {
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
		{name: "unsupported schema", mutate: func(model *Model) { model.SchemaVersion = "2.0" }},
		{name: "invalid system id", mutate: func(model *Model) { model.ID = "Checkout System" }},
		{name: "no repositories", mutate: func(model *Model) { model.Repositories = nil }},
		{
			name: "repositories out of order",
			mutate: func(model *Model) {
				model.Repositories[0], model.Repositories[1] = model.Repositories[1], model.Repositories[0]
			},
		},
		{name: "unsupported provider", mutate: func(model *Model) { model.Repositories[0].Provider = "other" }},
		{
			name: "duplicate provider repository location",
			mutate: func(model *Model) {
				model.Repositories[2].Provider = model.Repositories[1].Provider
				model.Repositories[2].Host = model.Repositories[1].Host
				model.Repositories[2].Namespace = model.Repositories[1].Namespace
				model.Repositories[2].Name = model.Repositories[1].Name
			},
		},
		{name: "uppercase host", mutate: func(model *Model) { model.Repositories[0].Host = "Bitbucket.org" }},
		{name: "host with path", mutate: func(model *Model) { model.Repositories[0].Host = "bitbucket.org/api" }},
		{name: "unsafe namespace", mutate: func(model *Model) { model.Repositories[1].Namespace = "../acme" }},
		{name: "uppercase namespace", mutate: func(model *Model) { model.Repositories[1].Namespace = "Acme" }},
		{name: "uppercase repository name", mutate: func(model *Model) { model.Repositories[1].Name = "Checkout-API" }},
		{name: "short revision", mutate: func(model *Model) { model.Repositories[1].Revision = "abc123" }},
		{name: "uppercase revision", mutate: func(model *Model) { model.Repositories[1].Revision = forty("A") }},
		{
			name: "default branch mismatch",
			mutate: func(model *Model) {
				model.Repositories[1].Reference = "develop"
			},
		},
		{
			name: "commit selector differs from revision",
			mutate: func(model *Model) {
				model.Repositories[0].Reference = forty("d")
			},
		},
		{
			name: "unsafe git reference",
			mutate: func(model *Model) {
				model.Repositories[2].Reference = "release..candidate"
			},
		},
		{
			name: "option-like git reference",
			mutate: func(model *Model) {
				model.Repositories[2].Reference = "-release"
			},
		},
		{
			name: "symbolic HEAD reference",
			mutate: func(model *Model) {
				model.Repositories[2].Reference = "HEAD"
			},
		},
		{
			name: "unknown reference kind",
			mutate: func(model *Model) {
				model.Repositories[2].ReferenceKind = "pull_request"
			},
		},
		{name: "invalid visibility", mutate: func(model *Model) { model.Repositories[0].Visibility = "secret" }},
		{
			name: "repository evidence references another repository",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0].RepositoryID = "github-api"
			},
		},
		{
			name: "repository has no provider evidence",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0] = Evidence{
					RepositoryID: "bitbucket-ops", Kind: EvidenceRepositoryFile, Path: "catalog.yaml",
				}
			},
		},
		{
			name: "provider evidence contains credential URL",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0].Reference = "https://token@bitbucket.org/acme/ops"
			},
		},
		{
			name: "provider evidence also contains a path",
			mutate: func(model *Model) {
				model.Repositories[0].Evidence[0].Path = "catalog.yaml"
			},
		},
		{
			name: "repository file evidence also contains a reference",
			mutate: func(model *Model) {
				model.Relationships[0].Evidence[0].Reference = "catalog:checkout"
			},
		},
		{
			name: "unknown evidence kind",
			mutate: func(model *Model) {
				model.Evidence[0].Kind = "provider_response"
			},
		},
		{
			name: "system evidence references unknown repository",
			mutate: func(model *Model) {
				model.Evidence[0].RepositoryID = "missing"
			},
		},
		{
			name: "relationship references unknown repository",
			mutate: func(model *Model) {
				model.Relationships[0].TargetID = "missing"
			},
		},
		{
			name: "self relationship",
			mutate: func(model *Model) {
				model.Relationships[0].TargetID = model.Relationships[0].SourceID
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
					Code: "RATE_LIMITED", Level: DiagnosticWarning,
					Provider: ProviderGitHub, Scope: "provider:github", Message: "Rate limit reached.",
				}}
			},
		},
		{name: "partial model without diagnostics", mutate: func(model *Model) { model.Partial = true }},
		{
			name: "partial model with only information",
			mutate: func(model *Model) {
				model.Partial = true
				model.Diagnostics = []Diagnostic{{
					Code: "NOTE", Level: DiagnosticInfo, Scope: "system:checkout", Message: "Mapping completed.",
				}}
			},
		},
		{
			name: "diagnostic provider conflicts with repository",
			mutate: func(model *Model) {
				model.Partial = true
				model.Diagnostics = []Diagnostic{{
					Code: "READ_FAILED", Level: DiagnosticError, Provider: ProviderGitLab,
					RepositoryID: "github-api", Scope: "repository:acme/checkout-api",
					Message: "Repository metadata could not be read.",
				}}
			},
		},
		{
			name: "diagnostic has unsupported provider",
			mutate: func(model *Model) {
				model.Partial = true
				model.Diagnostics = []Diagnostic{{
					Code: "READ_FAILED", Level: DiagnosticError, Provider: "other",
					Scope: "provider:other", Message: "Provider metadata could not be read.",
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

func TestModelValidateAcceptsSelfHostedProvidersAndPartialResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
	}{
		{name: "DNS with port", host: "gitlab.acme.internal:8443"},
		{name: "IPv4", host: "127.0.0.1:8443"},
		{name: "IPv4 without port", host: "127.0.0.1"},
		{name: "IPv6", host: "[2001:db8::1]:8443"},
		{name: "IPv6 without port", host: "[2001:db8::1]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			model.Repositories[2].Host = test.host
			model.Partial = true
			model.Diagnostics = []Diagnostic{{
				Code: "FILE_UNAVAILABLE", Level: DiagnosticWarning,
				Provider: ProviderGitLab, RepositoryID: "gitlab-web",
				Scope:   "repository:platform/apps/checkout-web",
				Message: "An optional catalog file was unavailable.",
			}}
			if err := model.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestModelValidateRejectsMalformedSelfHostedHosts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
	}{
		{name: "missing closing bracket", host: "[2001:db8::1"},
		{name: "bracketed DNS name", host: "[gitlab.acme.internal]"},
		{name: "bracketed IPv4", host: "[127.0.0.1]"},
		{name: "raw IPv6", host: "2001:db8::1"},
		{name: "empty port", host: "gitlab.acme.internal:"},
		{name: "zero port", host: "gitlab.acme.internal:0"},
		{name: "non-canonical port", host: "gitlab.acme.internal:080"},
		{name: "oversized port", host: "gitlab.acme.internal:65536"},
		{name: "empty DNS label", host: "gitlab..internal"},
		{name: "leading label separator", host: "-gitlab.internal"},
		{name: "trailing dot", host: "gitlab.internal."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			model.Repositories[2].Host = test.host
			if err := model.Validate(); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("Validate() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateAcceptsExplicitBranchSelection(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Repositories[2].Reference = "feature/remote-analysis"
	model.Repositories[2].ReferenceKind = ReferenceBranch
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

func TestModelValidateAcceptsProviderRepositoryNamesWithLeadingSeparators(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Repositories[1].Name = ".github"
	model.Repositories[1].Evidence[0].Reference = "repository:acme/.github"
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func validModel() Model {
	return Model{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "checkout-system",
		Name:          "Checkout System",
		Repositories: []Repository{
			{
				ID: "bitbucket-ops", Provider: ProviderBitbucket, Host: "bitbucket.org",
				Namespace: "acme", Name: "checkout-ops", Revision: forty("a"),
				Reference: forty("a"), ReferenceKind: ReferenceCommit, DefaultBranch: "main",
				Visibility: VisibilityPrivate,
				Evidence: []Evidence{{
					RepositoryID: "bitbucket-ops", Kind: EvidenceProviderMetadata,
					Reference: "repository:acme/checkout-ops",
				}},
			},
			{
				ID: "github-api", Provider: ProviderGitHub, Host: "github.com",
				Namespace: "acme", Name: "checkout-api", Revision: forty("b"),
				Reference: "main", ReferenceKind: ReferenceDefaultBranch, DefaultBranch: "main",
				Visibility: VisibilityPrivate,
				Evidence: []Evidence{{
					RepositoryID: "github-api", Kind: EvidenceProviderMetadata,
					Reference: "repository:acme/checkout-api",
				}},
			},
			{
				ID: "gitlab-web", Provider: ProviderGitLab, Host: "gitlab.acme.internal:8443",
				Namespace: "platform/apps", Name: "checkout-web", Revision: forty("c"),
				Reference: "v1.2.0", ReferenceKind: ReferenceTag, DefaultBranch: "main",
				Visibility: VisibilityInternal, IsFork: true,
				Evidence: []Evidence{{
					RepositoryID: "gitlab-web", Kind: EvidenceProviderMetadata,
					Reference: "repository:platform/apps/checkout-web",
				}},
			},
		},
		Relationships: []Relationship{
			{
				ID: "api-depends-web", SourceID: "github-api", TargetID: "gitlab-web", Kind: "depends_on",
				Evidence: []Evidence{{
					RepositoryID: "github-api", Kind: EvidenceRepositoryFile, Path: "system/catalog.yaml",
				}},
			},
			{
				ID: "ops-deploys-api", SourceID: "bitbucket-ops", TargetID: "github-api", Kind: "deploys",
				Evidence: []Evidence{{
					RepositoryID: "bitbucket-ops", Kind: EvidenceRepositoryFile, Path: "deploy/services.yaml",
				}},
			},
		},
		Evidence: []Evidence{{
			RepositoryID: "github-api", Kind: EvidenceProviderMetadata,
			Reference: "catalog:checkout-system",
		}},
		Diagnostics: []Diagnostic{},
	}
}

func forty(character string) string {
	return strings.Repeat(character, 40)
}
