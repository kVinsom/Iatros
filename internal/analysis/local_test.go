package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kVinsom/Iatros/internal/detection"
	"github.com/kVinsom/Iatros/internal/readiness"
)

func TestLocalAnalyzerBuildsCompleteReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeAnalysisTestFile(t, root, "README.md")
	writeAnalysisTestFile(t, root, "LICENSE")
	writeAnalysisTestFile(t, root, ".gitignore")
	writeAnalysisTestFile(t, root, "go.mod")
	writeAnalysisTestFile(t, root, "main.go")
	writeAnalysisTestFile(t, root, "main_test.go")
	writeAnalysisTestFile(t, root, ".github/workflows/ci.yml")

	analyzer, err := NewLocalAnalyzer()
	if err != nil {
		t.Fatalf("NewLocalAnalyzer() error = %v", err)
	}
	report, err := analyzer.Analyze(t.Context(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v; report = %#v", err, report)
	}
	if report.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", report.Status, StatusCompleted)
	}
	if report.Summary.DirectoriesScanned != 3 || report.Summary.FilesScanned != 7 {
		t.Fatalf("Summary = %#v, want 3 directories and 7 files", report.Summary)
	}
	if report.Summary.EcosystemsDetected != 3 || report.Summary.FindingsTotal != 0 {
		t.Fatalf("Summary = %#v, want 3 technologies and no findings", report.Summary)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want empty", report.Diagnostics)
	}

	want := []Ecosystem{
		{
			ID:       "github-actions",
			Category: "ci_cd",
			Evidence: []string{".github/workflows/ci.yml"},
		},
		{
			ID:       "go",
			Category: "language",
			Evidence: []string{"go.mod", "main.go", "main_test.go"},
		},
		{
			ID:       "go-modules",
			Category: "dependency_manager",
			Evidence: []string{"go.mod"},
		},
	}
	if !slices.EqualFunc(report.Ecosystems, want, func(left, right Ecosystem) bool {
		return left.ID == right.ID &&
			left.Category == right.Category &&
			left.EvidenceTruncated == right.EvidenceTruncated &&
			slices.Equal(left.Evidence, right.Evidence)
	}) {
		t.Fatalf("Ecosystems = %#v, want %#v", report.Ecosystems, want)
	}
}

func TestLocalAnalyzerReportsAndIsolatesNestedRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeAnalysisTestFile(t, root, "README.md")
	writeAnalysisTestFile(t, root, "LICENSE")
	writeAnalysisTestFile(t, root, ".gitignore")
	writeAnalysisTestFile(t, root, "go.mod")
	writeAnalysisTestFile(t, root, "vendor/library/Cargo.toml")
	gitmodules := "[submodule \"library\"]\npath = vendor/library\n"
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(gitmodules), 0o600); err != nil {
		t.Fatal(err)
	}

	analyzer, err := NewLocalAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	report, err := analyzer.Analyze(t.Context(), Request{Root: root})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Status != StatusCompleted || report.Summary.NestedRepositoriesSkipped != 1 ||
		report.Summary.FilesScanned != 5 {
		t.Fatalf("report = %#v, want one skipped nested repository", report)
	}
	for _, ecosystem := range report.Ecosystems {
		if ecosystem.ID == "rust" || ecosystem.ID == "cargo" {
			t.Fatalf("nested repository leaked into ecosystems: %#v", report.Ecosystems)
		}
	}
}

func TestLocalAnalyzerMapsPartialDiscovery(t *testing.T) {
	t.Parallel()

	discovery := discoveryFunc(func(context.Context, string) (Inventory, error) {
		return Inventory{
			Directories: []string{"."},
			Files:       []string{"go.mod"},
			Issues: []DiscoveryIssue{{
				Code:    DiscoveryIssueFileLimit,
				Path:    "later.go",
				Message: "The file limit was reached; remaining entries were skipped.",
			}},
			Partial: true,
		}, nil
	})
	detector := detectorFunc(func(_ context.Context, files []string) ([]detection.Technology, error) {
		if !slices.Equal(files, []string{"go.mod"}) {
			t.Fatalf("Detect() files = %#v, want go.mod", files)
		}
		return []detection.Technology{{
			ID:                "go",
			Category:          detection.CategoryLanguage,
			Evidence:          []string{"go.mod"},
			EvidenceTruncated: true,
		}}, nil
	})
	evaluator := evaluatorFunc(func(_ context.Context, snapshot readiness.Snapshot) ([]readiness.Finding, error) {
		if !snapshot.Partial || len(snapshot.Technologies) != 1 ||
			snapshot.Technologies[0].Category != readiness.TechnologyCategoryLanguage {
			t.Fatalf("Evaluate() snapshot = %#v, want partial Go snapshot", snapshot)
		}
		return []readiness.Finding{}, nil
	})

	report, err := newLocalAnalyzer(discovery, detector, evaluator).Analyze(
		t.Context(),
		Request{Root: "ignored"},
	)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v; report = %#v", err, report)
	}
	if report.Status != StatusPartial || len(report.Findings) != 0 {
		t.Fatalf("report = %#v, want partial report without absence findings", report)
	}
	if len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != DiscoveryIssueFileLimit ||
		report.Diagnostics[0].Message != "later.go: The file limit was reached; remaining entries were skipped." {
		t.Fatalf("Diagnostics = %#v, want mapped discovery issue", report.Diagnostics)
	}
	if len(report.Ecosystems) != 1 || !report.Ecosystems[0].EvidenceTruncated {
		t.Fatalf("Ecosystems = %#v, want truncated Go evidence", report.Ecosystems)
	}
}

func TestLocalAnalyzerMapsStageErrors(t *testing.T) {
	t.Parallel()

	internalErr := errors.New("stage failed")
	validInventory := Inventory{Directories: []string{"."}, Files: []string{}}
	validTechnologies := []detection.Technology{}

	tests := []struct {
		name       string
		analyzer   LocalAnalyzer
		wantError  error
		wantCode   string
		wantStatus Status
	}{
		{
			name: "invalid target",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return Inventory{}, ErrInvalidTarget
				}),
				detectorFunc(successfulDetection),
				evaluatorFunc(successfulEvaluation),
			),
			wantError:  ErrInvalidTarget,
			wantCode:   DiagnosticCodeTargetInvalid,
			wantStatus: StatusFailed,
		},
		{
			name: "detection failure",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return validInventory, nil
				}),
				detectorFunc(func(context.Context, []string) ([]detection.Technology, error) {
					return nil, internalErr
				}),
				evaluatorFunc(successfulEvaluation),
			),
			wantError:  internalErr,
			wantCode:   DiagnosticCodeAnalysisFailed,
			wantStatus: StatusFailed,
		},
		{
			name: "unsafe discovery issue",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return Inventory{
						Directories: []string{"."}, Files: []string{}, Partial: true,
						Issues: []DiscoveryIssue{{
							Code: DiscoveryIssuePathUnreadable, Path: `C:\Users\private`,
							Message: "The path could not be read.",
						}},
					}, nil
				}),
				detectorFunc(successfulDetection),
				evaluatorFunc(successfulEvaluation),
			),
			wantError:  ErrInvalidReport,
			wantCode:   DiagnosticCodeAnalysisFailed,
			wantStatus: StatusFailed,
		},
		{
			name: "internal deadline",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return Inventory{}, context.DeadlineExceeded
				}),
				detectorFunc(successfulDetection),
				evaluatorFunc(successfulEvaluation),
			),
			wantError:  context.DeadlineExceeded,
			wantCode:   DiagnosticCodeAnalysisFailed,
			wantStatus: StatusFailed,
		},
		{
			name: "readiness cancellation",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return validInventory, nil
				}),
				detectorFunc(func(context.Context, []string) ([]detection.Technology, error) {
					return validTechnologies, nil
				}),
				evaluatorFunc(func(context.Context, readiness.Snapshot) ([]readiness.Finding, error) {
					return nil, context.Canceled
				}),
			),
			wantError:  context.Canceled,
			wantCode:   DiagnosticCodeAnalysisCanceled,
			wantStatus: StatusFailed,
		},
		{
			name: "invalid stage result",
			analyzer: newLocalAnalyzer(
				discoveryFunc(func(context.Context, string) (Inventory, error) {
					return validInventory, nil
				}),
				detectorFunc(func(context.Context, []string) ([]detection.Technology, error) {
					return []detection.Technology{{
						ID:       "invalid",
						Category: detection.Category("INVALID"),
						Evidence: []string{"go.mod"},
					}}, nil
				}),
				evaluatorFunc(successfulEvaluation),
			),
			wantError:  ErrInvalidReport,
			wantCode:   DiagnosticCodeAnalysisFailed,
			wantStatus: StatusFailed,
		},
		{
			name:       "unconfigured analyzer",
			analyzer:   LocalAnalyzer{},
			wantError:  errLocalAnalyzerUnavailable,
			wantCode:   DiagnosticCodeAnalysisFailed,
			wantStatus: StatusFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report, err := test.analyzer.Analyze(t.Context(), Request{Root: "target"})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Analyze() error = %v, want %v", err, test.wantError)
			}
			if report.Status != test.wantStatus || len(report.Diagnostics) != 1 ||
				report.Diagnostics[0].Code != test.wantCode {
				t.Fatalf("report = %#v, want status %q and diagnostic %q", report, test.wantStatus, test.wantCode)
			}
		})
	}
}

func TestLocalAnalyzerHonorsInitialCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report, err := LocalAnalyzer{}.Analyze(ctx, Request{Root: "target"})

	if !errors.Is(err, context.Canceled) ||
		report.Status != StatusFailed ||
		len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != DiagnosticCodeAnalysisCanceled {
		t.Fatalf("Analyze() = (%#v, %v), want canonical cancellation", report, err)
	}
}

func TestLocalAnalyzerStopsAtCanceledStageBoundaries(t *testing.T) {
	t.Parallel()

	validInventory := Inventory{Directories: []string{"."}, Files: []string{}}
	tests := []struct {
		name  string
		build func(context.CancelFunc, *bool) LocalAnalyzer
	}{
		{
			name: "after discovery",
			build: func(cancel context.CancelFunc, downstreamCalled *bool) LocalAnalyzer {
				return newLocalAnalyzer(
					discoveryFunc(func(context.Context, string) (Inventory, error) {
						cancel()
						return validInventory, nil
					}),
					detectorFunc(func(context.Context, []string) ([]detection.Technology, error) {
						*downstreamCalled = true
						return nil, nil
					}),
					evaluatorFunc(successfulEvaluation),
				)
			},
		},
		{
			name: "after detection",
			build: func(cancel context.CancelFunc, downstreamCalled *bool) LocalAnalyzer {
				return newLocalAnalyzer(
					discoveryFunc(func(context.Context, string) (Inventory, error) {
						return validInventory, nil
					}),
					detectorFunc(func(context.Context, []string) ([]detection.Technology, error) {
						cancel()
						return []detection.Technology{}, nil
					}),
					evaluatorFunc(func(context.Context, readiness.Snapshot) ([]readiness.Finding, error) {
						*downstreamCalled = true
						return nil, nil
					}),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			downstreamCalled := false

			report, err := test.build(cancel, &downstreamCalled).Analyze(
				ctx,
				Request{Root: "target"},
			)

			if !errors.Is(err, context.Canceled) || downstreamCalled ||
				len(report.Diagnostics) != 1 ||
				report.Diagnostics[0].Code != DiagnosticCodeAnalysisCanceled {
				t.Fatalf(
					"Analyze() = (%#v, %v), downstream called %t; want cancellation",
					report,
					err,
					downstreamCalled,
				)
			}
		})
	}
}

func TestCompletedLocalReportOwnsEvidence(t *testing.T) {
	t.Parallel()

	technologyEvidence := []string{"go.mod"}
	findingEvidence := []string{"go.mod"}
	report := completedLocalReport(
		Inventory{Directories: []string{"."}, Files: []string{"go.mod"}},
		[]detection.Technology{{
			ID: "go", Category: detection.CategoryLanguage, Evidence: technologyEvidence,
		}},
		[]readiness.Finding{{
			Code: readiness.FindingCodeTestsNotDetected, Severity: readiness.SeverityWarning,
			Message: "No supported test evidence was detected.", Evidence: findingEvidence,
			Remediation: "Add automated tests.",
		}},
	)
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	technologyEvidence[0] = "changed"
	findingEvidence[0] = "changed"
	if report.Ecosystems[0].Evidence[0] != "go.mod" ||
		report.Findings[0].Evidence[0] != "go.mod" {
		t.Fatalf("report retained backend-owned evidence: %#v", report)
	}
}

func writeAnalysisTestFile(t *testing.T, root, relativePath string) {
	t.Helper()

	filename := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", relativePath, err)
	}
	if err := os.WriteFile(filename, []byte("test fixture"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", relativePath, err)
	}
}

func successfulDetection(context.Context, []string) ([]detection.Technology, error) {
	return []detection.Technology{}, nil
}

func successfulEvaluation(context.Context, readiness.Snapshot) ([]readiness.Finding, error) {
	return []readiness.Finding{}, nil
}

type discoveryFunc func(context.Context, string) (Inventory, error)

func (function discoveryFunc) Discover(ctx context.Context, root string) (Inventory, error) {
	return function(ctx, root)
}

type detectorFunc func(context.Context, []string) ([]detection.Technology, error)

func (function detectorFunc) Detect(
	ctx context.Context,
	files []string,
) ([]detection.Technology, error) {
	return function(ctx, files)
}

type evaluatorFunc func(context.Context, readiness.Snapshot) ([]readiness.Finding, error)

func (function evaluatorFunc) Evaluate(
	ctx context.Context,
	snapshot readiness.Snapshot,
) ([]readiness.Finding, error) {
	return function(ctx, snapshot)
}
