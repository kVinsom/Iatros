package codeanalysis

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

// ErrInvalidModel indicates that a code-analysis model violates its normalized contract.
var ErrInvalidModel = errors.New("code analysis model is invalid")

// Validate checks schema compatibility, references, ordering, and safety invariants.
func (m Model) Validate() error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) {
		return invalidModel("schema_version", "is unsupported")
	}

	serviceIDs, err := validateServices(m.Services)
	if err != nil {
		return err
	}
	if err := validateFrameworks(m.Frameworks, serviceIDs); err != nil {
		return err
	}
	if err := validatePortBindings(m.PortBindings, serviceIDs); err != nil {
		return err
	}
	if err := validateAPIEndpoints(m.APIEndpoints, serviceIDs); err != nil {
		return err
	}
	if err := validateEnvironmentVariables(m.EnvironmentVariables, serviceIDs); err != nil {
		return err
	}
	if err := validateResourceDependencies(m.ResourceDependencies, serviceIDs); err != nil {
		return err
	}
	if err := validateDiagnostics(m); err != nil {
		return err
	}
	return nil
}

// ValidateWithin checks the normalized model and verifies its retained facts against limits.
func (m Model) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Services) > limits.MaxServices || len(m.Frameworks) > limits.MaxFrameworks ||
		len(m.PortBindings) > limits.MaxPortBindings ||
		len(m.APIEndpoints) > limits.MaxAPIEndpoints ||
		len(m.EnvironmentVariables) > limits.MaxEnvironmentVariables ||
		len(m.ResourceDependencies) > limits.MaxResourceDependencies ||
		len(m.Diagnostics) > limits.MaxDiagnostics {
		return invalidModel("collections", "exceed configured retention limits")
	}
	if !factsFitLimits(m, limits) {
		return invalidModel("facts", "exceed configured evidence or text limits")
	}
	return m.Validate()
}

func validateServices(services []Service) (map[ServiceID]struct{}, error) {
	serviceIDs := make(map[ServiceID]struct{}, len(services))
	for index, service := range services {
		if !validIdentifier(string(service.ID)) || !validText(service.Name) ||
			!validIdentifier(service.Kind) || !repositorypath.IsValidDirectory(service.Root) ||
			!validCertainty(service.Certainty) || !validEvidence(service.Evidence) {
			return nil, invalidModel(indexedField("services", index), "is invalid")
		}
		if index > 0 && services[index-1].ID >= service.ID {
			return nil, invalidModel("services", "must be strictly ordered by id")
		}
		for entrypointIndex, entrypoint := range service.Entrypoints {
			if !repositorypath.IsValidFile(entrypoint) ||
				!repositorypath.Contains(service.Root, entrypoint) ||
				(entrypointIndex > 0 && service.Entrypoints[entrypointIndex-1] >= entrypoint) {
				return nil, invalidModel(
					indexedField("services", index)+".entrypoints",
					"must contain ordered repository files within the service root",
				)
			}
		}
		serviceIDs[service.ID] = struct{}{}
	}
	return serviceIDs, nil
}

func validateFrameworks(frameworks []Framework, serviceIDs map[ServiceID]struct{}) error {
	for index, framework := range frameworks {
		if !knownService(framework.ServiceID, serviceIDs) || !validIdentifier(framework.ID) ||
			!validText(framework.Name) || !validIdentifier(framework.Language) ||
			!validOptionalText(framework.VersionConstraint) ||
			!validCertainty(framework.Certainty) || !validEvidence(framework.Evidence) {
			return invalidModel(indexedField("frameworks", index), "is invalid")
		}
		if index > 0 && compareFrameworks(frameworks[index-1], framework) >= 0 {
			return invalidModel("frameworks", "must be strictly ordered")
		}
	}
	return nil
}

func validatePortBindings(bindings []PortBinding, serviceIDs map[ServiceID]struct{}) error {
	for index, binding := range bindings {
		if !knownService(binding.ServiceID, serviceIDs) || !validPortRepresentation(binding) ||
			!validOptionalText(binding.Name) ||
			!validProtocol(binding.Protocol) || !validCertainty(binding.Certainty) ||
			!validEvidence(binding.Evidence) {
			return invalidModel(indexedField("port_bindings", index), "is invalid")
		}
		if index > 0 && comparePortBindings(bindings[index-1], binding) >= 0 {
			return invalidModel("port_bindings", "must be strictly ordered")
		}
	}
	return nil
}

func validateAPIEndpoints(endpoints []APIEndpoint, serviceIDs map[ServiceID]struct{}) error {
	for index, endpoint := range endpoints {
		if !knownService(endpoint.ServiceID, serviceIDs) || !validProtocol(endpoint.Protocol) ||
			!validOptionalIdentifier(endpoint.Method) || !validText(endpoint.Path) ||
			!validOptionalText(endpoint.Operation) || !validCertainty(endpoint.Certainty) ||
			!validEvidence(endpoint.Evidence) {
			return invalidModel(indexedField("api_endpoints", index), "is invalid")
		}
		if index > 0 && compareAPIEndpoints(endpoints[index-1], endpoint) >= 0 {
			return invalidModel("api_endpoints", "must be strictly ordered")
		}
	}
	return nil
}

func validateEnvironmentVariables(
	variables []EnvironmentVariable,
	serviceIDs map[ServiceID]struct{},
) error {
	for index, variable := range variables {
		if !knownService(variable.ServiceID, serviceIDs) || !validEnvironmentName(variable.Name) ||
			(variable.IsRequired && variable.HasDefault) ||
			!validCertainty(variable.Certainty) || !validEvidence(variable.Evidence) {
			return invalidModel(indexedField("environment_variables", index), "is invalid")
		}
		if index > 0 && compareEnvironmentVariables(variables[index-1], variable) >= 0 {
			return invalidModel("environment_variables", "must be strictly ordered")
		}
	}
	return nil
}

func validateResourceDependencies(
	dependencies []ResourceDependency,
	serviceIDs map[ServiceID]struct{},
) error {
	for index, dependency := range dependencies {
		if !knownService(dependency.ServiceID, serviceIDs) || !validResourceKind(dependency.Kind) ||
			!validIdentifier(dependency.Technology) || !validOptionalText(dependency.Name) ||
			!validCertainty(dependency.Certainty) || !validEvidence(dependency.Evidence) {
			return invalidModel(indexedField("resource_dependencies", index), "is invalid")
		}
		if index > 0 && compareResourceDependencies(dependencies[index-1], dependency) >= 0 {
			return invalidModel("resource_dependencies", "must be strictly ordered")
		}
	}
	return nil
}

func validateDiagnostics(model Model) error {
	if model.Partial && len(model.Diagnostics) == 0 {
		return invalidModel("diagnostics", "must explain a partial result")
	}
	hasLimitingDiagnostic := false
	for index, diagnostic := range model.Diagnostics {
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!repositorypath.IsValidDirectory(diagnostic.Path) || !validText(diagnostic.Message) {
			return invalidModel(indexedField("diagnostics", index), "is invalid")
		}
		if !model.Partial && diagnostic.Level != DiagnosticInfo {
			return invalidModel(indexedField("diagnostics", index), "requires a partial result")
		}
		if diagnostic.Level == DiagnosticWarning || diagnostic.Level == DiagnosticError {
			hasLimitingDiagnostic = true
		}
		if index > 0 && compareDiagnostics(model.Diagnostics[index-1], diagnostic) >= 0 {
			return invalidModel("diagnostics", "must be strictly ordered")
		}
	}
	if model.Partial && !hasLimitingDiagnostic {
		return invalidModel("diagnostics", "must explain the partial result with a warning or error")
	}
	return nil
}

func validEvidence(evidence []Evidence) bool {
	if len(evidence) == 0 {
		return false
	}
	for index, observation := range evidence {
		if !validEvidenceKind(observation.Kind) || !repositorypath.IsValidFile(observation.Path) ||
			!validEvidencePosition(observation) ||
			(index > 0 && compareEvidence(evidence[index-1], observation) >= 0) {
			return false
		}
	}
	return true
}

func validEvidencePosition(evidence Evidence) bool {
	positions := [4]int{
		evidence.StartLine,
		evidence.StartColumn,
		evidence.EndLine,
		evidence.EndColumn,
	}
	if positions == [4]int{} {
		return true
	}
	if evidence.StartLine <= 0 || evidence.StartColumn <= 0 ||
		evidence.EndLine <= 0 || evidence.EndColumn <= 0 {
		return false
	}
	return evidence.EndLine > evidence.StartLine ||
		(evidence.EndLine == evidence.StartLine && evidence.EndColumn >= evidence.StartColumn)
}

func knownService(serviceID ServiceID, serviceIDs map[ServiceID]struct{}) bool {
	_, exists := serviceIDs[serviceID]
	return exists
}

func validCertainty(certainty Certainty) bool {
	return certainty == CertaintyObserved || certainty == CertaintyInferred
}

func validEvidenceKind(kind EvidenceKind) bool {
	switch kind {
	case EvidenceSource, EvidenceImport, EvidenceManifest, EvidenceConfiguration,
		EvidenceInfrastructure:
		return true
	default:
		return false
	}
}

func validResourceKind(kind ResourceKind) bool {
	return kind == ResourceDatabase || kind == ResourceCache || kind == ResourceMessageBroker
}

func validProtocol(protocol Protocol) bool {
	return validIdentifier(string(protocol))
}

func validPortRepresentation(binding PortBinding) bool {
	if binding.Port != 0 {
		return binding.Reference == "" && binding.ReferenceKind == ""
	}
	if binding.Reference == "" {
		return false
	}

	switch binding.ReferenceKind {
	case PortReferenceEnvironment:
		return validEnvironmentName(binding.Reference)
	case PortReferenceConfiguration:
		return validConfigurationReference(binding.Reference)
	default:
		return false
	}
}

func validConfigurationReference(reference string) bool {
	if reference == "" || !asciiAlphaNumeric(reference[0]) ||
		!asciiAlphaNumeric(reference[len(reference)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(reference) {
		character := reference[index]
		if asciiAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if (character != '-' && character != '_' && character != '.') || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func validDiagnosticLevel(level DiagnosticLevel) bool {
	return level == DiagnosticInfo || level == DiagnosticWarning || level == DiagnosticError
}

func validIdentifier(identifier string) bool {
	if identifier == "" || !lowerAlphaNumeric(identifier[0]) ||
		!lowerAlphaNumeric(identifier[len(identifier)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(identifier) {
		character := identifier[index]
		if lowerAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if (character != '-' && character != '_' && character != '.') || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func validOptionalIdentifier(identifier string) bool {
	return identifier == "" || validIdentifier(identifier)
}

func validEnvironmentName(name string) bool {
	if name == "" || (name[0] != '_' && !asciiLetter(name[0])) {
		return false
	}
	for index := 1; index < len(name); index++ {
		character := name[index]
		if character != '_' && !asciiLetter(character) &&
			(character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validDiagnosticCode(code string) bool {
	if code == "" || !upperAlphaNumeric(code[0]) || !upperAlphaNumeric(code[len(code)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(code) {
		character := code[index]
		if upperAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if character != '_' || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func validOptionalText(text string) bool {
	return text == "" || validText(text)
}

func validText(text string) bool {
	if text == "" || !utf8.ValidString(text) || strings.TrimSpace(text) != text {
		return false
	}
	for _, character := range text {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func asciiLetter(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z')
}

func asciiAlphaNumeric(character byte) bool {
	return asciiLetter(character) || (character >= '0' && character <= '9')
}

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}

func factsFitLimits(model Model, limits Limits) bool {
	for _, service := range model.Services {
		if len(service.Entrypoints) > limits.MaxFiles || !textsFitLimit(
			limits.MaxTextBytes,
			service.Entrypoints...,
		) || !factFitsLimits(
			service.Evidence,
			limits,
			string(service.ID),
			service.Name,
			service.Kind,
			service.Root,
			string(service.Certainty),
		) {
			return false
		}
	}
	for _, framework := range model.Frameworks {
		if !factFitsLimits(
			framework.Evidence,
			limits,
			framework.ID,
			framework.Name,
			framework.Language,
			framework.VersionConstraint,
			string(framework.ServiceID),
			string(framework.Certainty),
		) {
			return false
		}
	}
	for _, binding := range model.PortBindings {
		if !factFitsLimits(
			binding.Evidence,
			limits,
			string(binding.ServiceID),
			binding.Name,
			binding.Reference,
			string(binding.ReferenceKind),
			string(binding.Protocol),
			string(binding.Certainty),
		) {
			return false
		}
	}
	for _, endpoint := range model.APIEndpoints {
		if !factFitsLimits(
			endpoint.Evidence,
			limits,
			string(endpoint.Protocol),
			endpoint.Method,
			endpoint.Path,
			endpoint.Operation,
			string(endpoint.ServiceID),
			string(endpoint.Certainty),
		) {
			return false
		}
	}
	for _, variable := range model.EnvironmentVariables {
		if !factFitsLimits(
			variable.Evidence,
			limits,
			string(variable.ServiceID),
			variable.Name,
			string(variable.Certainty),
		) {
			return false
		}
	}
	for _, dependency := range model.ResourceDependencies {
		if !factFitsLimits(
			dependency.Evidence,
			limits,
			string(dependency.Kind),
			dependency.Technology,
			dependency.Name,
			string(dependency.ServiceID),
			string(dependency.Certainty),
		) {
			return false
		}
	}
	for _, diagnostic := range model.Diagnostics {
		if !textsFitLimit(
			limits.MaxTextBytes,
			diagnostic.Code,
			string(diagnostic.Level),
			diagnostic.Path,
			diagnostic.Message,
		) {
			return false
		}
	}
	return true
}

func factFitsLimits(evidence []Evidence, limits Limits, fields ...string) bool {
	if len(evidence) > limits.MaxEvidencePerFact || !textsFitLimit(limits.MaxTextBytes, fields...) {
		return false
	}
	for _, observation := range evidence {
		if !textsFitLimit(limits.MaxTextBytes, string(observation.Kind), observation.Path) {
			return false
		}
	}
	return true
}

func textsFitLimit(maximumBytes int, fields ...string) bool {
	for _, field := range fields {
		if len(field) > maximumBytes {
			return false
		}
	}
	return true
}
