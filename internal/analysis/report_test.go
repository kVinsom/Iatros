package analysis

import (
	"errors"
	"testing"
)

func TestReportConstructorsReturnValidReports(t *testing.T) {
	t.Parallel()

	reports := []Report{
		NewNotImplementedReport(),
		NewInvalidTargetReport(),
		NewAnalysisFailedReport(),
		NewFailedReport(Diagnostic{
			Code:    "IATROS_TEST_FAILURE",
			Level:   "error",
			Message: "The test analysis failed.",
		}),
		NewCanceledReport(),
	}

	for _, report := range reports {
		if err := report.Validate(); err != nil {
			t.Fatalf("Validate() error = %v for report %#v", err, report)
		}
	}
}

func TestReportValidateAcceptsImplementedResults(t *testing.T) {
	t.Parallel()

	completed := newReport(StatusCompleted)
	addValidEcosystem(&completed)
	completed.Ecosystems = append(completed.Ecosystems, Ecosystem{
		ID:       "nodejs",
		Category: "runtime",
		Evidence: []string{"package-lock.json", "package.json"},
	})
	completed.Summary.EcosystemsDetected = len(completed.Ecosystems)
	critical := testFinding()
	critical.Code = "delivery.ci.missing"
	critical.Severity = "critical"
	critical.Evidence = []string{"CI configuration not found"}
	completed.Findings = []Finding{critical, testFinding()}
	completed.Summary.FindingsTotal = len(completed.Findings)

	partial := completed
	partial.Status = StatusPartial
	partial.Diagnostics = []Diagnostic{{
		Code:    "IATROS_SCAN_LIMIT",
		Level:   "warning",
		Message: "A configured scan limit was reached.",
	}}

	for _, report := range []Report{completed, partial} {
		if err := report.Validate(); err != nil {
			t.Fatalf("Validate() error = %v for report %#v", err, report)
		}
	}
}

func TestReportValidateRejectsInvalidData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Report)
	}{
		{name: "schema", mutate: func(report *Report) { report.SchemaVersion = "9" }},
		{name: "profile", mutate: func(report *Report) { report.Profile = "large" }},
		{name: "status", mutate: func(report *Report) { report.Status = Status("unknown") }},
		{name: "target kind", mutate: func(report *Report) { report.Target.Kind = "remote" }},
		{name: "target path", mutate: func(report *Report) { report.Target.Path = "/private/root" }},
		{
			name: "negative directory count",
			mutate: func(report *Report) {
				report.Summary.DirectoriesScanned = -1
			},
		},
		{
			name: "negative file count",
			mutate: func(report *Report) {
				report.Summary.FilesScanned = -1
			},
		},
		{
			name: "negative nested repository count",
			mutate: func(report *Report) {
				report.Summary.NestedRepositoriesSkipped = -1
			},
		},
		{
			name: "ecosystem count mismatch",
			mutate: func(report *Report) {
				report.Ecosystems = append(report.Ecosystems, validEcosystem())
			},
		},
		{
			name: "finding count mismatch",
			mutate: func(report *Report) {
				report.Findings = append(report.Findings, testFinding())
			},
		},
		{
			name: "not implemented with result data",
			mutate: func(report *Report) {
				report.Status = StatusNotImplemented
				report.Diagnostics = NewNotImplementedReport().Diagnostics
				addValidEcosystem(report)
			},
		},
		{
			name: "not implemented with noncanonical diagnostic",
			mutate: func(report *Report) {
				report.Status = StatusNotImplemented
				report.Diagnostics = []Diagnostic{{
					Code:    "IATROS_OTHER_OUTCOME",
					Level:   "info",
					Message: "Another outcome was returned.",
				}}
			},
		},
		{
			name: "failed with result data",
			mutate: func(report *Report) {
				report.Status = StatusFailed
				report.Diagnostics = NewCanceledReport().Diagnostics
				addValidFinding(report)
			},
		},
		{name: "partial without diagnostic", mutate: func(report *Report) { report.Status = StatusPartial }},
		{name: "failed without diagnostic", mutate: func(report *Report) { report.Status = StatusFailed }},
		{
			name: "blank diagnostic code",
			mutate: func(report *Report) {
				report.Diagnostics = []Diagnostic{{Level: "info", Message: "Valid message."}}
			},
		},
		{
			name: "invalid diagnostic level",
			mutate: func(report *Report) {
				report.Diagnostics = []Diagnostic{{
					Code:    "IATROS_TEST",
					Level:   "fatal",
					Message: "Valid message.",
				}}
			},
		},
		{
			name: "control character in diagnostic",
			mutate: func(report *Report) {
				report.Diagnostics = []Diagnostic{{
					Code:    "IATROS_TEST",
					Level:   "info",
					Message: "forged\noutput",
				}}
			},
		},
		{
			name: "invalid UTF-8 in diagnostic",
			mutate: func(report *Report) {
				report.Diagnostics = []Diagnostic{{
					Code:    "IATROS_TEST",
					Level:   "info",
					Message: string([]byte{0xff}),
				}}
			},
		},
		{
			name: "absolute path in diagnostic",
			mutate: func(report *Report) {
				report.Status = StatusPartial
				report.Diagnostics = []Diagnostic{{
					Code: "IATROS_TEST", Level: "warning",
					Message: "open /private/secret: access denied",
				}}
			},
		},
		{
			name: "blank ecosystem id",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].ID = ""
			},
		},
		{
			name: "uppercase ecosystem id",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].ID = "Go"
			},
		},
		{
			name: "blank ecosystem category",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Category = ""
			},
		},
		{
			name: "invalid ecosystem category",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Category = "CI/CD"
			},
		},
		{
			name: "ecosystem without evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = nil
			},
		},
		{
			name: "absolute ecosystem evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = []string{"/private/go.mod"}
			},
		},
		{
			name: "parent traversal in ecosystem evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = []string{"../go.mod"}
			},
		},
		{
			name: "backslash in ecosystem evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = []string{`private\go.mod`}
			},
		},
		{
			name: "control character in ecosystem evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = []string{"go.mod\nforged"}
			},
		},
		{
			name: "unsorted ecosystem evidence",
			mutate: func(report *Report) {
				addValidEcosystem(report)
				report.Ecosystems[0].Evidence = []string{"go.sum", "go.mod"}
			},
		},
		{
			name: "unsorted ecosystems",
			mutate: func(report *Report) {
				report.Ecosystems = []Ecosystem{
					{ID: "nodejs", Category: "runtime", Evidence: []string{"package.json"}},
					validEcosystem(),
				}
				report.Summary.EcosystemsDetected = len(report.Ecosystems)
			},
		},
		{
			name: "blank finding code",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Code = ""
			},
		},
		{
			name: "undotted finding code",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Code = "readme"
			},
		},
		{
			name: "invalid finding severity",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Severity = "fatal"
			},
		},
		{
			name: "blank finding message",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Message = ""
			},
		},
		{
			name: "finding without evidence",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Evidence = nil
			},
		},
		{
			name: "control character in finding evidence",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Evidence = []string{"forged\x1b[31moutput"}
			},
		},
		{
			name: "unsorted finding evidence",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Evidence = []string{
					"README file not found",
					"License file not found",
				}
			},
		},
		{
			name: "unsorted findings",
			mutate: func(report *Report) {
				first := testFinding()
				first.Code = "repository.license.missing"
				first.Severity = "info"
				first.Evidence = []string{"License file not found"}
				second := testFinding()
				second.Code = "delivery.ci.missing"
				second.Severity = "critical"
				second.Evidence = []string{"CI configuration not found"}
				report.Findings = []Finding{first, second}
				report.Summary.FindingsTotal = len(report.Findings)
			},
		},
		{
			name: "blank remediation",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Remediation = ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report := newReport(StatusCompleted)
			test.mutate(&report)
			if err := report.Validate(); !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("Validate() error = %v, want ErrInvalidReport", err)
			}
		})
	}
}

func TestReportNormalizedUsesNonNilCollectionsWithoutMutatingInput(t *testing.T) {
	t.Parallel()

	report := Report{}

	normalized := report.Normalized()

	if normalized.Ecosystems == nil || normalized.Findings == nil || normalized.Diagnostics == nil {
		t.Fatal("Normalized() left a top-level collection nil")
	}
	if report.Ecosystems != nil || report.Findings != nil || report.Diagnostics != nil {
		t.Fatal("Normalized() mutated the input report")
	}
}

func addValidEcosystem(report *Report) {
	report.Ecosystems = append(report.Ecosystems, validEcosystem())
	report.Summary.EcosystemsDetected = len(report.Ecosystems)
}

func addValidFinding(report *Report) {
	report.Findings = append(report.Findings, testFinding())
	report.Summary.FindingsTotal = len(report.Findings)
}

func validEcosystem() Ecosystem {
	return Ecosystem{ID: "go", Category: "language", Evidence: []string{"go.mod"}}
}

func testFinding() Finding {
	return Finding{
		Code:        "repository.readme.missing",
		Severity:    "warning",
		Message:     "No README was detected.",
		Evidence:    []string{"README file not found at the repository root"},
		Remediation: "Add a root README.",
	}
}
