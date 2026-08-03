package project

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestProjectValidateAcceptsCanonicalModel(t *testing.T) {
	t.Parallel()

	if err := validProject().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProjectValidateAcceptsOpaqueExternalReference(t *testing.T) {
	t.Parallel()

	project := validProject()
	project.Dependencies[0].Target = ResourceReference{
		Kind: ResourceKindExternal,
		ID:   "arn:aws:rds:eu-central-1:123456789012:db/iatros-production",
	}
	if err := project.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProjectValidateRejectsInvalidModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Project)
	}{
		{name: "schema", mutate: func(project *Project) { project.SchemaVersion = "2.0" }},
		{name: "project id", mutate: func(project *Project) { project.ID = "IATROS" }},
		{name: "project name", mutate: func(project *Project) { project.Name = " IATROS" }},
		{name: "environment order", mutate: func(project *Project) {
			project.Environments[0], project.Environments[1] = project.Environments[1], project.Environments[0]
		}},
		{name: "duplicate environment", mutate: func(project *Project) {
			project.Environments[1].ID = project.Environments[0].ID
		}},
		{name: "environment kind", mutate: func(project *Project) {
			project.Environments[0].Kind = "Development"
		}},
		{name: "service order", mutate: func(project *Project) {
			project.Services[0], project.Services[1] = project.Services[1], project.Services[0]
		}},
		{name: "service source path", mutate: func(project *Project) {
			project.Services[0].SourcePath = "../api"
		}},
		{name: "service environment", mutate: func(project *Project) {
			project.Services[0].Environments[0].EnvironmentID = "unknown"
		}},
		{name: "service environment order", mutate: func(project *Project) {
			bindings := project.Services[0].Environments
			bindings[0], bindings[1] = bindings[1], bindings[0]
		}},
		{name: "dependency target", mutate: func(project *Project) {
			project.Dependencies[0].Target.ID = "unknown"
		}},
		{name: "dependency self reference", mutate: func(project *Project) {
			project.Dependencies[0].Target = project.Dependencies[0].Source
		}},
		{name: "dependency environment order", mutate: func(project *Project) {
			environments := project.Dependencies[0].Environments
			environments[0], environments[1] = environments[1], environments[0]
		}},
		{name: "dependency environment", mutate: func(project *Project) {
			project.Dependencies[0].Environments[0] = "unknown"
		}},
		{name: "resource kind", mutate: func(project *Project) {
			project.Dependencies[0].Source.Kind = "cluster"
		}},
		{name: "project reference", mutate: func(project *Project) {
			project.Dependencies[0].Source = ResourceReference{Kind: ResourceKindProject, ID: "other"}
		}},
		{name: "configuration order", mutate: func(project *Project) {
			entries := project.Configuration.Entries
			entries[0], entries[1] = entries[1], entries[0]
		}},
		{name: "literal secret", mutate: func(project *Project) {
			project.Configuration.Entries[0].Sensitive = true
		}},
		{name: "secret value", mutate: func(project *Project) {
			project.Environments[1].Configuration.Entries[0].Value = "plaintext"
		}},
		{name: "secret sensitivity", mutate: func(project *Project) {
			project.Environments[1].Configuration.Entries[0].Sensitive = false
		}},
		{name: "absolute file reference", mutate: func(project *Project) {
			project.Configuration.Entries[1].Reference = "/etc/iatros/config.yaml"
		}},
		{name: "configuration source", mutate: func(project *Project) {
			project.Configuration.Entries[0].Source = "provider"
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			project := validProject()
			test.mutate(&project)
			if err := project.Validate(); !errors.Is(err, ErrInvalidProject) {
				t.Fatalf("Validate() error = %v, want ErrInvalidProject", err)
			}
		})
	}
}

func TestProjectNormalizedSortsAndDetachesCollections(t *testing.T) {
	t.Parallel()

	project := validProject()
	project.Environments[0], project.Environments[1] = project.Environments[1], project.Environments[0]
	project.Services[0], project.Services[1] = project.Services[1], project.Services[0]
	project.Dependencies[0].Environments[0], project.Dependencies[0].Environments[1] =
		project.Dependencies[0].Environments[1], project.Dependencies[0].Environments[0]
	project.Configuration.Entries[0], project.Configuration.Entries[1] =
		project.Configuration.Entries[1], project.Configuration.Entries[0]

	original := project
	normalized := project.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if reflect.DeepEqual(normalized, original) {
		t.Fatal("Normalized() did not reorder the model")
	}

	normalized.Configuration.Entries[0].Key = "changed"
	normalized.Dependencies[0].Environments[0] = "changed"
	if project.Configuration.Entries[0].Key == "changed" ||
		project.Dependencies[0].Environments[0] == "changed" {
		t.Fatal("Normalized() retained an input collection")
	}
}

func TestProjectNormalizedInitializesEmptyCollections(t *testing.T) {
	t.Parallel()

	project := Project{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "empty",
		Name:          "Empty Project",
	}.Normalized()

	if project.Configuration.Entries == nil || project.Environments == nil ||
		project.Services == nil || project.Dependencies == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if err := project.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProjectJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validProject().Normalized()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Project
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip project = %#v, want %#v", got, want)
	}
}

func validProject() Project {
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
