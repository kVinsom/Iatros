package devopsanalysis

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBuiltInLimitsAreValidAndMonotonic(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeRepositoryLimits(), EnterpriseLimits()}
	for index, limits := range profiles {
		if err := limits.Validate(); err != nil {
			t.Fatalf("profile %d Validate() error = %v", index, err)
		}
	}

	for index := 1; index < len(profiles); index++ {
		assertLimitsIncrease(t, profiles[index-1], profiles[index])
	}
}

func TestLimitsValidateRejectsEveryInvalidBudget(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "files", mutate: func(limits *Limits) { limits.MaxFiles = 0 }},
		{name: "file bytes", mutate: func(limits *Limits) { limits.MaxFileBytes = 0 }},
		{name: "total bytes", mutate: func(limits *Limits) { limits.MaxTotalBytes = 1 }},
		{name: "nesting", mutate: func(limits *Limits) { limits.MaxNestingDepth = 0 }},
		{name: "tools", mutate: func(limits *Limits) { limits.MaxTools = 0 }},
		{name: "container builds", mutate: func(limits *Limits) { limits.MaxContainerBuilds = 0 }},
		{name: "compose services", mutate: func(limits *Limits) { limits.MaxComposeServices = 0 }},
		{name: "Kubernetes resources", mutate: func(limits *Limits) { limits.MaxKubernetesResources = 0 }},
		{name: "Helm charts", mutate: func(limits *Limits) { limits.MaxHelmCharts = 0 }},
		{name: "Terraform blocks", mutate: func(limits *Limits) { limits.MaxTerraformBlocks = 0 }},
		{name: "pipelines", mutate: func(limits *Limits) { limits.MaxPipelines = 0 }},
		{name: "pipeline jobs", mutate: func(limits *Limits) { limits.MaxJobsPerPipeline = 0 }},
		{name: "pipeline job dependencies", mutate: func(limits *Limits) {
			limits.MaxJobDependenciesPerPipeline = 0
		}},
		{name: "GitOps resources", mutate: func(limits *Limits) { limits.MaxGitOpsResources = 0 }},
		{name: "observability resources", mutate: func(limits *Limits) { limits.MaxObservabilityResources = 0 }},
		{name: "security controls", mutate: func(limits *Limits) { limits.MaxSecurityControls = 0 }},
		{name: "nested values", mutate: func(limits *Limits) { limits.MaxValuesPerFact = 0 }},
		{name: "evidence", mutate: func(limits *Limits) { limits.MaxEvidencePerFact = 0 }},
		{name: "diagnostics", mutate: func(limits *Limits) { limits.MaxDiagnostics = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "text exceeds file", mutate: func(limits *Limits) {
			limits.MaxTextBytes = int(limits.MaxFileBytes) + 1
		}},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			testCase.mutate(&limits)
			if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
			}
		})
	}
}

func assertLimitsIncrease(t *testing.T, smaller, larger Limits) {
	t.Helper()

	smallerValue := reflect.ValueOf(smaller)
	largerValue := reflect.ValueOf(larger)
	typeInfo := smallerValue.Type()
	for index := range smallerValue.NumField() {
		field := typeInfo.Field(index)
		switch smallerValue.Field(index).Kind() {
		case reflect.Int, reflect.Int64:
			if largerValue.Field(index).Int() <= smallerValue.Field(index).Int() {
				t.Errorf("%s did not increase", field.Name)
			}
		default:
			t.Fatalf("unsupported Limits field %s of type %s", field.Name, field.Type)
		}
	}
}

func TestLimitsUseBoundedDurations(t *testing.T) {
	t.Parallel()

	if DefaultLimits().Timeout > time.Minute || EnterpriseLimits().Timeout > 15*time.Minute {
		t.Fatal("built-in DevOps analysis timeouts are unexpectedly unbounded")
	}
}
