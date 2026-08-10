package codeanalysis

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestModelValidateAcceptsCanonicalFacts(t *testing.T) {
	t.Parallel()

	model := validModel()
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := model.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
}

func TestModelValidateAcceptsEnvironmentPortReference(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		reference     string
		referenceKind PortReferenceKind
	}{
		{name: "environment", reference: "HTTP_PORT", referenceKind: PortReferenceEnvironment},
		{name: "configuration", reference: "server.http_port", referenceKind: PortReferenceConfiguration},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			model.PortBindings[0].Port = 0
			model.PortBindings[0].Reference = testCase.reference
			model.PortBindings[0].ReferenceKind = testCase.referenceKind
			if err := model.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestModelValidateRejectsInvalidFacts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Model)
	}{
		{name: "schema", mutate: func(model *Model) { model.SchemaVersion = "2.0" }},
		{name: "service order", mutate: func(model *Model) {
			model.Services[0], model.Services[1] = model.Services[1], model.Services[0]
		}},
		{name: "unsafe service root", mutate: func(model *Model) {
			model.Services[0].Root = "../outside"
		}},
		{name: "entrypoint outside service root", mutate: func(model *Model) {
			model.Services[1].Entrypoints[0] = "cmd/api/main.go"
		}},
		{name: "service without evidence", mutate: func(model *Model) {
			model.Services[0].Evidence = nil
		}},
		{name: "unknown framework service", mutate: func(model *Model) {
			model.Frameworks[0].ServiceID = "unknown"
		}},
		{name: "ambiguous port representation", mutate: func(model *Model) {
			model.PortBindings[0].Reference = "PORT"
		}},
		{name: "missing port representation", mutate: func(model *Model) {
			model.PortBindings[0].Port = 0
		}},
		{name: "reference kind without reference", mutate: func(model *Model) {
			model.PortBindings[0].ReferenceKind = PortReferenceEnvironment
		}},
		{name: "invalid environment port reference", mutate: func(model *Model) {
			model.PortBindings[0].Port = 0
			model.PortBindings[0].Reference = "HTTP-PORT"
			model.PortBindings[0].ReferenceKind = PortReferenceEnvironment
		}},
		{name: "invalid configuration port reference", mutate: func(model *Model) {
			model.PortBindings[0].Port = 0
			model.PortBindings[0].Reference = "server port"
			model.PortBindings[0].ReferenceKind = PortReferenceConfiguration
		}},
		{name: "invalid environment variable", mutate: func(model *Model) {
			model.EnvironmentVariables[0].Name = "HTTP-PORT"
		}},
		{name: "required environment variable with default", mutate: func(model *Model) {
			model.EnvironmentVariables[0].IsRequired = true
		}},
		{name: "unknown resource kind", mutate: func(model *Model) {
			model.ResourceDependencies[0].Kind = "queue"
		}},
		{name: "unknown evidence kind", mutate: func(model *Model) {
			model.Frameworks[0].Evidence[0].Kind = "runtime"
		}},
		{name: "invalid evidence span", mutate: func(model *Model) {
			model.Frameworks[0].Evidence[0].EndColumn = 1
		}},
		{name: "partial without diagnostic", mutate: func(model *Model) {
			model.Partial = true
		}},
		{name: "partial with information only", mutate: func(model *Model) {
			model.Partial = true
			model.Diagnostics = []Diagnostic{{
				Code:    "IATROS_CODE_ANALYSIS_NOTE",
				Level:   DiagnosticInfo,
				Path:    ".",
				Message: "Static analysis completed with an informational note.",
			}}
		}},
		{name: "warning on complete result", mutate: func(model *Model) {
			model.Diagnostics = []Diagnostic{{
				Code:    "IATROS_CODE_ANALYSIS_AMBIGUOUS",
				Level:   DiagnosticWarning,
				Path:    ".",
				Message: "A dynamic expression could not be resolved.",
			}}
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			testCase.mutate(&model)
			if err := model.Validate(); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("Validate() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateAcceptsExplainedPartialResult(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Partial = true
	model.Diagnostics = []Diagnostic{{
		Code:    "IATROS_CODE_ANALYSIS_DYNAMIC_VALUE",
		Level:   DiagnosticWarning,
		Path:    "cmd/api/server.go",
		Message: "A dynamic port expression could not be resolved.",
	}}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelNormalizedOrdersAndDetachesCollections(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Services[0], model.Services[1] = model.Services[1], model.Services[0]
	model.Services[1].Entrypoints = []string{"cmd/api/server.go", "cmd/api/main.go", "cmd/api/main.go"}
	model.Frameworks[0].Evidence = []Evidence{
		model.Frameworks[0].Evidence[1],
		model.Frameworks[0].Evidence[0],
		model.Frameworks[0].Evidence[0],
	}

	normalized := model.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.Services[0].ID != "api" ||
		len(normalized.Services[0].Entrypoints) != 2 ||
		len(normalized.Frameworks[0].Evidence) != 2 {
		t.Fatalf("Normalized() = %#v", normalized)
	}

	normalized.Services[0].Entrypoints[0] = "changed.go"
	normalized.Frameworks[0].Evidence[0].Path = "changed.go"
	if model.Services[1].Entrypoints[0] == "changed.go" ||
		model.Frameworks[0].Evidence[0].Path == "changed.go" {
		t.Fatal("Normalized() retained caller-owned collection storage")
	}
}

func TestModelNormalizedInitializesEmptyCollections(t *testing.T) {
	t.Parallel()

	model := Model{SchemaVersion: CurrentSchemaVersion}.Normalized()
	if model.Services == nil || model.Frameworks == nil || model.PortBindings == nil ||
		model.APIEndpoints == nil || model.EnvironmentVariables == nil ||
		model.ResourceDependencies == nil || model.Diagnostics == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelNormalizedOrdersEveryFactCollection(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Frameworks = append(model.Frameworks, Framework{
		ServiceID: "api", ID: "chi", Name: "Chi", Language: "go",
		Certainty: CertaintyObserved,
		Evidence:  []Evidence{{Kind: EvidenceManifest, Path: "go.mod"}},
	})
	model.PortBindings = append(model.PortBindings, PortBinding{
		ServiceID: "api", Name: "admin", Port: 9090, Protocol: ProtocolHTTP,
		Certainty: CertaintyObserved,
		Evidence:  []Evidence{{Kind: EvidenceSource, Path: "cmd/api/server.go"}},
	})
	model.APIEndpoints = append(model.APIEndpoints, APIEndpoint{
		ServiceID: "api", Protocol: ProtocolHTTP, Method: "get", Path: "/admin",
		Certainty: CertaintyObserved,
		Evidence:  []Evidence{{Kind: EvidenceSource, Path: "cmd/api/server.go"}},
	})
	model.EnvironmentVariables = append(model.EnvironmentVariables, EnvironmentVariable{
		ServiceID: "api", Name: "API_TOKEN", IsRequired: true, IsSensitive: true,
		Certainty: CertaintyObserved,
		Evidence:  []Evidence{{Kind: EvidenceSource, Path: "cmd/api/server.go"}},
	})
	model.ResourceDependencies = append(model.ResourceDependencies, ResourceDependency{
		ServiceID: "api", Kind: ResourceCache, Technology: "redis",
		Certainty: CertaintyObserved,
		Evidence:  []Evidence{{Kind: EvidenceManifest, Path: "go.mod"}},
	})
	model.Diagnostics = []Diagnostic{
		{Code: "IATROS_CODE_NOTE_Z", Level: DiagnosticInfo, Path: ".", Message: "Last note."},
		{Code: "IATROS_CODE_NOTE_A", Level: DiagnosticInfo, Path: ".", Message: "First note."},
	}

	normalized := model.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.Frameworks[0].ID != "chi" ||
		normalized.PortBindings[0].Name != "admin" ||
		normalized.APIEndpoints[0].Path != "/admin" ||
		normalized.EnvironmentVariables[0].Name != "API_TOKEN" ||
		normalized.ResourceDependencies[0].Kind != ResourceCache ||
		normalized.Diagnostics[0].Code != "IATROS_CODE_NOTE_A" {
		t.Fatalf("Normalized() did not order every fact collection: %#v", normalized)
	}
}

func TestModelJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validModel()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Model
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded model = %#v, want %#v", decoded, want)
	}
}

func TestModelValidateWithinEnforcesRetentionLimits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Model, *Limits)
	}{
		{name: "services", mutate: func(_ *Model, limits *Limits) { limits.MaxServices = 1 }},
		{name: "entrypoints", mutate: func(_ *Model, limits *Limits) { limits.MaxFiles = 1 }},
		{name: "evidence", mutate: func(_ *Model, limits *Limits) {
			limits.MaxEvidencePerFact = 1
		}},
		{name: "framework text", mutate: func(model *Model, limits *Limits) {
			model.Frameworks[0].VersionConstraint = "version-too-long"
			limits.MaxTextBytes = 8
		}},
		{name: "port text", mutate: func(model *Model, limits *Limits) {
			model.PortBindings[0].Name = "listener-too-long"
			limits.MaxTextBytes = 8
		}},
		{name: "endpoint text", mutate: func(model *Model, limits *Limits) {
			model.APIEndpoints[0].Operation = "operation-too-long"
			limits.MaxTextBytes = 8
		}},
		{name: "environment text", mutate: func(model *Model, limits *Limits) {
			model.EnvironmentVariables[0].Name = "VARIABLE_TOO_LONG"
			limits.MaxTextBytes = 8
		}},
		{name: "resource text", mutate: func(model *Model, limits *Limits) {
			model.ResourceDependencies[0].Technology = "technology-too-long"
			limits.MaxTextBytes = 8
		}},
		{name: "diagnostic text", mutate: func(model *Model, limits *Limits) {
			model.Diagnostics = []Diagnostic{{
				Code: "IATROS_CODE_NOTE", Level: DiagnosticInfo, Path: ".",
				Message: "Diagnostic message exceeds the retained text budget.",
			}}
			limits.MaxTextBytes = 16
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			limits := DefaultLimits()
			testCase.mutate(&model, &limits)
			if err := model.ValidateWithin(limits); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("ValidateWithin() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateWithinRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxFiles = 0
	if err := validModel().ValidateWithin(limits); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("ValidateWithin() error = %v, want ErrInvalidLimits", err)
	}
}

func TestEvidencePositionValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		position   Evidence
		isAccepted bool
	}{
		{name: "whole file", position: Evidence{}, isAccepted: true},
		{name: "single point", position: Evidence{
			StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1,
		}, isAccepted: true},
		{name: "multiple lines", position: Evidence{
			StartLine: 2, StartColumn: 10, EndLine: 3, EndColumn: 1,
		}, isAccepted: true},
		{name: "partial position", position: Evidence{StartLine: 1}},
		{name: "reversed line", position: Evidence{
			StartLine: 2, StartColumn: 1, EndLine: 1, EndColumn: 1,
		}},
		{name: "reversed column", position: Evidence{
			StartLine: 1, StartColumn: 2, EndLine: 1, EndColumn: 1,
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if actual := validEvidencePosition(testCase.position); actual != testCase.isAccepted {
				t.Fatalf("validEvidencePosition(%+v) = %t, want %t", testCase.position, actual, testCase.isAccepted)
			}
		})
	}
}

func BenchmarkModelNormalizedAndValidated(b *testing.B) {
	model := validModel()
	for b.Loop() {
		normalized := model.Normalized()
		if err := normalized.ValidateWithin(DefaultLimits()); err != nil {
			b.Fatalf("ValidateWithin() error = %v", err)
		}
	}
}

func validModel() Model {
	frameworkEvidence := []Evidence{
		{
			Kind: EvidenceSource, Path: "cmd/api/main.go",
			StartLine: 8, StartColumn: 2, EndLine: 8, EndColumn: 35,
		},
		{Kind: EvidenceManifest, Path: "go.mod"},
	}
	return Model{
		SchemaVersion: CurrentSchemaVersion,
		Services: []Service{
			{
				ID: "api", Name: "API", Kind: "application", Root: ".",
				Entrypoints: []string{"cmd/api/main.go", "cmd/api/server.go"},
				Certainty:   CertaintyObserved,
				Evidence: []Evidence{{
					Kind: EvidenceSource, Path: "cmd/api/main.go",
					StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 13,
				}},
			},
			{
				ID: "worker", Name: "Worker", Kind: "worker", Root: "cmd/worker",
				Entrypoints: []string{"cmd/worker/main.go"}, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceSource, Path: "cmd/worker/main.go"}},
			},
		},
		Frameworks: []Framework{{
			ServiceID: "api", ID: "gin", Name: "Gin", Language: "go",
			VersionConstraint: "v1.10.0", Certainty: CertaintyObserved,
			Evidence: frameworkEvidence,
		}},
		PortBindings: []PortBinding{{
			ServiceID: "api", Name: "http", Port: 8080, Protocol: ProtocolHTTP,
			Certainty: CertaintyObserved,
			Evidence: []Evidence{{
				Kind: EvidenceSource, Path: "cmd/api/server.go",
				StartLine: 20, StartColumn: 15, EndLine: 20, EndColumn: 21,
			}},
		}},
		APIEndpoints: []APIEndpoint{{
			ServiceID: "api", Protocol: ProtocolHTTP, Method: "get", Path: "/health",
			Operation: "health", Certainty: CertaintyObserved,
			Evidence: []Evidence{{
				Kind: EvidenceSource, Path: "cmd/api/server.go",
				StartLine: 24, StartColumn: 2, EndLine: 24, EndColumn: 38,
			}},
		}},
		EnvironmentVariables: []EnvironmentVariable{{
			ServiceID: "api", Name: "HTTP_PORT", IsRequired: false, HasDefault: true,
			Certainty: CertaintyObserved,
			Evidence: []Evidence{{
				Kind: EvidenceSource, Path: "cmd/api/server.go",
				StartLine: 14, StartColumn: 10, EndLine: 14, EndColumn: 32,
			}},
		}},
		ResourceDependencies: []ResourceDependency{{
			ServiceID: "api", Kind: ResourceDatabase, Technology: "postgresql",
			Name: "primary", Certainty: CertaintyInferred,
			Evidence: []Evidence{{Kind: EvidenceManifest, Path: "go.mod"}},
		}},
		Diagnostics: make([]Diagnostic, 0),
	}
}
