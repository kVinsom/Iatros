package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
)

const expectedStubText = "IATROS Local Repository Analysis\n" +
	"Status: not implemented\n" +
	"Profile: small\n" +
	"Target: .\n\n" +
	"No analysis was performed.\n"

func TestAnalyzeTextStub(t *testing.T) {
	t.Parallel()

	result := runCLI(t, analysis.NewLocalStub(), "analyze", "--format", "text", t.TempDir())

	if result.exitCode != ExitNotImplemented {
		t.Fatalf("exit code = %d, want %d", result.exitCode, ExitNotImplemented)
	}
	if result.stdout != expectedStubText {
		t.Fatalf("stdout mismatch:\n%s", result.stdout)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestAnalyzeJSONStub(t *testing.T) {
	t.Parallel()

	result := runCLI(t, analysis.NewLocalStub(), "analyze", t.TempDir(), "--format", "json")
	report := decodeReport(t, result.stdout)

	if result.exitCode != ExitNotImplemented {
		t.Fatalf("exit code = %d, want %d", result.exitCode, ExitNotImplemented)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
	if report.SchemaVersion != analysis.SchemaVersion || report.Status != analysis.StatusNotImplemented {
		t.Fatalf("report = %#v, want current schema and not_implemented", report)
	}
	if report.Target.Kind != analysis.TargetKindLocalDirectory ||
		report.Target.Path != analysis.TargetRootPath {
		t.Fatalf("target = %#v, want local relative root", report.Target)
	}
	if report.Ecosystems == nil || report.Findings == nil || report.Diagnostics == nil {
		t.Fatal("JSON collections must be arrays, not null")
	}
	if len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != "IATROS_ANALYSIS_NOT_IMPLEMENTED" {
		t.Fatalf("diagnostics = %#v, want not-implemented diagnostic", report.Diagnostics)
	}
}

func TestAnalyzeJSONIsDeterministic(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	first := runCLI(t, analysis.NewLocalStub(), "analyze", "--format=json", target)
	second := runCLI(t, analysis.NewLocalStub(), "analyze", "--format=json", target)

	if first.exitCode != ExitNotImplemented || second.exitCode != ExitNotImplemented {
		t.Fatalf(
			"exit codes = %d, %d; want %d",
			first.exitCode,
			second.exitCode,
			ExitNotImplemented,
		)
	}
	if first.stdout != second.stdout {
		t.Fatalf(
			"JSON output is not deterministic:\nfirst:\n%s\nsecond:\n%s",
			first.stdout,
			second.stdout,
		)
	}
	if first.stderr != "" || second.stderr != "" {
		t.Fatalf("stderr = %q, %q; want empty", first.stderr, second.stderr)
	}
}

func TestAnalyzeLocalRepositoryJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relativePath := range []string{
		"README.md",
		"LICENSE",
		".gitignore",
		"go.mod",
		"main_test.go",
		".github/workflows/ci.yml",
	} {
		filename := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", relativePath, err)
		}
		if err := os.WriteFile(filename, []byte("fixture content is not read"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", relativePath, err)
		}
	}
	analyzer, err := analysis.NewLocalAnalyzer()
	if err != nil {
		t.Fatalf("NewLocalAnalyzer() error = %v", err)
	}

	result := runCLI(t, analyzer, "analyze", "--format=json", root)
	report := decodeReport(t, result.stdout)

	if result.exitCode != ExitSuccess || result.stderr != "" {
		t.Fatalf("Run() = exit %d, stderr %q; want success", result.exitCode, result.stderr)
	}
	if report.Status != analysis.StatusCompleted || report.Summary.EcosystemsDetected != 3 ||
		report.Summary.FindingsTotal != 0 {
		t.Fatalf("report = %#v, want complete ready Go repository", report)
	}
	if strings.Contains(result.stdout, filepath.ToSlash(root)) {
		t.Fatal("report exposed the absolute analysis root")
	}
	for _, ecosystem := range report.Ecosystems {
		if ecosystem.Category == "" {
			t.Fatalf("ecosystem = %#v, want category", ecosystem)
		}
	}
}

func TestAnalyzeInvalidTargetsProduceSafeJSON(t *testing.T) {
	t.Parallel()

	const privateTargetName = "private-target-7f3c2a"
	tests := []struct {
		name   string
		target string
	}{
		{name: "empty", target: ""},
		{
			name:   "missing",
			target: filepath.Join(t.TempDir(), privateTargetName),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCLI(
				t,
				analysis.NewLocalStub(),
				"analyze",
				"--format",
				"json",
				test.target,
			)
			report := decodeReport(t, result.stdout)

			if result.exitCode != ExitInvalidTarget {
				t.Fatalf("exit code = %d, want %d", result.exitCode, ExitInvalidTarget)
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want empty", result.stderr)
			}
			if report.Status != analysis.StatusFailed {
				t.Fatalf("status = %q, want %q", report.Status, analysis.StatusFailed)
			}
			if len(report.Diagnostics) != 1 ||
				report.Diagnostics[0].Code != "IATROS_TARGET_INVALID" {
				t.Fatalf("diagnostics = %#v, want invalid-target diagnostic", report.Diagnostics)
			}
			if strings.Contains(result.stdout, filepath.ToSlash(test.target)) && test.target != "" {
				t.Fatal("report exposed the selected local path")
			}
			if strings.Contains(result.stdout, privateTargetName) {
				t.Fatal("report exposed a private target component")
			}
		})
	}
}

func TestAnalyzeRendersCompleteTextReport(t *testing.T) {
	t.Parallel()

	report := completedReport()
	analyzer := analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
		return report, nil
	})
	result := runCLI(t, analyzer, "analyze")

	const want = "IATROS Local Repository Analysis\n" +
		"Status: completed\n" +
		"Profile: small\n" +
		"Target: .\n\n" +
		"Summary:\n" +
		"- Directories scanned: 2\n" +
		"- Files scanned: 3\n" +
		"- Nested repositories skipped: 0\n" +
		"- Ecosystems detected: 1\n" +
		"- Findings total: 1\n\n" +
		"Technologies:\n" +
		"- Language:\n" +
		"  - go\n" +
		"    Evidence:\n" +
		"      - go.mod\n" +
		"    Evidence truncated: false\n\n" +
		"Findings:\n" +
		"- [warning] repository.readme.missing: No README was detected.\n" +
		"  Evidence:\n" +
		"    - README file not found at the repository root\n" +
		"  Remediation: Add a root README.\n\n" +
		"Diagnostics:\n" +
		"- [info] IATROS_SCAN_NOTE: The scan used the local-only profile.\n"

	if result.exitCode != ExitSuccess {
		t.Fatalf("exit code = %d, want %d", result.exitCode, ExitSuccess)
	}
	if result.stdout != want {
		t.Fatalf("stdout mismatch:\n%s", result.stdout)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestAnalyzeOutcomeValidation(t *testing.T) {
	t.Parallel()

	partial := emptyReport(analysis.StatusPartial)
	partial.Diagnostics = []analysis.Diagnostic{{
		Code:    "IATROS_SCAN_LIMIT",
		Level:   "warning",
		Message: "A configured scan limit was reached.",
	}}
	failed := analysis.NewFailedReport(analysis.Diagnostic{
		Code:    "IATROS_CUSTOM_FAILURE",
		Level:   "error",
		Message: "The analyzer could not produce a trustworthy result.",
	})
	customCancellation := analysis.NewFailedReport(analysis.Diagnostic{
		Code:    "IATROS_CUSTOM_CANCELLED",
		Level:   "error",
		Message: "The analyzer stopped after cancellation.",
	})
	unsafeReport := completedReport()
	unsafeReport.Ecosystems[0].Evidence = []string{"/private/secret-project/go.mod"}

	tests := []struct {
		name           string
		report         analysis.Report
		err            error
		wantExit       int
		wantStatus     analysis.Status
		wantDiagnostic string
		forbidden      string
	}{
		{
			name:       "completed",
			report:     emptyReport(analysis.StatusCompleted),
			wantExit:   ExitSuccess,
			wantStatus: analysis.StatusCompleted,
		},
		{
			name:           "partial",
			report:         partial,
			wantExit:       ExitSuccess,
			wantStatus:     analysis.StatusPartial,
			wantDiagnostic: "IATROS_SCAN_LIMIT",
		},
		{
			name:           "failed status without error",
			report:         failed,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: "IATROS_CUSTOM_FAILURE",
		},
		{
			name:           "unexpected error preserves valid failure",
			report:         failed,
			err:            errors.New("internal failure"),
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: "IATROS_CUSTOM_FAILURE",
		},
		{
			name:           "malformed report",
			report:         analysis.Report{},
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: "IATROS_ANALYSIS_FAILED",
		},
		{
			name:           "unsafe evidence is not rendered",
			report:         unsafeReport,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisFailed,
			forbidden:      "secret-project",
		},
		{
			name:           "contradictory not implemented error",
			report:         emptyReport(analysis.StatusCompleted),
			err:            analysis.ErrNotImplemented,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: "IATROS_ANALYSIS_FAILED",
		},
		{
			name:           "joined sentinel errors",
			report:         analysis.NewNotImplementedReport(),
			err:            errors.Join(analysis.ErrNotImplemented, analysis.ErrInvalidTarget),
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisFailed,
		},
		{
			name:           "invalid target with wrong diagnostic",
			report:         failed,
			err:            analysis.ErrInvalidTarget,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisFailed,
		},
		{
			name:           "malformed cancellation",
			report:         analysis.Report{},
			err:            context.Canceled,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisCanceled,
		},
		{
			name:           "cancellation is canonicalized",
			report:         customCancellation,
			err:            context.Canceled,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisCanceled,
		},
		{
			name:           "internal deadline is not user cancellation",
			report:         customCancellation,
			err:            context.DeadlineExceeded,
			wantExit:       ExitFailure,
			wantStatus:     analysis.StatusFailed,
			wantDiagnostic: analysis.DiagnosticCodeAnalysisFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			analyzer := analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
				return test.report, test.err
			})
			result := runCLI(t, analyzer, "analyze", "--format", "json")
			report := decodeReport(t, result.stdout)

			if result.exitCode != test.wantExit {
				t.Fatalf("exit code = %d, want %d", result.exitCode, test.wantExit)
			}
			if report.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q", report.Status, test.wantStatus)
			}
			if test.wantDiagnostic != "" {
				if len(report.Diagnostics) != 1 ||
					report.Diagnostics[0].Code != test.wantDiagnostic {
					t.Fatalf(
						"diagnostics = %#v, want %q",
						report.Diagnostics,
						test.wantDiagnostic,
					)
				}
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want empty", result.stderr)
			}
			if test.forbidden != "" && strings.Contains(result.stdout, test.forbidden) {
				t.Fatalf("stdout exposed forbidden value %q", test.forbidden)
			}
		})
	}
}

func TestAnalyzeUnavailable(t *testing.T) {
	t.Parallel()

	result := runCLI(t, nil, "analyze", "--format=json")
	report := decodeReport(t, result.stdout)

	if result.exitCode != ExitFailure {
		t.Fatalf("exit code = %d, want %d", result.exitCode, ExitFailure)
	}
	if len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != analysis.DiagnosticCodeAnalysisUnavailable {
		t.Fatalf("diagnostics = %#v, want unavailable diagnostic", report.Diagnostics)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestAnalyzeCancellationOverridesSuccessfulResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		analyzer  func(context.CancelFunc, *bool) Analyzer
		preCancel bool
	}{
		{
			name: "before analyzer call",
			analyzer: func(_ context.CancelFunc, called *bool) Analyzer {
				return analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
					*called = true
					return emptyReport(analysis.StatusCompleted), nil
				})
			},
			preCancel: true,
		},
		{
			name: "during analyzer call",
			analyzer: func(cancel context.CancelFunc, called *bool) Analyzer {
				return analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
					*called = true
					cancel()
					return emptyReport(analysis.StatusCompleted), nil
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			called := false
			analyzer := test.analyzer(cancel, &called)
			if test.preCancel {
				cancel()
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run(
				ctx,
				"test-version",
				Services{Analysis: analyzer},
				[]string{"analyze", "--format=json"},
				&stdout,
				&stderr,
			)
			report := decodeReport(t, stdout.String())

			if exitCode != ExitFailure {
				t.Fatalf("exit code = %d, want %d", exitCode, ExitFailure)
			}
			if len(report.Diagnostics) != 1 ||
				report.Diagnostics[0].Code != analysis.DiagnosticCodeAnalysisCanceled {
				t.Fatalf("diagnostics = %#v, want canonical cancellation", report.Diagnostics)
			}
			if test.preCancel && called {
				t.Fatal("analyzer was called after the context was already canceled")
			}
			if !test.preCancel && !called {
				t.Fatal("analyzer was not called")
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestAnalyzeDetectsReportOutputFailure(t *testing.T) {
	t.Parallel()

	analyzer := analyzerFunc(func(context.Context, analysis.Request) (analysis.Report, error) {
		return emptyReport(analysis.StatusCompleted), nil
	})
	var stderr bytes.Buffer
	exitCode := Run(
		t.Context(),
		"test-version",
		Services{Analysis: analyzer},
		[]string{"analyze"},
		failingWriter{err: errors.New("write failed")},
		&stderr,
	)

	if exitCode != ExitFailure {
		t.Fatalf("exit code = %d, want %d", exitCode, ExitFailure)
	}
	if !strings.Contains(stderr.String(), "could not write analysis report") {
		t.Fatalf("stderr does not describe report output failure: %q", stderr.String())
	}
}

func completedReport() analysis.Report {
	return analysis.Report{
		SchemaVersion: analysis.SchemaVersion,
		Profile:       analysis.ScalingProfileSmall,
		Status:        analysis.StatusCompleted,
		Target: analysis.Target{
			Kind: analysis.TargetKindLocalDirectory,
			Path: analysis.TargetRootPath,
		},
		Summary: analysis.Summary{
			DirectoriesScanned: 2,
			FilesScanned:       3,
			EcosystemsDetected: 1,
			FindingsTotal:      1,
		},
		Ecosystems: []analysis.Ecosystem{{
			ID:       "go",
			Category: "language",
			Evidence: []string{"go.mod"},
		}},
		Findings: []analysis.Finding{{
			Code:        "repository.readme.missing",
			Severity:    "warning",
			Message:     "No README was detected.",
			Evidence:    []string{"README file not found at the repository root"},
			Remediation: "Add a root README.",
		}},
		Diagnostics: []analysis.Diagnostic{{
			Code:    "IATROS_SCAN_NOTE",
			Level:   "info",
			Message: "The scan used the local-only profile.",
		}},
	}
}

func emptyReport(status analysis.Status) analysis.Report {
	return analysis.Report{
		SchemaVersion: analysis.SchemaVersion,
		Profile:       analysis.ScalingProfileSmall,
		Status:        status,
		Target: analysis.Target{
			Kind: analysis.TargetKindLocalDirectory,
			Path: analysis.TargetRootPath,
		},
		Ecosystems:  []analysis.Ecosystem{},
		Findings:    []analysis.Finding{},
		Diagnostics: []analysis.Diagnostic{},
	}
}

func decodeReport(t *testing.T, output string) analysis.Report {
	t.Helper()

	var report analysis.Report
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output)
	}
	return report
}

type analyzerFunc func(context.Context, analysis.Request) (analysis.Report, error)

func (function analyzerFunc) Analyze(
	ctx context.Context,
	request analysis.Request,
) (analysis.Report, error) {
	return function(ctx, request)
}
