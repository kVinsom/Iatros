package devopsanalysis

import (
	"errors"
	"math"
	"time"
)

// ErrInvalidLimits indicates an unusable DevOps-analysis limit profile.
var ErrInvalidLimits = errors.New("devops analysis limits are invalid")

// Limits bound configuration I/O, parser complexity, retained facts, and diagnostics.
type Limits struct {
	MaxFiles                      int
	MaxFileBytes                  int64
	MaxTotalBytes                 int64
	MaxNestingDepth               int
	MaxTools                      int
	MaxContainerBuilds            int
	MaxComposeServices            int
	MaxKubernetesResources        int
	MaxHelmCharts                 int
	MaxTerraformBlocks            int
	MaxPipelines                  int
	MaxJobsPerPipeline            int
	MaxJobDependenciesPerPipeline int
	MaxGitOpsResources            int
	MaxObservabilityResources     int
	MaxSecurityControls           int
	MaxValuesPerFact              int
	MaxEvidencePerFact            int
	MaxDiagnostics                int
	MaxTextBytes                  int
	Timeout                       time.Duration
}

// DefaultLimits returns conservative budgets for small and typical local repositories.
func DefaultLimits() Limits {
	return Limits{
		MaxFiles:                      500,
		MaxFileBytes:                  2 * 1024 * 1024,
		MaxTotalBytes:                 32 * 1024 * 1024,
		MaxNestingDepth:               128,
		MaxTools:                      100,
		MaxContainerBuilds:            250,
		MaxComposeServices:            1_000,
		MaxKubernetesResources:        5_000,
		MaxHelmCharts:                 250,
		MaxTerraformBlocks:            10_000,
		MaxPipelines:                  250,
		MaxJobsPerPipeline:            500,
		MaxJobDependenciesPerPipeline: 5_000,
		MaxGitOpsResources:            2_000,
		MaxObservabilityResources:     2_000,
		MaxSecurityControls:           2_000,
		MaxValuesPerFact:              500,
		MaxEvidencePerFact:            20,
		MaxDiagnostics:                100,
		MaxTextBytes:                  16 * 1024,
		Timeout:                       15 * time.Second,
	}
}

// LargeRepositoryLimits returns starting budgets for large monorepositories.
func LargeRepositoryLimits() Limits {
	return Limits{
		MaxFiles:                      25_000,
		MaxFileBytes:                  8 * 1024 * 1024,
		MaxTotalBytes:                 2 * 1024 * 1024 * 1024,
		MaxNestingDepth:               256,
		MaxTools:                      1_000,
		MaxContainerBuilds:            10_000,
		MaxComposeServices:            50_000,
		MaxKubernetesResources:        500_000,
		MaxHelmCharts:                 10_000,
		MaxTerraformBlocks:            1_000_000,
		MaxPipelines:                  10_000,
		MaxJobsPerPipeline:            10_000,
		MaxJobDependenciesPerPipeline: 250_000,
		MaxGitOpsResources:            250_000,
		MaxObservabilityResources:     250_000,
		MaxSecurityControls:           250_000,
		MaxValuesPerFact:              10_000,
		MaxEvidencePerFact:            100,
		MaxDiagnostics:                2_000,
		MaxTextBytes:                  64 * 1024,
		Timeout:                       2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxFiles:                      100_000,
		MaxFileBytes:                  16 * 1024 * 1024,
		MaxTotalBytes:                 16 * 1024 * 1024 * 1024,
		MaxNestingDepth:               512,
		MaxTools:                      5_000,
		MaxContainerBuilds:            50_000,
		MaxComposeServices:            250_000,
		MaxKubernetesResources:        2_500_000,
		MaxHelmCharts:                 50_000,
		MaxTerraformBlocks:            5_000_000,
		MaxPipelines:                  50_000,
		MaxJobsPerPipeline:            50_000,
		MaxJobDependenciesPerPipeline: 1_000_000,
		MaxGitOpsResources:            1_000_000,
		MaxObservabilityResources:     1_000_000,
		MaxSecurityControls:           1_000_000,
		MaxValuesPerFact:              50_000,
		MaxEvidencePerFact:            500,
		MaxDiagnostics:                10_000,
		MaxTextBytes:                  256 * 1024,
		Timeout:                       10 * time.Minute,
	}
}

// Validate checks that every budget is positive and internally consistent.
func (l Limits) Validate() error {
	if l.MaxFiles <= 0 || l.MaxFileBytes <= 0 || l.MaxFileBytes == math.MaxInt64 ||
		l.MaxTotalBytes < l.MaxFileBytes || l.MaxTotalBytes == math.MaxInt64 ||
		l.MaxNestingDepth <= 0 || l.MaxTools <= 0 || l.MaxContainerBuilds <= 0 ||
		l.MaxComposeServices <= 0 || l.MaxKubernetesResources <= 0 || l.MaxHelmCharts <= 0 ||
		l.MaxTerraformBlocks <= 0 || l.MaxPipelines <= 0 || l.MaxJobsPerPipeline <= 0 ||
		l.MaxJobDependenciesPerPipeline <= 0 ||
		l.MaxGitOpsResources <= 0 || l.MaxObservabilityResources <= 0 ||
		l.MaxSecurityControls <= 0 || l.MaxValuesPerFact <= 0 || l.MaxEvidencePerFact <= 0 ||
		l.MaxDiagnostics <= 0 || l.MaxTextBytes <= 0 || int64(l.MaxTextBytes) > l.MaxFileBytes ||
		l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
