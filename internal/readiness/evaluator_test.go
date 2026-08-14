package readiness

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestEvaluatorReportsOnlyRootFoundationsForEmptyRepository(t *testing.T) {
	t.Parallel()

	findings, err := (Evaluator{}).Evaluate(t.Context(), Snapshot{
		Directories: []string{"."},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	wantCodes := []string{
		FindingCodeReadmeMissing,
		FindingCodeGitignoreMissing,
		FindingCodeLicenseMissing,
	}
	if !slices.Equal(findingCodes(findings), wantCodes) {
		t.Fatalf("finding codes = %#v, want %#v", findingCodes(findings), wantCodes)
	}
	for _, finding := range findings {
		assertValidFinding(t, finding)
	}
}

func TestEvaluatorReturnsNoFindingsForReadyCodeRepository(t *testing.T) {
	t.Parallel()

	findings, err := (Evaluator{}).Evaluate(t.Context(), Snapshot{
		Directories: []string{".", "internal", "internal/tests"},
		Files: []string{
			".gitignore",
			"LICENSE-APACHE",
			"ReadMe.md",
			"internal/main.go",
			"internal/tests/main_test.go",
		},
		Technologies: []Technology{
			{ID: "github-actions", Category: TechnologyCategoryCICD},
			{ID: "go", Category: TechnologyCategoryLanguage},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, want a non-nil empty slice", findings)
	}
}

func TestEvaluatorRequiresRepositoryFoundationsAtRoot(t *testing.T) {
	t.Parallel()

	findings, err := (Evaluator{}).Evaluate(t.Context(), Snapshot{
		Directories: []string{".", "config", "docs", "legal"},
		Files: []string{
			"config/.gitignore",
			"docs/README.md",
			"legal/LICENSE",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	wantCodes := []string{
		FindingCodeReadmeMissing,
		FindingCodeGitignoreMissing,
		FindingCodeLicenseMissing,
	}
	if !slices.Equal(findingCodes(findings), wantCodes) {
		t.Fatalf("finding codes = %#v, want %#v", findingCodes(findings), wantCodes)
	}
}

func TestEvaluatorReportsMissingTestsAndCIForCodeRepository(t *testing.T) {
	t.Parallel()

	snapshot := completeFoundationsSnapshot()
	snapshot.Files = append(snapshot.Files, "cmd/main.go", "web/server.ts")
	snapshot.Technologies = []Technology{
		{ID: "nodejs", Category: TechnologyCategoryRuntime},
		{ID: "go", Category: TechnologyCategoryLanguage},
		{ID: "docker", Category: TechnologyCategory("container")},
		{ID: "go", Category: TechnologyCategoryLanguage},
	}

	findings, err := (Evaluator{}).Evaluate(t.Context(), snapshot)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	wantCodes := []string{FindingCodeCINotDetected, FindingCodeTestsNotDetected}
	if !slices.Equal(findingCodes(findings), wantCodes) {
		t.Fatalf("finding codes = %#v, want %#v", findingCodes(findings), wantCodes)
	}

	testsFinding := findFinding(t, findings, FindingCodeTestsNotDetected)
	wantEvidence := "Detected code ecosystems: go, nodejs."
	if len(testsFinding.Evidence) != 1 || testsFinding.Evidence[0].Description != wantEvidence {
		t.Fatalf("test evidence = %#v, want %q", testsFinding.Evidence, wantEvidence)
	}
}

func TestEvaluatorIgnoresTestMarkersFromDependencyDirectories(t *testing.T) {
	t.Parallel()

	snapshot := completeFoundationsSnapshot()
	snapshot.Directories = append(snapshot.Directories, "node_modules", "node_modules/library/test")
	snapshot.Files = append(snapshot.Files, "cmd/main.go", "node_modules/library/helper_test.go")
	snapshot.Technologies = []Technology{{ID: "go", Category: TechnologyCategoryLanguage}}

	findings, err := (Evaluator{}).Evaluate(t.Context(), snapshot)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if findFinding(t, findings, FindingCodeTestsNotDetected).RuleID != FindingCodeTestsNotDetected {
		t.Fatalf("findings = %#v, want missing-tests finding", findings)
	}
}

func TestEvaluatorDoesNotRequireCodeTestsForInfrastructureRepository(t *testing.T) {
	t.Parallel()

	snapshot := completeFoundationsSnapshot()
	snapshot.Files = append(snapshot.Files, "infra/main.tf")
	snapshot.Technologies = []Technology{
		{ID: "terraform-compatible", Category: TechnologyCategory("infrastructure_as_code")},
	}

	findings, err := (Evaluator{}).Evaluate(t.Context(), snapshot)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	wantCodes := []string{FindingCodeCINotDetected}
	if !slices.Equal(findingCodes(findings), wantCodes) {
		t.Fatalf("finding codes = %#v, want %#v", findingCodes(findings), wantCodes)
	}
}

func TestEvaluatorSuppressesAbsenceRulesForPartialSnapshot(t *testing.T) {
	t.Parallel()

	findings, err := (Evaluator{}).Evaluate(t.Context(), Snapshot{
		Directories: []string{"."},
		Technologies: []Technology{
			{ID: "go", Category: TechnologyCategoryLanguage},
		},
		Partial: true,
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, want suppressed absence findings", findings)
	}
}

func TestEvaluatorUsesNestedTestAndCIMarkers(t *testing.T) {
	t.Parallel()

	snapshot := completeFoundationsSnapshot()
	snapshot.Directories = append(snapshot.Directories, "services/api/spec")
	snapshot.Files = append(snapshot.Files, "services/api/main.rb", "services/api/spec/api_spec.rb")
	snapshot.Technologies = []Technology{
		{ID: "ruby", Category: TechnologyCategoryLanguage},
		{ID: "github-actions", Category: TechnologyCategoryCICD},
	}

	findings, err := (Evaluator{}).Evaluate(t.Context(), snapshot)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, want no findings", findings)
	}
}

func TestEvaluatorIsDeterministic(t *testing.T) {
	t.Parallel()

	firstSnapshot := Snapshot{
		Directories: []string{"src", ".", "src"},
		Files:       []string{"src/server.ts", ".gitignore", "LICENSE", "README.md"},
		Technologies: []Technology{
			{ID: "nodejs", Category: TechnologyCategoryRuntime},
			{ID: "typescript", Category: TechnologyCategoryLanguage},
			{ID: "nodejs", Category: TechnologyCategoryRuntime},
		},
	}
	secondSnapshot := firstSnapshot
	secondSnapshot.Directories = slices.Clone(firstSnapshot.Directories)
	secondSnapshot.Files = slices.Clone(firstSnapshot.Files)
	secondSnapshot.Technologies = slices.Clone(firstSnapshot.Technologies)
	slices.Reverse(secondSnapshot.Directories)
	slices.Reverse(secondSnapshot.Files)
	slices.Reverse(secondSnapshot.Technologies)

	first, err := (Evaluator{}).Evaluate(t.Context(), firstSnapshot)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	second, err := (Evaluator{}).Evaluate(t.Context(), secondSnapshot)
	if err != nil {
		t.Fatalf("second Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("evaluation is not deterministic:\nfirst: %#v\nsecond: %#v", first, second)
	}
}

func TestEvaluatorRejectsInvalidSnapshots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		snapshot Snapshot
	}{
		{name: "empty directory", snapshot: Snapshot{Directories: []string{""}}},
		{name: "parent directory", snapshot: Snapshot{Directories: []string{"../outside"}}},
		{name: "unclean directory", snapshot: Snapshot{Directories: []string{"a/../b"}}},
		{name: "root as file", snapshot: Snapshot{Files: []string{"."}}},
		{name: "absolute file", snapshot: Snapshot{Files: []string{"/go.mod"}}},
		{name: "windows file", snapshot: Snapshot{Files: []string{"C:/project/go.mod"}}},
		{name: "backslash file", snapshot: Snapshot{Files: []string{`src\main.go`}}},
		{name: "control file", snapshot: Snapshot{Files: []string{"unsafe\nname.go"}}},
		{
			name: "partial snapshot with unsafe file",
			snapshot: Snapshot{
				Files:   []string{"../outside"},
				Partial: true,
			},
		},
		{
			name: "invalid technology ID",
			snapshot: Snapshot{Technologies: []Technology{
				{ID: "Go", Category: TechnologyCategoryLanguage},
			}},
		},
		{
			name: "invalid technology category",
			snapshot: Snapshot{Technologies: []Technology{
				{ID: "go", Category: TechnologyCategory("ci-cd")},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			findings, err := (Evaluator{}).Evaluate(t.Context(), test.snapshot)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("Evaluate() error = %v, want ErrInvalidSnapshot", err)
			}
			if findings == nil || len(findings) != 0 {
				t.Fatalf("findings = %#v, want a non-nil empty slice", findings)
			}
		})
	}
}

func TestEvaluatorHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	findings, err := (Evaluator{}).Evaluate(ctx, completeFoundationsSnapshot())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate() error = %v, want context.Canceled", err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, want a non-nil empty slice", findings)
	}
}

func TestEvaluatorHonorsCancellationDuringIndexing(t *testing.T) {
	t.Parallel()

	ctx := &cancelAfterChecksContext{
		Context:         t.Context(),
		checksRemaining: 3,
	}
	findings, err := (Evaluator{}).Evaluate(ctx, Snapshot{
		Directories: []string{".", "cmd"},
		Files:       []string{"cmd/main.go"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate() error = %v, want context.Canceled", err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, want a non-nil empty slice", findings)
	}
}

func TestRecognizedRepositoryFoundationNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"readme",
		"README.adoc",
		"README.markdown",
		"README.md",
		"README.rst",
		"README.txt",
	} {
		if !recognizedReadmeName(name) {
			t.Errorf("recognizedReadmeName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{
		"COPYING",
		"LICENCE.md",
		"LICENSE",
		"LICENSE-APACHE",
		"LICENSE.txt",
		"UNLICENSE",
	} {
		if !recognizedLicenseName(name) {
			t.Errorf("recognizedLicenseName(%q) = false, want true", name)
		}
	}
}

func TestTestMarkersCoverSupportedConventions(t *testing.T) {
	t.Parallel()

	files := []string{
		"main_test.go",
		"api.test.ts",
		"api.spec.js",
		"test_application.py",
		"application_test.py",
		"UserServiceTest.java",
		"ApplicationTests.cs",
		"auth_spec.rb",
		"api_test.exs",
		"application_SUITE.erl",
		"vector_test.cpp",
		"state_test.clj",
		"conftest.py",
		"pytest.ini",
		"phpunit.xml.dist",
	}
	for _, file := range files {
		if !isTestFile("nested/" + file) {
			t.Errorf("isTestFile(%q) = false, want true", file)
		}
	}

	for _, directory := range []string{"test", "tests", "spec", "specs", "__tests__"} {
		if !isTestDirectory("nested/" + directory + "/unit") {
			t.Errorf("isTestDirectory(%q) = false, want true", directory)
		}
	}
}

func TestTestMarkersIgnoreUnreliableNames(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"contest.go",
		"latest.py",
		"api.specification.ts",
		"Protest.java",
		"test_results.json",
		"testing.md",
	} {
		if isTestFile(file) {
			t.Errorf("isTestFile(%q) = true, want false", file)
		}
	}
	for _, directory := range []string{"contest", "latest", "testing"} {
		if isTestDirectory(directory) {
			t.Errorf("isTestDirectory(%q) = true, want false", directory)
		}
	}
}

func BenchmarkEvaluator(b *testing.B) {
	snapshot := completeFoundationsSnapshot()
	snapshot.Files = make([]string, 2_000)
	for index := range snapshot.Files {
		snapshot.Files[index] = fmt.Sprintf("services/%04d/main.go", index)
	}
	snapshot.Files[0] = ".gitignore"
	snapshot.Files[1] = "LICENSE"
	snapshot.Files[2] = "README.md"
	snapshot.Technologies = []Technology{
		{ID: "go", Category: TechnologyCategoryLanguage},
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := (Evaluator{}).Evaluate(context.Background(), snapshot); err != nil {
			b.Fatalf("Evaluate() error = %v", err)
		}
	}
}

func completeFoundationsSnapshot() Snapshot {
	return Snapshot{
		Directories: []string{"."},
		Files:       []string{".gitignore", "LICENSE", "README.md"},
	}
}

func recognizedReadmeName(name string) bool {
	return recognizedReadme(strings.ToLower(name))
}

func recognizedLicenseName(name string) bool {
	return recognizedLicense(strings.ToLower(name))
}

func findingCodes(findings []Finding) []string {
	codes := make([]string, 0, len(findings))
	for _, finding := range findings {
		codes = append(codes, finding.RuleID)
	}
	return codes
}

func findFinding(t *testing.T, findings []Finding, code string) Finding {
	t.Helper()

	for _, finding := range findings {
		if finding.RuleID == code {
			return finding
		}
	}
	t.Fatalf("finding %q not found in %#v", code, findings)
	return Finding{}
}

func assertValidFinding(t *testing.T, finding Finding) {
	t.Helper()

	if err := finding.Validate(); err != nil {
		t.Fatalf("finding %#v validation error = %v", finding, err)
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
