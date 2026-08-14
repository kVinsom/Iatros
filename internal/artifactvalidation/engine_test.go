package artifactvalidation

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDefaultEngineValidatesExistingAndGeneratedArtifacts(t *testing.T) {
	t.Parallel()

	contents := map[string][]byte{
		"compose":    []byte("services:\n  api:\n    image: api:1.0\n"),
		"dockerfile": []byte("FROM alpine:3.20\nUSER 1000\n"),
		"json":       []byte(`{"enabled":true}`),
		"kubernetes": []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n"),
		"terraform":  []byte("resource \"test_instance\" \"api\" {}\n"),
		"xml":        []byte("<configuration><enabled>true</enabled></configuration>"),
	}
	source := NewMemorySource(contents)
	requests := []Request{
		requestFor("terraform", OriginExisting, KindTerraform, FormatHCL, source),
		requestFor("compose", OriginExisting, KindCompose, FormatYAML, source),
		requestFor("xml", OriginGenerated, KindGeneric, FormatXML, source),
		requestFor("json", OriginGenerated, KindGeneric, FormatJSON, source),
		requestFor("dockerfile", OriginGenerated, KindDockerfile, FormatDockerfile, source),
		requestFor("kubernetes", OriginGenerated, KindKubernetes, FormatYAML, source),
	}
	engine, err := NewDefaultEngine(DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultEngine() error = %v", err)
	}
	report, err := engine.Validate(t.Context(), requests)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status != StatusPassed || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v, want passed without diagnostics", report)
	}
	if len(report.Artifacts) != len(requests) || report.Artifacts[0].Artifact.ID != "compose" {
		t.Fatalf("artifacts = %#v, want every artifact in ID order", report.Artifacts)
	}
	for _, result := range report.Artifacts {
		if result.Outcome != OutcomeValid || !strings.HasPrefix(result.Digest, "sha256:") ||
			len(result.Validators) < 2 {
			t.Fatalf("result = %#v, want valid digest and validators", result)
		}
	}
	if err := report.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
}

func TestDefaultEngineReturnsNormalizedSyntaxAndStructureDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       Kind
		format     Format
		content    string
		wantedCode string
	}{
		{name: "multiple JSON values", kind: KindGeneric, format: FormatJSON, content: "{} {}", wantedCode: "JSON_MULTIPLE_VALUES"},
		{name: "XML directive", kind: KindGeneric, format: FormatXML, content: "<!DOCTYPE x><x/>", wantedCode: "XML_DIRECTIVE_UNSUPPORTED"},
		{name: "duplicate YAML key", kind: KindGeneric, format: FormatYAML, content: "a: 1\na: 2\n", wantedCode: "YAML_DUPLICATE_KEY"},
		{name: "invalid HCL", kind: KindTerraform, format: FormatHCL, content: "resource {", wantedCode: "HCL_SYNTAX_INVALID"},
		{name: "Dockerfile missing FROM", kind: KindDockerfile, format: FormatDockerfile, content: "RUN true\n", wantedCode: "DOCKERFILE_BEFORE_FROM"},
		{name: "Compose missing services", kind: KindCompose, format: FormatYAML, content: "name: demo\n", wantedCode: "COMPOSE_SERVICES_MISSING"},
		{name: "Kubernetes missing name", kind: KindKubernetes, format: FormatYAML, content: "apiVersion: v1\nkind: Service\n", wantedCode: "KUBERNETES_NAME_MISSING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := NewMemorySource(map[string][]byte{"artifact": []byte(test.content)})
			engine, err := NewDefaultEngine(DefaultLimits())
			if err != nil {
				t.Fatalf("NewDefaultEngine() error = %v", err)
			}
			report, err := engine.Validate(t.Context(), []Request{
				requestFor("artifact", OriginGenerated, test.kind, test.format, source),
			})
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if report.Status != StatusFailed || !hasDiagnosticCode(report.Diagnostics, test.wantedCode) {
				t.Fatalf("report = %#v, want failed with %s", report, test.wantedCode)
			}
		})
	}
}

func TestEngineEnforcesByteDepthNodeAndAliasBudgets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		mutate     func(*Limits)
		wantedCode string
		status     Status
	}{
		{name: "artifact bytes", content: strings.Repeat("x", 513), mutate: func(limits *Limits) {
			limits.MaxArtifactBytes = 512
			limits.MaxTotalBytes = 1024
			limits.MaxTextBytes = 256
		}, wantedCode: "ARTIFACT_BYTES_EXCEEDED", status: StatusPartial},
		{name: "JSON depth", content: `[[[1]]]`, mutate: func(limits *Limits) {
			limits.MaxNestingDepth = 2
		}, wantedCode: "JSON_DEPTH_EXCEEDED", status: StatusFailed},
		{name: "JSON nodes", content: `[1,2,3]`, mutate: func(limits *Limits) {
			limits.MaxSyntaxNodes = 3
		}, wantedCode: "JSON_NODES_EXCEEDED", status: StatusFailed},
		{name: "YAML aliases", content: "a: &a value\nb: *a\nc: *a\n", mutate: func(limits *Limits) {
			limits.MaxAliases = 1
		}, wantedCode: "YAML_ALIASES_EXCEEDED", status: StatusFailed},
		{name: "YAML cumulative nodes", content: "---\na: 1\n---\nb: 2\n", mutate: func(limits *Limits) {
			limits.MaxSyntaxNodes = 6
		}, wantedCode: "YAML_NODES_EXCEEDED", status: StatusFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limits := DefaultLimits()
			test.mutate(&limits)
			engine, err := NewDefaultEngine(limits)
			if err != nil {
				t.Fatalf("NewDefaultEngine() error = %v", err)
			}
			format := FormatJSON
			if strings.Contains(test.name, "YAML") {
				format = FormatYAML
			}
			if strings.Contains(test.name, "artifact bytes") {
				format = FormatText
			}
			source := NewMemorySource(map[string][]byte{"artifact": []byte(test.content)})
			report, err := engine.Validate(t.Context(), []Request{
				requestFor("artifact", OriginGenerated, KindGeneric, format, source),
			})
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if report.Status != test.status || !hasDiagnosticCode(report.Diagnostics, test.wantedCode) {
				t.Fatalf("report = %#v, want %s with %s", report, test.status, test.wantedCode)
			}
		})
	}
}

func TestEngineHandlesSourceAndValidatorFailuresWithoutLeakingErrors(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	engine, err := NewEngine(limits, failingValidator{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	report, err := engine.Validate(t.Context(), []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatText, failingSource{}),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status != StatusPartial || !hasDiagnosticCode(report.Diagnostics, "SOURCE_UNAVAILABLE") ||
		strings.Contains(report.Diagnostics[0].Message, "secret") {
		t.Fatalf("report = %#v, want safe source diagnostic", report)
	}

	source := NewMemorySource(map[string][]byte{"artifact": []byte("text")})
	report, err = engine.Validate(t.Context(), []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatText, source),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status != StatusPartial || !hasDiagnosticCode(report.Diagnostics, "VALIDATOR_FAILED") ||
		strings.Contains(report.Diagnostics[0].Message, "secret") {
		t.Fatalf("report = %#v, want safe validator diagnostic", report)
	}
}

func TestEngineRejectsInvalidConfigurationAndRequests(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	if _, err := NewEngine(limits); !errors.Is(err, ErrInvalidValidator) {
		t.Fatalf("NewEngine() error = %v, want ErrInvalidValidator", err)
	}
	if _, err := NewEngine(limits, nil); !errors.Is(err, ErrInvalidValidator) {
		t.Fatalf("NewEngine(nil) error = %v, want ErrInvalidValidator", err)
	}
	if _, err := NewEngine(limits, contentValidator{}, contentValidator{}); !errors.Is(err, ErrInvalidValidator) {
		t.Fatalf("NewEngine(duplicate) error = %v, want ErrInvalidValidator", err)
	}
	engine, err := NewDefaultEngine(limits)
	if err != nil {
		t.Fatalf("NewDefaultEngine() error = %v", err)
	}
	if _, err := engine.Validate(t.Context(), nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate(nil) error = %v, want ErrInvalidRequest", err)
	}
	source := NewMemorySource(map[string][]byte{"artifact": []byte("text")})
	request := requestFor("artifact", OriginGenerated, KindDockerfile, FormatYAML, source)
	if _, err := engine.Validate(t.Context(), []Request{request}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate(incompatible) error = %v, want ErrInvalidRequest", err)
	}
	valid := requestFor("artifact", OriginGenerated, KindGeneric, FormatText, source)
	if _, err := engine.Validate(t.Context(), []Request{valid, valid}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate(duplicate) error = %v, want ErrInvalidRequest", err)
	}
}

func TestEngineNormalizesMalformedAndExcessiveValidatorDiagnostics(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxDiagnosticsPerArtifact = 2
	limits.MaxDiagnostics = 2
	engine, err := NewEngine(limits, noisyValidator{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	source := NewMemorySource(map[string][]byte{"artifact": []byte("text")})
	report, err := engine.Validate(t.Context(), []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatText, source),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status != StatusPartial || !hasDiagnosticCode(report.Diagnostics, "DIAGNOSTICS_TRUNCATED") {
		t.Fatalf("report = %#v, want partial diagnostic truncation", report)
	}

	engine, err = NewEngine(DefaultLimits(), malformedDiagnosticValidator{})
	if err != nil {
		t.Fatalf("NewEngine(malformed) error = %v", err)
	}
	report, err = engine.Validate(t.Context(), []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatText, source),
	})
	if err != nil {
		t.Fatalf("Validate(malformed) error = %v", err)
	}
	if report.Status != StatusFailed || !hasDiagnosticCode(report.Diagnostics, "INVALID_VALIDATOR_DIAGNOSTIC") {
		t.Fatalf("report = %#v, want normalized malformed diagnostic", report)
	}
}

func TestDockerfileValidatorStopsAtDiagnosticBudgetBoundary(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxDiagnosticsPerArtifact = 2
	document := Document{content: []byte(strings.Repeat("UNKNOWN argument\n", 100))}
	diagnostics, err := (dockerfileSyntaxValidator{}).Validate(t.Context(), document, limits)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(diagnostics) != limits.MaxDiagnosticsPerArtifact+1 {
		t.Fatalf("len(diagnostics) = %d, want one overflow marker candidate", len(diagnostics))
	}
}

func TestEngineReportsGlobalDiagnosticTruncationWithoutUnavailableArtifact(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxDiagnosticsPerArtifact = 1
	limits.MaxDiagnostics = 1
	engine, err := NewEngine(limits, oneDiagnosticValidator{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	source := NewMemorySource(map[string][]byte{
		"first": []byte("first"), "second": []byte("second"),
	})
	report, err := engine.Validate(t.Context(), []Request{
		requestFor("first", OriginGenerated, KindGeneric, FormatText, source),
		requestFor("second", OriginGenerated, KindGeneric, FormatText, source),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status != StatusPartial || !hasDiagnosticCode(report.Diagnostics, "DIAGNOSTICS_TRUNCATED") {
		t.Fatalf("report = %#v, want explicit global truncation", report)
	}
	for _, result := range report.Artifacts {
		if result.Outcome == OutcomeUnavailable {
			t.Fatalf("result = %#v, global truncation should not rewrite artifact outcome", result)
		}
	}
}

func TestReportNormalizationIsDetachedAndValidationRejectsStatusMismatch(t *testing.T) {
	t.Parallel()

	engine, err := NewDefaultEngine(DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultEngine() error = %v", err)
	}
	source := NewMemorySource(map[string][]byte{"artifact": []byte(`{"ok":true}`)})
	report, err := engine.Validate(t.Context(), []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatJSON, source),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	normalized := report.Normalized()
	report.Artifacts[0].Validators[0] = "changed.validator"
	if normalized.Artifacts[0].Validators[0] == "changed.validator" {
		t.Fatal("Normalized() retained mutable validator storage")
	}
	normalized.Status = StatusFailed
	if err := normalized.Validate(); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("Validate(status mismatch) error = %v, want ErrInvalidReport", err)
	}
}

func TestEngineHonorsCancellation(t *testing.T) {
	t.Parallel()

	engine, err := NewDefaultEngine(DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultEngine() error = %v", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	source := NewMemorySource(map[string][]byte{"artifact": []byte("text")})
	report, err := engine.Validate(cancelled, []Request{
		requestFor("artifact", OriginGenerated, KindGeneric, FormatText, source),
	})
	if !errors.Is(err, context.Canceled) || report.Status != StatusPartial {
		t.Fatalf("Validate() = (%#v, %v), want partial context cancellation", report, err)
	}
}

func requestFor(id string, origin Origin, kind Kind, format Format, source Source) Request {
	return Request{
		Artifact: Artifact{
			ID: id, Path: id + ".config", Origin: origin, Kind: kind, Format: format,
			Producer: "iatros.test", ProducerVersion: "1.0",
		},
		Source: source,
	}
}

func hasDiagnosticCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

type failingSource struct{}

func (failingSource) Open(context.Context, Artifact) (io.ReadCloser, error) {
	return nil, errors.New("secret source failure")
}

type failingValidator struct{}

func (failingValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "test.validator", Version: "1.0"}
}

func (failingValidator) AppliesTo(Artifact) bool {
	return true
}

func (failingValidator) Validate(context.Context, Document, Limits) ([]Diagnostic, error) {
	return nil, errors.New("secret validator failure")
}

type noisyValidator struct{}

func (noisyValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "test.noisy", Version: "1.0"}
}

func (noisyValidator) AppliesTo(Artifact) bool {
	return true
}

func (noisyValidator) Validate(context.Context, Document, Limits) ([]Diagnostic, error) {
	return []Diagnostic{
		{Code: "NOISY_ONE", Level: DiagnosticWarning, Message: "First warning."},
		{Code: "NOISY_TWO", Level: DiagnosticWarning, Message: "Second warning."},
		{Code: "NOISY_THREE", Level: DiagnosticWarning, Message: "Third warning."},
	}, nil
}

type malformedDiagnosticValidator struct{}

func (malformedDiagnosticValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "test.malformed", Version: "1.0"}
}

func (malformedDiagnosticValidator) AppliesTo(Artifact) bool {
	return true
}

func (malformedDiagnosticValidator) Validate(context.Context, Document, Limits) ([]Diagnostic, error) {
	return []Diagnostic{{Code: "unsafe-code", Message: "unsafe\x00message"}}, nil
}

type oneDiagnosticValidator struct{}

func (oneDiagnosticValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "test.one-diagnostic", Version: "1.0"}
}

func (oneDiagnosticValidator) AppliesTo(Artifact) bool {
	return true
}

func (oneDiagnosticValidator) Validate(context.Context, Document, Limits) ([]Diagnostic, error) {
	return []Diagnostic{{Code: "TEST_ERROR", Level: DiagnosticError, Message: "Test error."}}, nil
}
