package systemmap

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
	"github.com/kVinsom/Iatros/internal/security"
)

// ErrInvalidModel indicates that a system map violates its normalized contract.
var ErrInvalidModel = errors.New("system map is invalid")

// Validate checks schema compatibility, references, ordering, evidence, and safety invariants.
func (m Model) Validate() error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) {
		return invalidModel("schema_version", "is unsupported")
	}
	if !validIdentifier(m.ID) || !validText(m.Name) {
		return invalidModel("system", "identity is invalid")
	}
	if len(m.Repositories) == 0 {
		return invalidModel("repositories", "must identify at least one source repository")
	}

	entities, err := validateEntityCollections(m)
	if err != nil {
		return err
	}
	if err := validateRelationships(m.Relationships, entities); err != nil {
		return err
	}
	return validateDiagnostics(m, entities.repositories)
}

// ValidateWithin checks the normalized model and verifies retained content against limits.
func (m Model) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Repositories) > limits.MaxRepositories || len(m.Services) > limits.MaxServices ||
		len(m.Libraries) > limits.MaxLibraries ||
		len(m.Infrastructure) > limits.MaxInfrastructure ||
		len(m.Environments) > limits.MaxEnvironments || len(m.Owners) > limits.MaxOwners ||
		len(m.ExternalResources) > limits.MaxExternalResources ||
		len(m.Relationships) > limits.MaxRelationships ||
		len(m.Diagnostics) > limits.MaxDiagnostics {
		return invalidModel("collections", "exceed configured retention limits")
	}
	if !modelFitsLimits(m, limits) {
		return invalidModel("facts", "exceed configured evidence, environment, or text limits")
	}
	return m.Validate()
}

type entityIndex struct {
	repositories      map[RepositoryID]struct{}
	services          map[ServiceID]struct{}
	libraries         map[LibraryID]struct{}
	infrastructure    map[InfrastructureID]struct{}
	environments      map[EnvironmentID]struct{}
	owners            map[OwnerID]struct{}
	externalResources map[ExternalResourceID]struct{}
}

func validateEntityCollections(model Model) (entityIndex, error) {
	entities := entityIndex{}
	var err error
	entities.repositories, err = validateRepositories(model.Repositories)
	if err != nil {
		return entityIndex{}, err
	}
	entities.environments, err = validateEnvironments(model.Environments, entities.repositories)
	if err != nil {
		return entityIndex{}, err
	}
	entities.services, err = validateServices(
		model.Services,
		entities.repositories,
		entities.environments,
	)
	if err != nil {
		return entityIndex{}, err
	}
	entities.libraries, err = validateLibraries(model.Libraries, entities.repositories)
	if err != nil {
		return entityIndex{}, err
	}
	entities.infrastructure, err = validateInfrastructure(
		model.Infrastructure,
		entities.repositories,
		entities.environments,
	)
	if err != nil {
		return entityIndex{}, err
	}
	entities.owners, err = validateOwners(model.Owners, entities.repositories)
	if err != nil {
		return entityIndex{}, err
	}
	entities.externalResources, err = validateExternalResources(
		model.ExternalResources,
		entities.repositories,
	)
	if err != nil {
		return entityIndex{}, err
	}
	return entities, nil
}

func validateRepositories(repositories []Repository) (map[RepositoryID]struct{}, error) {
	repositoryIDs := make(map[RepositoryID]struct{}, len(repositories))
	repositoryLocations := make(map[repositoryLocation]struct{}, len(repositories))
	for index, repository := range repositories {
		if !validIdentifier(string(repository.ID)) || !validText(repository.Name) ||
			!validRepositoryLocation(repository) {
			return nil, invalidModel(indexedField("repositories", index), "is invalid")
		}
		if index > 0 && repositories[index-1].ID >= repository.ID {
			return nil, invalidModel("repositories", "must be strictly ordered by id")
		}
		location := repositoryLocation{
			source: repository.Source, provider: repository.Provider, locator: repository.Locator,
		}
		if _, exists := repositoryLocations[location]; exists {
			return nil, invalidModel("repositories", "must not repeat a source location")
		}
		repositoryLocations[location] = struct{}{}
		repositoryIDs[repository.ID] = struct{}{}
	}
	for index, repository := range repositories {
		if !validEvidence(repository.Evidence, repositoryIDs) ||
			!evidenceReferencesOnly(repository.Evidence, repository.ID) ||
			!hasEvidenceKind(repository.Evidence, requiredRepositoryEvidence(repository.Source)) {
			return nil, invalidModel(indexedField("repositories", index)+".evidence", "is invalid")
		}
	}
	return repositoryIDs, nil
}

type repositoryLocation struct {
	source   RepositorySource
	provider string
	locator  string
}

func evidenceReferencesOnly(evidence []Evidence, repositoryID RepositoryID) bool {
	for _, observation := range evidence {
		if observation.RepositoryID != repositoryID {
			return false
		}
	}
	return true
}

func requiredRepositoryEvidence(source RepositorySource) EvidenceKind {
	if source == RepositorySourceRemote {
		return EvidenceProvider
	}
	return EvidenceTarget
}

func hasEvidenceKind(evidence []Evidence, kind EvidenceKind) bool {
	for _, observation := range evidence {
		if observation.Kind == kind {
			return true
		}
	}
	return false
}

func validateServices(
	services []Service,
	repositoryIDs map[RepositoryID]struct{},
	environmentIDs map[EnvironmentID]struct{},
) (map[ServiceID]struct{}, error) {
	serviceIDs := make(map[ServiceID]struct{}, len(services))
	for index, service := range services {
		if !validIdentifier(string(service.ID)) || !knownRepository(service.RepositoryID, repositoryIDs) ||
			!validText(service.Name) || !validIdentifier(service.Kind) ||
			!repositorypath.IsValidDirectory(service.Root) ||
			!validEnvironmentIDs(service.EnvironmentIDs, environmentIDs) ||
			!validEvidence(service.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("services", index), "is invalid")
		}
		if index > 0 && services[index-1].ID >= service.ID {
			return nil, invalidModel("services", "must be strictly ordered by id")
		}
		serviceIDs[service.ID] = struct{}{}
	}
	return serviceIDs, nil
}

func validateLibraries(
	libraries []Library,
	repositoryIDs map[RepositoryID]struct{},
) (map[LibraryID]struct{}, error) {
	libraryIDs := make(map[LibraryID]struct{}, len(libraries))
	for index, library := range libraries {
		if !validIdentifier(string(library.ID)) || !knownRepository(library.RepositoryID, repositoryIDs) ||
			!validText(library.Name) || !validIdentifier(library.Ecosystem) ||
			!validOptionalText(library.Version) || !repositorypath.IsValidDirectory(library.Root) ||
			!validEvidence(library.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("libraries", index), "is invalid")
		}
		if index > 0 && libraries[index-1].ID >= library.ID {
			return nil, invalidModel("libraries", "must be strictly ordered by id")
		}
		libraryIDs[library.ID] = struct{}{}
	}
	return libraryIDs, nil
}

func validateInfrastructure(
	infrastructure []Infrastructure,
	repositoryIDs map[RepositoryID]struct{},
	environmentIDs map[EnvironmentID]struct{},
) (map[InfrastructureID]struct{}, error) {
	infrastructureIDs := make(map[InfrastructureID]struct{}, len(infrastructure))
	for index, component := range infrastructure {
		if !validIdentifier(string(component.ID)) ||
			!knownRepository(component.RepositoryID, repositoryIDs) || !validText(component.Name) ||
			!validIdentifier(component.Kind) || !validIdentifier(component.Technology) ||
			!repositorypath.IsValidDirectory(component.Root) ||
			!validEnvironmentIDs(component.EnvironmentIDs, environmentIDs) ||
			!validEvidence(component.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("infrastructure", index), "is invalid")
		}
		if index > 0 && infrastructure[index-1].ID >= component.ID {
			return nil, invalidModel("infrastructure", "must be strictly ordered by id")
		}
		infrastructureIDs[component.ID] = struct{}{}
	}
	return infrastructureIDs, nil
}

func validateEnvironments(
	environments []Environment,
	repositoryIDs map[RepositoryID]struct{},
) (map[EnvironmentID]struct{}, error) {
	environmentIDs := make(map[EnvironmentID]struct{}, len(environments))
	for index, environment := range environments {
		if !validIdentifier(string(environment.ID)) || !validText(environment.Name) ||
			!validIdentifier(environment.Kind) || !validEvidence(environment.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("environments", index), "is invalid")
		}
		if index > 0 && environments[index-1].ID >= environment.ID {
			return nil, invalidModel("environments", "must be strictly ordered by id")
		}
		environmentIDs[environment.ID] = struct{}{}
	}
	return environmentIDs, nil
}

func validateOwners(
	owners []Owner,
	repositoryIDs map[RepositoryID]struct{},
) (map[OwnerID]struct{}, error) {
	ownerIDs := make(map[OwnerID]struct{}, len(owners))
	for index, owner := range owners {
		if !validIdentifier(string(owner.ID)) || !validOwnerKind(owner.Kind) ||
			!validText(owner.Name) || !validOptionalText(owner.Reference) ||
			!validEvidence(owner.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("owners", index), "is invalid")
		}
		if index > 0 && owners[index-1].ID >= owner.ID {
			return nil, invalidModel("owners", "must be strictly ordered by id")
		}
		ownerIDs[owner.ID] = struct{}{}
	}
	return ownerIDs, nil
}

func validateExternalResources(
	resources []ExternalResource,
	repositoryIDs map[RepositoryID]struct{},
) (map[ExternalResourceID]struct{}, error) {
	resourceIDs := make(map[ExternalResourceID]struct{}, len(resources))
	for index, resource := range resources {
		if !validIdentifier(string(resource.ID)) || !validText(resource.Name) ||
			!validIdentifier(resource.Kind) || !validOptionalIdentifier(resource.Provider) ||
			!validEvidence(resource.Evidence, repositoryIDs) {
			return nil, invalidModel(indexedField("external_resources", index), "is invalid")
		}
		if index > 0 && resources[index-1].ID >= resource.ID {
			return nil, invalidModel("external_resources", "must be strictly ordered by id")
		}
		resourceIDs[resource.ID] = struct{}{}
	}
	return resourceIDs, nil
}

func validateRelationships(relationships []Relationship, entities entityIndex) error {
	for index, relationship := range relationships {
		if !validIdentifier(string(relationship.ID)) || !entities.contains(relationship.Source) ||
			!entities.contains(relationship.Target) || relationship.Source == relationship.Target ||
			!validIdentifier(relationship.Kind) ||
			!validEnvironmentIDs(relationship.EnvironmentIDs, entities.environments) ||
			!validEvidence(relationship.Evidence, entities.repositories) {
			return invalidModel(indexedField("relationships", index), "is invalid")
		}
		if index > 0 && relationships[index-1].ID >= relationship.ID {
			return invalidModel("relationships", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateDiagnostics(model Model, repositoryIDs map[RepositoryID]struct{}) error {
	if model.Partial && len(model.Diagnostics) == 0 {
		return invalidModel("diagnostics", "must explain a partial result")
	}
	hasLimitingDiagnostic := false
	for index, diagnostic := range model.Diagnostics {
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!validDiagnosticLocation(diagnostic, repositoryIDs) || !validText(diagnostic.Message) {
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

func (i entityIndex) contains(reference EntityReference) bool {
	switch reference.Kind {
	case EntityRepository:
		_, exists := i.repositories[RepositoryID(reference.ID)]
		return exists
	case EntityService:
		_, exists := i.services[ServiceID(reference.ID)]
		return exists
	case EntityLibrary:
		_, exists := i.libraries[LibraryID(reference.ID)]
		return exists
	case EntityInfrastructure:
		_, exists := i.infrastructure[InfrastructureID(reference.ID)]
		return exists
	case EntityEnvironment:
		_, exists := i.environments[EnvironmentID(reference.ID)]
		return exists
	case EntityOwner:
		_, exists := i.owners[OwnerID(reference.ID)]
		return exists
	case EntityExternalResource:
		_, exists := i.externalResources[ExternalResourceID(reference.ID)]
		return exists
	default:
		return false
	}
}

func validRepositoryLocation(repository Repository) bool {
	switch repository.Source {
	case RepositorySourceLocal:
		return repository.Provider == "" && repositorypath.IsValidDirectory(repository.Locator) &&
			validOptionalText(repository.Revision)
	case RepositorySourceRemote:
		return validIdentifier(repository.Provider) && validRemoteLocator(repository.Locator) &&
			security.IsSafeProviderReference(repository.Revision)
	default:
		return false
	}
}

func validRemoteLocator(locator string) bool {
	if !validText(locator) || locator != strings.ToLower(locator) ||
		strings.Contains(locator, "://") ||
		strings.ContainsAny(locator, "@?#\\") || strings.HasPrefix(locator, "/") ||
		strings.HasSuffix(locator, "/") {
		return false
	}
	for segment := range strings.SplitSeq(locator, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func validEvidence(evidence []Evidence, repositoryIDs map[RepositoryID]struct{}) bool {
	if len(evidence) == 0 {
		return false
	}
	for index, observation := range evidence {
		if !knownRepository(observation.RepositoryID, repositoryIDs) ||
			!validEvidenceLocation(observation) ||
			(index > 0 && compareEvidence(evidence[index-1], observation) >= 0) {
			return false
		}
	}
	return true
}

func validEvidenceLocation(evidence Evidence) bool {
	if evidence.Kind == EvidenceProvider {
		return evidence.Path == "" && security.IsSafeProviderReference(evidence.Reference)
	}
	if evidence.Reference != "" {
		return false
	}
	switch evidence.Kind {
	case EvidenceTarget:
		return repositorypath.IsValidDirectory(evidence.Path)
	case EvidenceSource, EvidenceManifest, EvidenceConfiguration, EvidenceOwnership:
		return repositorypath.IsValidFile(evidence.Path)
	default:
		return false
	}
}

func validEnvironmentIDs(
	environmentIDs []EnvironmentID,
	knownEnvironmentIDs map[EnvironmentID]struct{},
) bool {
	for index, environmentID := range environmentIDs {
		if _, exists := knownEnvironmentIDs[environmentID]; !exists {
			return false
		}
		if index > 0 && environmentIDs[index-1] >= environmentID {
			return false
		}
	}
	return true
}

func validDiagnosticLocation(
	diagnostic Diagnostic,
	repositoryIDs map[RepositoryID]struct{},
) bool {
	if diagnostic.RepositoryID == "" {
		return diagnostic.Path == "."
	}
	return knownRepository(diagnostic.RepositoryID, repositoryIDs) &&
		repositorypath.IsValidDirectory(diagnostic.Path)
}

func knownRepository(repositoryID RepositoryID, repositoryIDs map[RepositoryID]struct{}) bool {
	_, exists := repositoryIDs[repositoryID]
	return exists
}

func validOwnerKind(kind OwnerKind) bool {
	return kind == OwnerPerson || kind == OwnerTeam || kind == OwnerOrganization
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

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}

func modelFitsLimits(model Model, limits Limits) bool {
	if !textsFitLimit(limits.MaxTextBytes, model.ID, model.Name) {
		return false
	}
	for _, repository := range model.Repositories {
		if !factFitsLimits(repository.Evidence, limits, string(repository.ID), repository.Name,
			string(repository.Source), repository.Provider, repository.Locator, repository.Revision) {
			return false
		}
	}
	for _, service := range model.Services {
		if len(service.EnvironmentIDs) > limits.MaxEnvironmentsPerEntity ||
			!factFitsLimits(service.Evidence, limits, string(service.ID),
				string(service.RepositoryID), service.Name, service.Kind, service.Root) {
			return false
		}
	}
	for _, library := range model.Libraries {
		if !factFitsLimits(library.Evidence, limits, string(library.ID),
			string(library.RepositoryID), library.Name, library.Ecosystem, library.Version, library.Root) {
			return false
		}
	}
	for _, component := range model.Infrastructure {
		if len(component.EnvironmentIDs) > limits.MaxEnvironmentsPerEntity ||
			!factFitsLimits(component.Evidence, limits, string(component.ID),
				string(component.RepositoryID), component.Name, component.Kind,
				component.Technology, component.Root) {
			return false
		}
	}
	for _, environment := range model.Environments {
		if !factFitsLimits(environment.Evidence, limits, string(environment.ID),
			environment.Name, environment.Kind) {
			return false
		}
	}
	for _, owner := range model.Owners {
		if !factFitsLimits(owner.Evidence, limits, string(owner.ID),
			string(owner.Kind), owner.Name, owner.Reference) {
			return false
		}
	}
	for _, resource := range model.ExternalResources {
		if !factFitsLimits(resource.Evidence, limits, string(resource.ID),
			resource.Name, resource.Kind, resource.Provider) {
			return false
		}
	}
	for _, relationship := range model.Relationships {
		if len(relationship.EnvironmentIDs) > limits.MaxEnvironmentsPerEntity ||
			!factFitsLimits(relationship.Evidence, limits, string(relationship.ID),
				string(relationship.Source.Kind), relationship.Source.ID,
				string(relationship.Target.Kind), relationship.Target.ID, relationship.Kind) {
			return false
		}
	}
	for _, diagnostic := range model.Diagnostics {
		if !textsFitLimit(limits.MaxTextBytes, diagnostic.Code, string(diagnostic.Level),
			string(diagnostic.RepositoryID), diagnostic.Path, diagnostic.Message) {
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
