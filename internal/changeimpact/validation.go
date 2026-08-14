package changeimpact

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

var (
	// ErrInvalidChangeSet indicates unsafe, unknown, duplicate, or unordered change input.
	ErrInvalidChangeSet = errors.New("change set is invalid")
	// ErrInvalidModel indicates that a change-impact model violates its normalized contract.
	ErrInvalidModel = errors.New("change impact model is invalid")
)

// Validate checks change identity, repository references, paths, operation shape, and ordering.
func (s ChangeSet) Validate(system systemmap.Model) error {
	if err := system.Validate(); err != nil {
		return fmt.Errorf("%w: system map: %v", ErrInvalidChangeSet, err)
	}
	if !validIdentifier(s.ID) {
		return invalidChangeSet("id", "is invalid")
	}
	repositories := repositoryIDs(system.Repositories)
	for index, change := range s.Changes {
		if !validIdentifier(string(change.ID)) || !knownRepository(change.RepositoryID, repositories) ||
			!validChangeShape(change) {
			return invalidChangeSet(indexedField("changes", index), "is invalid")
		}
		if index > 0 && s.Changes[index-1].ID >= change.ID {
			return invalidChangeSet("changes", "must be strictly ordered by id")
		}
	}
	return nil
}

// ValidateWithin validates change input and verifies it against configured retention limits.
func (s ChangeSet) ValidateWithin(system systemmap.Model, limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(s.Changes) > limits.MaxChanges {
		return invalidChangeSet("changes", "exceed the configured limit")
	}
	if len(s.ID) > limits.MaxTextBytes {
		return invalidChangeSet("id", "exceeds the configured text limit")
	}
	for index, change := range s.Changes {
		if !textsFitLimit(limits.MaxTextBytes, string(change.ID), string(change.RepositoryID),
			string(change.Kind), change.Path, change.PreviousPath) {
			return invalidChangeSet(indexedField("changes", index), "exceeds the configured text limit")
		}
	}
	return s.Validate(system)
}

// Validate checks references, ordering, causes, diagnostics, and partial-result invariants.
func (m Model) Validate(system systemmap.Model, changes ChangeSet) error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) {
		return invalidModel("schema_version", "is unsupported")
	}
	if m.ChangeSetID != changes.ID || !validIdentifier(m.ChangeSetID) {
		return invalidModel("change_set_id", "does not identify the input change set")
	}
	if err := changes.Validate(system); err != nil {
		return invalidModel("change_set", "is invalid")
	}

	index := newValidationIndex(system, changes)
	if err := validateServiceImpacts(m.Services, index); err != nil {
		return err
	}
	if err := validateEnvironmentImpacts(m.Environments, index); err != nil {
		return err
	}
	if err := validateConfigurationImpacts(m.Configurations, index); err != nil {
		return err
	}
	return validateDiagnostics(m, index)
}

// ValidateWithin validates the model and verifies retained content against limits.
func (m Model) ValidateWithin(system systemmap.Model, changes ChangeSet, limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Services) > limits.MaxServices || len(m.Environments) > limits.MaxEnvironments ||
		len(m.Configurations) > limits.MaxConfigurations ||
		len(m.Diagnostics) > limits.MaxDiagnostics {
		return invalidModel("collections", "exceed configured retention limits")
	}
	if !impactFactsFitLimits(m, limits) {
		return invalidModel("facts", "exceed configured cause or text limits")
	}
	return m.Validate(system, changes)
}

type validationIndex struct {
	repositories   map[systemmap.RepositoryID]struct{}
	services       map[systemmap.ServiceID]systemmap.RepositoryID
	environments   map[systemmap.EnvironmentID]struct{}
	configurations map[systemmap.InfrastructureID]systemmap.RepositoryID
	changes        map[ChangeID]Change
	relationships  map[systemmap.RelationshipID]systemmap.Relationship
	system         systemIndex
	entityCount    int
}

func newValidationIndex(system systemmap.Model, changes ChangeSet) validationIndex {
	index := validationIndex{
		repositories:   repositoryIDs(system.Repositories),
		services:       make(map[systemmap.ServiceID]systemmap.RepositoryID, len(system.Services)),
		environments:   make(map[systemmap.EnvironmentID]struct{}, len(system.Environments)),
		configurations: make(map[systemmap.InfrastructureID]systemmap.RepositoryID, len(system.Infrastructure)),
		changes:        make(map[ChangeID]Change, len(changes.Changes)),
		relationships:  make(map[systemmap.RelationshipID]systemmap.Relationship, len(system.Relationships)),
		system:         buildSystemIndex(system),
		entityCount: len(system.Services) + len(system.Libraries) + len(system.Infrastructure) +
			len(system.Environments),
	}
	for _, service := range system.Services {
		index.services[service.ID] = service.RepositoryID
	}
	for _, environment := range system.Environments {
		index.environments[environment.ID] = struct{}{}
	}
	for _, configuration := range system.Infrastructure {
		index.configurations[configuration.ID] = configuration.RepositoryID
	}
	for _, change := range changes.Changes {
		index.changes[change.ID] = change
	}
	for _, relationship := range system.Relationships {
		index.relationships[relationship.ID] = relationship
	}
	return index
}

func validateServiceImpacts(impacts []ServiceImpact, index validationIndex) error {
	for position, impact := range impacts {
		target := entityKey{kind: systemmap.EntityService, id: string(impact.ServiceID)}
		repositoryID, exists := index.services[impact.ServiceID]
		if !exists || repositoryID != impact.RepositoryID || !validImpactKind(impact.Kind, false) ||
			!validTargetCauses(impact.Causes, index, target, impact.Kind, "") {
			return invalidModel(indexedField("services", position), "is invalid")
		}
		if position > 0 && impacts[position-1].ServiceID >= impact.ServiceID {
			return invalidModel("services", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateEnvironmentImpacts(impacts []EnvironmentImpact, index validationIndex) error {
	for position, impact := range impacts {
		target := entityKey{kind: systemmap.EntityEnvironment, id: string(impact.EnvironmentID)}
		if _, exists := index.environments[impact.EnvironmentID]; !exists ||
			!validImpactKind(impact.Kind, true) ||
			!validTargetCauses(impact.Causes, index, target, impact.Kind, impact.EnvironmentID) {
			return invalidModel(indexedField("environments", position), "is invalid")
		}
		if position > 0 && impacts[position-1].EnvironmentID >= impact.EnvironmentID {
			return invalidModel("environments", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateConfigurationImpacts(impacts []ConfigurationImpact, index validationIndex) error {
	for position, impact := range impacts {
		target := entityKey{kind: systemmap.EntityInfrastructure, id: string(impact.ConfigurationID)}
		repositoryID, exists := index.configurations[impact.ConfigurationID]
		if !exists || repositoryID != impact.RepositoryID || !validImpactKind(impact.Kind, false) ||
			!validTargetCauses(impact.Causes, index, target, impact.Kind, "") {
			return invalidModel(indexedField("configurations", position), "is invalid")
		}
		if position > 0 && impacts[position-1].ConfigurationID >= impact.ConfigurationID {
			return invalidModel("configurations", "must be strictly ordered by id")
		}
	}
	return nil
}

func validTargetCauses(
	causes []Cause,
	index validationIndex,
	target entityKey,
	kind ImpactKind,
	associatedEnvironmentID systemmap.EnvironmentID,
) bool {
	if len(causes) == 0 {
		return false
	}
	hasDirectCause := false
	hasDependentCause := false
	hasAssociatedCause := false
	for position, cause := range causes {
		ends, isValid := resolveCause(cause, index)
		if !isValid {
			return false
		}
		if containsEntity(ends, target) {
			if len(cause.RelationshipIDs) == 0 {
				hasDirectCause = true
			} else {
				hasDependentCause = true
			}
		} else if associatedEnvironmentID != "" &&
			causeAssociatesEnvironment(ends, cause, index, associatedEnvironmentID) {
			hasAssociatedCause = true
		} else {
			return false
		}
		if position > 0 && compareCauses(causes[position-1], cause) >= 0 {
			return false
		}
	}
	switch kind {
	case ImpactDirect:
		return hasDirectCause
	case ImpactDependent:
		return !hasDirectCause && hasDependentCause
	case ImpactAssociated:
		return !hasDirectCause && !hasDependentCause && hasAssociatedCause
	default:
		return false
	}
}

func resolveCause(cause Cause, index validationIndex) ([]entityKey, bool) {
	change, exists := index.changes[cause.ChangeID]
	if !exists || !changeContainsPath(change, cause.Path) {
		return nil, false
	}
	entities, isTruncated := index.system.directEntities(
		change.RepositoryID,
		cause.Path,
		index.entityCount,
	)
	if isTruncated || len(entities) == 0 {
		return nil, false
	}
	for _, relationshipID := range cause.RelationshipIDs {
		relationship, exists := index.relationships[relationshipID]
		if !exists || !propagatesImpact(relationship.Kind) {
			return nil, false
		}
		target := entityKey{kind: relationship.Target.Kind, id: relationship.Target.ID}
		if !containsEntity(entities, target) {
			return nil, false
		}
		entities = []entityKey{{kind: relationship.Source.Kind, id: relationship.Source.ID}}
	}
	return entities, true
}

func causeAssociatesEnvironment(
	ends []entityKey,
	cause Cause,
	index validationIndex,
	environmentID systemmap.EnvironmentID,
) bool {
	for _, end := range ends {
		if slices.Contains(index.system.entityEnvironments[end], environmentID) {
			return true
		}
	}
	if len(cause.RelationshipIDs) == 0 {
		return false
	}
	lastRelationshipID := cause.RelationshipIDs[len(cause.RelationshipIDs)-1]
	return slices.Contains(index.relationships[lastRelationshipID].EnvironmentIDs, environmentID)
}

func containsEntity(entities []entityKey, target entityKey) bool {
	return slices.Contains(entities, target)
}

func validateDiagnostics(model Model, index validationIndex) error {
	if model.Partial && len(model.Diagnostics) == 0 {
		return invalidModel("diagnostics", "must explain a partial result")
	}
	hasLimitingDiagnostic := false
	for position, diagnostic := range model.Diagnostics {
		if !validDiagnosticCode(diagnostic.Code) || !validDiagnosticLevel(diagnostic.Level) ||
			!validDiagnosticReferences(diagnostic, index) || !validText(diagnostic.Message) {
			return invalidModel(indexedField("diagnostics", position), "is invalid")
		}
		if !model.Partial && diagnostic.Level != DiagnosticInfo {
			return invalidModel(indexedField("diagnostics", position), "requires a partial result")
		}
		if diagnostic.Level == DiagnosticWarning || diagnostic.Level == DiagnosticError {
			hasLimitingDiagnostic = true
		}
		if position > 0 && compareDiagnostics(model.Diagnostics[position-1], diagnostic) >= 0 {
			return invalidModel("diagnostics", "must be strictly ordered")
		}
	}
	if model.Partial && !hasLimitingDiagnostic {
		return invalidModel("diagnostics", "must contain a warning or error")
	}
	return nil
}

func validDiagnosticReferences(diagnostic Diagnostic, index validationIndex) bool {
	if diagnostic.ChangeID == "" {
		return diagnostic.RepositoryID == "" && diagnostic.Path == "."
	}
	change, exists := index.changes[diagnostic.ChangeID]
	return exists && diagnostic.RepositoryID == change.RepositoryID &&
		changeContainsPath(change, diagnostic.Path)
}

func validChangeShape(change Change) bool {
	if !repositorypath.IsValidFile(change.Path) {
		return false
	}
	switch change.Kind {
	case ChangeAdded, ChangeModified, ChangeDeleted:
		return change.PreviousPath == ""
	case ChangeRenamed:
		return repositorypath.IsValidFile(change.PreviousPath) && change.PreviousPath != change.Path
	default:
		return false
	}
}

func changeContainsPath(change Change, candidate string) bool {
	return candidate == change.Path || (change.Kind == ChangeRenamed && candidate == change.PreviousPath)
}

func validImpactKind(kind ImpactKind, allowAssociated bool) bool {
	return kind == ImpactDirect || kind == ImpactDependent || (allowAssociated && kind == ImpactAssociated)
}

func validDiagnosticLevel(level DiagnosticLevel) bool {
	return level == DiagnosticInfo || level == DiagnosticWarning || level == DiagnosticError
}

func repositoryIDs(repositories []systemmap.Repository) map[systemmap.RepositoryID]struct{} {
	ids := make(map[systemmap.RepositoryID]struct{}, len(repositories))
	for _, repository := range repositories {
		ids[repository.ID] = struct{}{}
	}
	return ids
}

func knownRepository(
	repositoryID systemmap.RepositoryID,
	repositories map[systemmap.RepositoryID]struct{},
) bool {
	_, exists := repositories[repositoryID]
	return exists
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

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func upperAlphaNumeric(character byte) bool {
	return (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func invalidChangeSet(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidChangeSet, field, reason)
}

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}

func impactFactsFitLimits(model Model, limits Limits) bool {
	if len(model.ChangeSetID) > limits.MaxTextBytes {
		return false
	}
	for _, impact := range model.Services {
		if !impactFitsLimits(impact.Causes, limits, string(impact.ServiceID),
			string(impact.RepositoryID), string(impact.Kind)) {
			return false
		}
	}
	for _, impact := range model.Environments {
		if !impactFitsLimits(impact.Causes, limits, string(impact.EnvironmentID), string(impact.Kind)) {
			return false
		}
	}
	for _, impact := range model.Configurations {
		if !impactFitsLimits(impact.Causes, limits, string(impact.ConfigurationID),
			string(impact.RepositoryID), string(impact.Kind)) {
			return false
		}
	}
	for _, diagnostic := range model.Diagnostics {
		if !textsFitLimit(limits.MaxTextBytes, diagnostic.Code, string(diagnostic.Level),
			string(diagnostic.ChangeID), string(diagnostic.RepositoryID),
			diagnostic.Path, diagnostic.Message) {
			return false
		}
	}
	return true
}

func impactFitsLimits(causes []Cause, limits Limits, fields ...string) bool {
	if len(causes) > limits.MaxCausesPerImpact || !textsFitLimit(limits.MaxTextBytes, fields...) {
		return false
	}
	for _, cause := range causes {
		if len(cause.RelationshipIDs) > limits.MaxRelationshipsPerCause ||
			!textsFitLimit(limits.MaxTextBytes, string(cause.ChangeID), cause.Path) {
			return false
		}
		for _, relationshipID := range cause.RelationshipIDs {
			if len(relationshipID) > limits.MaxTextBytes {
				return false
			}
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
