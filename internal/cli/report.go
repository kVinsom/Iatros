package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/analysis"
)

const (
	formatText = "text"
	formatJSON = "json"
)

func writeReport(writer io.Writer, format string, report analysis.Report) error {
	if format == formatJSON {
		return writeJSON(writer, report.Normalized())
	}

	return writeTextReport(writer, report)
}

func validateReportFormat(format string) error {
	if format == formatText || format == formatJSON {
		return nil
	}
	return fmt.Errorf(
		"unsupported format %q; supported formats are text and json",
		format,
	)
}

func commandTarget(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func writeJSON(writer io.Writer, document any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func writeTextReport(writer io.Writer, report analysis.Report) error {
	output := newTextWriter(writer)

	output.line("IATROS Local Repository Analysis")
	output.printf("Status: %s\n", textStatus(report.Status))
	output.printf("Profile: %s\n", report.Profile)
	output.printf("Target: %s\n\n", report.Target.Path)

	switch report.Status {
	case analysis.StatusNotImplemented:
		output.line("No analysis was performed.")
	case analysis.StatusFailed:
		output.line("No analysis was performed.")
		writeTextDiagnostics(output, report.Diagnostics)
	default:
		writeTextSummary(output, report.Summary)
		writeTextTechnologies(output, report.Ecosystems)
		writeTextFindings(output, report.Findings)
		writeTextDiagnostics(output, report.Diagnostics)
	}

	return output.flush()
}

func writeTextSummary(output *textWriter, summary analysis.Summary) {
	output.line("Summary:")
	output.printf("- Directories scanned: %d\n", summary.DirectoriesScanned)
	output.printf("- Files scanned: %d\n", summary.FilesScanned)
	output.printf("- Nested repositories skipped: %d\n", summary.NestedRepositoriesSkipped)
	output.printf("- Ecosystems detected: %d\n", summary.EcosystemsDetected)
	output.printf("- Findings total: %d\n", summary.FindingsTotal)
}

func writeTextTechnologies(output *textWriter, ecosystems []analysis.Ecosystem) {
	output.line()
	output.line("Technologies:")
	if len(ecosystems) == 0 {
		output.line("- None")
		return
	}

	byCategory := make(map[string][]analysis.Ecosystem)
	for _, ecosystem := range ecosystems {
		byCategory[ecosystem.Category] = append(byCategory[ecosystem.Category], ecosystem)
	}
	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	slices.Sort(categories)

	for _, category := range categories {
		output.printf("- %s:\n", technologyCategoryLabel(category))
		for _, ecosystem := range byCategory[category] {
			output.printf("  - %s\n", ecosystem.ID)
			writeIndentedTextEvidence(output, ecosystem.Evidence)
			output.printf("    Evidence truncated: %t\n", ecosystem.EvidenceTruncated)
		}
	}
}

func technologyCategoryLabel(category string) string {
	switch category {
	case "ci_cd":
		return "CI/CD"
	case "gitops":
		return "GitOps"
	}

	label := strings.ReplaceAll(category, "_", " ")
	return strings.ToUpper(label[:1]) + label[1:]
}

func writeIndentedTextEvidence(output *textWriter, evidence []string) {
	if len(evidence) == 0 {
		output.line("    Evidence: none")
		return
	}

	output.line("    Evidence:")
	for _, evidencePath := range evidence {
		output.printf("      - %s\n", evidencePath)
	}
}

func writeTextFindings(output *textWriter, findings []analysis.Finding) {
	output.line()
	output.line("Findings:")
	if len(findings) == 0 {
		output.line("- None")
		return
	}

	for _, finding := range findings {
		output.printf("- [%s] %s: %s\n", finding.Severity, finding.Code, finding.Message)
		writeTextEvidence(output, finding.Evidence)
		output.printf("  Remediation: %s\n", finding.Remediation)
	}
}

func writeTextEvidence(output *textWriter, evidence []string) {
	if len(evidence) == 0 {
		output.line("  Evidence: none")
		return
	}

	output.line("  Evidence:")
	for _, evidenceText := range evidence {
		output.printf("    - %s\n", evidenceText)
	}
}

func writeTextDiagnostics(output *textWriter, diagnostics []analysis.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}

	output.line()
	output.line("Diagnostics:")
	for _, diagnostic := range diagnostics {
		output.printf(
			"- [%s] %s: %s\n",
			diagnostic.Level,
			diagnostic.Code,
			diagnostic.Message,
		)
	}
}

func textStatus(status analysis.Status) string {
	return strings.ReplaceAll(string(status), "_", " ")
}

type textWriter struct {
	buffer *bufio.Writer
	err    error
}

func newTextWriter(writer io.Writer) *textWriter {
	return &textWriter{buffer: bufio.NewWriter(writer)}
}

func (w *textWriter) line(arguments ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(w.buffer, arguments...)
}

func (w *textWriter) printf(format string, arguments ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.buffer, format, arguments...)
}

func (w *textWriter) flush() error {
	if w.err != nil {
		return w.err
	}
	return w.buffer.Flush()
}
