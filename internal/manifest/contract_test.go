package manifest

import (
	"context"
	"errors"
	"io"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestLimitProfilesAreValidatedAndScalable(t *testing.T) {
	t.Parallel()

	defaultLimits := DefaultLimits()
	largeLimits := LargeRepositoryLimits()
	if err := defaultLimits.Validate(); err != nil {
		t.Fatalf("DefaultLimits().Validate() error = %v", err)
	}
	if err := largeLimits.Validate(); err != nil {
		t.Fatalf("LargeRepositoryLimits().Validate() error = %v", err)
	}
	if largeLimits.MaxFiles <= defaultLimits.MaxFiles ||
		largeLimits.MaxFileBytes <= defaultLimits.MaxFileBytes ||
		largeLimits.MaxTotalBytes <= defaultLimits.MaxTotalBytes {
		t.Fatalf("large limits = %+v, default limits = %+v", largeLimits, defaultLimits)
	}

	invalid := defaultLimits
	invalid.MaxTotalBytes = invalid.MaxFileBytes - 1
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
	}

	invalid = defaultLimits
	invalid.MaxFileBytes = math.MaxInt64
	invalid.MaxTotalBytes = math.MaxInt64
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Validate() error = %v, want overflow-safe ErrInvalidLimits", err)
	}
}

func TestBoundedByteTotalDoesNotOverflow(t *testing.T) {
	t.Parallel()

	if got := boundedByteTotal(math.MaxInt64-4, 10, math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("boundedByteTotal() = %d, want %d", got, int64(math.MaxInt64))
	}
	if got := boundedByteTotal(3, 4, 10); got != 7 {
		t.Fatalf("boundedByteTotal() = %d, want 7", got)
	}
}

func TestNewAnalyzerRejectsDuplicateBackends(t *testing.T) {
	t.Parallel()

	if _, err := NewAnalyzer(DefaultLimits(), nodeParser{}, nodeParser{}); !errors.Is(err, ErrInvalidParser) {
		t.Fatalf("NewAnalyzer() error = %v, want ErrInvalidParser", err)
	}
	var parser *nilContractParser
	if _, err := NewAnalyzer(DefaultLimits(), parser); !errors.Is(err, ErrInvalidParser) {
		t.Fatalf("NewAnalyzer(typed nil) error = %v, want ErrInvalidParser", err)
	}
}

type contractParser struct {
	format    Format
	filenames []string
	consume   bool
	manifest  Manifest
}

type nilContractParser struct{}

func (*nilContractParser) Format() Format { return "nil_parser" }

func (*nilContractParser) Filenames() []string { return []string{"nil.parser"} }

func (*nilContractParser) Parse(context.Context, Document, Limits) (Manifest, error) {
	return Manifest{}, nil
}

func (p contractParser) Format() Format { return p.format }

func (p contractParser) Filenames() []string { return p.filenames }

func (p contractParser) Parse(_ context.Context, document Document, _ Limits) (Manifest, error) {
	if p.consume {
		if _, err := io.Copy(io.Discard, document.Reader); err != nil {
			return Manifest{}, err
		}
	}
	return p.manifest, nil
}

func TestAnalyzerAcceptsCustomAndReplacementBackends(t *testing.T) {
	t.Parallel()

	parser := contractParser{
		format:    "custom_manifest",
		filenames: []string{"project.custom"},
		consume:   true,
		manifest: Manifest{
			Path:   "untrusted",
			Format: FormatGoModule,
			Name:   "custom",
			Dependencies: []Dependency{
				{Name: "z", Scope: ScopeRuntime},
				{Name: "a", Scope: ScopeRuntime},
			},
		},
	}
	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"project.custom": []byte("custom content"),
	}, Snapshot{Files: []string{"project.custom"}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Partial || len(result.Manifests) != 1 {
		t.Fatalf("Analyze() = %+v", result)
	}
	manifest := result.Manifests[0]
	if manifest.Path != "project.custom" || manifest.Format != "custom_manifest" ||
		manifest.Dependencies[0].Name != "a" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestAnalyzerUsesPublicFormatIdentifierGrammar(t *testing.T) {
	t.Parallel()

	for _, format := range []Format{"2custom", "custom-manifest.v2", "custom_manifest"} {
		parser := contractParser{
			format:    format,
			filenames: []string{"project.custom"},
			consume:   true,
		}
		if _, err := NewAnalyzer(DefaultLimits(), parser); err != nil {
			t.Errorf("NewAnalyzer(format %q) error = %v", format, err)
		}
	}

	for _, format := range []Format{
		"", "Custom", "-custom", "custom-", "custom..manifest", "custom-_manifest", "custom/manifest",
	} {
		parser := contractParser{format: format, filenames: []string{"project.custom"}}
		if _, err := NewAnalyzer(DefaultLimits(), parser); !errors.Is(err, ErrInvalidParser) {
			t.Errorf("NewAnalyzer(format %q) error = %v, want ErrInvalidParser", format, err)
		}
	}
}

func TestAnalyzerDoesNotMutateParserOwnedCollections(t *testing.T) {
	t.Parallel()

	parser := contractParser{
		format:    "custom_manifest",
		filenames: []string{"project.custom"},
		consume:   true,
		manifest: Manifest{
			Dependencies: []Dependency{
				{Name: "z", Constraint: "https://example.com/private", Scope: ScopeRuntime},
				{Name: "a", Constraint: "1.0.0", Scope: ScopeRuntime},
			},
			Constraints: []Constraint{
				{Name: "z", Value: "2", Scope: ScopeBuild},
				{Name: "a", Value: "1", Scope: ScopeBuild},
			},
			WorkspaceMembers:  []string{"z", "a"},
			WorkspaceExcludes: []string{"private/z", "private/a"},
		},
	}
	original := parser.manifest
	original.Dependencies = slices.Clone(original.Dependencies)
	original.Constraints = slices.Clone(original.Constraints)
	original.WorkspaceMembers = slices.Clone(original.WorkspaceMembers)
	original.WorkspaceExcludes = slices.Clone(original.WorkspaceExcludes)

	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatal(err)
	}
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"project.custom": []byte("content"),
	}, Snapshot{Files: []string{"project.custom"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Partial || len(result.Manifests) != 1 {
		t.Fatalf("Analyze() = %+v", result)
	}
	if !reflect.DeepEqual(parser.manifest, original) {
		t.Fatalf("parser-owned manifest mutated:\ngot  %+v\nwant %+v", parser.manifest, original)
	}

	result.Manifests[0].Dependencies[0].Name = "changed"
	result.Manifests[0].Constraints[0].Name = "changed"
	result.Manifests[0].WorkspaceMembers[0] = "changed"
	result.Manifests[0].WorkspaceExcludes[0] = "changed"
	if !reflect.DeepEqual(parser.manifest, original) {
		t.Fatalf("result aliases parser-owned manifest:\ngot  %+v\nwant %+v", parser.manifest, original)
	}
}

func TestAnalyzerRejectsBackendThatLeavesUnreadContent(t *testing.T) {
	t.Parallel()

	parser := contractParser{
		format:    FormatNodePackage,
		filenames: []string{"package.json"},
		manifest:  Manifest{Name: "untrusted"},
	}
	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatal(err)
	}
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"package.json": []byte(`{}`),
	}, Snapshot{Files: []string{"package.json"}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !result.Partial || len(result.Manifests) != 0 || len(result.Issues) != 1 ||
		result.Issues[0].Code != IssueParseFailed {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerRejectsInvalidBackendValues(t *testing.T) {
	t.Parallel()

	parser := contractParser{
		format:    "custom",
		filenames: []string{"project.custom"},
		consume:   true,
		manifest:  Manifest{Name: "invalid\nname"},
	}
	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatal(err)
	}
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"project.custom": []byte("content"),
	}, Snapshot{Files: []string{"project.custom"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 0 || result.Issues[0].Code != IssueParseFailed {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerRejectsImpossibleWorkspaceStateFromBackend(t *testing.T) {
	t.Parallel()

	parser := contractParser{
		format: FormatGoModule, filenames: []string{"go.mod"}, consume: true,
		manifest: Manifest{WorkspaceDeclared: true},
	}
	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatal(err)
	}
	result, err := analyzer.Analyze(t.Context(), memorySource{
		"go.mod": []byte("module example.com/test\n"),
	}, Snapshot{Files: []string{"go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 0 || len(result.Issues) != 1 ||
		result.Issues[0].Code != IssueParseFailed {
		t.Fatalf("Analyze() = %+v, want rejected workspace state", result)
	}
}

func TestNewAnalyzerRejectsUnsafeCustomRegistration(t *testing.T) {
	t.Parallel()

	tests := []contractParser{
		{format: "Custom", filenames: []string{"project.custom"}},
		{format: "custom", filenames: []string{"../project.custom"}},
		{format: "custom", filenames: []string{"package.json"}},
		{format: FormatNodePackage, filenames: []string{"project.node"}},
	}
	for _, parser := range tests {
		if _, err := NewAnalyzer(DefaultLimits(), parser); !errors.Is(err, ErrInvalidParser) {
			t.Errorf("NewAnalyzer(%+v) error = %v, want ErrInvalidParser", parser, err)
		}
	}
}

func TestAnalyzerRedactsDependencyReferences(t *testing.T) {
	t.Parallel()

	analyzer := mustDefaultAnalyzer(t, DefaultLimits())
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"package.json": []byte(`{"dependencies":{"private":"https://user:secret@example.com/package.tgz?token=secret"}}`),
	}, Snapshot{Files: []string{"package.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Partial || result.Manifests[0].Dependencies[0].Constraint != redactedReference {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestSanitizeReferenceRedactsLocationsAndPreservesVersionProtocols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "remote URL", value: "https://user:secret@example.com/private", want: redactedReference},
		{name: "link path", value: `link:C:\Users\alice\private`, want: redactedReference},
		{name: "portal path", value: "portal:/srv/private", want: redactedReference},
		{name: "SCP remote", value: "git@github.com:owner/private.git", want: redactedReference},
		{name: "relative parent", value: "../private", want: redactedReference},
		{name: "relative current", value: "./private", want: redactedReference},
		{name: "PEP direct relative", value: "@ ../dist/private.whl", want: redactedReference},
		{name: "UNC path", value: `\\server\share\private`, want: redactedReference},
		{name: "workspace relative", value: "workspace:../private", want: redactedReference},
		{name: "workspace version", value: "workspace:*", want: "workspace:*"},
		{name: "npm alias", value: "npm:@scope/package@^1.0.0", want: "npm:@scope/package@^1.0.0"},
		{name: "catalog alias", value: "catalog:react18", want: "catalog:react18"},
		{name: "semantic version", value: "^1.2.3", want: "^1.2.3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeReference(test.value); got != test.want {
				t.Fatalf("sanitizeReference(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
