package artifactvalidation

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlSyntaxValidator struct{}

func (xmlSyntaxValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.syntax.xml", Version: "1.0"}
}

func (xmlSyntaxValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Format == FormatXML
}

func (xmlSyntaxValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	decoder := xml.NewDecoder(document.Reader())
	decoder.Strict = true
	depth := 0
	nodes := 0
	roots := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []Diagnostic{{
				Code: "XML_SYNTAX_INVALID", Level: DiagnosticError,
				Message:  fmt.Sprintf("XML syntax is invalid: %v", err),
				Location: byteOffsetLocation(document.content, decoder.InputOffset()),
			}}, nil
		}
		nodes++
		if nodes > limits.MaxSyntaxNodes {
			return []Diagnostic{{
				Code: "XML_NODES_EXCEEDED", Level: DiagnosticError,
				Message: "XML syntax exceeds the configured token limit.",
			}}, nil
		}
		switch typedToken := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			if depth > limits.MaxNestingDepth {
				return []Diagnostic{{
					Code: "XML_DEPTH_EXCEEDED", Level: DiagnosticError,
					Message: "XML nesting exceeds the configured depth limit.",
				}}, nil
			}
			if len(typedToken.Attr) > limits.MaxSyntaxNodes {
				return []Diagnostic{{
					Code: "XML_ATTRIBUTES_EXCEEDED", Level: DiagnosticError,
					Message: "An XML element exceeds the configured attribute limit.",
				}}, nil
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(typedToken)) != "" {
				return []Diagnostic{{
					Code: "XML_TEXT_OUTSIDE_ROOT", Level: DiagnosticError,
					Message: "XML contains text outside its root element.",
				}}, nil
			}
		case xml.Directive:
			return []Diagnostic{{
				Code: "XML_DIRECTIVE_UNSUPPORTED", Level: DiagnosticError,
				Message: "XML directives are not accepted by the safe validation profile.",
			}}, nil
		case xml.ProcInst:
			if !strings.EqualFold(typedToken.Target, "xml") {
				return []Diagnostic{{
					Code: "XML_PROCESSING_INSTRUCTION_UNSUPPORTED", Level: DiagnosticError,
					Message: "XML processing instructions are not accepted by the safe validation profile.",
				}}, nil
			}
		}
	}
	if roots != 1 {
		return []Diagnostic{{
			Code: "XML_ROOT_INVALID", Level: DiagnosticError,
			Message: "XML must contain exactly one root element.",
		}}, nil
	}
	return make([]Diagnostic, 0), nil
}
