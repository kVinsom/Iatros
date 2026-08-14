package doctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kVinsom/Iatros/internal/artifactvalidation"
	"github.com/kVinsom/Iatros/internal/devopsanalysis"
	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

// Auditor coordinates bounded, deterministic Doctor rules.
type Auditor struct {
	limits        Limits
	findingLimits finding.Limits
	rules         []registeredRule
}

type registeredRule struct {
	descriptor RuleDescriptor
	rule       Rule
}

// NewAuditor constructs an Auditor with an explicit rule set.
func NewAuditor(limits Limits, findingLimits finding.Limits, rules ...Rule) (*Auditor, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if err := findingLimits.Validate(); err != nil {
		return nil, err
	}
	if limits.MaxRepositoryFindings > findingLimits.MaxFindings ||
		len(rules) == 0 || len(rules) > limits.MaxRules {
		return nil, ErrInvalidRule
	}
	registered := make([]registeredRule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			return nil, ErrInvalidRule
		}
		descriptor := rule.Descriptor()
		if !validRuleDescriptor(descriptor) {
			return nil, ErrInvalidRule
		}
		registered = append(registered, registeredRule{descriptor: descriptor, rule: rule})
	}
	slices.SortFunc(registered, func(left, right registeredRule) int {
		return strings.Compare(left.descriptor.ID, right.descriptor.ID)
	})
	categories := make(map[Category]int, 6)
	for index, rule := range registered {
		if index > 0 && registered[index-1].descriptor.ID == rule.descriptor.ID {
			return nil, fmt.Errorf("%w: duplicate id %q", ErrInvalidRule, rule.descriptor.ID)
		}
		categories[rule.descriptor.Category]++
	}
	for _, category := range auditCategories() {
		if categories[category] == 0 {
			return nil, fmt.Errorf("%w: category %q has no rules", ErrInvalidRule, category)
		}
	}
	return &Auditor{limits: limits, findingLimits: findingLimits, rules: registered}, nil
}

// NewDefaultAuditor constructs an Auditor with the built-in evidence-based rules.
func NewDefaultAuditor(limits Limits, findingLimits finding.Limits) (*Auditor, error) {
	return NewAuditor(limits, findingLimits, BuiltInRules()...)
}

// Audit evaluates the snapshot and applies controlled finding exclusions.
func (a *Auditor) Audit(
	ctx context.Context,
	snapshot Snapshot,
	exclusions []finding.Exclusion,
	evaluatedAt time.Time,
) (Report, error) {
	if a == nil || ctx == nil {
		return Report{}, ErrInvalidSnapshot
	}
	normalizedSnapshot, err := normalizeAndValidateSnapshot(snapshot, a.limits, a.findingLimits)
	if err != nil {
		return Report{}, err
	}

	auditContext, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()
	if err := auditContext.Err(); err != nil {
		return Report{}, err
	}

	findings := cloneSlice(normalizedSnapshot.RepositoryFindings)
	coverage := newCoverageCounters()
	diagnostics := newDiagnosticCollector(a.limits.MaxDiagnostics)
	seenFindingIDs := make(map[string]struct{}, len(findings))
	for _, repositoryFinding := range findings {
		seenFindingIDs[repositoryFinding.ID] = struct{}{}
	}
	for _, registered := range a.rules {
		if err := auditContext.Err(); err != nil {
			return Report{}, err
		}
		descriptor := registered.descriptor
		inputState := normalizedSnapshot.state(descriptor.RequiredInputs)
		if inputState == evidenceUnavailable {
			coverage.skipped(descriptor.Category, false)
			diagnostics.add(ruleDiagnostic(
				descriptor,
				"DOCTOR_INPUT_UNAVAILABLE",
				DiagnosticWarning,
				"Required normalized evidence is unavailable; the rule was not evaluated.",
			))
			continue
		}
		if inputState == evidencePartial && descriptor.RequiresComplete {
			coverage.skipped(descriptor.Category, true)
			diagnostics.add(ruleDiagnostic(
				descriptor,
				"DOCTOR_INPUT_PARTIAL",
				DiagnosticWarning,
				"Required normalized evidence is partial; the absence rule was not evaluated.",
			))
			continue
		}

		remainingFindings := a.findingLimits.MaxFindings - len(findings)
		ruleFindings, ruleErr := registered.rule.Evaluate(
			auditContext,
			normalizedSnapshot,
			RuleBudget{
				MaxFindings:           remainingFindings,
				MaxSubjectsPerFinding: a.findingLimits.MaxSubjectsPerFinding,
				MaxEvidencePerFinding: a.findingLimits.MaxEvidencePerFinding,
				MaxActionsPerFinding:  a.findingLimits.MaxActionsPerFinding,
				MaxTextBytes:          a.findingLimits.MaxTextBytes,
			},
		)
		if ruleErr != nil {
			coverage.skipped(descriptor.Category, inputState == evidencePartial)
			if errors.Is(ruleErr, ErrRuleBudgetExceeded) {
				diagnostics.add(ruleDiagnostic(
					descriptor,
					"DOCTOR_FINDINGS_LIMIT_REACHED",
					DiagnosticError,
					"The Doctor rule exceeded the remaining finding retention limit.",
				))
				continue
			}
			diagnostics.add(ruleDiagnostic(
				descriptor,
				"DOCTOR_RULE_FAILED",
				DiagnosticError,
				"The Doctor rule could not complete.",
			))
			continue
		}
		normalizedFindings, validationErr := validateRuleFindings(
			descriptor,
			ruleFindings,
			a.findingLimits,
			seenFindingIDs,
		)
		if errors.Is(validationErr, ErrRuleBudgetExceeded) {
			coverage.skipped(descriptor.Category, inputState == evidencePartial)
			diagnostics.add(ruleDiagnostic(
				descriptor,
				"DOCTOR_FINDINGS_LIMIT_REACHED",
				DiagnosticError,
				"The Doctor rule exceeded the remaining finding retention limit.",
			))
			continue
		}
		if validationErr != nil {
			coverage.skipped(descriptor.Category, inputState == evidencePartial)
			diagnostics.add(ruleDiagnostic(
				descriptor,
				"DOCTOR_RULE_OUTPUT_INVALID",
				DiagnosticError,
				"The Doctor rule returned invalid or excessive findings.",
			))
			continue
		}
		coverage.evaluated(descriptor.Category, inputState == evidencePartial)
		for _, auditFinding := range normalizedFindings {
			seenFindingIDs[auditFinding.ID] = struct{}{}
			findings = append(findings, auditFinding)
		}
	}

	findingModel, err := finding.ApplyExclusions(
		auditContext,
		findings,
		exclusions,
		evaluatedAt,
		a.findingLimits,
	)
	if err != nil {
		return Report{}, fmt.Errorf("apply Doctor finding exclusions: %w", err)
	}
	report := Report{
		SchemaVersion: CurrentSchemaVersion,
		SnapshotID:    normalizedSnapshot.ID,
		Coverage:      coverage.results(),
		Findings:      findingModel,
		Diagnostics:   diagnostics.results(),
	}
	report.Status = reportStatus(report.Coverage, report.Findings.Findings)
	report = report.Normalized()
	if err := report.ValidateWithin(a.limits, a.findingLimits); err != nil {
		return Report{}, fmt.Errorf("validate Doctor report: %w", err)
	}
	return report, nil
}

func normalizeAndValidateSnapshot(
	snapshot Snapshot,
	limits Limits,
	findingLimits finding.Limits,
) (Snapshot, error) {
	if !validIdentifier(snapshot.ID) {
		return Snapshot{}, ErrInvalidSnapshot
	}
	if len(snapshot.RepositoryFindings) > limits.MaxRepositoryFindings {
		return Snapshot{}, ErrInvalidSnapshot
	}
	inputFacts, hasInput := snapshotFactCount(snapshot)
	if !hasInput || inputFacts > uint64(limits.MaxInputFacts) {
		return Snapshot{}, ErrInvalidSnapshot
	}
	if snapshot.CodeAnalysis != nil {
		model := snapshot.CodeAnalysis.Normalized()
		if err := model.Validate(); err != nil {
			return Snapshot{}, fmt.Errorf("%w: code analysis", ErrInvalidSnapshot)
		}
		snapshot.CodeAnalysis = &model
	}
	if snapshot.DevOpsAnalysis != nil {
		model := snapshot.DevOpsAnalysis.Normalized()
		if err := model.Validate(); err != nil {
			return Snapshot{}, fmt.Errorf("%w: DevOps analysis", ErrInvalidSnapshot)
		}
		snapshot.DevOpsAnalysis = &model
	}
	if snapshot.SystemMap != nil {
		model := snapshot.SystemMap.Normalized()
		if err := model.Validate(); err != nil {
			return Snapshot{}, fmt.Errorf("%w: system map", ErrInvalidSnapshot)
		}
		snapshot.SystemMap = &model
	}
	snapshot.ValidationReports = cloneSlice(snapshot.ValidationReports)
	for index := range snapshot.ValidationReports {
		snapshot.ValidationReports[index] = snapshot.ValidationReports[index].Normalized()
		if err := snapshot.ValidationReports[index].Validate(); err != nil {
			return Snapshot{}, fmt.Errorf("%w: artifact validation report", ErrInvalidSnapshot)
		}
	}
	snapshot.RepositoryFindings = cloneSlice(snapshot.RepositoryFindings)
	for index := range snapshot.RepositoryFindings {
		snapshot.RepositoryFindings[index] = snapshot.RepositoryFindings[index].Normalized()
		repositoryFinding := snapshot.RepositoryFindings[index]
		if repositoryFinding.Disposition != finding.DispositionActive ||
			repositoryFinding.Exclusion != nil || repositoryFinding.ValidateWithin(findingLimits) != nil {
			return Snapshot{}, fmt.Errorf("%w: repository finding", ErrInvalidSnapshot)
		}
	}
	return snapshot, nil
}

func snapshotFactCount(snapshot Snapshot) (uint64, bool) {
	facts := uint64(len(snapshot.RepositoryFindings))
	hasInput := len(snapshot.RepositoryFindings) > 0
	if snapshot.CodeAnalysis != nil {
		hasInput = true
		model := snapshot.CodeAnalysis
		facts += uint64(len(model.Services)) + uint64(len(model.Frameworks)) +
			uint64(len(model.PortBindings)) + uint64(len(model.APIEndpoints)) +
			uint64(len(model.EnvironmentVariables)) + uint64(len(model.ResourceDependencies)) +
			uint64(len(model.Diagnostics))
	}
	if snapshot.DevOpsAnalysis != nil {
		hasInput = true
		facts += devOpsFactCount(*snapshot.DevOpsAnalysis)
	}
	if snapshot.SystemMap != nil {
		hasInput = true
		facts += systemFactCount(*snapshot.SystemMap)
	}
	if len(snapshot.ValidationReports) > 0 {
		hasInput = true
		for _, report := range snapshot.ValidationReports {
			facts += uint64(len(report.Artifacts)) + uint64(len(report.Diagnostics))
		}
	}
	return facts, hasInput
}

func devOpsFactCount(model devopsanalysis.Model) uint64 {
	return uint64(len(model.Tools)) + uint64(len(model.ContainerBuilds)) +
		uint64(len(model.ComposeServices)) + uint64(len(model.KubernetesResources)) +
		uint64(len(model.HelmCharts)) + uint64(len(model.TerraformBlocks)) +
		uint64(len(model.Pipelines)) + uint64(len(model.GitOpsResources)) +
		uint64(len(model.ObservabilityResources)) + uint64(len(model.SecurityControls)) +
		uint64(len(model.Diagnostics))
}

func systemFactCount(model systemmap.Model) uint64 {
	return uint64(len(model.Repositories)) + uint64(len(model.Services)) +
		uint64(len(model.Libraries)) + uint64(len(model.Infrastructure)) +
		uint64(len(model.Environments)) + uint64(len(model.Owners)) +
		uint64(len(model.ExternalResources)) + uint64(len(model.Relationships)) +
		uint64(len(model.Diagnostics))
}

type evidenceState uint8

const (
	evidenceUnavailable evidenceState = iota
	evidencePartial
	evidenceComplete
)

func (s Snapshot) state(required InputSet) evidenceState {
	state := evidenceComplete
	for _, input := range []InputSet{
		InputCodeAnalysis,
		InputDevOpsAnalysis,
		InputSystemMap,
		InputArtifactValidation,
	} {
		if required&input == 0 {
			continue
		}
		inputState := s.inputState(input)
		if inputState == evidenceUnavailable {
			return evidenceUnavailable
		}
		if inputState == evidencePartial {
			state = evidencePartial
		}
	}
	return state
}

func (s Snapshot) inputState(input InputSet) evidenceState {
	switch input {
	case InputCodeAnalysis:
		if s.CodeAnalysis == nil {
			return evidenceUnavailable
		}
		if s.CodeAnalysis.Partial {
			return evidencePartial
		}
	case InputDevOpsAnalysis:
		if s.DevOpsAnalysis == nil {
			return evidenceUnavailable
		}
		if s.DevOpsAnalysis.Partial {
			return evidencePartial
		}
	case InputSystemMap:
		if s.SystemMap == nil {
			return evidenceUnavailable
		}
		if s.SystemMap.Partial {
			return evidencePartial
		}
	case InputArtifactValidation:
		if len(s.ValidationReports) == 0 {
			return evidenceUnavailable
		}
		for _, report := range s.ValidationReports {
			if report.Status == artifactvalidation.StatusPartial {
				return evidencePartial
			}
		}
	}
	return evidenceComplete
}

func validateRuleFindings(
	descriptor RuleDescriptor,
	findings []finding.Finding,
	limits finding.Limits,
	seenIDs map[string]struct{},
) ([]finding.Finding, error) {
	if len(seenIDs) > limits.MaxFindings || len(findings) > limits.MaxFindings-len(seenIDs) {
		return nil, ErrRuleBudgetExceeded
	}
	for _, auditFinding := range findings {
		if len(auditFinding.Subjects) > limits.MaxSubjectsPerFinding ||
			len(auditFinding.Evidence) > limits.MaxEvidencePerFinding ||
			len(auditFinding.Recommendation.Actions) > limits.MaxActionsPerFinding {
			return nil, ErrInvalidRule
		}
	}
	normalized := cloneSlice(findings)
	localIDs := make(map[string]struct{}, len(normalized))
	for index := range normalized {
		normalized[index] = normalized[index].Normalized()
		if normalized[index].RuleID != descriptor.ID ||
			normalized[index].Disposition != finding.DispositionActive ||
			normalized[index].Exclusion != nil || normalized[index].ValidateWithin(limits) != nil {
			return nil, ErrInvalidRule
		}
		if _, exists := seenIDs[normalized[index].ID]; exists {
			return nil, ErrInvalidRule
		}
		if _, exists := localIDs[normalized[index].ID]; exists {
			return nil, ErrInvalidRule
		}
		localIDs[normalized[index].ID] = struct{}{}
	}
	return normalized, nil
}

type coverageCounter struct {
	evaluated int
	skipped   int
	isPartial bool
}

type coverageCounters map[Category]*coverageCounter

func newCoverageCounters() coverageCounters {
	counters := make(coverageCounters, 6)
	for _, category := range auditCategories() {
		counters[category] = &coverageCounter{}
	}
	return counters
}

func (c coverageCounters) evaluated(category Category, partialInput bool) {
	c[category].evaluated++
	c[category].isPartial = c[category].isPartial || partialInput
}

func (c coverageCounters) skipped(category Category, partialInput bool) {
	c[category].skipped++
	c[category].isPartial = c[category].isPartial || partialInput
}

func (c coverageCounters) results() []Coverage {
	results := make([]Coverage, 0, 6)
	for _, category := range auditCategories() {
		counter := c[category]
		coverage := Coverage{
			Category:       category,
			RulesEvaluated: counter.evaluated,
			RulesSkipped:   counter.skipped,
			Status:         CoverageEvaluated,
		}
		if counter.evaluated == 0 {
			coverage.Status = CoverageNotEvaluated
			coverage.Reason = "Required normalized evidence was unavailable or incomplete."
		} else if counter.skipped > 0 || counter.isPartial {
			coverage.Status = CoveragePartial
			coverage.Reason = "At least one rule lacked complete evidence or could not complete."
		}
		results = append(results, coverage)
	}
	return results
}

type diagnosticCollector struct {
	max         int
	diagnostics []Diagnostic
	isTruncated bool
}

func newDiagnosticCollector(maxDiagnostics int) *diagnosticCollector {
	return &diagnosticCollector{max: maxDiagnostics, diagnostics: make([]Diagnostic, 0)}
}

func (c *diagnosticCollector) add(diagnostic Diagnostic) {
	if len(c.diagnostics) < c.max {
		c.diagnostics = append(c.diagnostics, diagnostic)
		return
	}
	if !c.isTruncated {
		c.diagnostics[len(c.diagnostics)-1] = Diagnostic{
			Code: "DOCTOR_DIAGNOSTICS_TRUNCATED", Level: DiagnosticError,
			Message:  "Doctor diagnostics exceeded the configured retention limit.",
			Category: diagnostic.Category,
		}
		c.isTruncated = true
	}
}

func (c *diagnosticCollector) results() []Diagnostic {
	return c.diagnostics
}

func ruleDiagnostic(
	descriptor RuleDescriptor,
	code string,
	level DiagnosticLevel,
	message string,
) Diagnostic {
	return Diagnostic{
		Code: code, Level: level, Message: message,
		Category: descriptor.Category, RuleID: descriptor.ID,
	}
}

func auditCategories() []Category {
	return []Category{
		CategoryProduction,
		CategoryReliability,
		CategoryPerformance,
		CategorySecurity,
		CategoryDeployment,
		CategoryCost,
	}
}
