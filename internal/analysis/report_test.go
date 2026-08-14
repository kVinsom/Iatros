package analysis

import (
	"errors"
	"testing"

	"github.com/kVinsom/Iatros/internal/finding"
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
	critical.ID = "delivery.ci.missing.local"
	critical.RuleID = "delivery.ci.missing"
	critical.Severity = finding.SeverityCritical
	critical.Evidence = []finding.Evidence{{
		Kind: finding.EvidenceObservation, Description: "CI configuration was not found.",
	}}
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
			name: "blank finding id",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].ID = ""
			},
		},
		{
			name: "invalid finding rule",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].RuleID = "Readme"
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
				report.Findings[0].Description = ""
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
				report.Findings[0].Evidence = []finding.Evidence{{
					Kind: finding.EvidenceObservation, Description: "forged\x1b[31moutput",
				}}
			},
		},
		{
			name: "unsorted finding evidence",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Evidence = []finding.Evidence{
					{Kind: finding.EvidenceObservation, Description: "README file was not found."},
					{Kind: finding.EvidenceObservation, Description: "License file was not found."},
				}
			},
		},
		{
			name: "unsorted findings",
			mutate: func(report *Report) {
				first := testFinding()
				first.ID = "repository.license.missing.local"
				first.RuleID = "repository.license.missing"
				first.Severity = finding.SeverityInformational
				second := testFinding()
				second.ID = "delivery.ci.missing.local"
				second.RuleID = "delivery.ci.missing"
				second.Severity = finding.SeverityCritical
				report.Findings = []Finding{first, second}
				report.Summary.FindingsTotal = len(report.Findings)
			},
		},
		{
			name: "blank recommendation",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Recommendation.Summary = ""
			},
		},
		{
			name: "blank confidence",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Confidence = ""
			},
		},
		{
			name: "blank provenance",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Provenance.Producer = ""
			},
		},
		{
			name: "blank risk",
			mutate: func(report *Report) {
				addValidFinding(report)
				report.Findings[0].Risk.Summary = ""
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
		ID:          "repository.readme.missing.local",
		RuleID:      "repository.readme.missing",
		Title:       "Root README is missing",
		Description: "No root README was detected.",
		Severity:    finding.SeverityMedium,
		Confidence:  finding.ConfidenceHigh,
		Subjects: []finding.Subject{{
			Kind: "repository", ID: "local",
		}},
		Evidence: []finding.Evidence{{
			Kind:        finding.EvidenceObservation,
			Description: "A complete scan found no README at the repository root.",
		}},
		Provenance: finding.Provenance{
			Producer: "iatros.readiness", ProducerVersion: "1.0",
			RuleVersion: "1.0", Source: "repository_snapshot",
		},
		Risk: finding.Risk{
			Level: finding.RiskMedium, Likelihood: finding.LikelihoodLikely,
			Summary: "Contributors may not have verified project instructions.",
		},
		Recommendation: finding.Recommendation{
			Summary: "Document the project at the repository root.",
			Actions: []string{"Add a root README with verified usage instructions."},
		},
		Disposition: finding.DispositionActive,
	}.Normalized()
}
