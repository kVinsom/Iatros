package devopsanalysis

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

// ErrInvalidModel indicates that a DevOps-analysis model violates its normalized contract.
var ErrInvalidModel = errors.New("devops analysis model is invalid")

// Validate checks schema compatibility, references, ordering, and safety invariants.
func (m Model) Validate() error {
	if !CurrentSchemaVersion.Supports(m.SchemaVersion) {
		return invalidModel("schema_version", "is unsupported")
	}

	toolCategories, err := validateTools(m.Tools)
	if err != nil {
		return err
	}
	if err := validateContainerBuilds(m.ContainerBuilds, toolCategories); err != nil {
		return err
	}
	if err := validateComposeServices(m.ComposeServices, toolCategories); err != nil {
		return err
	}
	if err := validateKubernetesResources(m.KubernetesResources, toolCategories); err != nil {
		return err
	}
	if err := validateHelmCharts(m.HelmCharts, toolCategories); err != nil {
		return err
	}
	if err := validateTerraformBlocks(m.TerraformBlocks, toolCategories); err != nil {
		return err
	}
	if err := validatePipelines(m.Pipelines, toolCategories); err != nil {
		return err
	}
	if err := validateGitOpsResources(m.GitOpsResources, toolCategories); err != nil {
		return err
	}
	if err := validateObservabilityResources(m.ObservabilityResources, toolCategories); err != nil {
		return err
	}
	if err := validateSecurityControls(m.SecurityControls, toolCategories); err != nil {
		return err
	}
	return validateDiagnostics(m)
}

// ValidateWithin checks the normalized model and verifies retained facts against limits.
func (m Model) ValidateWithin(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if len(m.Tools) > limits.MaxTools || len(m.ContainerBuilds) > limits.MaxContainerBuilds ||
		len(m.ComposeServices) > limits.MaxComposeServices ||
		len(m.KubernetesResources) > limits.MaxKubernetesResources ||
		len(m.HelmCharts) > limits.MaxHelmCharts ||
		len(m.TerraformBlocks) > limits.MaxTerraformBlocks ||
		len(m.Pipelines) > limits.MaxPipelines ||
		len(m.GitOpsResources) > limits.MaxGitOpsResources ||
		len(m.ObservabilityResources) > limits.MaxObservabilityResources ||
		len(m.SecurityControls) > limits.MaxSecurityControls ||
		len(m.Diagnostics) > limits.MaxDiagnostics {
		return invalidModel("collections", "exceed configured retention limits")
	}
	if !factsFitLimits(m, limits) {
		return invalidModel("facts", "exceed configured evidence, nested-value, or text limits")
	}
	return m.Validate()
}

func validateTools(tools []Tool) (map[ToolID]Category, error) {
	categories := make(map[ToolID]Category, len(tools))
	for index, tool := range tools {
		if !validIdentifier(string(tool.ID)) || !validText(tool.Name) ||
			!validCategory(tool.Category) || !validOptionalText(tool.VersionConstraint) ||
			!validCertainty(tool.Certainty) || !validEvidence(tool.Evidence) {
			return nil, invalidModel(indexedField("tools", index), "is invalid")
		}
		if index > 0 && tools[index-1].ID >= tool.ID {
			return nil, invalidModel("tools", "must be strictly ordered by id")
		}
		categories[tool.ID] = tool.Category
	}
	return categories, nil
}

func validateContainerBuilds(builds []ContainerBuild, tools map[ToolID]Category) error {
	for index, build := range builds {
		if !validFact(build.ID, build.ToolID, CategoryContainer, build.Certainty, build.Evidence, tools) ||
			!validOptionalIdentifier(build.Target) || !validSortedText(build.BaseImages) ||
			!validSortedEnvironmentNames(build.BuildArguments) || !validSortedPorts(build.ExposedPorts) {
			return invalidModel(indexedField("container_builds", index), "is invalid")
		}
		if index > 0 && builds[index-1].ID >= build.ID {
			return invalidModel("container_builds", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateComposeServices(services []ComposeService, tools map[ToolID]Category) error {
	for index, service := range services {
		if !validFact(service.ID, service.ToolID, CategoryContainer, service.Certainty, service.Evidence, tools) ||
			!validText(service.Name) || !validOptionalText(service.Image) ||
			!validOptionalDirectory(service.BuildContext) || !validSortedIdentifiers(service.Profiles) ||
			!validSortedIdentifiers(service.DependsOn) ||
			!validSortedEnvironmentNames(service.EnvironmentVariables) || !validSortedPorts(service.Ports) {
			return invalidModel(indexedField("compose_services", index), "is invalid")
		}
		if index > 0 && services[index-1].ID >= service.ID {
			return invalidModel("compose_services", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateKubernetesResources(resources []KubernetesResource, tools map[ToolID]Category) error {
	for index, resource := range resources {
		if !validFact(resource.ID, resource.ToolID, CategoryOrchestration, resource.Certainty, resource.Evidence, tools) ||
			!validText(resource.APIVersion) || !validText(resource.Kind) || !validText(resource.Name) ||
			!validOptionalText(resource.Namespace) || !validSortedText(resource.Images) ||
			!validSortedPorts(resource.Ports) {
			return invalidModel(indexedField("kubernetes_resources", index), "is invalid")
		}
		if index > 0 && resources[index-1].ID >= resource.ID {
			return invalidModel("kubernetes_resources", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateHelmCharts(charts []HelmChart, tools map[ToolID]Category) error {
	for index, chart := range charts {
		if !validFact(chart.ID, chart.ToolID, CategoryOrchestration, chart.Certainty, chart.Evidence, tools) ||
			!repositorypath.IsValidDirectory(chart.Root) || !validText(chart.Name) ||
			!validOptionalText(chart.Version) || !validOptionalText(chart.AppVersion) ||
			!validSortedText(chart.Dependencies) {
			return invalidModel(indexedField("helm_charts", index), "is invalid")
		}
		if index > 0 && charts[index-1].ID >= chart.ID {
			return invalidModel("helm_charts", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateTerraformBlocks(blocks []TerraformBlock, tools map[ToolID]Category) error {
	for index, block := range blocks {
		if !validFact(block.ID, block.ToolID, CategoryInfrastructureAsCode, block.Certainty, block.Evidence, tools) ||
			!validTerraformBlockKind(block.Kind) || !validOptionalText(block.Type) ||
			!validText(block.Name) || !validOptionalText(block.Provider) ||
			!validOptionalText(block.Source) || !validOptionalText(block.VersionConstraint) ||
			!validTerraformBlockShape(block) {
			return invalidModel(indexedField("terraform_blocks", index), "is invalid")
		}
		if index > 0 && blocks[index-1].ID >= block.ID {
			return invalidModel("terraform_blocks", "must be strictly ordered by id")
		}
	}
	return nil
}

func validatePipelines(pipelines []Pipeline, tools map[ToolID]Category) error {
	for index, pipeline := range pipelines {
		if !validFact(pipeline.ID, pipeline.ToolID, CategoryCICD, pipeline.Certainty, pipeline.Evidence, tools) ||
			!validText(pipeline.Name) || !validSortedText(pipeline.Triggers) || !validPipelineJobs(pipeline.Jobs) {
			return invalidModel(indexedField("pipelines", index), "is invalid")
		}
		if index > 0 && pipelines[index-1].ID >= pipeline.ID {
			return invalidModel("pipelines", "must be strictly ordered by id")
		}
	}
	return nil
}

func validPipelineJobs(jobs []PipelineJob) bool {
	jobIndexes := make(map[string]int, len(jobs))
	for index, job := range jobs {
		if !validIdentifier(job.ID) || !validText(job.Name) || !validOptionalText(job.Environment) ||
			!validSortedIdentifiers(job.Needs) || !validSemanticEvidence(job.Evidence) ||
			(index > 0 && jobs[index-1].ID >= job.ID) {
			return false
		}
		jobIndexes[job.ID] = index
	}
	return pipelineJobGraphIsAcyclic(jobs, jobIndexes)
}

type jobVisit struct {
	jobIndex            int
	nextDependencyIndex int
}

func pipelineJobGraphIsAcyclic(jobs []PipelineJob, jobIndexes map[string]int) bool {
	const (
		jobUnvisited uint8 = iota
		jobVisiting
		jobVisited
	)

	states := make([]uint8, len(jobs))
	var stack []jobVisit
	for startIndex := range jobs {
		if states[startIndex] != jobUnvisited {
			continue
		}

		states[startIndex] = jobVisiting
		stack = append(stack, jobVisit{jobIndex: startIndex})
		for len(stack) > 0 {
			visit := &stack[len(stack)-1]
			dependencies := jobs[visit.jobIndex].Needs
			if visit.nextDependencyIndex == len(dependencies) {
				states[visit.jobIndex] = jobVisited
				stack = stack[:len(stack)-1]
				continue
			}

			dependencyID := dependencies[visit.nextDependencyIndex]
			visit.nextDependencyIndex++
			dependencyIndex, exists := jobIndexes[dependencyID]
			if !exists || states[dependencyIndex] == jobVisiting {
				return false
			}
			if states[dependencyIndex] == jobVisited {
				continue
			}

			states[dependencyIndex] = jobVisiting
			stack = append(stack, jobVisit{jobIndex: dependencyIndex})
		}
	}
	return true
}

func validateGitOpsResources(resources []GitOpsResource, tools map[ToolID]Category) error {
	for index, resource := range resources {
		if !validFact(resource.ID, resource.ToolID, CategoryGitOps, resource.Certainty, resource.Evidence, tools) ||
			!validIdentifier(resource.Kind) || !validText(resource.Name) ||
			!validOptionalText(resource.Namespace) || !validOptionalDirectory(resource.SourcePath) ||
			!validOptionalText(resource.TargetNamespace) {
			return invalidModel(indexedField("gitops_resources", index), "is invalid")
		}
		if index > 0 && resources[index-1].ID >= resource.ID {
			return invalidModel("gitops_resources", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateObservabilityResources(resources []ObservabilityResource, tools map[ToolID]Category) error {
	for index, resource := range resources {
		if !validFact(resource.ID, resource.ToolID, CategoryObservability, resource.Certainty, resource.Evidence, tools) ||
			!validIdentifier(resource.Kind) || !validText(resource.Name) || !validSortedSignals(resource.Signals) {
			return invalidModel(indexedField("observability_resources", index), "is invalid")
		}
		if index > 0 && resources[index-1].ID >= resource.ID {
			return invalidModel("observability_resources", "must be strictly ordered by id")
		}
	}
	return nil
}

func validateSecurityControls(controls []SecurityControl, tools map[ToolID]Category) error {
	for index, control := range controls {
		category, exists := tools[control.ToolID]
		if !exists || (category != CategorySecurity && category != CategorySecrets) ||
			!validIdentifier(control.ID) || !validIdentifier(control.Kind) || !validText(control.Name) ||
			!validOptionalText(control.Scope) || !validEnforcement(control.Enforcement) ||
			!validCertainty(control.Certainty) || !validSemanticEvidence(control.Evidence) {
			return invalidModel(indexedField("security_controls", index), "is invalid")
		}
		if index > 0 && controls[index-1].ID >= control.ID {
			return invalidModel("security_controls", "must be strictly ordered by id")
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

func validFact(
	id string,
	toolID ToolID,
	expectedCategory Category,
	certainty Certainty,
	evidence []Evidence,
	tools map[ToolID]Category,
) bool {
	category, exists := tools[toolID]
	return exists && category == expectedCategory && validIdentifier(id) &&
		validCertainty(certainty) && validSemanticEvidence(evidence)
}

func validSemanticEvidence(evidence []Evidence) bool {
	inspection := inspectEvidence(evidence)
	return inspection.isValid && inspection.hasSemanticEvidence
}

func validEvidence(evidence []Evidence) bool {
	return inspectEvidence(evidence).isValid
}

type evidenceInspection struct {
	isValid             bool
	hasSemanticEvidence bool
}

func inspectEvidence(evidence []Evidence) evidenceInspection {
	if len(evidence) == 0 {
		return evidenceInspection{}
	}
	inspection := evidenceInspection{isValid: true}
	for index, observation := range evidence {
		if !validEvidenceKind(observation.Kind) || !repositorypath.IsValidFile(observation.Path) ||
			!validEvidencePosition(observation) ||
			(index > 0 && compareEvidence(evidence[index-1], observation) >= 0) {
			return evidenceInspection{}
		}
		if observation.Kind == EvidenceConfiguration || observation.Kind == EvidenceReference {
			inspection.hasSemanticEvidence = true
		}
	}
	return inspection
}

func validEvidencePosition(evidence Evidence) bool {
	positions := [4]int{evidence.StartLine, evidence.StartColumn, evidence.EndLine, evidence.EndColumn}
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

func validCategory(category Category) bool {
	switch category {
	case CategoryContainer, CategoryOrchestration, CategoryInfrastructureAsCode, CategoryCICD,
		CategoryGitOps, CategoryObservability, CategorySecurity, CategorySecrets:
		return true
	default:
		return false
	}
}

func validCertainty(certainty Certainty) bool {
	return certainty == CertaintyObserved || certainty == CertaintyInferred
}

func validEvidenceKind(kind EvidenceKind) bool {
	return kind == EvidenceFilename || kind == EvidenceConfiguration || kind == EvidenceReference
}

func validTerraformBlockKind(kind TerraformBlockKind) bool {
	switch kind {
	case TerraformBlockProvider, TerraformBlockModule, TerraformBlockResource, TerraformBlockData,
		TerraformBlockVariable, TerraformBlockOutput:
		return true
	default:
		return false
	}
}

func validTerraformBlockShape(block TerraformBlock) bool {
	if block.Kind == TerraformBlockResource || block.Kind == TerraformBlockData {
		return block.Type != ""
	}
	return block.Type == ""
}

func validEnforcement(enforcement Enforcement) bool {
	return enforcement == EnforcementUnknown || enforcement == EnforcementAdvisory ||
		enforcement == EnforcementBlocking
}

func validDiagnosticLevel(level DiagnosticLevel) bool {
	return level == DiagnosticInfo || level == DiagnosticWarning || level == DiagnosticError
}

func validSortedText(values []string) bool {
	return validSortedStrings(values, validText)
}

func validSortedIdentifiers(values []string) bool {
	return validSortedStrings(values, validIdentifier)
}

func validSortedEnvironmentNames(values []string) bool {
	return validSortedStrings(values, validEnvironmentName)
}

func validSortedStrings(values []string, validate func(string) bool) bool {
	for index, current := range values {
		if !validate(current) || (index > 0 && values[index-1] >= current) {
			return false
		}
	}
	return true
}

func validSortedPorts(ports []uint16) bool {
	for index, port := range ports {
		if port == 0 || (index > 0 && ports[index-1] >= port) {
			return false
		}
	}
	return true
}

func validSortedSignals(signals []Signal) bool {
	for index, signal := range signals {
		if !validSignal(signal) || (index > 0 && signals[index-1] >= signal) {
			return false
		}
	}
	return true
}

func validSignal(signal Signal) bool {
	switch signal {
	case SignalMetrics, SignalLogs, SignalTraces, SignalProfiles, SignalAlerts:
		return true
	default:
		return false
	}
}

func validOptionalDirectory(directoryPath string) bool {
	return directoryPath == "" || repositorypath.IsValidDirectory(directoryPath)
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

func indexedField(collection string, index int) string {
	return fmt.Sprintf("%s[%d]", collection, index)
}

func invalidModel(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidModel, field, reason)
}

func factsFitLimits(model Model, limits Limits) bool {
	for _, tool := range model.Tools {
		if !factFitsLimits(tool.Evidence, limits, string(tool.ID), tool.Name,
			string(tool.Category), tool.VersionConstraint, string(tool.Certainty)) {
			return false
		}
	}
	for _, build := range model.ContainerBuilds {
		if !nestedValuesFit(limits, build.BaseImages, build.BuildArguments) ||
			len(build.ExposedPorts) > limits.MaxValuesPerFact ||
			!factFitsLimits(build.Evidence, limits, build.ID, string(build.ToolID),
				build.Target, string(build.Certainty)) {
			return false
		}
	}
	for _, service := range model.ComposeServices {
		if !nestedValuesFit(limits, service.Profiles, service.DependsOn, service.EnvironmentVariables) ||
			len(service.Ports) > limits.MaxValuesPerFact ||
			!factFitsLimits(service.Evidence, limits, service.ID, string(service.ToolID), service.Name,
				service.Image, service.BuildContext, string(service.Certainty)) {
			return false
		}
	}
	for _, resource := range model.KubernetesResources {
		if !nestedValuesFit(limits, resource.Images) || len(resource.Ports) > limits.MaxValuesPerFact ||
			!factFitsLimits(resource.Evidence, limits, resource.ID, string(resource.ToolID),
				resource.APIVersion, resource.Kind, resource.Name, resource.Namespace,
				string(resource.Certainty)) {
			return false
		}
	}
	for _, chart := range model.HelmCharts {
		if !nestedValuesFit(limits, chart.Dependencies) ||
			!factFitsLimits(chart.Evidence, limits, chart.ID, string(chart.ToolID), chart.Root,
				chart.Name, chart.Version, chart.AppVersion, string(chart.Certainty)) {
			return false
		}
	}
	for _, block := range model.TerraformBlocks {
		if !factFitsLimits(block.Evidence, limits, block.ID, string(block.ToolID), string(block.Kind),
			block.Type, block.Name, block.Provider, block.Source, block.VersionConstraint,
			string(block.Certainty)) {
			return false
		}
	}
	for _, pipeline := range model.Pipelines {
		if len(pipeline.Jobs) > limits.MaxJobsPerPipeline || !nestedValuesFit(limits, pipeline.Triggers) ||
			!factFitsLimits(pipeline.Evidence, limits, pipeline.ID, string(pipeline.ToolID),
				pipeline.Name, string(pipeline.Certainty)) {
			return false
		}
		dependencyCount := 0
		for _, job := range pipeline.Jobs {
			if len(job.Needs) > limits.MaxJobDependenciesPerPipeline-dependencyCount ||
				!nestedValuesFit(limits, job.Needs) ||
				!factFitsLimits(job.Evidence, limits, job.ID, job.Name, job.Environment) {
				return false
			}
			dependencyCount += len(job.Needs)
		}
	}
	for _, resource := range model.GitOpsResources {
		if !factFitsLimits(resource.Evidence, limits, resource.ID, string(resource.ToolID), resource.Kind,
			resource.Name, resource.Namespace, resource.SourcePath, resource.TargetNamespace,
			string(resource.Certainty)) {
			return false
		}
	}
	for _, resource := range model.ObservabilityResources {
		if len(resource.Signals) > limits.MaxValuesPerFact ||
			!factFitsLimits(resource.Evidence, limits, resource.ID, string(resource.ToolID), resource.Kind,
				resource.Name, string(resource.Certainty)) {
			return false
		}
	}
	for _, control := range model.SecurityControls {
		if !factFitsLimits(control.Evidence, limits, control.ID, string(control.ToolID), control.Kind,
			control.Name, control.Scope, string(control.Enforcement), string(control.Certainty)) {
			return false
		}
	}
	for _, diagnostic := range model.Diagnostics {
		if !textsFitLimit(limits.MaxTextBytes, diagnostic.Code, string(diagnostic.Level),
			diagnostic.Path, diagnostic.Message) {
			return false
		}
	}
	return true
}

func nestedValuesFit(limits Limits, collections ...[]string) bool {
	for _, collection := range collections {
		if len(collection) > limits.MaxValuesPerFact || !textsFitLimit(limits.MaxTextBytes, collection...) {
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
