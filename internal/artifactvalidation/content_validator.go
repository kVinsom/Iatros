package artifactvalidation

import (
	"context"
	"unicode/utf8"
)

type contentValidator struct{}

func (contentValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.content", Version: "1.0"}
}

func (contentValidator) AppliesTo(Artifact) bool {
	return true
}

func (contentValidator) Validate(
	ctx context.Context,
	document Document,
	_ Limits,
) ([]Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content := document.content
	if len(content) == 0 {
		return []Diagnostic{{
			Code: "ARTIFACT_EMPTY", Level: DiagnosticError,
			Message: "The artifact is empty.",
		}}, nil
	}
	if !utf8.Valid(content) {
		return []Diagnostic{{
			Code: "CONTENT_NOT_UTF8", Level: DiagnosticError,
			Message: "The artifact is not valid UTF-8 text.",
		}}, nil
	}
	for _, character := range content {
		if character == 0 {
			return []Diagnostic{{
				Code: "CONTENT_NUL_BYTE", Level: DiagnosticError,
				Message: "The artifact contains a NUL byte.",
			}}, nil
		}
	}
	return make([]Diagnostic, 0), nil
}
