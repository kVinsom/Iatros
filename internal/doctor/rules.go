package doctor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/kVinsom/Iatros/internal/artifactvalidation"
	"github.com/kVinsom/Iatros/internal/devopsanalysis"
	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

const (
	doctorProducer        = "iatros.doctor"
	doctorProducerVersion = "1.0"
	doctorRuleVersion     = "1.0"
)

// BuiltInRules returns the provider-neutral evidence-based Doctor rules.
func BuiltInRules() []Rule {
	return []Rule{
		productionEnvironmentRule{},
		productionOwnershipRule{},
		reliabilityObservabilityRule{},
		reliabilityAlertingRule{},
		performancePinnedImageRule{},
		securityControlsRule{},
		securitySensitiveDefaultRule{},
		deploymentPipelineRule{},
		deploymentGitOpsRule{},
		deploymentArtifactRule{},
		costEnvironmentScopeRule{},
		costInfrastructureAsCodeRule{},
	}
}

type productionEnvironmentRule struct{}

func (productionEnvironmentRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "production.environment.not_detected", Version: doctorRuleVersion,
		Category: CategoryProduction, RequiredInputs: InputSystemMap, RequiresComplete: true,
	}
}

func (productionEnvironmentRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, environment := range snapshot.SystemMap.Environments {
		if isProductionEnvironment(environment) {
			return make([]finding.Finding, 0), nil
		}
	}
	descriptor := (productionEnvironmentRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Production environment was not detected",
		Description: "The normalized system map does not identify a production environment.",
		Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "A complete system map contains no environment identified as production.",
		RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodLikely,
		RiskSummary:    "Production deployment, ownership, and policy scope may remain ambiguous.",
		Recommendation: "Define an explicit production environment in the system model.",
		Actions:        []string{"Add evidence-backed production environment metadata and associate deployed services and infrastructure with it."},
	})}, nil
}

type productionOwnershipRule struct{}

func (productionOwnershipRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "production.service.owner_missing", Version: doctorRuleVersion,
		Category: CategoryProduction, RequiredInputs: InputSystemMap, RequiresComplete: true,
	}
}

func (productionOwnershipRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	budget RuleBudget,
) ([]finding.Finding, error) {
	productionEnvironments := productionEnvironmentIDs(snapshot.SystemMap.Environments)
	ownedServices := make(map[string]struct{})
	for _, relationship := range snapshot.SystemMap.Relationships {
		if relationship.Kind == "owns" && relationship.Source.Kind == systemmap.EntityOwner &&
			relationship.Target.Kind == systemmap.EntityService {
			ownedServices[relationship.Target.ID] = struct{}{}
		}
	}
	descriptor := (productionOwnershipRule{}).Descriptor()
	findings := make([]finding.Finding, 0)
	for _, service := range snapshot.SystemMap.Services {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !intersectsEnvironments(service.EnvironmentIDs, productionEnvironments) {
			continue
		}
		if _, exists := ownedServices[string(service.ID)]; exists {
			continue
		}
		if len(findings) >= budget.MaxFindings {
			return nil, ErrRuleBudgetExceeded
		}
		findings = append(findings, newFinding(descriptor, findingSpec{
			Key: string(service.ID), Title: "Production service has no detected owner",
			Description: "A service assigned to production has no ownership relationship in the system map.",
			Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
			Subjects: []finding.Subject{{
				Kind: "service", ID: string(service.ID), RepositoryID: string(service.RepositoryID),
			}},
			Evidence:  "The complete system map has no owner relationship targeting this production service.",
			RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodLikely,
			RiskSummary:    "Production incidents and changes may lack accountable responders.",
			Recommendation: "Assign an accountable owner to every production service.",
			Actions:        []string{"Add an evidence-backed owner relationship for the service."},
		}))
	}
	return findings, nil
}

type reliabilityObservabilityRule struct{}

func (reliabilityObservabilityRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "reliability.observability.not_detected", Version: doctorRuleVersion,
		Category:       CategoryReliability,
		RequiredInputs: InputSystemMap | InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (reliabilityObservabilityRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.SystemMap.Services) == 0 || len(snapshot.DevOpsAnalysis.ObservabilityResources) > 0 {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (reliabilityObservabilityRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Observability configuration was not detected",
		Description: "Services exist, but no monitoring, logging, tracing, profiling, or alerting resource was detected.",
		Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete system and DevOps models contain services but no observability resources.",
		RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodAlmostCertain,
		RiskSummary:    "Failures and degradation may remain invisible until users report them.",
		Recommendation: "Add observable service health and failure signals.",
		Actions:        []string{"Define metrics, logs, traces, and operational dashboards appropriate to each service."},
	})}, nil
}

type reliabilityAlertingRule struct{}

func (reliabilityAlertingRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "reliability.alerting.not_detected", Version: doctorRuleVersion,
		Category:       CategoryReliability,
		RequiredInputs: InputSystemMap | InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (reliabilityAlertingRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.SystemMap.Services) == 0 || hasAlerting(snapshot) {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (reliabilityAlertingRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Operational alerting was not detected",
		Description: "Services exist, but no observability resource exposes an alert signal.",
		Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete DevOps evidence contains no observability resource with alerting signals.",
		RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodLikely,
		RiskSummary:    "Operators may not receive timely notification of production failures.",
		Recommendation: "Define actionable alerts for service-level failure conditions.",
		Actions:        []string{"Add alert rules with ownership, routing, and runbook references."},
	})}, nil
}

type performancePinnedImageRule struct{}

func (performancePinnedImageRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "performance.container_image.unpinned", Version: doctorRuleVersion,
		Category: CategoryPerformance, RequiredInputs: InputDevOpsAnalysis,
	}
}

func (performancePinnedImageRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	budget RuleBudget,
) ([]finding.Finding, error) {
	descriptor := (performancePinnedImageRule{}).Descriptor()
	findings := make([]finding.Finding, 0)
	appendImageFinding := func(subjectKind, subjectID, image string) error {
		if image == "" || image == "scratch" || isPinnedImage(image) {
			return nil
		}
		if len(findings) >= budget.MaxFindings {
			return ErrRuleBudgetExceeded
		}
		findings = append(findings, newFinding(descriptor, findingSpec{
			Key:         subjectKind + "." + subjectID + "." + image,
			Title:       "Container image is not pinned",
			Description: "A container image reference uses a floating or implicit tag.",
			Severity:    finding.SeverityMedium, Confidence: finding.ConfidenceConfirmed,
			Subjects:  []finding.Subject{{Kind: subjectKind, ID: subjectID}},
			Evidence:  "Observed container image reference: " + image + ".",
			RiskLevel: finding.RiskMedium, Likelihood: finding.LikelihoodLikely,
			RiskSummary:    "Uncontrolled image changes can alter startup time, resource use, and runtime behavior.",
			Recommendation: "Use immutable container image references.",
			Actions:        []string{"Pin the image to an immutable digest or controlled version tag."},
		}))
		return nil
	}
	for _, build := range snapshot.DevOpsAnalysis.ContainerBuilds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, image := range build.BaseImages {
			if err := appendImageFinding("container_build", build.ID, image); err != nil {
				return nil, err
			}
		}
	}
	for _, service := range snapshot.DevOpsAnalysis.ComposeServices {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := appendImageFinding("compose_service", service.ID, service.Image); err != nil {
			return nil, err
		}
	}
	for _, resource := range snapshot.DevOpsAnalysis.KubernetesResources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, image := range resource.Images {
			if err := appendImageFinding("kubernetes_resource", resource.ID, image); err != nil {
				return nil, err
			}
		}
	}
	return findings, nil
}

type securityControlsRule struct{}

func (securityControlsRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "security.blocking_control.not_detected", Version: doctorRuleVersion,
		Category:       CategorySecurity,
		RequiredInputs: InputSystemMap | InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (securityControlsRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.SystemMap.Services) == 0 || hasBlockingSecurityControl(snapshot) {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (securityControlsRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Blocking security control was not detected",
		Description: "Services exist, but no detected security control blocks unsafe changes or artifacts.",
		Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete DevOps evidence contains no security control with blocking enforcement.",
		RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodLikely,
		RiskSummary:    "Known vulnerable or non-compliant artifacts may reach deployment.",
		Recommendation: "Add at least one blocking security gate.",
		Actions:        []string{"Configure a supported security control to fail delivery on defined high-risk results."},
	})}, nil
}

type securitySensitiveDefaultRule struct{}

func (securitySensitiveDefaultRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "security.sensitive_environment.default_detected", Version: doctorRuleVersion,
		Category: CategorySecurity, RequiredInputs: InputCodeAnalysis,
	}
}

func (securitySensitiveDefaultRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	budget RuleBudget,
) ([]finding.Finding, error) {
	descriptor := (securitySensitiveDefaultRule{}).Descriptor()
	findings := make([]finding.Finding, 0)
	for _, environmentVariable := range snapshot.CodeAnalysis.EnvironmentVariables {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !environmentVariable.IsSensitive || !environmentVariable.HasDefault {
			continue
		}
		if len(findings) >= budget.MaxFindings {
			return nil, ErrRuleBudgetExceeded
		}
		findings = append(findings, newFinding(descriptor, findingSpec{
			Key:         string(environmentVariable.ServiceID) + "." + environmentVariable.Name,
			Title:       "Sensitive environment variable has a default",
			Description: "Static code evidence marks an environment variable as sensitive and defaulted.",
			Severity:    finding.SeverityCritical, Confidence: finding.ConfidenceConfirmed,
			Subjects:  []finding.Subject{{Kind: "service", ID: string(environmentVariable.ServiceID)}},
			Evidence:  "Sensitive variable " + environmentVariable.Name + " has a detected default value.",
			RiskLevel: finding.RiskCritical, Likelihood: finding.LikelihoodLikely,
			RiskSummary:    "A default secret can be reused, exposed, or deployed unintentionally.",
			Recommendation: "Remove defaults for sensitive configuration.",
			Actions:        []string{"Require the value from an approved secret provider at deployment time."},
		}))
	}
	return findings, nil
}

type deploymentPipelineRule struct{}

func (deploymentPipelineRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "deployment.pipeline.not_detected", Version: doctorRuleVersion,
		Category:       CategoryDeployment,
		RequiredInputs: InputSystemMap | InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (deploymentPipelineRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.SystemMap.Services) == 0 || len(snapshot.DevOpsAnalysis.Pipelines) > 0 {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (deploymentPipelineRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Deployment pipeline was not detected",
		Description: "Services exist, but no normalized CI/CD pipeline was detected.",
		Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete DevOps evidence contains no CI/CD pipelines.",
		RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodLikely,
		RiskSummary:    "Deployments may be inconsistent, unaudited, or difficult to reproduce.",
		Recommendation: "Automate build, validation, and deployment promotion.",
		Actions:        []string{"Add a delivery pipeline with validation gates and explicit environment promotion."},
	})}, nil
}

type deploymentGitOpsRule struct{}

func (deploymentGitOpsRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "deployment.gitops.not_detected", Version: doctorRuleVersion,
		Category: CategoryDeployment, RequiredInputs: InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (deploymentGitOpsRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.DevOpsAnalysis.KubernetesResources) == 0 ||
		len(snapshot.DevOpsAnalysis.GitOpsResources) > 0 {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (deploymentGitOpsRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "kubernetes", Title: "GitOps reconciliation was not detected",
		Description: "Kubernetes resources exist, but no normalized GitOps resource was detected.",
		Severity:    finding.SeverityMedium, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete DevOps evidence contains Kubernetes resources but no GitOps resources.",
		RiskLevel: finding.RiskMedium, Likelihood: finding.LikelihoodPossible,
		RiskSummary:    "Cluster state may drift from reviewed repository state.",
		Recommendation: "Use controlled reconciliation for Kubernetes environments.",
		Actions:        []string{"Adopt a GitOps controller or document an equivalent drift-controlled deployment mechanism."},
	})}, nil
}

type deploymentArtifactRule struct{}

func (deploymentArtifactRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "deployment.artifact.validation_failed", Version: doctorRuleVersion,
		Category: CategoryDeployment, RequiredInputs: InputArtifactValidation,
	}
}

func (deploymentArtifactRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	budget RuleBudget,
) ([]finding.Finding, error) {
	descriptor := (deploymentArtifactRule{}).Descriptor()
	findings := make([]finding.Finding, 0)
	for _, report := range snapshot.ValidationReports {
		for _, result := range report.Artifacts {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if result.Outcome != artifactvalidation.OutcomeInvalid {
				continue
			}
			if len(findings) >= budget.MaxFindings {
				return nil, ErrRuleBudgetExceeded
			}
			findings = append(findings, newFinding(descriptor, findingSpec{
				Key: result.Artifact.ID, Title: "DevOps artifact validation failed",
				Description: "An existing or generated DevOps artifact has normalized validation errors.",
				Severity:    finding.SeverityHigh, Confidence: finding.ConfidenceConfirmed,
				Subjects: []finding.Subject{{
					Kind: "artifact", ID: result.Artifact.ID, RepositoryID: result.Artifact.RepositoryID,
				}},
				Evidence:  "Artifact " + result.Artifact.Path + " failed validation with digest " + result.Digest + ".",
				RiskLevel: finding.RiskHigh, Likelihood: finding.LikelihoodAlmostCertain,
				RiskSummary:    "Using an invalid artifact can block or corrupt deployment.",
				Recommendation: "Correct the artifact before generation output is accepted or existing configuration is used.",
				Actions:        []string{"Resolve its normalized validation diagnostics and validate the exact digest again."},
			}))
		}
	}
	return findings, nil
}

type costEnvironmentScopeRule struct{}

func (costEnvironmentScopeRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "cost.infrastructure.environment_unassigned", Version: doctorRuleVersion,
		Category: CategoryCost, RequiredInputs: InputSystemMap,
	}
}

func (costEnvironmentScopeRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	budget RuleBudget,
) ([]finding.Finding, error) {
	descriptor := (costEnvironmentScopeRule{}).Descriptor()
	findings := make([]finding.Finding, 0)
	for _, infrastructure := range snapshot.SystemMap.Infrastructure {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(infrastructure.EnvironmentIDs) > 0 {
			continue
		}
		if len(findings) >= budget.MaxFindings {
			return nil, ErrRuleBudgetExceeded
		}
		findings = append(findings, newFinding(descriptor, findingSpec{
			Key: string(infrastructure.ID), Title: "Infrastructure has no environment scope",
			Description: "An infrastructure component is not assigned to any normalized environment.",
			Severity:    finding.SeverityMedium, Confidence: finding.ConfidenceConfirmed,
			Subjects: []finding.Subject{{
				Kind: "infrastructure", ID: string(infrastructure.ID),
				RepositoryID: string(infrastructure.RepositoryID),
			}},
			Evidence:  "The infrastructure record has an empty environment assignment.",
			RiskLevel: finding.RiskMedium, Likelihood: finding.LikelihoodLikely,
			RiskSummary:    "Unscoped resources are difficult to attribute, budget, and retire safely.",
			Recommendation: "Assign infrastructure to an explicit environment and cost boundary.",
			Actions:        []string{"Add evidence-backed environment assignment and ownership metadata."},
		}))
	}
	return findings, nil
}

type costInfrastructureAsCodeRule struct{}

func (costInfrastructureAsCodeRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "cost.infrastructure_as_code.not_detected", Version: doctorRuleVersion,
		Category:       CategoryCost,
		RequiredInputs: InputSystemMap | InputDevOpsAnalysis, RequiresComplete: true,
	}
}

func (costInfrastructureAsCodeRule) Evaluate(
	ctx context.Context,
	snapshot Snapshot,
	_ RuleBudget,
) ([]finding.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(snapshot.SystemMap.Infrastructure)+len(snapshot.SystemMap.ExternalResources) == 0 ||
		len(snapshot.DevOpsAnalysis.TerraformBlocks) > 0 {
		return make([]finding.Finding, 0), nil
	}
	descriptor := (costInfrastructureAsCodeRule{}).Descriptor()
	return []finding.Finding{newFinding(descriptor, findingSpec{
		Key: "system", Title: "Infrastructure-as-code configuration was not detected",
		Description: "Infrastructure or external resources exist, but no Terraform-compatible blocks were detected.",
		Severity:    finding.SeverityMedium, Confidence: finding.ConfidenceHigh,
		Subjects:  systemSubject(snapshot.ID),
		Evidence:  "Complete system and DevOps evidence contains infrastructure without Terraform-compatible blocks.",
		RiskLevel: finding.RiskMedium, Likelihood: finding.LikelihoodPossible,
		RiskSummary:    "Resource changes and decommissioning may be difficult to review, reproduce, and cost-control.",
		Recommendation: "Manage cost-bearing infrastructure through reviewed declarative configuration.",
		Actions:        []string{"Adopt Terraform-compatible configuration or document an equivalent declarative control plane."},
	})}, nil
}

type findingSpec struct {
	Key            string
	Title          string
	Description    string
	Severity       finding.Severity
	Confidence     finding.Confidence
	Subjects       []finding.Subject
	Evidence       string
	RiskLevel      finding.RiskLevel
	Likelihood     finding.Likelihood
	RiskSummary    string
	Recommendation string
	Actions        []string
}

func newFinding(descriptor RuleDescriptor, spec findingSpec) finding.Finding {
	digest := sha256.Sum256([]byte(spec.Key))
	return finding.Finding{
		ID:          descriptor.ID + "." + hex.EncodeToString(digest[:8]),
		RuleID:      descriptor.ID,
		Title:       spec.Title,
		Description: spec.Description,
		Severity:    spec.Severity,
		Confidence:  spec.Confidence,
		Subjects:    spec.Subjects,
		Evidence: []finding.Evidence{{
			Kind: finding.EvidenceObservation, Description: spec.Evidence,
		}},
		Provenance: finding.Provenance{
			Producer: doctorProducer, ProducerVersion: doctorProducerVersion,
			RuleVersion: descriptor.Version, Source: "normalized_snapshot",
		},
		Risk: finding.Risk{
			Level: spec.RiskLevel, Likelihood: spec.Likelihood, Summary: spec.RiskSummary,
		},
		Recommendation: finding.Recommendation{
			Summary: spec.Recommendation, Actions: spec.Actions,
		},
		Disposition: finding.DispositionActive,
	}.Normalized()
}

func systemSubject(snapshotID string) []finding.Subject {
	return []finding.Subject{{Kind: "system", ID: snapshotID}}
}

func isProductionEnvironment(environment systemmap.Environment) bool {
	kind := strings.ToLower(environment.Kind)
	name := strings.ToLower(environment.Name)
	identifier := strings.ToLower(string(environment.ID))
	return kind == "production" || kind == "prod" || name == "production" ||
		identifier == "production" || identifier == "prod"
}

func productionEnvironmentIDs(environments []systemmap.Environment) map[systemmap.EnvironmentID]struct{} {
	identifiers := make(map[systemmap.EnvironmentID]struct{})
	for _, environment := range environments {
		if isProductionEnvironment(environment) {
			identifiers[environment.ID] = struct{}{}
		}
	}
	return identifiers
}

func intersectsEnvironments(
	serviceEnvironments []systemmap.EnvironmentID,
	productionEnvironments map[systemmap.EnvironmentID]struct{},
) bool {
	for _, environmentID := range serviceEnvironments {
		if _, exists := productionEnvironments[environmentID]; exists {
			return true
		}
	}
	return false
}

func hasAlerting(snapshot Snapshot) bool {
	for _, resource := range snapshot.DevOpsAnalysis.ObservabilityResources {
		for _, signal := range resource.Signals {
			if signal == devopsanalysis.SignalAlerts {
				return true
			}
		}
	}
	return false
}

func hasBlockingSecurityControl(snapshot Snapshot) bool {
	for _, control := range snapshot.DevOpsAnalysis.SecurityControls {
		if control.Enforcement == devopsanalysis.EnforcementBlocking {
			return true
		}
	}
	return false
}

func isPinnedImage(image string) bool {
	if strings.Contains(image, "@sha256:") {
		return true
	}
	lastSlash := strings.LastIndexByte(image, '/')
	lastColon := strings.LastIndexByte(image, ':')
	return lastColon > lastSlash && !strings.EqualFold(image[lastColon+1:], "latest")
}
