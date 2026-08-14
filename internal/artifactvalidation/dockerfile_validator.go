package artifactvalidation

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

var dockerfileInstructions = map[string]struct{}{
	"ADD": {}, "ARG": {}, "CMD": {}, "COPY": {}, "ENTRYPOINT": {}, "ENV": {},
	"EXPOSE": {}, "FROM": {}, "HEALTHCHECK": {}, "LABEL": {}, "MAINTAINER": {},
	"ONBUILD": {}, "RUN": {}, "SHELL": {}, "STOPSIGNAL": {}, "USER": {},
	"VOLUME": {}, "WORKDIR": {},
}

type dockerfileSyntaxValidator struct{}

func (dockerfileSyntaxValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.syntax.dockerfile", Version: "1.0"}
}

func (dockerfileSyntaxValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Format == FormatDockerfile
}

func (dockerfileSyntaxValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	scanner := bufio.NewScanner(document.Reader())
	scanner.Buffer(make([]byte, 64*1024), int(limits.MaxArtifactBytes)+1)
	escapeCharacter := byte('\\')
	logicalLine := strings.Builder{}
	logicalStart := 0
	hasFrom := false
	instructions := 0
	diagnostics := make([]Diagnostic, 0)
	recordDiagnostic := func(diagnostic Diagnostic) bool {
		if len(diagnostics) > limits.MaxDiagnosticsPerArtifact {
			return true
		}
		diagnostics = append(diagnostics, diagnostic)
		return len(diagnostics) > limits.MaxDiagnosticsPerArtifact
	}
	lineNumber := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lineNumber++
		line := strings.TrimRight(scanner.Text(), " \t\r")
		trimmed := strings.TrimSpace(line)
		if logicalLine.Len() == 0 && strings.HasPrefix(trimmed, "#") {
			escapeCharacter = dockerfileEscapeCharacter(trimmed, escapeCharacter)
			continue
		}
		if logicalLine.Len() == 0 && trimmed == "" {
			continue
		}
		if logicalLine.Len() == 0 {
			logicalStart = lineNumber
		}
		hasContinuation := len(line) > 0 && line[len(line)-1] == escapeCharacter
		if hasContinuation {
			line = strings.TrimSpace(line[:len(line)-1])
		}
		if logicalLine.Len() > 0 {
			logicalLine.WriteByte(' ')
		}
		logicalLine.WriteString(strings.TrimSpace(line))
		if hasContinuation {
			continue
		}

		instruction, argument := dockerfileInstruction(logicalLine.String())
		logicalLine.Reset()
		instructions++
		if instructions > limits.MaxSyntaxNodes {
			return []Diagnostic{{
				Code: "DOCKERFILE_INSTRUCTIONS_EXCEEDED", Level: DiagnosticError,
				Message: "Dockerfile instructions exceed the configured limit.",
			}}, nil
		}
		location := Location{StartLine: logicalStart, StartColumn: 1}
		if instruction == "" {
			if recordDiagnostic(Diagnostic{
				Code: "DOCKERFILE_INSTRUCTION_INVALID", Level: DiagnosticError,
				Message: "Dockerfile contains a malformed instruction.", Location: location,
			}) {
				break
			}
			continue
		}
		if _, known := dockerfileInstructions[instruction]; !known {
			if recordDiagnostic(Diagnostic{
				Code: "DOCKERFILE_INSTRUCTION_UNKNOWN", Level: DiagnosticWarning,
				Message:  fmt.Sprintf("Dockerfile instruction %s is not recognized by the built-in validator.", instruction),
				Location: location,
			}) {
				break
			}
			continue
		}
		if argument == "" {
			if recordDiagnostic(Diagnostic{
				Code: "DOCKERFILE_ARGUMENT_MISSING", Level: DiagnosticError,
				Message:  fmt.Sprintf("Dockerfile instruction %s requires an argument.", instruction),
				Location: location,
			}) {
				break
			}
			continue
		}
		if !hasFrom && instruction != "ARG" && instruction != "FROM" {
			if recordDiagnostic(Diagnostic{
				Code: "DOCKERFILE_BEFORE_FROM", Level: DiagnosticError,
				Message: "Only ARG may appear before the first FROM instruction.", Location: location,
			}) {
				break
			}
		}
		if instruction == "FROM" {
			hasFrom = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if logicalLine.Len() > 0 {
		recordDiagnostic(Diagnostic{
			Code: "DOCKERFILE_CONTINUATION_UNFINISHED", Level: DiagnosticError,
			Message:  "Dockerfile ends with an unfinished line continuation.",
			Location: Location{StartLine: logicalStart, StartColumn: 1},
		})
	}
	if !hasFrom {
		recordDiagnostic(Diagnostic{
			Code: "DOCKERFILE_FROM_MISSING", Level: DiagnosticError,
			Message: "Dockerfile does not contain a FROM instruction.",
		})
	}
	return diagnostics, nil
}

func dockerfileEscapeCharacter(line string, current byte) byte {
	const prefix = "# escape="
	if !strings.HasPrefix(strings.ToLower(line), prefix) || len(line) != len(prefix)+1 {
		return current
	}
	candidate := line[len(line)-1]
	if candidate == '\\' || candidate == '`' {
		return candidate
	}
	return current
}

func dockerfileInstruction(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	instruction := strings.ToUpper(fields[0])
	for _, character := range instruction {
		if character < 'A' || character > 'Z' {
			return "", ""
		}
	}
	argument := strings.TrimSpace(line[len(fields[0]):])
	return instruction, argument
}
