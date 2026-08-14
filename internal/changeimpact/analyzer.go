package changeimpact

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/systemmap"
)

const (
	diagnosticSystemMapPartial   = "SYSTEM_MAP_PARTIAL"
	diagnosticChangeUnmapped     = "CHANGE_UNMAPPED"
	diagnosticDirectMatchLimit   = "DIRECT_MATCH_LIMIT"
	diagnosticTraversalLimit     = "TRAVERSAL_LIMIT"
	diagnosticDepthLimit         = "DEPTH_LIMIT"
	diagnosticCauseLimit         = "CAUSE_LIMIT"
	diagnosticServiceLimit       = "SERVICE_LIMIT"
	diagnosticEnvironmentLimit   = "ENVIRONMENT_LIMIT"
	diagnosticConfigurationLimit = "CONFIGURATION_LIMIT"
)

// Analyzer determines direct and dependency-propagated effects without external side effects.
type Analyzer struct {
	limits Limits
}

// NewAnalyzer creates a change-impact analyzer with validated limits.
func NewAnalyzer(limits Limits) (Analyzer, error) {
	if err := limits.Validate(); err != nil {
		return Analyzer{}, err
	}
	return Analyzer{limits: limits}, nil
}

// Analyze maps repository changes to affected services, environments, and configurations.
func (a Analyzer) Analyze(
	ctx context.Context,
	system systemmap.Model,
	changeSet ChangeSet,
) (Model, error) {
	if err := a.limits.Validate(); err != nil {
		return emptyModel(changeSet.ID), err
	}
	changeSet = changeSet.Normalized()
	if err := changeSet.ValidateWithin(system, a.limits); err != nil {
		return emptyModel(changeSet.ID), err
	}

	analysisContext, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()
	if err := analysisContext.Err(); err != nil {
		return emptyModel(changeSet.ID), err
	}

	index := buildSystemIndex(system)
	state := newAnalysisState(changeSet.ID, a.limits, index)
	if system.Partial {
		state.addLimitingDiagnostic(Diagnostic{
			Code:    diagnosticSystemMapPartial,
			Level:   DiagnosticWarning,
			Path:    ".",
			Message: "The system map is partial, so the reported change impact may be incomplete.",
		})
	}

	queue := make([]propagationState, 0)
	visited := make(map[visitKey]struct{})

collectChanges:
	for _, change := range changeSet.Changes {
		for _, changedPath := range pathsForChange(change) {
			if err := analysisContext.Err(); err != nil {
				return emptyModel(changeSet.ID), err
			}
			remainingMatches := a.limits.MaxDirectMatches - state.directMatches
			seeds, isTruncated := index.directEntities(
				change.RepositoryID,
				changedPath,
				remainingMatches,
			)
			state.directMatches += len(seeds)
			if len(seeds) == 0 {
				if isTruncated {
					state.addLimitingDiagnostic(Diagnostic{
						Code: diagnosticDirectMatchLimit, Level: DiagnosticWarning, Path: ".",
						Message: "Additional direct entity matches were omitted by the configured work limit.",
					})
					break collectChanges
				}
				state.addInformationalDiagnostic(Diagnostic{
					Code:         diagnosticChangeUnmapped,
					Level:        DiagnosticInfo,
					ChangeID:     change.ID,
					RepositoryID: change.RepositoryID,
					Path:         changedPath,
					Message:      "The changed path is not mapped to a known system entity.",
				})
				continue
			}
			for _, seed := range seeds {
				cause := Cause{ChangeID: change.ID, Path: changedPath, RelationshipIDs: []systemmap.RelationshipID{}}
				state.recordImpact(seed, ImpactDirect, cause, nil)
				key := visitKey{entity: seed, changeID: change.ID, path: changedPath}
				if _, exists := visited[key]; exists {
					continue
				}
				visited[key] = struct{}{}
				queue = append(queue, propagationState{entity: seed, cause: cause})
			}
			if isTruncated {
				state.addLimitingDiagnostic(Diagnostic{
					Code: diagnosticDirectMatchLimit, Level: DiagnosticWarning, Path: ".",
					Message: "Additional direct entity matches were omitted by the configured work limit.",
				})
				break collectChanges
			}
		}
	}

propagateImpact:
	for len(queue) > 0 {
		if err := analysisContext.Err(); err != nil {
			return emptyModel(changeSet.ID), err
		}
		current := queue[0]
		queue[0] = propagationState{}
		queue = queue[1:]
		edges := index.reverseDependencies[current.entity]
		if len(current.cause.RelationshipIDs) >= a.limits.MaxTraversalDepth {
			if len(edges) > 0 {
				state.addLimitingDiagnostic(Diagnostic{
					Code:    diagnosticDepthLimit,
					Level:   DiagnosticWarning,
					Path:    ".",
					Message: "Additional dependency impact was omitted by the traversal-depth limit.",
				})
			}
			continue
		}
		for _, edge := range edges {
			if state.relationshipTraversals >= a.limits.MaxRelationshipTraversals {
				state.addLimitingDiagnostic(Diagnostic{
					Code:    diagnosticTraversalLimit,
					Level:   DiagnosticWarning,
					Path:    ".",
					Message: "Additional dependency impact was omitted by the traversal-work limit.",
				})
				break propagateImpact
			}
			state.relationshipTraversals++
			cause := Cause{
				ChangeID: current.cause.ChangeID,
				Path:     current.cause.Path,
				RelationshipIDs: append(
					slices.Clone(current.cause.RelationshipIDs),
					edge.relationshipID,
				),
			}
			state.recordImpact(edge.source, ImpactDependent, cause, edge.environmentIDs)
			key := visitKey{
				entity: edge.source, changeID: cause.ChangeID, path: cause.Path,
			}
			if _, exists := visited[key]; exists {
				continue
			}
			visited[key] = struct{}{}
			queue = append(queue, propagationState{entity: edge.source, cause: cause})
		}
	}

	result := state.model().Normalized()
	if err := result.ValidateWithin(system, changeSet, a.limits); err != nil {
		return emptyModel(changeSet.ID), err
	}
	return result, nil
}

type entityKey struct {
	kind systemmap.EntityKind
	id   string
}

type repositoryPath struct {
	repositoryID systemmap.RepositoryID
	path         string
}

type dependencyEdge struct {
	source         entityKey
	relationshipID systemmap.RelationshipID
	environmentIDs []systemmap.EnvironmentID
}

type systemIndex struct {
	rootEntities        map[repositoryPath][]entityKey
	evidenceEntities    map[repositoryPath][]entityKey
	reverseDependencies map[entityKey][]dependencyEdge
	services            map[entityKey]systemmap.Service
	configurations      map[entityKey]systemmap.Infrastructure
	environments        map[entityKey]systemmap.Environment
	entityEnvironments  map[entityKey][]systemmap.EnvironmentID
}

func buildSystemIndex(system systemmap.Model) systemIndex {
	index := systemIndex{
		rootEntities:        make(map[repositoryPath][]entityKey),
		evidenceEntities:    make(map[repositoryPath][]entityKey),
		reverseDependencies: make(map[entityKey][]dependencyEdge),
		services:            make(map[entityKey]systemmap.Service, len(system.Services)),
		configurations:      make(map[entityKey]systemmap.Infrastructure, len(system.Infrastructure)),
		environments:        make(map[entityKey]systemmap.Environment, len(system.Environments)),
		entityEnvironments:  make(map[entityKey][]systemmap.EnvironmentID),
	}
	for _, service := range system.Services {
		key := entityKey{kind: systemmap.EntityService, id: string(service.ID)}
		index.services[key] = service
		index.entityEnvironments[key] = service.EnvironmentIDs
		index.addRoot(service.RepositoryID, service.Root, key)
		index.addEvidence(service.Evidence, key)
	}
	for _, library := range system.Libraries {
		key := entityKey{kind: systemmap.EntityLibrary, id: string(library.ID)}
		index.addRoot(library.RepositoryID, library.Root, key)
		index.addEvidence(library.Evidence, key)
	}
	for _, configuration := range system.Infrastructure {
		key := entityKey{kind: systemmap.EntityInfrastructure, id: string(configuration.ID)}
		index.configurations[key] = configuration
		index.entityEnvironments[key] = configuration.EnvironmentIDs
		index.addRoot(configuration.RepositoryID, configuration.Root, key)
		index.addEvidence(configuration.Evidence, key)
	}
	for _, environment := range system.Environments {
		key := entityKey{kind: systemmap.EntityEnvironment, id: string(environment.ID)}
		index.environments[key] = environment
		index.addEvidence(environment.Evidence, key)
	}
	for _, relationship := range system.Relationships {
		if !propagatesImpact(relationship.Kind) {
			continue
		}
		target := entityKey{kind: relationship.Target.Kind, id: relationship.Target.ID}
		index.reverseDependencies[target] = append(
			index.reverseDependencies[target],
			dependencyEdge{
				source:         entityKey{kind: relationship.Source.Kind, id: relationship.Source.ID},
				relationshipID: relationship.ID,
				environmentIDs: relationship.EnvironmentIDs,
			},
		)
	}
	return index
}

func (i *systemIndex) addRoot(
	repositoryID systemmap.RepositoryID,
	root string,
	entity entityKey,
) {
	key := repositoryPath{repositoryID: repositoryID, path: root}
	i.rootEntities[key] = append(i.rootEntities[key], entity)
}

func (i *systemIndex) addEvidence(evidence []systemmap.Evidence, entity entityKey) {
	for _, observation := range evidence {
		if observation.Path == "" || observation.Path == "." {
			continue
		}
		key := repositoryPath{repositoryID: observation.RepositoryID, path: observation.Path}
		i.evidenceEntities[key] = append(i.evidenceEntities[key], entity)
	}
}

func (i systemIndex) directEntities(
	repositoryID systemmap.RepositoryID,
	changedPath string,
	maximum int,
) ([]entityKey, bool) {
	entities := make([]entityKey, 0)
	seen := make(map[entityKey]struct{})
	add := func(candidates []entityKey) bool {
		for _, candidate := range candidates {
			if _, exists := seen[candidate]; exists {
				continue
			}
			if len(entities) >= maximum {
				return true
			}
			seen[candidate] = struct{}{}
			entities = append(entities, candidate)
		}
		return false
	}
	if add(i.evidenceEntities[repositoryPath{repositoryID: repositoryID, path: changedPath}]) {
		slices.SortFunc(entities, compareEntityKeys)
		return entities, true
	}
	for directory := path.Dir(changedPath); ; directory = path.Dir(directory) {
		if add(i.rootEntities[repositoryPath{repositoryID: repositoryID, path: directory}]) {
			slices.SortFunc(entities, compareEntityKeys)
			return entities, true
		}
		if directory == "." {
			break
		}
	}
	slices.SortFunc(entities, compareEntityKeys)
	return entities, false
}

func compareEntityKeys(left, right entityKey) int {
	if compared := strings.Compare(string(left.kind), string(right.kind)); compared != 0 {
		return compared
	}
	return strings.Compare(left.id, right.id)
}

func propagatesImpact(kind string) bool {
	switch kind {
	case "calls", "configured_by", "consumes", "depends_on", "deploys", "publishes_to",
		"reads_from", "runs_on", "uses":
		return true
	default:
		return false
	}
}

func pathsForChange(change Change) []string {
	if change.Kind == ChangeRenamed {
		return []string{change.PreviousPath, change.Path}
	}
	return []string{change.Path}
}

type propagationState struct {
	entity entityKey
	cause  Cause
}

type visitKey struct {
	entity   entityKey
	changeID ChangeID
	path     string
}

type impactAccumulator struct {
	kind   ImpactKind
	causes []Cause
	seen   map[string]struct{}
}

type analysisState struct {
	changeSetID            string
	limits                 Limits
	index                  systemIndex
	services               map[entityKey]*impactAccumulator
	environments           map[entityKey]*impactAccumulator
	configurations         map[entityKey]*impactAccumulator
	diagnostics            []Diagnostic
	diagnosticCodes        map[string]struct{}
	partial                bool
	relationshipTraversals int
	directMatches          int
}

func newAnalysisState(changeSetID string, limits Limits, index systemIndex) *analysisState {
	return &analysisState{
		changeSetID:     changeSetID,
		limits:          limits,
		index:           index,
		services:        make(map[entityKey]*impactAccumulator),
		environments:    make(map[entityKey]*impactAccumulator),
		configurations:  make(map[entityKey]*impactAccumulator),
		diagnostics:     make([]Diagnostic, 0),
		diagnosticCodes: make(map[string]struct{}),
	}
}

func (s *analysisState) recordImpact(
	entity entityKey,
	kind ImpactKind,
	cause Cause,
	relationshipEnvironmentIDs []systemmap.EnvironmentID,
) {
	switch entity.kind {
	case systemmap.EntityService:
		if s.addImpact(s.services, entity, kind, cause, s.limits.MaxServices, diagnosticServiceLimit,
			"Additional affected services were omitted by the configured limit.") {
			s.recordAssociatedEnvironments(entity, cause, relationshipEnvironmentIDs)
		}
	case systemmap.EntityInfrastructure:
		if s.addImpact(s.configurations, entity, kind, cause, s.limits.MaxConfigurations,
			diagnosticConfigurationLimit,
			"Additional affected configurations were omitted by the configured limit.") {
			s.recordAssociatedEnvironments(entity, cause, relationshipEnvironmentIDs)
		}
	case systemmap.EntityEnvironment:
		s.addImpact(s.environments, entity, kind, cause, s.limits.MaxEnvironments,
			diagnosticEnvironmentLimit,
			"Additional affected environments were omitted by the configured limit.")
	}
}

func (s *analysisState) recordAssociatedEnvironments(
	entity entityKey,
	cause Cause,
	relationshipEnvironmentIDs []systemmap.EnvironmentID,
) {
	environmentIDs := append(
		slices.Clone(s.index.entityEnvironments[entity]),
		relationshipEnvironmentIDs...,
	)
	slices.Sort(environmentIDs)
	environmentIDs = slices.Compact(environmentIDs)
	for _, environmentID := range environmentIDs {
		environment := entityKey{kind: systemmap.EntityEnvironment, id: string(environmentID)}
		s.addImpact(s.environments, environment, ImpactAssociated, cause, s.limits.MaxEnvironments,
			diagnosticEnvironmentLimit,
			"Additional affected environments were omitted by the configured limit.")
	}
}

func (s *analysisState) addImpact(
	impacts map[entityKey]*impactAccumulator,
	entity entityKey,
	kind ImpactKind,
	cause Cause,
	maximumImpacts int,
	diagnosticCode string,
	diagnosticMessage string,
) bool {
	impact, exists := impacts[entity]
	if !exists {
		if len(impacts) >= maximumImpacts {
			s.addLimitingDiagnostic(Diagnostic{
				Code: diagnosticCode, Level: DiagnosticWarning, Path: ".", Message: diagnosticMessage,
			})
			return false
		}
		impact = &impactAccumulator{kind: kind, causes: make([]Cause, 0), seen: make(map[string]struct{})}
		impacts[entity] = impact
	}
	if impact.kind != ImpactDirect && kind == ImpactDirect {
		impact.kind = ImpactDirect
	}
	if impact.kind == ImpactAssociated && kind == ImpactDependent {
		impact.kind = ImpactDependent
	}
	key := causeIdentity(cause)
	if _, exists := impact.seen[key]; exists {
		return true
	}
	if len(impact.causes) >= s.limits.MaxCausesPerImpact {
		s.addLimitingDiagnostic(Diagnostic{
			Code: diagnosticCauseLimit, Level: DiagnosticWarning, Path: ".",
			Message: "Additional causes for an affected target were omitted by the configured limit.",
		})
		return true
	}
	impact.seen[key] = struct{}{}
	impact.causes = append(impact.causes, cause)
	return true
}

func causeIdentity(cause Cause) string {
	var builder strings.Builder
	builder.Grow(len(cause.ChangeID) + len(cause.Path) + len(cause.RelationshipIDs)*8 + 2)
	builder.WriteString(string(cause.ChangeID))
	builder.WriteByte(0)
	builder.WriteString(cause.Path)
	for _, relationshipID := range cause.RelationshipIDs {
		builder.WriteByte(0)
		builder.WriteString(string(relationshipID))
	}
	return builder.String()
}

func (s *analysisState) addInformationalDiagnostic(diagnostic Diagnostic) {
	if len(s.diagnostics) >= s.limits.MaxDiagnostics {
		return
	}
	s.diagnostics = append(s.diagnostics, diagnostic)
}

func (s *analysisState) addLimitingDiagnostic(diagnostic Diagnostic) {
	s.partial = true
	if _, exists := s.diagnosticCodes[diagnostic.Code]; exists {
		return
	}
	s.diagnosticCodes[diagnostic.Code] = struct{}{}
	if len(s.diagnostics) < s.limits.MaxDiagnostics {
		s.diagnostics = append(s.diagnostics, diagnostic)
		return
	}
	for index := len(s.diagnostics) - 1; index >= 0; index-- {
		if s.diagnostics[index].Level == DiagnosticInfo {
			s.diagnostics[index] = diagnostic
			return
		}
	}
}

func (s *analysisState) model() Model {
	model := emptyModel(s.changeSetID)
	model.Partial = s.partial
	model.Diagnostics = slices.Clone(s.diagnostics)
	for key, impact := range s.services {
		service := s.index.services[key]
		model.Services = append(model.Services, ServiceImpact{
			ServiceID: service.ID, RepositoryID: service.RepositoryID,
			Kind: impact.kind, Causes: slices.Clone(impact.causes),
		})
	}
	for key, impact := range s.environments {
		environment := s.index.environments[key]
		model.Environments = append(model.Environments, EnvironmentImpact{
			EnvironmentID: environment.ID, Kind: impact.kind, Causes: slices.Clone(impact.causes),
		})
	}
	for key, impact := range s.configurations {
		configuration := s.index.configurations[key]
		model.Configurations = append(model.Configurations, ConfigurationImpact{
			ConfigurationID: configuration.ID, RepositoryID: configuration.RepositoryID,
			Kind: impact.kind, Causes: slices.Clone(impact.causes),
		})
	}
	return model
}

func emptyModel(changeSetID string) Model {
	return Model{
		SchemaVersion:  CurrentSchemaVersion,
		ChangeSetID:    changeSetID,
		Services:       make([]ServiceImpact, 0),
		Environments:   make([]EnvironmentImpact, 0),
		Configurations: make([]ConfigurationImpact, 0),
		Diagnostics:    make([]Diagnostic, 0),
	}
}
