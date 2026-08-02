package detection

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDefaultLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	if limits.MaxEvidencePerTechnology != 20 {
		t.Fatalf("MaxEvidencePerTechnology = %d, want 20", limits.MaxEvidencePerTechnology)
	}
	if err := limits.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMarkerDetectorRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxEvidencePerTechnology = 0
	if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
	}
	if _, err := NewMarkerDetector(limits); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("NewMarkerDetector() error = %v, want ErrInvalidLimits", err)
	}

	var detector MarkerDetector
	results, err := detector.Detect(t.Context(), []string{"go.mod"})
	if !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Detect() error = %v, want ErrInvalidLimits", err)
	}
	if results == nil || len(results) != 0 {
		t.Fatalf("results = %#v, want a non-nil empty slice", results)
	}
}

func TestMarkerDetectorDetectsBroadTechnologyCatalog(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	files := []string{
		"services/api/go.mod",
		"apps/web/package.json",
		"apps/web/pnpm-lock.yaml",
		"infra/main.tf",
		"deploy/base/kustomization.yaml",
		"deploy/charts/api/Chart.yaml",
		"services/api/Dockerfile",
		"automation/ansible.cfg",
		"nested/.github/workflows/ci.yml",
		"clusters/production/gotk-sync.yaml",
		"monitoring/prometheus.yaml",
		"gateway/nginx.conf",
		"nested/.github/dependabot.yml",
		"environments/.sops.yaml",
		"workers/wrangler.toml",
		"native/CMakeLists.txt",
		"services/api/go.mod",
	}

	first, err := detector.Detect(t.Context(), files)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	slices.Reverse(files)
	second, err := detector.Detect(t.Context(), files)
	if err != nil {
		t.Fatalf("second Detect() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("detection is not deterministic:\nfirst: %#v\nsecond: %#v", first, second)
	}

	want := map[string]Category{
		"ansible":              CategoryConfigurationManagement,
		"cloudflare":           CategoryCloudPlatform,
		"cmake":                CategoryBuildSystem,
		"dependabot":           CategorySecurity,
		"docker":               CategoryContainer,
		"flux":                 CategoryGitOps,
		"github-actions":       CategoryCICD,
		"go":                   CategoryLanguage,
		"go-modules":           CategoryDependencyManager,
		"helm":                 CategoryOrchestration,
		"kubernetes":           CategoryOrchestration,
		"kustomize":            CategoryOrchestration,
		"nginx":                CategoryNetworking,
		"nodejs":               CategoryRuntime,
		"pnpm":                 CategoryDependencyManager,
		"prometheus":           CategoryObservability,
		"sops":                 CategorySecrets,
		"terraform-compatible": CategoryInfrastructureAsCode,
	}
	if len(first) != len(want) {
		t.Fatalf("results = %#v, want %d technologies", first, len(want))
	}
	for _, technology := range first {
		category, exists := want[technology.ID]
		if !exists {
			t.Fatalf("unexpected technology %#v", technology)
		}
		if technology.Category != category {
			t.Fatalf("technology %q category = %q, want %q", technology.ID, technology.Category, category)
		}
		if len(technology.Evidence) == 0 || technology.EvidenceTruncated {
			t.Fatalf("technology = %#v, want complete direct evidence", technology)
		}
	}
	if !technologyIDsSorted(first) {
		t.Fatalf("technology IDs are not sorted: %#v", first)
	}

	goTechnology := findTechnology(t, first, "go")
	if !slices.Equal(goTechnology.Evidence, []string{"services/api/go.mod"}) {
		t.Fatalf("go evidence = %#v, want deduplicated go.mod", goTechnology.Evidence)
	}
}

func TestMarkerDetectorSupportsBroadDependencyManagers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   string
		file string
	}{
		{id: "bun", file: "bun.lock"},
		{id: "cargo", file: "Cargo.lock"},
		{id: "clojure-cli", file: "deps.edn"},
		{id: "composer", file: "composer.lock"},
		{id: "conan", file: "conanfile.py"},
		{id: "conda", file: "environment.yml"},
		{id: "go-modules", file: "go.mod"},
		{id: "gradle", file: "build.gradle.kts"},
		{id: "leiningen", file: "project.clj"},
		{id: "maven", file: "pom.xml"},
		{id: "mix", file: "mix.lock"},
		{id: "npm", file: "package-lock.json"},
		{id: "nuget", file: "Directory.Packages.props"},
		{id: "pdm", file: "pdm.lock"},
		{id: "pip", file: "requirements-dev.txt"},
		{id: "pipenv", file: "Pipfile.lock"},
		{id: "pnpm", file: "pnpm-lock.yaml"},
		{id: "poetry", file: "poetry.lock"},
		{id: "rebar3", file: "rebar.config"},
		{id: "sbt", file: "build.sbt"},
		{id: "uv", file: "uv.lock"},
		{id: "vcpkg", file: "vcpkg.json"},
		{id: "yarn", file: "yarn.lock"},
	}

	detector := newTestDetector(t, DefaultLimits())
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			t.Parallel()

			results, err := detector.Detect(t.Context(), []string{"nested/" + test.file})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			technology := findTechnology(t, results, test.id)
			if technology.Category != CategoryDependencyManager {
				t.Fatalf("category = %q, want %q", technology.Category, CategoryDependencyManager)
			}
		})
	}
}

func TestMarkerDetectorSupportsCommonBackendLanguages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   string
		file string
	}{
		{id: "c", file: "main.c"},
		{id: "clojure", file: "core.clj"},
		{id: "cpp", file: "main.cpp"},
		{id: "csharp", file: "Program.cs"},
		{id: "elixir", file: "application.ex"},
		{id: "erlang", file: "application.erl"},
		{id: "fsharp", file: "Program.fs"},
		{id: "go", file: "main.go"},
		{id: "java", file: "Main.java"},
		{id: "javascript", file: "server.mjs"},
		{id: "kotlin", file: "Application.kt"},
		{id: "php", file: "index.php"},
		{id: "python", file: "application.py"},
		{id: "ruby", file: "application.rb"},
		{id: "rust", file: "main.rs"},
		{id: "scala", file: "Main.scala"},
		{id: "typescript", file: "server.ts"},
		{id: "visual-basic-dotnet", file: "Program.vb"},
	}

	detector := newTestDetector(t, DefaultLimits())
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			t.Parallel()

			results, err := detector.Detect(t.Context(), []string{"service/" + test.file})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			technology := findTechnology(t, results, test.id)
			if technology.Category != CategoryLanguage {
				t.Fatalf("category = %q, want %q", technology.Category, CategoryLanguage)
			}
		})
	}
}

func TestMarkerDetectorBoundsEvidenceDeterministically(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxEvidencePerTechnology = 2
	detector := newTestDetector(t, limits)
	results, err := detector.Detect(t.Context(), []string{
		"z/main.go",
		"a/main.go",
		"m/main.go",
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	technology := findTechnology(t, results, "go")
	if !slices.Equal(technology.Evidence, []string{"a/main.go", "m/main.go"}) {
		t.Fatalf("Evidence = %#v, want deterministic bounded evidence", technology.Evidence)
	}
	if !technology.EvidenceTruncated {
		t.Fatal("EvidenceTruncated = false, want true")
	}
}

func TestMarkerDetectorRejectsUnsafeEvidencePaths(t *testing.T) {
	t.Parallel()

	unsafePaths := []string{
		"",
		".",
		"../go.mod",
		"a/../go.mod",
		"/go.mod",
		"C:/repository/go.mod",
		`nested\go.mod`,
		"unsafe\nname.go",
	}
	detector := newTestDetector(t, DefaultLimits())
	for _, unsafePath := range unsafePaths {
		results, err := detector.Detect(t.Context(), []string{unsafePath})
		if !errors.Is(err, ErrInvalidEvidencePath) {
			t.Fatalf("Detect(%q) error = %v, want ErrInvalidEvidencePath", unsafePath, err)
		}
		if results == nil || len(results) != 0 {
			t.Fatalf("Detect(%q) results = %#v, want non-nil empty slice", unsafePath, results)
		}
	}
}

func TestMarkerDetectorHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	detector := newTestDetector(t, DefaultLimits())
	results, err := detector.Detect(ctx, []string{"go.mod"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Detect() error = %v, want context.Canceled", err)
	}
	if results == nil || len(results) != 0 {
		t.Fatalf("results = %#v, want non-nil empty slice", results)
	}
}

func TestMarkerDetectorDiscardsResultsAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx := &cancelAfterChecksContext{
		Context:         t.Context(),
		checksRemaining: 4,
	}
	detector := newTestDetector(t, DefaultLimits())
	results, err := detector.Detect(ctx, []string{"a/main.go", "b/main.py"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Detect() error = %v, want context.Canceled", err)
	}
	if results == nil || len(results) != 0 {
		t.Fatalf("results = %#v, want discarded partial results", results)
	}
}

func TestMarkerDetectorIgnoresUnreliableGenericNames(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	results, err := detector.Detect(t.Context(), []string{
		"config.yaml",
		"deployment.yaml",
		"lock.json",
		"manifest.json",
		"template.yaml",
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %#v, want no unsupported inference", results)
	}
}

func TestDefaultMarkerCatalogIsValid(t *testing.T) {
	t.Parallel()

	ids := make(map[string]struct{}, len(defaultMarkerRules))
	for _, rule := range defaultMarkerRules {
		if !validTestIdentifier(rule.id) {
			t.Fatalf("invalid marker ID %q", rule.id)
		}
		if _, exists := ids[rule.id]; exists {
			t.Fatalf("duplicate marker ID %q", rule.id)
		}
		ids[rule.id] = struct{}{}
		if !knownCategory(rule.category) {
			t.Fatalf("rule %q has unknown category %q", rule.id, rule.category)
		}
		if len(rule.patterns) == 0 {
			t.Fatalf("rule %q has no marker patterns", rule.id)
		}
		for _, pattern := range rule.patterns {
			if pattern == "" || strings.Contains(pattern, "\\") {
				t.Fatalf("rule %q has invalid pattern %q", rule.id, pattern)
			}
			if _, err := path.Match(pattern, "probe"); err != nil {
				t.Fatalf("rule %q pattern %q error = %v", rule.id, pattern, err)
			}
		}
	}
}

func BenchmarkMarkerDetector(b *testing.B) {
	detector := newTestDetector(b, DefaultLimits())
	files := make([]string, 2_000)
	markers := []string{
		"main.go",
		"package-lock.json",
		"pyproject.toml",
		"main.tf",
		"Dockerfile",
		"kustomization.yaml",
		"prometheus.yaml",
		"nginx.conf",
		".github/workflows/ci.yml",
		"README.md",
	}
	for index := range files {
		files[index] = fmt.Sprintf("projects/%04d/%s", index, markers[index%len(markers)])
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := detector.Detect(context.Background(), files); err != nil {
			b.Fatalf("Detect() error = %v", err)
		}
	}
}

func newTestDetector(t testing.TB, limits Limits) MarkerDetector {
	t.Helper()

	detector, err := NewMarkerDetector(limits)
	if err != nil {
		t.Fatalf("NewMarkerDetector() error = %v", err)
	}
	return detector
}

func findTechnology(t *testing.T, technologies []Technology, id string) Technology {
	t.Helper()

	for _, technology := range technologies {
		if technology.ID == id {
			return technology
		}
	}
	t.Fatalf("technology %q not found in %#v", id, technologies)
	return Technology{}
}

func technologyIDsSorted(technologies []Technology) bool {
	for index := 1; index < len(technologies); index++ {
		if technologies[index-1].ID >= technologies[index].ID {
			return false
		}
	}
	return true
}

func validTestIdentifier(value string) bool {
	if value == "" || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func knownCategory(category Category) bool {
	switch category {
	case CategoryLanguage,
		CategoryRuntime,
		CategoryDependencyManager,
		CategoryBuildSystem,
		CategoryContainer,
		CategoryOrchestration,
		CategoryInfrastructureAsCode,
		CategoryConfigurationManagement,
		CategoryCICD,
		CategoryGitOps,
		CategoryObservability,
		CategoryNetworking,
		CategorySecurity,
		CategorySecrets,
		CategoryCloudPlatform:
		return true
	default:
		return false
	}
}

type cancelAfterChecksContext struct {
	context.Context
	checksRemaining int
}

func (ctx *cancelAfterChecksContext) Err() error {
	if ctx.checksRemaining == 0 {
		return context.Canceled
	}
	ctx.checksRemaining--
	return nil
}
