package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kVinsom/Iatros/internal/analysis"
)

const (
	formatText = "text"
	formatJSON = "json"
)

func writeReport(writer io.Writer, format string, report analysis.Report) error {
	report = report.Normalized()

	if format == formatJSON {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	return writeTextReport(writer, report)
}

func writeTextReport(writer io.Writer, report analysis.Report) error {
	output := newTextWriter(writer)

	output.line("IATROS Local Repository Analysis")
	output.printf("Status: %s\n", textStatus(report.Status))
	output.printf("Target: %s\n\n", report.Target.Path)

	switch report.Status {
	case analysis.StatusNotImplemented:
		output.line("No analysis was performed.")
	case analysis.StatusFailed:
		output.line("No analysis was performed.")
		writeTextDiagnostics(output, report.Diagnostics)
	default:
		writeTextSummary(output, report.Summary)
		writeTextEcosystems(output, report.Ecosystems)
		writeTextFindings(output, report.Findings)
		writeTextDiagnostics(output, report.Diagnostics)
	}

	return output.flush()
}

func writeTextSummary(output *textWriter, summary analysis.Summary) {
	output.line("Summary:")
	output.printf("- Directories scanned: %d\n", summary.DirectoriesScanned)
	output.printf("- Files scanned: %d\n", summary.FilesScanned)
	output.printf("- Ecosystems detected: %d\n", summary.EcosystemsDetected)
	output.printf("- Findings total: %d\n", summary.FindingsTotal)
}

func writeTextEcosystems(output *textWriter, ecosystems []analysis.Ecosystem) {
	output.line()
	output.line("Ecosystems:")
	if len(ecosystems) == 0 {
		output.line("- None")
		return
	}

	for _, ecosystem := range ecosystems {
		output.printf("- %s\n", ecosystem.ID)
		writeTextEvidence(output, ecosystem.Evidence)
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
	for _, item := range evidence {
		output.printf("    - %s\n", item)
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

func (w *textWriter) line(values ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(w.buffer, values...)
}

func (w *textWriter) printf(format string, values ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.buffer, format, values...)
}

func (w *textWriter) flush() error {
	if w.err != nil {
		return w.err
	}
	return w.buffer.Flush()
}
