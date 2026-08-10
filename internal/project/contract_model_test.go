package project

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestCanonicalProjectNormalizesAndValidates(t *testing.T) {
	t.Parallel()

	projectModel := Project{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "iatros",
		Name:          "IATROS",
		Services: []Service{{
			ID: "api", Name: "API", Kind: "application", SourcePath: ".",
		}},
	}.Normalized()

	if err := projectModel.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if projectModel.Configuration.Entries == nil || projectModel.Environments == nil ||
		projectModel.Services[0].Configuration.Entries == nil ||
		projectModel.Services[0].Environments == nil || projectModel.Dependencies == nil {
		t.Fatal("Normalized() left a canonical project collection nil")
	}
}

func TestCanonicalProjectRejectsEscapingServicePath(t *testing.T) {
	t.Parallel()

	projectModel := Project{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "iatros",
		Name:          "IATROS",
		Services: []Service{{
			ID: "api", Name: "API", Kind: "application", SourcePath: "../outside",
		}},
	}.Normalized()

	if err := projectModel.Validate(); !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("Validate() error = %v, want ErrInvalidProject", err)
	}
}

func TestCanonicalProjectAcceptsCompleteProviderNeutralModel(t *testing.T) {
	t.Parallel()

	projectModel := validCanonicalProject()
	if err := projectModel.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCanonicalProjectRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Project)
	}{
		{name: "schema", mutate: func(model *Project) { model.SchemaVersion = "2.0" }},
		{name: "project id", mutate: func(model *Project) { model.ID = "IATROS" }},
		{name: "project name", mutate: func(model *Project) { model.Name = " IATROS" }},
		{name: "environment order", mutate: func(model *Project) {
			model.Environments[0], model.Environments[1] = model.Environments[1], model.Environments[0]
		}},
		{name: "environment kind", mutate: func(model *Project) {
			model.Environments[0].Kind = "Development"
		}},
		{name: "service order", mutate: func(model *Project) {
			model.Services[0], model.Services[1] = model.Services[1], model.Services[0]
		}},
		{name: "service source path", mutate: func(model *Project) {
			model.Services[0].SourcePath = "../outside"
		}},
		{name: "unknown service environment", mutate: func(model *Project) {
			model.Services[0].Environments[0].EnvironmentID = "missing"
		}},
		{name: "service environment order", mutate: func(model *Project) {
			model.Services[0].Environments[0], model.Services[0].Environments[1] =
				model.Services[0].Environments[1], model.Services[0].Environments[0]
		}},
		{name: "dependency order", mutate: func(model *Project) {
			model.Dependencies[0], model.Dependencies[1] = model.Dependencies[1], model.Dependencies[0]
		}},
		{name: "dependency kind", mutate: func(model *Project) {
			model.Dependencies[0].Kind = "Database"
		}},
		{name: "unknown service reference", mutate: func(model *Project) {
			model.Dependencies[0].Source.ID = "missing"
		}},
		{name: "unknown project reference", mutate: func(model *Project) {
			model.Dependencies[0].Target = ResourceReference{Kind: ResourceKindProject, ID: "other"}
		}},
		{name: "unknown environment reference", mutate: func(model *Project) {
			model.Dependencies[0].Target = ResourceReference{Kind: ResourceKindEnvironment, ID: "missing"}
		}},
		{name: "invalid external reference", mutate: func(model *Project) {
			model.Dependencies[0].Target.ID = "external\nvalue"
		}},
		{name: "self dependency", mutate: func(model *Project) {
			model.Dependencies[0].Target = model.Dependencies[0].Source
		}},
		{name: "unknown dependency environment", mutate: func(model *Project) {
			model.Dependencies[0].Environments[0] = "missing"
		}},
		{name: "dependency environment order", mutate: func(model *Project) {
			model.Dependencies[0].Environments[0], model.Dependencies[0].Environments[1] =
				model.Dependencies[0].Environments[1], model.Dependencies[0].Environments[0]
		}},
		{name: "configuration key", mutate: func(model *Project) {
			model.Configuration.Entries[0].Key = "invalid key"
		}},
		{name: "configuration order", mutate: func(model *Project) {
			model.Configuration.Entries[0], model.Configuration.Entries[1] =
				model.Configuration.Entries[1], model.Configuration.Entries[0]
		}},
		{name: "sensitive literal", mutate: func(model *Project) {
			model.Configuration.Entries[1].Sensitive = true
		}},
		{name: "environment value", mutate: func(model *Project) {
			model.Services[0].Configuration.Entries[0].Value = "secret"
		}},
		{name: "environment reference", mutate: func(model *Project) {
			model.Services[0].Configuration.Entries[0].Reference = "DATABASE-URL"
		}},
		{name: "file root reference", mutate: func(model *Project) {
			model.Configuration.Entries[0].Reference = "."
		}},
		{name: "file value", mutate: func(model *Project) {
			model.Configuration.Entries[0].Value = "inline"
		}},
		{name: "non-sensitive secret", mutate: func(model *Project) {
			model.Configuration.Entries[2].Sensitive = false
		}},
		{name: "unsupported source", mutate: func(model *Project) {
			model.Configuration.Entries[0].Source = "provider"
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			projectModel := validCanonicalProject()
			testCase.mutate(&projectModel)
			if err := projectModel.Validate(); !errors.Is(err, ErrInvalidProject) {
				t.Fatalf("Validate() error = %v, want ErrInvalidProject", err)
			}
		})
	}
}

func TestCanonicalProjectNormalizationOrdersAndDetachesCollections(t *testing.T) {
	t.Parallel()

	projectModel := validCanonicalProject()
	projectModel.Configuration.Entries[0], projectModel.Configuration.Entries[2] =
		projectModel.Configuration.Entries[2], projectModel.Configuration.Entries[0]
	projectModel.Environments[0], projectModel.Environments[1] =
		projectModel.Environments[1], projectModel.Environments[0]
	projectModel.Services[0], projectModel.Services[1] = projectModel.Services[1], projectModel.Services[0]
	projectModel.Dependencies[0], projectModel.Dependencies[1] =
		projectModel.Dependencies[1], projectModel.Dependencies[0]
	projectModel.Services[1].Environments[0], projectModel.Services[1].Environments[1] =
		projectModel.Services[1].Environments[1], projectModel.Services[1].Environments[0]

	normalized := projectModel.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.Configuration.Entries[0].Key != "config" ||
		normalized.Environments[0].ID != "development" ||
		normalized.Services[0].ID != "api" ||
		normalized.Services[0].Environments[0].EnvironmentID != "development" ||
		normalized.Dependencies[0].ID != "api-database" {
		t.Fatalf("Normalized() did not order the complete project: %#v", normalized)
	}

	normalized.Configuration.Entries[0].Key = "changed"
	normalized.Services[0].Environments[0].EnvironmentID = "changed"
	if projectModel.Configuration.Entries[2].Key == "changed" ||
		projectModel.Services[1].Environments[1].EnvironmentID == "changed" {
		t.Fatal("Normalized() retained caller-owned collection storage")
	}
}

func TestCanonicalProjectJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validCanonicalProject()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Project
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded project = %#v, want %#v", decoded, want)
	}
}

func validCanonicalProject() Project {
	return Project{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "iatros",
		Name:          "IATROS",
		Configuration: Configuration{Entries: []ConfigurationEntry{
			{Key: "config", Source: ConfigurationSourceFile, Reference: "config/iatros.yaml", Required: true},
			{Key: "log_level", Source: ConfigurationSourceLiteral, Value: "info"},
			{Key: "token", Source: ConfigurationSourceSecret, Reference: "vault://iatros/token", Required: true, Sensitive: true},
		}},
		Environments: []Environment{
			{ID: "development", Name: "Development", Kind: "development"},
			{ID: "production", Name: "Production", Kind: "production"},
		},
		Services: []Service{
			{
				ID: "api", Name: "API", Kind: "application", SourcePath: ".",
				Configuration: Configuration{Entries: []ConfigurationEntry{{
					Key: "database_url", Source: ConfigurationSourceEnvironment,
					Reference: "DATABASE_URL", Required: true, Sensitive: true,
				}}},
				Environments: []ServiceEnvironment{
					{EnvironmentID: "development"},
					{EnvironmentID: "production"},
				},
			},
			{
				ID: "worker", Name: "Worker", Kind: "worker", SourcePath: "cmd/worker",
				Environments: []ServiceEnvironment{{EnvironmentID: "production"}},
			},
		},
		Dependencies: []Dependency{
			{
				ID: "api-database", Kind: "database", Required: true,
				Source:       ResourceReference{Kind: ResourceKindService, ID: "api"},
				Target:       ResourceReference{Kind: ResourceKindExternal, ID: "postgresql:primary"},
				Environments: []EnvironmentID{"development", "production"},
			},
			{
				ID: "worker-api", Kind: "http", Required: true,
				Source: ResourceReference{Kind: ResourceKindService, ID: "worker"},
				Target: ResourceReference{Kind: ResourceKindService, ID: "api"},
			},
		},
	}.Normalized()
}
