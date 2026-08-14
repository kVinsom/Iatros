package artifactvalidation

import (
	"context"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type hclSyntaxValidator struct{}

func (hclSyntaxValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.syntax.hcl", Version: "1.0"}
}

func (hclSyntaxValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Format == FormatHCL
}

func (hclSyntaxValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, hclDiagnostics := hclsyntax.ParseConfig(document.content, document.Artifact.Path, hcl.InitialPos)
	diagnostics := make([]Diagnostic, 0, min(len(hclDiagnostics), limits.MaxDiagnosticsPerArtifact+1))
	for _, hclDiagnostic := range hclDiagnostics {
		if len(diagnostics) > limits.MaxDiagnosticsPerArtifact {
			break
		}
		level := DiagnosticWarning
		if hclDiagnostic.Severity == hcl.DiagError {
			level = DiagnosticError
		}
		diagnostic := Diagnostic{
			Code:    "HCL_SYNTAX_INVALID",
			Level:   level,
			Message: strings.TrimSpace(hclDiagnostic.Summary + ": " + hclDiagnostic.Detail),
		}
		if hclDiagnostic.Subject != nil {
			diagnostic.Location = Location{
				StartLine:   hclDiagnostic.Subject.Start.Line,
				StartColumn: hclDiagnostic.Subject.Start.Column,
				EndLine:     hclDiagnostic.Subject.End.Line,
				EndColumn:   hclDiagnostic.Subject.End.Column,
			}
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics, nil
}
