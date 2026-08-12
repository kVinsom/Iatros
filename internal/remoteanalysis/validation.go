package remoteanalysis

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
	"github.com/kVinsom/Iatros/internal/security"
)

// ErrInvalidModel indicates that a remote-analysis model violates its normalized contract.
var ErrInvalidModel = errors.New("remote analysis model is invalid")

// Validate checks schema compatibility, provider metadata, references, evidence, and partial state.
func (m Model) Validate() error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) {
		return invalidModel("schema_version", "is unsupported")
	}
	if !validIdentifier(m.ID) || !validText(m.Name) {
		return invalidModel("system", "identity is invalid")
	}
	if len(m.Repositories) == 0 {
		return invalidModel("repositories", "must identify at least one remote repository")
	}

	repositoryIndex, err := validateRepositories(m.Repositories)
	if err != nil {
		return err
	}
	if !validEvidence(m.Evidence, repositoryIndex) {
		return invalidModel("evidence", "is invalid")
	}
	if err := validateRelationships(m.Relationships, repositoryIndex); err != nil {
		return err
	}
	return validateDiagnostics(m, repositoryIndex)
}

// ValidateWithin checks the normalized model and verifies retained content against limits.
func (m Model) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Repositories) > limits.MaxRepositories ||
		len(m.Relationships) > limits.MaxRelationships ||
		len(m.Diagnostics) > limits.MaxDiagnostics {
		return invalidModel("collections", "exceed configured retention limits")
	}
	if !modelFitsLimits(m, limits) {
		return invalidModel("facts", "exceed configured evidence or text limits")
	}
	return m.Validate()
}

type repositoryMetadata struct {
	provider Provider
}

func validateRepositories(
	repositories []Repository,
) (map[RepositoryID]repositoryMetadata, error) {
	repositoryIndex := make(map[RepositoryID]repositoryMetadata, len(repositories))
	repositoryLocations := make(map[repositoryLocation]struct{}, len(repositories))
	for index, repository := range repositories {
		if !validIdentifier(string(repository.ID)) || !validProvider(repository.Provider) ||
			!validHost(repository.Host) || !validRemotePath(repository.Namespace) ||
			repository.Name != strings.ToLower(repository.Name) ||
			!validRemotePathSegment(repository.Name) || !validRevision(repository.Revision) ||
			!validReferenceSelection(repository) || !validVisibility(repository.Visibility) {
			return nil, invalidModel(indexedField("repositories", index), "is invalid")
		}
		if index > 0 && repositories[index-1].ID >= repository.ID {
			return nil, invalidModel("repositories", "must be strictly ordered by id")
		}
		location := repositoryLocation{
			provider:  repository.Provider,
			host:      repository.Host,
			namespace: repository.Namespace,
			name:      repository.Name,
		}
		if _, exists := repositoryLocations[location]; exists {
			return nil, invalidModel("repositories", "must not repeat a provider location")
		}
		repositoryLocations[location] = struct{}{}
		repositoryIndex[repository.ID] = repositoryMetadata{provider: repository.Provider}
	}
	for index, repository := range repositories {
		if !validEvidence(repository.Evidence, repositoryIndex) ||
			!evidenceReferencesOnly(repository.Evidence, repository.ID) ||
			!hasProviderEvidence(repository.Evidence) {
			return nil, invalidModel(indexedField("repositories", index)+".evidence", "is invalid")
		}
	}
	return repositoryIndex, nil
}

type repositoryLocation struct {
	provider  Provider
	host      string
	namespace string
	name      string
}

func validateRelationships(
	relationships []Relationship,
	repositoryIndex map[RepositoryID]repositoryMetadata,
) error {
	for index, relationship := range relationships {
		if !validIdentifier(string(relationship.ID)) ||
			!knownRepository(relationship.SourceID, repositoryIndex) ||
			!knownRepository(relationship.TargetID, repositoryIndex) ||
			relationship.SourceID == relationship.TargetID || !validIdentifier(relationship.Kind) ||
			!validEvidence(relationship.Evidence, repositoryIndex) {
			return invalidModel(indexedField("relationships", index), "is invalid")
		}
		if index > 0 && relationships[index-1].ID >= relationship.ID {
			return invalidModel("relationships", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateDiagnostics(
	model Model,
	repositoryIndex map[RepositoryID]repositoryMetadata,
) error {
	if model.Partial && len(model.Diagnostics) == 0 {
		return invalidModel("diagnostics", "must explain a partial result")
	}
	hasLimitingDiagnostic := false
	for index, diagnostic := range model.Diagnostics {
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!validDiagnosticSource(diagnostic, repositoryIndex) ||
			!security.IsSafeProviderReference(diagnostic.Scope) || !validText(diagnostic.Message) {
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

func validReferenceSelection(repository Repository) bool {
	if !validGitReference(repository.Reference) ||
		(repository.DefaultBranch != "" && !validGitReference(repository.DefaultBranch)) {
		return false
	}
	switch repository.ReferenceKind {
	case ReferenceDefaultBranch:
		return repository.DefaultBranch != "" && repository.Reference == repository.DefaultBranch
	case ReferenceBranch, ReferenceTag:
		return true
	case ReferenceCommit:
		return repository.Reference == repository.Revision
	default:
		return false
	}
}

func validHost(host string) bool {
	if !validText(host) || host != strings.ToLower(host) || strings.ContainsAny(host, "/@?#\\") {
		return false
	}
	hostname, port, isBracketed, valid := splitHostPort(host)
	if !valid || (port != "" && !validPort(port)) {
		return false
	}
	parsedIP := net.ParseIP(hostname)
	if isBracketed {
		return parsedIP != nil && strings.Contains(hostname, ":")
	}
	if parsedIP != nil {
		return true
	}
	if len(hostname) > 253 || strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if !validHostLabel(label) {
			return false
		}
	}
	return true
}

func splitHostPort(host string) (string, string, bool, bool) {
	if strings.HasPrefix(host, "[") {
		closingBracket := strings.IndexByte(host, ']')
		if closingBracket <= 1 {
			return "", "", false, false
		}
		hostname := host[1:closingBracket]
		remainder := host[closingBracket+1:]
		if remainder == "" {
			return hostname, "", true, true
		}
		if !strings.HasPrefix(remainder, ":") || len(remainder) == 1 {
			return "", "", false, false
		}
		return hostname, remainder[1:], true, true
	}
	if strings.Count(host, ":") == 0 {
		return host, "", false, true
	}
	if strings.Count(host, ":") != 1 {
		return "", "", false, false
	}
	hostname, port, found := strings.Cut(host, ":")
	return hostname, port, false, found && hostname != "" && port != ""
}

func validPort(port string) bool {
	number, err := strconv.ParseUint(port, 10, 16)
	return err == nil && number != 0 && strconv.FormatUint(number, 10) == port
}

func validHostLabel(label string) bool {
	if label == "" || len(label) > 63 || !asciiAlphaNumeric(label[0]) ||
		!asciiAlphaNumeric(label[len(label)-1]) {
		return false
	}
	for index := range len(label) {
		if !asciiAlphaNumeric(label[index]) && label[index] != '-' {
			return false
		}
	}
	return true
}

func validRemotePath(remotePath string) bool {
	if !validText(remotePath) || remotePath != strings.ToLower(remotePath) ||
		strings.HasPrefix(remotePath, "/") ||
		strings.HasSuffix(remotePath, "/") {
		return false
	}
	for segment := range strings.SplitSeq(remotePath, "/") {
		if !validRemotePathSegment(segment) {
			return false
		}
	}
	return true
}

func validRemotePathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." ||
		strings.Contains(segment, "..") || strings.HasSuffix(segment, ".") {
		return false
	}
	hasAlphaNumeric := false
	for index := range len(segment) {
		character := segment[index]
		if asciiAlphaNumeric(character) {
			hasAlphaNumeric = true
			continue
		}
		if character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return hasAlphaNumeric
}

func validRevision(revision string) bool {
	if len(revision) != 40 && len(revision) != 64 {
		return false
	}
	for index := range len(revision) {
		if (revision[index] < '0' || revision[index] > '9') &&
			(revision[index] < 'a' || revision[index] > 'f') {
			return false
		}
	}
	return true
}

func validGitReference(reference string) bool {
	if !validText(reference) || reference == "@" || reference == "HEAD" ||
		strings.HasPrefix(reference, ".") || strings.HasPrefix(reference, "-") ||
		strings.HasPrefix(reference, "/") || strings.HasSuffix(reference, ".") ||
		strings.HasSuffix(reference, "/") || strings.Contains(reference, "..") ||
		strings.Contains(reference, "@{") || strings.Contains(reference, "//") ||
		strings.ContainsAny(reference, " ~^:?*[\\") {
		return false
	}
	for component := range strings.SplitSeq(reference, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validEvidence(
	evidence []Evidence,
	repositoryIndex map[RepositoryID]repositoryMetadata,
) bool {
	if len(evidence) == 0 {
		return false
	}
	for index, observation := range evidence {
		if !knownRepository(observation.RepositoryID, repositoryIndex) ||
			!validEvidenceLocation(observation) ||
			(index > 0 && compareEvidence(evidence[index-1], observation) >= 0) {
			return false
		}
	}
	return true
}

func validEvidenceLocation(evidence Evidence) bool {
	switch evidence.Kind {
	case EvidenceProviderMetadata:
		return evidence.Path == "" && security.IsSafeProviderReference(evidence.Reference)
	case EvidenceRepositoryFile:
		return evidence.Reference == "" && repositorypath.IsValidFile(evidence.Path)
	default:
		return false
	}
}

func evidenceReferencesOnly(evidence []Evidence, repositoryID RepositoryID) bool {
	for _, observation := range evidence {
		if observation.RepositoryID != repositoryID {
			return false
		}
	}
	return true
}

func hasProviderEvidence(evidence []Evidence) bool {
	for _, observation := range evidence {
		if observation.Kind == EvidenceProviderMetadata {
			return true
		}
	}
	return false
}

func validDiagnosticSource(
	diagnostic Diagnostic,
	repositoryIndex map[RepositoryID]repositoryMetadata,
) bool {
	if diagnostic.RepositoryID == "" {
		return diagnostic.Provider == "" || validProvider(diagnostic.Provider)
	}
	metadata, exists := repositoryIndex[diagnostic.RepositoryID]
	return exists && (diagnostic.Provider == "" || diagnostic.Provider == metadata.provider)
}

func knownRepository(
	repositoryID RepositoryID,
	repositoryIndex map[RepositoryID]repositoryMetadata,
) bool {
	_, exists := repositoryIndex[repositoryID]
	return exists
}

func validProvider(provider Provider) bool {
	return provider == ProviderGitHub || provider == ProviderGitLab || provider == ProviderBitbucket
}

func validVisibility(visibility Visibility) bool {
	return visibility == VisibilityUnknown || visibility == VisibilityPublic ||
		visibility == VisibilityPrivate || visibility == VisibilityInternal
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

func asciiAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}

func modelFitsLimits(model Model, limits Limits) bool {
	if !textsFitLimit(limits.MaxTextBytes, model.ID, model.Name) ||
		!evidenceFitsLimits(model.Evidence, limits) {
		return false
	}
	for _, repository := range model.Repositories {
		if !textsFitLimit(limits.MaxTextBytes, string(repository.ID), string(repository.Provider),
			repository.Host, repository.Namespace, repository.Name, repository.Revision,
			repository.Reference, string(repository.ReferenceKind), repository.DefaultBranch,
			string(repository.Visibility)) || !evidenceFitsLimits(repository.Evidence, limits) {
			return false
		}
	}
	for _, relationship := range model.Relationships {
		if !textsFitLimit(limits.MaxTextBytes, string(relationship.ID),
			string(relationship.SourceID), string(relationship.TargetID), relationship.Kind) ||
			!evidenceFitsLimits(relationship.Evidence, limits) {
			return false
		}
	}
	for _, diagnostic := range model.Diagnostics {
		if !textsFitLimit(limits.MaxTextBytes, diagnostic.Code, string(diagnostic.Level),
			string(diagnostic.Provider), string(diagnostic.RepositoryID),
			diagnostic.Scope, diagnostic.Message) {
			return false
		}
	}
	return true
}

func evidenceFitsLimits(evidence []Evidence, limits Limits) bool {
	if len(evidence) > limits.MaxEvidencePerFact {
		return false
	}
	for _, observation := range evidence {
		if !textsFitLimit(limits.MaxTextBytes, string(observation.RepositoryID),
			string(observation.Kind), observation.Path, observation.Reference) {
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
