package artifactvalidation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	engineValidatorID      = "iatros.validation.engine"
	engineValidatorVersion = "1.0"
)

// Engine coordinates bounded sources and replaceable validators.
type Engine struct {
	limits     Limits
	validators []registeredValidator
}

type registeredValidator struct {
	descriptor ValidatorDescriptor
	validator  Validator
}

// NewEngine constructs an engine with an explicit validator set.
func NewEngine(limits Limits, validators ...Validator) (*Engine, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if len(validators) == 0 || len(validators) > limits.MaxValidators {
		return nil, ErrInvalidValidator
	}
	registered := make([]registeredValidator, 0, len(validators))
	for _, validator := range validators {
		if validator == nil {
			return nil, ErrInvalidValidator
		}
		descriptor := validator.Descriptor()
		if !validValidatorDescriptor(descriptor) {
			return nil, ErrInvalidValidator
		}
		registered = append(registered, registeredValidator{
			descriptor: descriptor,
			validator:  validator,
		})
	}
	slices.SortFunc(registered, func(left, right registeredValidator) int {
		return strings.Compare(left.descriptor.ID, right.descriptor.ID)
	})
	for index, validator := range registered {
		if index > 0 && registered[index-1].descriptor.ID == validator.descriptor.ID {
			return nil, fmt.Errorf("%w: duplicate id %q", ErrInvalidValidator, validator.descriptor.ID)
		}
	}
	return &Engine{limits: limits, validators: registered}, nil
}

// NewDefaultEngine constructs an engine with the provider-neutral built-in validators.
func NewDefaultEngine(limits Limits) (*Engine, error) {
	return NewEngine(limits, BuiltInValidators()...)
}

// Validate reads and validates artifacts in deterministic ID order.
func (e *Engine) Validate(ctx context.Context, requests []Request) (Report, error) {
	if e == nil || ctx == nil {
		return Report{}, ErrInvalidRequest
	}
	if err := e.limits.Validate(); err != nil {
		return Report{}, err
	}
	normalizedRequests, err := validateRequests(requests, e.limits)
	if err != nil {
		return Report{}, err
	}

	validationContext, cancel := context.WithTimeout(ctx, e.limits.Timeout)
	defer cancel()

	report := Report{
		SchemaVersion: CurrentSchemaVersion,
		Artifacts:     make([]ArtifactResult, 0, len(normalizedRequests)),
		Diagnostics:   make([]Diagnostic, 0),
	}
	var totalBytes int64
	isPartial := false
	for _, request := range normalizedRequests {
		if err := validationContext.Err(); err != nil {
			isPartial = true
			break
		}
		result, diagnostics, artifactPartial := e.validateArtifact(
			validationContext,
			request,
			e.limits.MaxTotalBytes-totalBytes,
		)
		totalBytes += result.BytesRead
		report.Artifacts = append(report.Artifacts, result)
		isPartial = isPartial || artifactPartial
		availableDiagnostics := e.limits.MaxDiagnostics - len(report.Diagnostics)
		if availableDiagnostics <= 0 {
			report.Diagnostics[len(report.Diagnostics)-1] = engineDiagnostic(
				request.Artifact,
				"DIAGNOSTICS_TRUNCATED",
				DiagnosticError,
				"Validation diagnostics exceeded the configured report limit.",
				e.limits.MaxTextBytes,
			)
			isPartial = true
			break
		}
		if len(diagnostics) > availableDiagnostics {
			diagnostics = diagnostics[:availableDiagnostics]
			diagnostics[len(diagnostics)-1] = engineDiagnostic(
				request.Artifact,
				"DIAGNOSTICS_TRUNCATED",
				DiagnosticError,
				"Validation diagnostics exceeded the configured report limit.",
				e.limits.MaxTextBytes,
			)
			isPartial = true
		}
		report.Diagnostics = append(report.Diagnostics, diagnostics...)
	}
	for len(report.Artifacts) < len(normalizedRequests) {
		request := normalizedRequests[len(report.Artifacts)]
		report.Artifacts = append(report.Artifacts, ArtifactResult{
			Artifact:   request.Artifact,
			Outcome:    OutcomeUnavailable,
			Validators: make([]string, 0),
		})
	}
	report.Status = derivedStatus(report.Artifacts, report.Diagnostics, isPartial)
	report = report.Normalized()
	if validationErr := report.ValidateWithin(e.limits); validationErr != nil {
		return Report{}, fmt.Errorf("validate artifact report: %w", validationErr)
	}
	if err := validationContext.Err(); err != nil {
		return report, err
	}
	return report, nil
}

func (e *Engine) validateArtifact(
	ctx context.Context,
	request Request,
	remainingBytes int64,
) (ArtifactResult, []Diagnostic, bool) {
	result := ArtifactResult{
		Artifact:   request.Artifact,
		Outcome:    OutcomeUnavailable,
		Validators: make([]string, 0),
	}
	if remainingBytes <= 0 {
		return result, []Diagnostic{engineDiagnostic(
			request.Artifact,
			"TOTAL_BYTES_EXCEEDED",
			DiagnosticError,
			"The total artifact validation byte budget was exhausted.",
			e.limits.MaxTextBytes,
		)}, true
	}

	content, bytesRead, readCode, readMessage := readArtifact(
		ctx,
		request,
		e.limits.MaxArtifactBytes,
		remainingBytes,
	)
	result.BytesRead = bytesRead
	if readCode != "" {
		return result, []Diagnostic{engineDiagnostic(
			request.Artifact,
			readCode,
			DiagnosticError,
			readMessage,
			e.limits.MaxTextBytes,
		)}, true
	}
	digest := sha256.Sum256(content)
	result.Digest = "sha256:" + hex.EncodeToString(digest[:])
	document := Document{
		Artifact: request.Artifact,
		Digest:   result.Digest,
		Size:     int64(len(content)),
		content:  content,
	}

	diagnostics := make([]Diagnostic, 0)
	isPartial := false
	for _, registered := range e.validators {
		if !registered.validator.AppliesTo(request.Artifact) {
			continue
		}
		descriptor := registered.descriptor
		result.Validators = append(result.Validators, descriptor.ID)
		validatorDiagnostics, err := registered.validator.Validate(ctx, document, e.limits)
		if err != nil {
			isPartial = true
			validatorDiagnostics = []Diagnostic{{
				Code:    "VALIDATOR_FAILED",
				Level:   DiagnosticError,
				Message: "The validator could not complete.",
			}}
		}
		availableDiagnostics := e.limits.MaxDiagnosticsPerArtifact - len(diagnostics)
		if availableDiagnostics <= 0 {
			diagnostics[len(diagnostics)-1] = engineDiagnostic(
				request.Artifact,
				"DIAGNOSTICS_TRUNCATED",
				DiagnosticError,
				"Artifact diagnostics exceeded the configured retention limit.",
				e.limits.MaxTextBytes,
			)
			isPartial = true
			break
		}
		diagnosticsExceeded := len(validatorDiagnostics) > availableDiagnostics
		if diagnosticsExceeded {
			validatorDiagnostics = validatorDiagnostics[:availableDiagnostics]
		}
		for index := range validatorDiagnostics {
			validatorDiagnostics[index] = completeDiagnostic(
				request.Artifact,
				descriptor,
				validatorDiagnostics[index],
				e.limits.MaxTextBytes,
			)
		}
		diagnostics = append(diagnostics, validatorDiagnostics...)
		if diagnosticsExceeded {
			lastIndex := len(diagnostics) - 1
			diagnostics[lastIndex] = engineDiagnostic(
				request.Artifact,
				"DIAGNOSTICS_TRUNCATED",
				DiagnosticError,
				"Artifact diagnostics exceeded the configured retention limit.",
				e.limits.MaxTextBytes,
			)
			isPartial = true
			break
		}
		if err := ctx.Err(); err != nil {
			isPartial = true
			break
		}
	}
	if len(result.Validators) == 0 {
		diagnostics = append(diagnostics, engineDiagnostic(
			request.Artifact,
			"VALIDATOR_NOT_FOUND",
			DiagnosticError,
			"No registered validator supports this artifact.",
			e.limits.MaxTextBytes,
		))
		isPartial = true
	}
	result.Outcome = derivedOutcome(diagnostics, isPartial)
	return result, diagnostics, isPartial
}

func validateRequests(requests []Request, limits Limits) ([]Request, error) {
	if len(requests) == 0 || len(requests) > limits.MaxArtifacts {
		return nil, ErrInvalidRequest
	}
	normalized := slices.Clone(requests)
	slices.SortFunc(normalized, func(left, right Request) int {
		return strings.Compare(left.Artifact.ID, right.Artifact.ID)
	})
	for index, request := range normalized {
		if err := request.Artifact.Validate(); err != nil || request.Source == nil {
			return nil, fmt.Errorf("%w: artifact %d", ErrInvalidRequest, index)
		}
		if index > 0 && normalized[index-1].Artifact.ID == request.Artifact.ID {
			return nil, fmt.Errorf("%w: duplicate artifact id %q", ErrInvalidRequest, request.Artifact.ID)
		}
	}
	return normalized, nil
}

func readArtifact(
	ctx context.Context,
	request Request,
	maxArtifactBytes int64,
	remainingBytes int64,
) ([]byte, int64, string, string) {
	reader, err := request.Source.Open(ctx, request.Artifact)
	if err != nil {
		if reader != nil {
			_ = reader.Close()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, 0, "SOURCE_CANCELLED", "Artifact content reading was cancelled."
		}
		return nil, 0, "SOURCE_UNAVAILABLE", "Artifact content could not be opened."
	}
	if reader == nil {
		return nil, 0, "SOURCE_UNAVAILABLE", "Artifact content could not be opened."
	}
	readLimit := min(maxArtifactBytes, remainingBytes)
	content, readErr := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: reader}, readLimit+1))
	closeErr := reader.Close()
	bytesRead := int64(len(content))
	if readErr != nil {
		return nil, bytesRead, "SOURCE_READ_FAILED", "Artifact content could not be read completely."
	}
	if closeErr != nil {
		return nil, bytesRead, "SOURCE_CLOSE_FAILED", "Artifact content could not be closed cleanly."
	}
	if bytesRead > remainingBytes {
		return nil, bytesRead, "TOTAL_BYTES_EXCEEDED", "The total artifact validation byte budget was exceeded."
	}
	if bytesRead > maxArtifactBytes {
		return nil, bytesRead, "ARTIFACT_BYTES_EXCEEDED", "The artifact exceeds the configured byte limit."
	}
	return content, bytesRead, "", ""
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func completeDiagnostic(
	artifact Artifact,
	descriptor ValidatorDescriptor,
	diagnostic Diagnostic,
	maxTextBytes int,
) Diagnostic {
	diagnostic.ArtifactID = artifact.ID
	diagnostic.Path = artifact.Path
	diagnostic.ValidatorID = descriptor.ID
	diagnostic.ValidatorVersion = descriptor.Version
	diagnostic.Message = boundedMessage(diagnostic.Message, maxTextBytes)
	if !validDiagnosticCode(diagnostic.Code) {
		diagnostic.Code = "INVALID_VALIDATOR_DIAGNOSTIC"
		diagnostic.Level = DiagnosticError
		diagnostic.Message = "The validator returned a malformed diagnostic."
		diagnostic.Location = Location{}
	}
	if !validDiagnosticLevel(diagnostic.Level) || !validLocation(diagnostic.Location) {
		diagnostic.Level = DiagnosticError
		diagnostic.Message = "The validator returned a malformed diagnostic."
		diagnostic.Location = Location{}
	}
	diagnostic.Message = boundedMessage(diagnostic.Message, maxTextBytes)
	return diagnostic
}

func engineDiagnostic(
	artifact Artifact,
	code string,
	level DiagnosticLevel,
	message string,
	maxTextBytes int,
) Diagnostic {
	return Diagnostic{
		Code:             code,
		Level:            level,
		Message:          boundedMessage(message, maxTextBytes),
		ArtifactID:       artifact.ID,
		Path:             artifact.Path,
		ValidatorID:      engineValidatorID,
		ValidatorVersion: engineValidatorVersion,
	}
}

func boundedMessage(message string, maxBytes int) string {
	message = strings.Join(strings.Fields(message), " ")
	message = strings.Map(func(character rune) rune {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return -1
		}
		return character
	}, message)
	if message == "" {
		return "The validator reported a diagnostic without a safe message."
	}
	if len(message) <= maxBytes {
		return message
	}
	message = message[:maxBytes]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}
	return strings.TrimSpace(message)
}

func validValidatorDescriptor(descriptor ValidatorDescriptor) bool {
	return validNamespacedIdentifier(descriptor.ID) && validVersion(descriptor.Version)
}

func derivedOutcome(diagnostics []Diagnostic, isPartial bool) Outcome {
	if isPartial {
		return OutcomeUnavailable
	}
	result := OutcomeValid
	for _, diagnostic := range diagnostics {
		switch diagnostic.Level {
		case DiagnosticError:
			return OutcomeInvalid
		case DiagnosticWarning:
			result = OutcomeWarning
		}
	}
	return result
}

func derivedStatus(results []ArtifactResult, diagnostics []Diagnostic, isPartial bool) Status {
	if isPartial || hasUnavailableArtifact(results) {
		return StatusPartial
	}
	status := StatusPassed
	for _, result := range results {
		switch result.Outcome {
		case OutcomeInvalid:
			return StatusFailed
		case OutcomeWarning:
			status = StatusPassedWithWarnings
		}
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == DiagnosticError {
			return StatusFailed
		}
		if diagnostic.Level == DiagnosticWarning {
			status = StatusPassedWithWarnings
		}
	}
	return status
}

func hasUnavailableArtifact(results []ArtifactResult) bool {
	return slices.ContainsFunc(results, func(result ArtifactResult) bool {
		return result.Outcome == OutcomeUnavailable
	})
}
