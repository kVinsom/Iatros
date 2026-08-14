package doctor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kVinsom/Iatros/internal/artifactvalidation"
	"github.com/kVinsom/Iatros/internal/codeanalysis"
	"github.com/kVinsom/Iatros/internal/devopsanalysis"
	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/systemmap"
)

func TestDefaultAuditorReturnsReadyForCompleteEvidence(t *testing.T) {
	t.Parallel()

	auditor := newTestAuditor(t)
	report, err := auditor.Audit(t.Context(), readySnapshot(t), nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusReady || len(report.Findings.Findings) != 0 || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v, want ready without findings or diagnostics", report)
	}
	for _, coverage := range report.Coverage {
		if coverage.Status != CoverageEvaluated || coverage.RulesEvaluated == 0 || coverage.RulesSkipped != 0 {
			t.Fatalf("coverage = %#v, want fully evaluated", coverage)
		}
	}
	if err := report.ValidateWithin(DefaultLimits(), finding.DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
}

func TestDefaultAuditorFindsEveryAuditDomain(t *testing.T) {
	t.Parallel()

	snapshot := readySnapshot(t)
	snapshot.SystemMap.Environments = nil
	snapshot.SystemMap.Services[0].EnvironmentIDs = nil
	snapshot.SystemMap.Infrastructure[0].EnvironmentIDs = nil
	snapshot.SystemMap.Relationships[1].EnvironmentIDs = nil
	*snapshot.SystemMap = snapshot.SystemMap.Normalized()
	snapshot.DevOpsAnalysis.ObservabilityResources = nil
	snapshot.DevOpsAnalysis.SecurityControls = nil
	snapshot.DevOpsAnalysis.Pipelines = nil
	snapshot.DevOpsAnalysis.GitOpsResources = nil
	snapshot.DevOpsAnalysis.TerraformBlocks = nil
	snapshot.DevOpsAnalysis.ContainerBuilds[0].BaseImages = []string{"alpine:latest"}
	*snapshot.DevOpsAnalysis = snapshot.DevOpsAnalysis.Normalized()
	*snapshot.CodeAnalysis = codeModelWithSensitiveDefault()
	snapshot.ValidationReports = []artifactvalidation.Report{invalidArtifactReport(t)}

	auditor := newTestAuditor(t)
	report, err := auditor.Audit(t.Context(), snapshot, nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusNotReady {
		t.Fatalf("status = %q, want not_ready", report.Status)
	}
	wantedRules := []string{
		"production.environment.not_detected",
		"reliability.observability.not_detected",
		"reliability.alerting.not_detected",
		"performance.container_image.unpinned",
		"security.blocking_control.not_detected",
		"security.sensitive_environment.default_detected",
		"deployment.pipeline.not_detected",
		"deployment.gitops.not_detected",
		"deployment.artifact.validation_failed",
		"cost.infrastructure.environment_unassigned",
		"cost.infrastructure_as_code.not_detected",
	}
	for _, ruleID := range wantedRules {
		if !hasFindingRule(report.Findings.Findings, ruleID) {
			t.Errorf("findings do not contain rule %q; diagnostics = %#v; findings = %#v", ruleID, report.Diagnostics, report.Findings.Findings)
		}
	}
	for _, coverage := range report.Coverage {
		if coverage.Status != CoverageEvaluated {
			t.Fatalf("coverage = %#v, want evaluated", coverage)
		}
	}
}

func TestAuditorReportsUnavailableInputsInsteadOfFalseReadiness(t *testing.T) {
	t.Parallel()

	complete := readySnapshot(t)
	snapshot := Snapshot{ID: complete.ID, SystemMap: complete.SystemMap}
	auditor := newTestAuditor(t)
	report, err := auditor.Audit(t.Context(), snapshot, nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusPartial || len(report.Diagnostics) == 0 {
		t.Fatalf("report = %#v, want partial with diagnostics", report)
	}
	for _, category := range []Category{CategoryReliability, CategoryPerformance, CategorySecurity, CategoryDeployment} {
		coverage := coverageFor(report.Coverage, category)
		if coverage.Status == CoverageEvaluated {
			t.Fatalf("coverage %q = %#v, want incomplete", category, coverage)
		}
	}
	if hasFindingRule(report.Findings.Findings, "reliability.observability.not_detected") ||
		hasFindingRule(report.Findings.Findings, "deployment.pipeline.not_detected") {
		t.Fatalf("absence findings were emitted without required evidence: %#v", report.Findings.Findings)
	}
}

func TestAuditorAppliesControlledExclusion(t *testing.T) {
	t.Parallel()

	snapshot := readySnapshot(t)
	snapshot.DevOpsAnalysis.ContainerBuilds[0].BaseImages = []string{"alpine:latest"}
	*snapshot.DevOpsAnalysis = snapshot.DevOpsAnalysis.Normalized()
	auditor := newTestAuditor(t)
	evaluatedAt := testEvaluationTime()
	initial, err := auditor.Audit(t.Context(), snapshot, nil, evaluatedAt)
	if err != nil {
		t.Fatalf("initial Audit() error = %v", err)
	}
	var target finding.Finding
	for _, auditFinding := range initial.Findings.Findings {
		if auditFinding.RuleID == "performance.container_image.unpinned" {
			target = auditFinding
			break
		}
	}
	if target.ID == "" {
		t.Fatal("initial report did not contain image finding")
	}
	exclusion := finding.Exclusion{
		ID: "doctor.image.accepted", FindingID: target.ID,
		Reason: "Temporary compatibility requirement.", RequestedBy: "platform-team",
		ApprovedBy: "security-team", CreatedAt: evaluatedAt.Add(-time.Hour),
		ExpiresAt: evaluatedAt.Add(24 * time.Hour),
	}
	report, err := auditor.Audit(t.Context(), snapshot, []finding.Exclusion{exclusion}, evaluatedAt)
	if err != nil {
		t.Fatalf("Audit(exclusion) error = %v", err)
	}
	for _, auditFinding := range report.Findings.Findings {
		if auditFinding.ID == target.ID &&
			(auditFinding.Disposition != finding.DispositionExcluded || auditFinding.Exclusion == nil) {
			t.Fatalf("finding = %#v, want controlled exclusion", auditFinding)
		}
	}
	if report.Status != StatusReady {
		t.Fatalf("status = %q, want ready after only active finding was excluded", report.Status)
	}
}

func TestAuditorIsolatesRuleFailureAndRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	findingLimits := finding.DefaultLimits()
	rules := append(BuiltInRules(), failingRule{})
	auditor, err := NewAuditor(limits, findingLimits, rules...)
	if err != nil {
		t.Fatalf("NewAuditor() error = %v", err)
	}
	report, err := auditor.Audit(t.Context(), readySnapshot(t), nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusPartial || !hasDoctorDiagnostic(report.Diagnostics, "DOCTOR_RULE_FAILED") {
		t.Fatalf("report = %#v, want isolated rule failure", report)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "DOCTOR_RULE_FAILED" && diagnostic.Message != "The Doctor rule could not complete." {
			t.Fatalf("diagnostic leaked rule error: %#v", diagnostic)
		}
	}

	if _, err := NewAuditor(limits, findingLimits, productionEnvironmentRule{}); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("NewAuditor(incomplete categories) error = %v, want ErrInvalidRule", err)
	}
	duplicates := append(BuiltInRules(), productionEnvironmentRule{})
	if _, err := NewAuditor(limits, findingLimits, duplicates...); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("NewAuditor(duplicate) error = %v, want ErrInvalidRule", err)
	}
	withNil := append(BuiltInRules(), nil)
	if _, err := NewAuditor(limits, findingLimits, withNil...); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("NewAuditor(nil) error = %v, want ErrInvalidRule", err)
	}
}

func TestAuditorNormalizesMalformedOutputAndDiagnosticOverflow(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxDiagnostics = 1
	rules := append(BuiltInRules(), malformedRule{}, failingRule{})
	auditor, err := NewAuditor(limits, finding.DefaultLimits(), rules...)
	if err != nil {
		t.Fatalf("NewAuditor() error = %v", err)
	}
	report, err := auditor.Audit(t.Context(), readySnapshot(t), nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusPartial || len(report.Diagnostics) != 1 ||
		report.Diagnostics[0].Code != "DOCTOR_DIAGNOSTICS_TRUNCATED" {
		t.Fatalf("report = %#v, want bounded explicit diagnostic truncation", report)
	}
}

func TestAuditorRejectsExcessiveSnapshotBeforeNormalization(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxInputFacts = 1
	auditor, err := NewDefaultAuditor(limits, finding.DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultAuditor() error = %v", err)
	}
	if _, err := auditor.Audit(t.Context(), readySnapshot(t), nil, testEvaluationTime()); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Audit(excessive) error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestAuditorBoundsFindingsWhileRulesEvaluate(t *testing.T) {
	t.Parallel()

	snapshot := readySnapshot(t)
	snapshot.DevOpsAnalysis.ContainerBuilds[0].BaseImages = []string{"alpine:latest", "busybox:latest"}
	*snapshot.DevOpsAnalysis = snapshot.DevOpsAnalysis.Normalized()
	findingLimits := finding.DefaultLimits()
	findingLimits.MaxFindings = 1
	limits := DefaultLimits()
	limits.MaxRepositoryFindings = 1
	auditor, err := NewDefaultAuditor(limits, findingLimits)
	if err != nil {
		t.Fatalf("NewDefaultAuditor() error = %v", err)
	}
	report, err := auditor.Audit(t.Context(), snapshot, nil, testEvaluationTime())
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if report.Status != StatusPartial ||
		!hasDoctorDiagnostic(report.Diagnostics, "DOCTOR_FINDINGS_LIMIT_REACHED") ||
		len(report.Findings.Findings) != 0 {
		t.Fatalf("report = %#v, want bounded partial result", report)
	}
}

func TestAuditorHonorsCancellationAndRejectsMalformedSnapshots(t *testing.T) {
	t.Parallel()

	auditor := newTestAuditor(t)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := auditor.Audit(cancelled, readySnapshot(t), nil, testEvaluationTime()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Audit(cancelled) error = %v, want context cancellation", err)
	}
	invalid := readySnapshot(t)
	invalid.ID = "Unsafe ID"
	if _, err := auditor.Audit(t.Context(), invalid, nil, testEvaluationTime()); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Audit(invalid) error = %v, want ErrInvalidSnapshot", err)
	}
}

func newTestAuditor(t testing.TB) *Auditor {
	t.Helper()
	auditor, err := NewDefaultAuditor(DefaultLimits(), finding.DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultAuditor() error = %v", err)
	}
	return auditor
}

func readySnapshot(t testing.TB) Snapshot {
	t.Helper()
	codeModel := codeanalysis.Model{
		SchemaVersion: codeanalysis.CurrentSchemaVersion,
		Diagnostics:   []codeanalysis.Diagnostic{},
	}.Normalized()
	devOpsModel := readyDevOpsModel()
	systemModel := readySystemMap()
	return Snapshot{
		ID: "checkout-system", CodeAnalysis: &codeModel, DevOpsAnalysis: &devOpsModel,
		SystemMap: &systemModel, ValidationReports: []artifactvalidation.Report{validArtifactReport(t)},
		RepositoryFindings: []finding.Finding{},
	}
}

func readyDevOpsModel() devopsanalysis.Model {
	return devopsanalysis.Model{
		SchemaVersion: devopsanalysis.CurrentSchemaVersion,
		Tools: []devopsanalysis.Tool{
			{ID: "argo-cd", Name: "Argo CD", Category: devopsanalysis.CategoryGitOps,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("deploy/application.yaml")},
			{ID: "docker", Name: "Docker", Category: devopsanalysis.CategoryContainer,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("Dockerfile")},
			{ID: "github-actions", Name: "GitHub Actions", Category: devopsanalysis.CategoryCICD,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence(".github/workflows/ci.yml")},
			{ID: "kubernetes", Name: "Kubernetes", Category: devopsanalysis.CategoryOrchestration,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("deploy/api.yaml")},
			{ID: "prometheus", Name: "Prometheus", Category: devopsanalysis.CategoryObservability,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("monitoring/alerts.yaml")},
			{ID: "terraform", Name: "Terraform", Category: devopsanalysis.CategoryInfrastructureAsCode,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("infra/main.tf")},
			{ID: "trivy", Name: "Trivy", Category: devopsanalysis.CategorySecurity,
				Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("security/trivy.yaml")},
		},
		ContainerBuilds: []devopsanalysis.ContainerBuild{{
			ID: "api-image", ToolID: "docker", BaseImages: []string{"alpine:3.20"},
			BuildArguments: []string{}, ExposedPorts: []uint16{8080},
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("Dockerfile"),
		}},
		KubernetesResources: []devopsanalysis.KubernetesResource{{
			ID: "api-deployment", ToolID: "kubernetes", APIVersion: "apps/v1",
			Kind: "Deployment", Name: "api", Namespace: "production",
			Images: []string{"registry.example/api:1.0.0"}, Ports: []uint16{8080},
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("deploy/api.yaml"),
		}},
		TerraformBlocks: []devopsanalysis.TerraformBlock{{
			ID: "cluster", ToolID: "terraform", Kind: devopsanalysis.TerraformBlockResource,
			Type: "test_cluster", Name: "production", Provider: "example/cloud",
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("infra/main.tf"),
		}},
		Pipelines: []devopsanalysis.Pipeline{{
			ID: "delivery", ToolID: "github-actions", Name: "Delivery", Triggers: []string{"push"},
			Jobs: []devopsanalysis.PipelineJob{{
				ID: "deploy", Name: "Deploy", Needs: []string{}, Environment: "production",
				Evidence: devOpsEvidence(".github/workflows/ci.yml"),
			}},
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence(".github/workflows/ci.yml"),
		}},
		GitOpsResources: []devopsanalysis.GitOpsResource{{
			ID: "api-application", ToolID: "argo-cd", Kind: "application", Name: "api",
			SourcePath: "deploy", TargetNamespace: "production",
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("deploy/application.yaml"),
		}},
		ObservabilityResources: []devopsanalysis.ObservabilityResource{{
			ID: "api-alerts", ToolID: "prometheus", Kind: "alert-rules", Name: "API alerts",
			Signals:   []devopsanalysis.Signal{devopsanalysis.SignalAlerts, devopsanalysis.SignalMetrics},
			Certainty: devopsanalysis.CertaintyObserved, Evidence: devOpsEvidence("monitoring/alerts.yaml"),
		}},
		SecurityControls: []devopsanalysis.SecurityControl{{
			ID: "image-scan", ToolID: "trivy", Kind: "vulnerability-scan", Name: "Image scan",
			Enforcement: devopsanalysis.EnforcementBlocking, Certainty: devopsanalysis.CertaintyObserved,
			Evidence: devOpsEvidence("security/trivy.yaml"),
		}},
		Diagnostics: []devopsanalysis.Diagnostic{},
	}.Normalized()
}

func devOpsEvidence(path string) []devopsanalysis.Evidence {
	return []devopsanalysis.Evidence{{Kind: devopsanalysis.EvidenceConfiguration, Path: path}}
}

func readySystemMap() systemmap.Model {
	return systemmap.Model{
		SchemaVersion: systemmap.CurrentSchemaVersion,
		ID:            "checkout-system", Name: "Checkout System",
		Repositories: []systemmap.Repository{{
			ID: "api-repo", Name: "API Repository", Source: systemmap.RepositorySourceLocal,
			Locator: ".", Evidence: []systemmap.Evidence{{
				RepositoryID: "api-repo", Kind: systemmap.EvidenceTarget, Path: ".",
			}},
		}},
		Services: []systemmap.Service{{
			ID: "checkout-api", RepositoryID: "api-repo", Name: "Checkout API", Kind: "api", Root: ".",
			EnvironmentIDs: []systemmap.EnvironmentID{"production"},
			Evidence: []systemmap.Evidence{{
				RepositoryID: "api-repo", Kind: systemmap.EvidenceSource, Path: "cmd/api/main.go",
			}},
		}},
		Infrastructure: []systemmap.Infrastructure{{
			ID: "production-cluster", RepositoryID: "api-repo", Name: "Production Cluster",
			Kind: "cluster", Technology: "kubernetes", Root: "infra",
			EnvironmentIDs: []systemmap.EnvironmentID{"production"},
			Evidence: []systemmap.Evidence{{
				RepositoryID: "api-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/main.tf",
			}},
		}},
		Environments: []systemmap.Environment{{
			ID: "production", Name: "Production", Kind: "production",
			Evidence: []systemmap.Evidence{{
				RepositoryID: "api-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/main.tf",
			}},
		}},
		Owners: []systemmap.Owner{{
			ID: "platform-team", Kind: systemmap.OwnerTeam, Name: "Platform Team", Reference: "@platform",
			Evidence: []systemmap.Evidence{{
				RepositoryID: "api-repo", Kind: systemmap.EvidenceOwnership, Path: ".github/CODEOWNERS",
			}},
		}},
		Relationships: []systemmap.Relationship{
			{
				ID: "owns-api", Source: systemmap.EntityReference{Kind: systemmap.EntityOwner, ID: "platform-team"},
				Target: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "checkout-api"}, Kind: "owns",
				Evidence: []systemmap.Evidence{{
					RepositoryID: "api-repo", Kind: systemmap.EvidenceOwnership, Path: ".github/CODEOWNERS",
				}},
			},
			{
				ID: "runs-on-cluster", Source: systemmap.EntityReference{Kind: systemmap.EntityService, ID: "checkout-api"},
				Target: systemmap.EntityReference{Kind: systemmap.EntityInfrastructure, ID: "production-cluster"},
				Kind:   "runs_on", EnvironmentIDs: []systemmap.EnvironmentID{"production"},
				Evidence: []systemmap.Evidence{{
					RepositoryID: "api-repo", Kind: systemmap.EvidenceConfiguration, Path: "infra/main.tf",
				}},
			},
		},
		Diagnostics: []systemmap.Diagnostic{},
	}.Normalized()
}

func codeModelWithSensitiveDefault() codeanalysis.Model {
	return codeanalysis.Model{
		SchemaVersion: codeanalysis.CurrentSchemaVersion,
		Services: []codeanalysis.Service{{
			ID: "checkout-api", Name: "Checkout API", Kind: "api", Root: ".",
			Entrypoints: []string{"cmd/api/main.go"}, Certainty: codeanalysis.CertaintyObserved,
			Evidence: []codeanalysis.Evidence{{Kind: codeanalysis.EvidenceSource, Path: "cmd/api/main.go"}},
		}},
		EnvironmentVariables: []codeanalysis.EnvironmentVariable{{
			ServiceID: "checkout-api", Name: "API_TOKEN", HasDefault: true,
			IsSensitive: true, Certainty: codeanalysis.CertaintyObserved,
			Evidence: []codeanalysis.Evidence{{Kind: codeanalysis.EvidenceSource, Path: "internal/config.go"}},
		}},
		Diagnostics: []codeanalysis.Diagnostic{},
	}.Normalized()
}

func validArtifactReport(t testing.TB) artifactvalidation.Report {
	t.Helper()
	return artifactReport(t, []byte(`{"valid":true}`))
}

func invalidArtifactReport(t testing.TB) artifactvalidation.Report {
	t.Helper()
	return artifactReport(t, []byte(`{"valid":`))
}

func artifactReport(t testing.TB, content []byte) artifactvalidation.Report {
	t.Helper()
	engine, err := artifactvalidation.NewDefaultEngine(artifactvalidation.DefaultLimits())
	if err != nil {
		t.Fatalf("NewDefaultEngine() error = %v", err)
	}
	source := artifactvalidation.NewMemorySource(map[string][]byte{"deployment": content})
	report, err := engine.Validate(t.Context(), []artifactvalidation.Request{{
		Artifact: artifactvalidation.Artifact{
			ID: "deployment", RepositoryID: "api-repo", Path: "deploy/config.json",
			Origin: artifactvalidation.OriginGenerated, Kind: artifactvalidation.KindGeneric,
			Format: artifactvalidation.FormatJSON, Producer: "iatros.test", ProducerVersion: "1.0",
		},
		Source: source,
	}})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	return report
}

func hasFindingRule(findings []finding.Finding, ruleID string) bool {
	for _, auditFinding := range findings {
		if auditFinding.RuleID == ruleID {
			return true
		}
	}
	return false
}

func coverageFor(coverage []Coverage, category Category) Coverage {
	for _, categoryCoverage := range coverage {
		if categoryCoverage.Category == category {
			return categoryCoverage
		}
	}
	return Coverage{}
}

func hasDoctorDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func testEvaluationTime() time.Time {
	return time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
}

type failingRule struct{}

func (failingRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "test.production.failure", Version: "1.0", Category: CategoryProduction,
		RequiredInputs: InputSystemMap,
	}
}

func (failingRule) Evaluate(context.Context, Snapshot, RuleBudget) ([]finding.Finding, error) {
	return nil, errors.New("secret rule failure")
}

type malformedRule struct{}

func (malformedRule) Descriptor() RuleDescriptor {
	return RuleDescriptor{
		ID: "test.security.malformed", Version: "1.0", Category: CategorySecurity,
		RequiredInputs: InputCodeAnalysis,
	}
}

func (malformedRule) Evaluate(context.Context, Snapshot, RuleBudget) ([]finding.Finding, error) {
	return []finding.Finding{{RuleID: "test.security.malformed"}}, nil
}
