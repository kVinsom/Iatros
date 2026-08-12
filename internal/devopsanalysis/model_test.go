package devopsanalysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestModelAcceptsCompleteDevOpsStack(t *testing.T) {
	t.Parallel()

	model := validModel()
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := model.ValidateWithin(DefaultLimits()); err != nil {
		t.Fatalf("ValidateWithin() error = %v", err)
	}
}

func TestModelRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Model)
	}{
		{name: "schema", mutate: func(model *Model) { model.SchemaVersion = "2.0" }},
		{name: "tool order", mutate: func(model *Model) {
			model.Tools[0], model.Tools[1] = model.Tools[1], model.Tools[0]
		}},
		{name: "tool category", mutate: func(model *Model) { model.Tools[0].Category = "unknown" }},
		{name: "unsafe evidence", mutate: func(model *Model) {
			model.Tools[0].Evidence[0].Path = "../outside"
		}},
		{name: "partial evidence position", mutate: func(model *Model) {
			model.Tools[0].Evidence[0].StartLine = 1
		}},
		{name: "reversed evidence position", mutate: func(model *Model) {
			model.Tools[0].Evidence[0].StartLine = 2
			model.Tools[0].Evidence[0].StartColumn = 1
			model.Tools[0].Evidence[0].EndLine = 1
			model.Tools[0].Evidence[0].EndColumn = 1
		}},
		{name: "unknown evidence kind", mutate: func(model *Model) {
			model.Tools[0].Evidence[0].Kind = "runtime"
		}},
		{name: "duplicate evidence", mutate: func(model *Model) {
			model.Tools[0].Evidence = append(model.Tools[0].Evidence, model.Tools[0].Evidence[0])
		}},
		{name: "unknown container tool", mutate: func(model *Model) {
			model.ContainerBuilds[0].ToolID = "missing"
		}},
		{name: "semantic fact with filename evidence only", mutate: func(model *Model) {
			model.ContainerBuilds[0].Evidence[0].Kind = EvidenceFilename
		}},
		{name: "wrong container tool category", mutate: func(model *Model) {
			model.ContainerBuilds[0].ToolID = "helm"
		}},
		{name: "container argument order", mutate: func(model *Model) {
			model.ContainerBuilds[0].BuildArguments = []string{"TOKEN", "PORT"}
		}},
		{name: "zero container port", mutate: func(model *Model) {
			model.ContainerBuilds[0].ExposedPorts[0] = 0
		}},
		{name: "unsafe Compose context", mutate: func(model *Model) {
			model.ComposeServices[0].BuildContext = "../api"
		}},
		{name: "invalid Compose environment", mutate: func(model *Model) {
			model.ComposeServices[0].EnvironmentVariables[0] = "DATABASE-URL"
		}},
		{name: "Kubernetes tool category", mutate: func(model *Model) {
			model.KubernetesResources[0].ToolID = "docker"
		}},
		{name: "unsafe Helm root", mutate: func(model *Model) { model.HelmCharts[0].Root = "../chart" }},
		{name: "Terraform resource without type", mutate: func(model *Model) {
			model.TerraformBlocks[0].Type = ""
		}},
		{name: "Terraform variable with type", mutate: func(model *Model) {
			model.TerraformBlocks[0].Kind = TerraformBlockVariable
		}},
		{name: "pipeline job order", mutate: func(model *Model) {
			model.Pipelines[0].Jobs[0], model.Pipelines[0].Jobs[1] =
				model.Pipelines[0].Jobs[1], model.Pipelines[0].Jobs[0]
		}},
		{name: "unknown pipeline dependency", mutate: func(model *Model) {
			model.Pipelines[0].Jobs[1].Needs[0] = "missing"
		}},
		{name: "pipeline self dependency", mutate: func(model *Model) {
			model.Pipelines[0].Jobs[1].Needs[0] = "deploy"
		}},
		{name: "pipeline dependency cycle", mutate: func(model *Model) {
			model.Pipelines[0].Jobs[0].Needs = []string{"deploy"}
		}},
		{name: "unsafe GitOps source", mutate: func(model *Model) {
			model.GitOpsResources[0].SourcePath = "../../clusters"
		}},
		{name: "unknown signal", mutate: func(model *Model) {
			model.ObservabilityResources[0].Signals[0] = "events"
		}},
		{name: "security tool category", mutate: func(model *Model) {
			model.SecurityControls[0].ToolID = "prometheus"
		}},
		{name: "security enforcement", mutate: func(model *Model) {
			model.SecurityControls[0].Enforcement = "mandatory"
		}},
		{name: "control character in text", mutate: func(model *Model) {
			model.SecurityControls[0].Name = "Container\x00scan"
		}},
		{name: "partial without diagnostic", mutate: func(model *Model) { model.Partial = true }},
		{name: "warning on complete model", mutate: func(model *Model) {
			model.Diagnostics = limitingDiagnostics()
		}},
		{name: "partial with information only", mutate: func(model *Model) {
			model.Partial = true
			model.Diagnostics = []Diagnostic{{
				Code: "IATROS_DEVOPS_NOTE", Level: DiagnosticInfo, Path: ".", Message: "Analysis note.",
			}}
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			testCase.mutate(&model)
			if err := model.Validate(); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("Validate() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelAcceptsEveryTerraformBlockKind(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		kind      TerraformBlockKind
		blockType string
	}{
		{kind: TerraformBlockProvider},
		{kind: TerraformBlockModule},
		{kind: TerraformBlockResource, blockType: "aws_instance"},
		{kind: TerraformBlockData, blockType: "aws_ami"},
		{kind: TerraformBlockVariable},
		{kind: TerraformBlockOutput},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.kind), func(t *testing.T) {
			t.Parallel()

			model := validModel()
			model.TerraformBlocks[0].Kind = testCase.kind
			model.TerraformBlocks[0].Type = testCase.blockType
			if err := model.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestModelAcceptsSecretManagementControl(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Tools = append(model.Tools, Tool{
		ID: "vault", Name: "Vault", Category: CategorySecrets, Certainty: CertaintyObserved,
		Evidence: []Evidence{{Kind: EvidenceFilename, Path: "security/vault.hcl"}},
	})
	model.SecurityControls[0].ToolID = "vault"
	model = model.Normalized()
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelAcceptsExplainedPartialResult(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Partial = true
	model.Diagnostics = limitingDiagnostics()
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPipelineValidationAcceptsAcyclicForwardReferences(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Pipelines[0].Jobs = []PipelineJob{
		{ID: "build", Name: "Build", Needs: []string{"test"},
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}},
		{ID: "deploy", Name: "Deploy", Needs: []string{"build"},
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}},
		{ID: "test", Name: "Test", Needs: []string{},
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPipelineValidationHandlesDeepAcyclicGraphIteratively(t *testing.T) {
	t.Parallel()

	const jobCount = 5_000
	jobs := make([]PipelineJob, jobCount)
	evidence := []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}
	for index := range jobs {
		jobID := fmt.Sprintf("job-%04d", index)
		jobs[index] = PipelineJob{ID: jobID, Name: jobID, Needs: []string{}, Evidence: evidence}
		if index > 0 {
			jobs[index].Needs = []string{jobs[index-1].ID}
		}
	}

	model := Model{
		SchemaVersion: CurrentSchemaVersion,
		Tools: []Tool{{
			ID: "github-actions", Name: "GitHub Actions", Category: CategoryCICD,
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceFilename, Path: ".github/workflows/ci.yml"}},
		}},
		Pipelines: []Pipeline{{
			ID: "ci", ToolID: "github-actions", Name: "CI", Triggers: []string{}, Jobs: jobs,
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}},
		}},
	}.Normalized()
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelNormalizedOrdersDeduplicatesAndDetaches(t *testing.T) {
	t.Parallel()

	model := validModel()
	slices.Reverse(model.Tools)
	model.ContainerBuilds[0].BaseImages = []string{"golang:1.25", "alpine:3", "alpine:3"}
	model.ComposeServices[0].Profiles = []string{"production", "default", "default"}
	model.KubernetesResources[0].Images = []string{"worker:latest", "api:latest", "api:latest"}
	model.HelmCharts[0].Dependencies = []string{"redis", "postgresql", "postgresql"}
	model.Pipelines[0].Triggers = []string{"push", "pull_request", "push"}
	model.Pipelines[0].Jobs[0], model.Pipelines[0].Jobs[1] =
		model.Pipelines[0].Jobs[1], model.Pipelines[0].Jobs[0]
	model.ObservabilityResources[0].Signals = []Signal{SignalTraces, SignalMetrics, SignalMetrics}

	normalized := model.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.Tools[0].ID != "argo-cd" || normalized.ContainerBuilds[0].BaseImages[0] != "alpine:3" ||
		len(normalized.ContainerBuilds[0].BaseImages) != 2 || normalized.Pipelines[0].Jobs[0].ID != "build" ||
		normalized.ObservabilityResources[0].Signals[0] != SignalMetrics {
		t.Fatalf("Normalized() did not produce canonical ordering: %#v", normalized)
	}

	normalized.ContainerBuilds[0].BaseImages[0] = "changed"
	normalized.Pipelines[0].Jobs[1].Needs[0] = "changed"
	if model.ContainerBuilds[0].BaseImages[0] == "changed" ||
		slices.Contains(model.Pipelines[0].Jobs[0].Needs, "changed") {
		t.Fatal("Normalized() retained caller-owned nested storage")
	}
}

func TestModelNormalizedOrdersEveryFactCollection(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.ContainerBuilds = reversedWithLaterCopy(model.ContainerBuilds, func(build *ContainerBuild) {
		build.ID = "worker-image"
	})
	model.ComposeServices = reversedWithLaterCopy(model.ComposeServices, func(service *ComposeService) {
		service.ID = "worker-compose"
	})
	model.KubernetesResources = reversedWithLaterCopy(
		model.KubernetesResources,
		func(resource *KubernetesResource) { resource.ID = "worker-deployment" },
	)
	model.HelmCharts = reversedWithLaterCopy(model.HelmCharts, func(chart *HelmChart) {
		chart.ID = "worker-chart"
	})
	model.TerraformBlocks = reversedWithLaterCopy(model.TerraformBlocks, func(block *TerraformBlock) {
		block.ID = "worker-instance"
	})
	model.Pipelines = reversedWithLaterCopy(model.Pipelines, func(pipeline *Pipeline) {
		pipeline.ID = "release"
	})
	model.GitOpsResources = reversedWithLaterCopy(model.GitOpsResources, func(resource *GitOpsResource) {
		resource.ID = "worker-application"
	})
	model.ObservabilityResources = reversedWithLaterCopy(
		model.ObservabilityResources,
		func(resource *ObservabilityResource) { resource.ID = "worker-alerts" },
	)
	model.SecurityControls = reversedWithLaterCopy(model.SecurityControls, func(control *SecurityControl) {
		control.ID = "dependency-scan"
	})
	model.Diagnostics = []Diagnostic{
		{Code: "IATROS_DEVOPS_NOTE_Z", Level: DiagnosticInfo, Path: ".", Message: "Last note."},
		{Code: "IATROS_DEVOPS_NOTE_A", Level: DiagnosticInfo, Path: ".", Message: "First note."},
	}

	normalized := model.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.ContainerBuilds[0].ID != "api-image" ||
		normalized.ComposeServices[0].ID != "api-compose" ||
		normalized.KubernetesResources[0].ID != "api-deployment" ||
		normalized.HelmCharts[0].ID != "api-chart" ||
		normalized.TerraformBlocks[0].ID != "api-instance" ||
		normalized.Pipelines[0].ID != "ci" || normalized.GitOpsResources[0].ID != "api-application" ||
		normalized.ObservabilityResources[0].ID != "api-alerts" ||
		normalized.SecurityControls[0].ID != "container-scan" ||
		normalized.Diagnostics[0].Code != "IATROS_DEVOPS_NOTE_A" {
		t.Fatalf("Normalized() did not order every fact collection: %#v", normalized)
	}
}

func TestModelNormalizedOrdersAndDeduplicatesEvidenceAndDiagnostics(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.ContainerBuilds[0].Evidence = []Evidence{
		{Kind: EvidenceConfiguration, Path: "Dockerfile", StartLine: 2, StartColumn: 1, EndLine: 3, EndColumn: 1},
		{Kind: EvidenceConfiguration, Path: "Dockerfile", StartLine: 2, StartColumn: 1, EndLine: 2, EndColumn: 4},
		{Kind: EvidenceConfiguration, Path: "Dockerfile", StartLine: 2, StartColumn: 1, EndLine: 2, EndColumn: 3},
		{Kind: EvidenceConfiguration, Path: "Dockerfile", StartLine: 2, StartColumn: 2, EndLine: 2, EndColumn: 2},
		{Kind: EvidenceFilename, Path: "Dockerfile"},
		{Kind: EvidenceConfiguration, Path: "Dockerfile", StartLine: 2, StartColumn: 1, EndLine: 2, EndColumn: 3},
	}
	model.Partial = true
	model.Diagnostics = []Diagnostic{
		{Code: "IATROS_DEVOPS_LIMIT", Level: DiagnosticWarning, Path: ".", Message: "Analysis limited."},
		{Code: "IATROS_DEVOPS_LIMIT", Level: DiagnosticInfo, Path: ".", Message: "Analysis limited."},
		{Code: "IATROS_DEVOPS_LIMIT", Level: DiagnosticError, Path: ".", Message: "Analysis limited."},
	}

	normalized := model.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(normalized.ContainerBuilds[0].Evidence) != 5 ||
		normalized.ContainerBuilds[0].Evidence[0].Kind != EvidenceConfiguration ||
		normalized.ContainerBuilds[0].Evidence[0].EndColumn != 3 ||
		normalized.Diagnostics[0].Level != DiagnosticError ||
		normalized.Diagnostics[2].Level != DiagnosticWarning {
		t.Fatalf("Normalized() ordering = %#v", normalized)
	}
}

func TestModelNormalizedInitializesEveryCollection(t *testing.T) {
	t.Parallel()

	model := (Model{SchemaVersion: CurrentSchemaVersion}).Normalized()
	if model.Tools == nil || model.ContainerBuilds == nil || model.ComposeServices == nil ||
		model.KubernetesResources == nil || model.HelmCharts == nil || model.TerraformBlocks == nil ||
		model.Pipelines == nil || model.GitOpsResources == nil || model.ObservabilityResources == nil ||
		model.SecurityControls == nil || model.Diagnostics == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestModelJSONRoundTripPreservesContract(t *testing.T) {
	t.Parallel()

	want := validModel()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Model
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded model = %#v, want %#v", decoded, want)
	}
}

func TestModelValidateWithinEnforcesRetentionLimits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Model, *Limits)
	}{
		{name: "tools", mutate: func(model *Model, limits *Limits) {
			model.Tools = append(model.Tools, model.Tools[0])
			limits.MaxTools = len(model.Tools) - 1
		}},
		{name: "container builds", mutate: func(model *Model, limits *Limits) {
			model.ContainerBuilds = append(model.ContainerBuilds, model.ContainerBuilds[0])
			limits.MaxContainerBuilds = 1
		}},
		{name: "Compose services", mutate: func(model *Model, limits *Limits) {
			model.ComposeServices = append(model.ComposeServices, model.ComposeServices[0])
			limits.MaxComposeServices = 1
		}},
		{name: "Kubernetes resources", mutate: func(model *Model, limits *Limits) {
			model.KubernetesResources = append(model.KubernetesResources, model.KubernetesResources[0])
			limits.MaxKubernetesResources = 1
		}},
		{name: "Helm charts", mutate: func(model *Model, limits *Limits) {
			model.HelmCharts = append(model.HelmCharts, model.HelmCharts[0])
			limits.MaxHelmCharts = 1
		}},
		{name: "Terraform blocks", mutate: func(model *Model, limits *Limits) {
			model.TerraformBlocks = append(model.TerraformBlocks, model.TerraformBlocks[0])
			limits.MaxTerraformBlocks = 1
		}},
		{name: "pipelines", mutate: func(model *Model, limits *Limits) {
			model.Pipelines = append(model.Pipelines, model.Pipelines[0])
			limits.MaxPipelines = 1
		}},
		{name: "pipeline jobs", mutate: func(_ *Model, limits *Limits) { limits.MaxJobsPerPipeline = 1 }},
		{name: "pipeline job dependencies", mutate: func(model *Model, limits *Limits) {
			model.Pipelines[0].Jobs = append(model.Pipelines[0].Jobs, PipelineJob{
				ID: "release", Name: "Release", Needs: []string{"build", "deploy"},
				Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}},
			})
			limits.MaxJobDependenciesPerPipeline = 2
		}},
		{name: "GitOps resources", mutate: func(model *Model, limits *Limits) {
			model.GitOpsResources = append(model.GitOpsResources, model.GitOpsResources[0])
			limits.MaxGitOpsResources = 1
		}},
		{name: "observability resources", mutate: func(model *Model, limits *Limits) {
			model.ObservabilityResources = append(
				model.ObservabilityResources,
				model.ObservabilityResources[0],
			)
			limits.MaxObservabilityResources = 1
		}},
		{name: "security controls", mutate: func(model *Model, limits *Limits) {
			model.SecurityControls = append(model.SecurityControls, model.SecurityControls[0])
			limits.MaxSecurityControls = 1
		}},
		{name: "nested values", mutate: func(_ *Model, limits *Limits) { limits.MaxValuesPerFact = 1 }},
		{name: "evidence", mutate: func(model *Model, limits *Limits) {
			model.Tools[0].Evidence = append(model.Tools[0].Evidence, Evidence{
				Kind: EvidenceConfiguration, Path: "clusters/production/application.yaml",
			})
			limits.MaxEvidencePerFact = 1
		}},
		{name: "text", mutate: func(model *Model, limits *Limits) {
			model.ContainerBuilds[0].Target = "target-too-long"
			limits.MaxTextBytes = 8
		}},
		{name: "diagnostics", mutate: func(model *Model, limits *Limits) {
			model.Diagnostics = []Diagnostic{
				{Code: "IATROS_DEVOPS_NOTE_A", Level: DiagnosticInfo, Path: ".", Message: "First note."},
				{Code: "IATROS_DEVOPS_NOTE_B", Level: DiagnosticInfo, Path: ".", Message: "Second note."},
			}
			limits.MaxDiagnostics = 1
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			limits := DefaultLimits()
			testCase.mutate(&model, &limits)
			if err := model.ValidateWithin(limits); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("ValidateWithin() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateWithinChecksTextBudgetsForEveryFactCategory(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Model, string)
	}{
		{name: "tool", mutate: func(model *Model, oversized string) { model.Tools[0].Name = oversized }},
		{name: "container", mutate: func(model *Model, oversized string) {
			model.ContainerBuilds[0].Target = oversized
		}},
		{name: "Compose", mutate: func(model *Model, oversized string) {
			model.ComposeServices[0].Image = oversized
		}},
		{name: "Kubernetes", mutate: func(model *Model, oversized string) {
			model.KubernetesResources[0].Name = oversized
		}},
		{name: "Helm", mutate: func(model *Model, oversized string) { model.HelmCharts[0].Name = oversized }},
		{name: "Terraform", mutate: func(model *Model, oversized string) {
			model.TerraformBlocks[0].Name = oversized
		}},
		{name: "pipeline", mutate: func(model *Model, oversized string) {
			model.Pipelines[0].Name = oversized
		}},
		{name: "pipeline job", mutate: func(model *Model, oversized string) {
			model.Pipelines[0].Jobs[0].Name = oversized
		}},
		{name: "GitOps", mutate: func(model *Model, oversized string) {
			model.GitOpsResources[0].Name = oversized
		}},
		{name: "observability", mutate: func(model *Model, oversized string) {
			model.ObservabilityResources[0].Name = oversized
		}},
		{name: "security", mutate: func(model *Model, oversized string) {
			model.SecurityControls[0].Name = oversized
		}},
		{name: "diagnostic", mutate: func(model *Model, oversized string) {
			model.Diagnostics = []Diagnostic{{
				Code: "IATROS_DEVOPS_NOTE", Level: DiagnosticInfo, Path: ".", Message: oversized,
			}}
		}},
	}

	limits := DefaultLimits()
	oversized := strings.Repeat("x", limits.MaxTextBytes+1)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := validModel()
			testCase.mutate(&model, oversized)
			if err := model.ValidateWithin(limits); !errors.Is(err, ErrInvalidModel) {
				t.Fatalf("ValidateWithin() error = %v, want ErrInvalidModel", err)
			}
		})
	}
}

func TestModelValidateWithinRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxFiles = 0
	if err := validModel().ValidateWithin(limits); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("ValidateWithin() error = %v, want ErrInvalidLimits", err)
	}
}

func BenchmarkModelNormalizedAndValidated(b *testing.B) {
	model := validModel()
	limits := DefaultLimits()
	for b.Loop() {
		normalized := model.Normalized()
		if err := normalized.ValidateWithin(limits); err != nil {
			b.Fatalf("ValidateWithin() error = %v", err)
		}
	}
}

func BenchmarkModelValidationWithMaximumEvidence(b *testing.B) {
	evidence := make([]Evidence, DefaultLimits().MaxEvidencePerFact)
	for index := range evidence {
		evidence[index] = Evidence{
			Kind: EvidenceConfiguration,
			Path: fmt.Sprintf("deploy/config-%02d.yaml", index),
		}
	}
	model := validModel()
	model.ContainerBuilds[0].Evidence = evidence

	b.ResetTimer()
	for b.Loop() {
		if err := model.Validate(); err != nil {
			b.Fatalf("Validate() error = %v", err)
		}
	}
}

func limitingDiagnostics() []Diagnostic {
	return []Diagnostic{{
		Code:    "IATROS_DEVOPS_DYNAMIC_CONFIGURATION",
		Level:   DiagnosticWarning,
		Path:    "infrastructure/main.tf",
		Message: "A dynamic expression could not be resolved from local evidence.",
	}}
}

func reversedWithLaterCopy[S ~[]E, E any](entries S, update func(*E)) S {
	later := entries[0]
	update(&later)
	return S{later, entries[0]}
}

func validModel() Model {
	return Model{
		SchemaVersion: CurrentSchemaVersion,
		Tools: []Tool{
			{ID: "argo-cd", Name: "Argo CD", Category: CategoryGitOps, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "clusters/production/application.yaml"}}},
			{ID: "docker", Name: "Docker", Category: CategoryContainer, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceFilename, Path: "Dockerfile"}}},
			{ID: "github-actions", Name: "GitHub Actions", Category: CategoryCICD, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceFilename, Path: ".github/workflows/ci.yml"}}},
			{ID: "helm", Name: "Helm", Category: CategoryOrchestration, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceFilename, Path: "deploy/chart/Chart.yaml"}}},
			{ID: "kubernetes", Name: "Kubernetes", Category: CategoryOrchestration, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "deploy/kubernetes/api.yaml"}}},
			{ID: "prometheus", Name: "Prometheus", Category: CategoryObservability, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "monitoring/prometheus.yaml"}}},
			{ID: "terraform-compatible", Name: "Terraform-compatible", Category: CategoryInfrastructureAsCode,
				Certainty: CertaintyObserved, Evidence: []Evidence{{Kind: EvidenceFilename, Path: "infrastructure/main.tf"}}},
			{ID: "trivy", Name: "Trivy", Category: CategorySecurity, Certainty: CertaintyObserved,
				Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "security/trivy.yaml"}}},
		},
		ContainerBuilds: []ContainerBuild{{
			ID: "api-image", ToolID: "docker", Target: "runtime", BaseImages: []string{"alpine:3"},
			BuildArguments: []string{"PORT", "VERSION"}, ExposedPorts: []uint16{8080},
			Certainty: CertaintyObserved, Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "Dockerfile"}},
		}},
		ComposeServices: []ComposeService{{
			ID: "api-compose", ToolID: "docker", Name: "api", Image: "iatros/api:local", BuildContext: ".",
			Profiles: []string{"default"}, DependsOn: []string{"database"},
			EnvironmentVariables: []string{"DATABASE_URL", "HTTP_PORT"}, Ports: []uint16{8080},
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceConfiguration, Path: "compose.yaml"}},
		}},
		KubernetesResources: []KubernetesResource{{
			ID: "api-deployment", ToolID: "kubernetes", APIVersion: "apps/v1", Kind: "Deployment",
			Name: "api", Namespace: "iatros", Images: []string{"iatros/api:latest"}, Ports: []uint16{8080},
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceConfiguration, Path: "deploy/kubernetes/api.yaml"}},
		}},
		HelmCharts: []HelmChart{{
			ID: "api-chart", ToolID: "helm", Root: "deploy/chart", Name: "iatros-api", Version: "0.1.0",
			AppVersion: "1.0.0", Dependencies: []string{"postgresql"}, Certainty: CertaintyObserved,
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "deploy/chart/Chart.yaml"}},
		}},
		TerraformBlocks: []TerraformBlock{{
			ID: "api-instance", ToolID: "terraform-compatible", Kind: TerraformBlockResource,
			Type: "aws_instance", Name: "api", Provider: "hashicorp/aws", Certainty: CertaintyObserved,
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "infrastructure/main.tf"}},
		}},
		Pipelines: []Pipeline{{
			ID: "ci", ToolID: "github-actions", Name: "CI", Triggers: []string{"pull_request", "push"},
			Jobs: []PipelineJob{
				{ID: "build", Name: "Build", Needs: []string{},
					Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}},
				{ID: "deploy", Name: "Deploy", Needs: []string{"build"}, Environment: "production",
					Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}}},
			},
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceConfiguration, Path: ".github/workflows/ci.yml"}},
		}},
		GitOpsResources: []GitOpsResource{{
			ID: "api-application", ToolID: "argo-cd", Kind: "application", Name: "api",
			Namespace: "argocd", SourcePath: "deploy/chart", TargetNamespace: "iatros",
			Certainty: CertaintyObserved,
			Evidence:  []Evidence{{Kind: EvidenceConfiguration, Path: "clusters/production/application.yaml"}},
		}},
		ObservabilityResources: []ObservabilityResource{{
			ID: "api-alerts", ToolID: "prometheus", Kind: "alert-rules", Name: "API alerts",
			Signals: []Signal{SignalAlerts, SignalMetrics}, Certainty: CertaintyObserved,
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "monitoring/prometheus.yaml"}},
		}},
		SecurityControls: []SecurityControl{{
			ID: "container-scan", ToolID: "trivy", Kind: "vulnerability-scan", Name: "Container scan",
			Scope: "container-images", Enforcement: EnforcementBlocking, Certainty: CertaintyObserved,
			Evidence: []Evidence{{Kind: EvidenceConfiguration, Path: "security/trivy.yaml"}},
		}},
		Diagnostics: []Diagnostic{},
	}.Normalized()
}
