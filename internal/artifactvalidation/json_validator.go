package artifactvalidation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type jsonSyntaxValidator struct{}

func (jsonSyntaxValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.syntax.json", Version: "1.0"}
}

func (jsonSyntaxValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Format == FormatJSON
}

func (jsonSyntaxValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	decoder := json.NewDecoder(document.Reader())
	decoder.UseNumber()
	depth := 0
	nodes := 0
	rootComplete := false
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
				Code: "JSON_SYNTAX_INVALID", Level: DiagnosticError,
				Message:  fmt.Sprintf("JSON syntax is invalid: %v", err),
				Location: byteOffsetLocation(document.content, decoder.InputOffset()),
			}}, nil
		}
		if depth == 0 && rootComplete {
			return []Diagnostic{{
				Code: "JSON_MULTIPLE_VALUES", Level: DiagnosticError,
				Message:  "JSON contains more than one top-level value.",
				Location: byteOffsetLocation(document.content, decoder.InputOffset()),
			}}, nil
		}
		nodes++
		if nodes > limits.MaxSyntaxNodes {
			return []Diagnostic{{
				Code: "JSON_NODES_EXCEEDED", Level: DiagnosticError,
				Message: "JSON syntax exceeds the configured node limit.",
			}}, nil
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			if depth == 0 {
				rootComplete = true
			}
			continue
		}
		switch delimiter {
		case '{', '[':
			depth++
			if depth > limits.MaxNestingDepth {
				return []Diagnostic{{
					Code: "JSON_DEPTH_EXCEEDED", Level: DiagnosticError,
					Message: "JSON nesting exceeds the configured depth limit.",
				}}, nil
			}
		case '}', ']':
			depth--
			if depth == 0 {
				rootComplete = true
			}
		}
	}
	if nodes == 0 {
		return []Diagnostic{{
			Code: "JSON_EMPTY", Level: DiagnosticError,
			Message: "JSON does not contain a value.",
		}}, nil
	}
	if depth != 0 || !rootComplete {
		return []Diagnostic{{
			Code: "JSON_SYNTAX_INVALID", Level: DiagnosticError,
			Message:  "JSON ends before the top-level value is complete.",
			Location: byteOffsetLocation(document.content, decoder.InputOffset()),
		}}, nil
	}
	return make([]Diagnostic, 0), nil
}

func byteOffsetLocation(content []byte, offset int64) Location {
	if offset < 0 {
		return Location{}
	}
	if offset > int64(len(content)) {
		offset = int64(len(content))
	}
	line := 1
	column := 1
	for _, character := range string(content[:offset]) {
		if character == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return Location{StartLine: line, StartColumn: column}
}
