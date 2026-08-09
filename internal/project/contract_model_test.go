package project

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestProjectContractAcceptsCanonicalModel(t *testing.T) {
	t.Parallel()

	projectModel := validCanonicalProject()
	if err := projectModel.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProjectContractRejectsInvalidModels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Project)
	}{
		{name: "unsupported schema", mutate: func(projectModel *Project) {
			projectModel.SchemaVersion = "2.0"
		}},
		{name: "unsafe source path", mutate: func(projectModel *Project) {
			projectModel.Services[0].SourcePath = "../api"
		}},
		{name: "unknown service environment", mutate: func(projectModel *Project) {
			projectModel.Services[0].Environments[0].EnvironmentID = "unknown"
		}},
		{name: "unknown dependency target", mutate: func(projectModel *Project) {
			projectModel.Dependencies[0].Target.ID = "unknown"
		}},
		{name: "self dependency", mutate: func(projectModel *Project) {
			projectModel.Dependencies[0].Target = projectModel.Dependencies[0].Source
		}},
		{name: "literal secret", mutate: func(projectModel *Project) {
			projectModel.Configuration.Entries[0].Sensitive = true
		}},
		{name: "plaintext secret", mutate: func(projectModel *Project) {
			projectModel.Environments[1].Configuration.Entries[0].Value = "plaintext"
		}},
		{name: "absolute file reference", mutate: func(projectModel *Project) {
			projectModel.Configuration.Entries[1].Reference = "/etc/iatros/config.yaml"
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

func TestProjectNormalizedDetachesAndOrdersCollections(t *testing.T) {
	t.Parallel()

	projectModel := validCanonicalProject()
	projectModel.Environments[0], projectModel.Environments[1] =
		projectModel.Environments[1], projectModel.Environments[0]
	projectModel.Services[0], projectModel.Services[1] =
		projectModel.Services[1], projectModel.Services[0]
	projectModel.Configuration.Entries[0], projectModel.Configuration.Entries[1] =
		projectModel.Configuration.Entries[1], projectModel.Configuration.Entries[0]

	normalized := projectModel.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if reflect.DeepEqual(normalized, projectModel) {
		t.Fatal("Normalized() did not order the model")
	}

	normalized.Configuration.Entries[0].Key = "changed"
	normalized.Dependencies[0].Environments[0] = "changed"
	if projectModel.Configuration.Entries[0].Key == "changed" ||
		projectModel.Dependencies[0].Environments[0] == "changed" {
		t.Fatal("Normalized() retained caller-owned collection storage")
	}
}

func TestProjectJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validCanonicalProject().Normalized()
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
			{
				Key:      "log.level",
				Source:   ConfigurationSourceLiteral,
				Value:    "info",
				Required: true,
			},
			{
				Key:       "runtime.config",
				Source:    ConfigurationSourceFile,
				Reference: "deploy/config.yaml",
				Required:  true,
			},
		}},
		Environments: []Environment{
			{
				ID:            "development",
				Name:          "Development",
				Kind:          "development",
				Configuration: Configuration{Entries: []ConfigurationEntry{}},
			},
			{
				ID:   "production",
				Name: "Production",
				Kind: "production",
				Configuration: Configuration{Entries: []ConfigurationEntry{
					{
						Key:       "database.password",
						Source:    ConfigurationSourceSecret,
						Reference: "vault://iatros/production/database-password",
						Required:  true,
						Sensitive: true,
					},
				}},
			},
		},
		Services: []Service{
			{
				ID:         "api",
				Name:       "API",
				Kind:       "application",
				SourcePath: ".",
				Configuration: Configuration{Entries: []ConfigurationEntry{
					{
						Key:       "http.port",
						Source:    ConfigurationSourceEnvironment,
						Reference: "HTTP_PORT",
						Required:  true,
					},
				}},
				Environments: []ServiceEnvironment{
					{EnvironmentID: "development", Configuration: Configuration{Entries: []ConfigurationEntry{}}},
					{EnvironmentID: "production", Configuration: Configuration{Entries: []ConfigurationEntry{}}},
				},
			},
			{
				ID:            "postgres",
				Name:          "PostgreSQL",
				Kind:          "database",
				Configuration: Configuration{Entries: []ConfigurationEntry{}},
				Environments: []ServiceEnvironment{
					{EnvironmentID: "development", Configuration: Configuration{Entries: []ConfigurationEntry{}}},
					{EnvironmentID: "production", Configuration: Configuration{Entries: []ConfigurationEntry{}}},
				},
			},
		},
		Dependencies: []Dependency{
			{
				ID:       "api-postgres",
				Source:   ResourceReference{Kind: ResourceKindService, ID: "api"},
				Target:   ResourceReference{Kind: ResourceKindService, ID: "postgres"},
				Kind:     "data",
				Required: true,
				Environments: []EnvironmentID{
					"development",
					"production",
				},
			},
		},
	}
}
