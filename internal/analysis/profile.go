package analysis

import (
	"errors"
	"fmt"
	"time"

	"github.com/kVinsom/Iatros/internal/artifactvalidation"
	"github.com/kVinsom/Iatros/internal/changeimpact"
	"github.com/kVinsom/Iatros/internal/codeanalysis"
	"github.com/kVinsom/Iatros/internal/detection"
	"github.com/kVinsom/Iatros/internal/devopsanalysis"
	"github.com/kVinsom/Iatros/internal/doctor"
	"github.com/kVinsom/Iatros/internal/finding"
	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
	"github.com/kVinsom/Iatros/internal/remoteanalysis"
	"github.com/kVinsom/Iatros/internal/systemmap"
	"github.com/kVinsom/Iatros/internal/topology"
)

var (
	// ErrInvalidScalingProfile indicates a malformed or internally inconsistent profile.
	ErrInvalidScalingProfile = errors.New("scaling profile is invalid")
	// ErrScalingProfileUnavailable indicates that a valid profile is not enabled by the composition root.
	ErrScalingProfileUnavailable = errors.New("scaling profile is unavailable")
)

// ScalingProfileName identifies a complete resource profile for local repository analysis.
type ScalingProfileName string

const (
	// ScalingProfileSmall is the conservative default for small and typical repositories.
	ScalingProfileSmall ScalingProfileName = "small"
	// ScalingProfileMonorepo increases bounded work for large multi-project repositories.
	ScalingProfileMonorepo ScalingProfileName = "monorepo"
	// ScalingProfileEnterprise is a per-worker profile for calibrated company installations.
	ScalingProfileEnterprise ScalingProfileName = "enterprise"
)

// ScalingProfile composes every resource limit used by bounded analysis capabilities.
type ScalingProfile struct {
	Name               ScalingProfileName
	Discovery          DiscoveryLimits
	Detection          detection.Limits
	Project            project.Limits
	Manifest           manifest.Limits
	Topology           topology.Limits
	CodeAnalysis       codeanalysis.Limits
	DevOpsAnalysis     devopsanalysis.Limits
	SystemMap          systemmap.Limits
	RemoteAnalysis     remoteanalysis.Limits
	ChangeImpact       changeimpact.Limits
	ArtifactValidation artifactvalidation.Limits
	Doctor             doctor.Limits
	Findings           finding.Limits
}

// SmallScalingProfile returns the conservative default used by local Basic workflows.
func SmallScalingProfile() ScalingProfile {
	return ScalingProfile{
		Name:               ScalingProfileSmall,
		Discovery:          DefaultDiscoveryLimits(),
		Detection:          detection.DefaultLimits(),
		Project:            project.DefaultLimits(),
		Manifest:           manifest.DefaultLimits(),
		Topology:           topology.DefaultLimits(),
		CodeAnalysis:       codeanalysis.DefaultLimits(),
		DevOpsAnalysis:     devopsanalysis.DefaultLimits(),
		SystemMap:          systemmap.DefaultLimits(),
		RemoteAnalysis:     remoteanalysis.DefaultLimits(),
		ChangeImpact:       changeimpact.DefaultLimits(),
		ArtifactValidation: artifactvalidation.DefaultLimits(),
		Doctor:             doctor.DefaultLimits(),
		Findings:           finding.DefaultLimits(),
	}
}

// MonorepoScalingProfile returns an opt-in profile for large multi-project repositories.
func MonorepoScalingProfile() ScalingProfile {
	return ScalingProfile{
		Name: ScalingProfileMonorepo,
		Discovery: DiscoveryLimits{
			MaxFiles:               100_000,
			MaxDirectories:         25_000,
			MaxDepth:               40,
			MaxIssues:              500,
			MaxEntriesPerDirectory: 20_000,
			MaxIgnoreFiles:         2_000,
			MaxControlFileBytes:    1024 * 1024,
			MaxIgnorePatternBytes:  32 * 1024,
			MaxIgnoreRules:         100_000,
			MaxNestedRepositories:  5_000,
			Timeout:                time.Minute,
		},
		Detection:          detection.Limits{MaxEvidencePerTechnology: 100},
		Project:            project.Limits{MaxEvidencePerMarker: 100},
		Manifest:           manifest.LargeRepositoryLimits(),
		Topology:           topology.LargeRepositoryLimits(),
		CodeAnalysis:       codeanalysis.LargeRepositoryLimits(),
		DevOpsAnalysis:     devopsanalysis.LargeRepositoryLimits(),
		SystemMap:          systemmap.LargeSystemLimits(),
		RemoteAnalysis:     remoteanalysis.LargeSystemLimits(),
		ChangeImpact:       changeimpact.LargeSystemLimits(),
		ArtifactValidation: artifactvalidation.LargeRepositoryLimits(),
		Doctor:             doctor.LargeSystemLimits(),
		Findings:           finding.LargeSystemLimits(),
	}
}

// EnterpriseScalingProfile returns a per-worker starting profile for calibrated company installations.
// Enabling this profile remains an entitlement-aware composition decision.
func EnterpriseScalingProfile() ScalingProfile {
	return ScalingProfile{
		Name: ScalingProfileEnterprise,
		Discovery: DiscoveryLimits{
			MaxFiles:               500_000,
			MaxDirectories:         100_000,
			MaxDepth:               64,
			MaxIssues:              2_000,
			MaxEntriesPerDirectory: 50_000,
			MaxIgnoreFiles:         10_000,
			MaxControlFileBytes:    4 * 1024 * 1024,
			MaxIgnorePatternBytes:  64 * 1024,
			MaxIgnoreRules:         500_000,
			MaxNestedRepositories:  25_000,
			Timeout:                5 * time.Minute,
		},
		Detection: detection.Limits{MaxEvidencePerTechnology: 500},
		Project:   project.Limits{MaxEvidencePerMarker: 500},
		Manifest: manifest.Limits{
			MaxFiles:                        10_000,
			MaxFileBytes:                    16 * 1024 * 1024,
			MaxTotalBytes:                   1024 * 1024 * 1024,
			MaxDependenciesPerManifest:      50_000,
			MaxConstraintsPerManifest:       5_000,
			MaxWorkspaceMembersPerManifest:  25_000,
			MaxWorkspaceExcludesPerManifest: 25_000,
			MaxIssues:                       1_000,
			MaxNestingDepth:                 256,
			MaxValueBytes:                   256 * 1024,
			Timeout:                         2 * time.Minute,
		},
		Topology: topology.Limits{
			MaxProjects:              500_000,
			MaxWorkspaces:            50_000,
			MaxManifests:             10_000,
			MaxComponentsPerBoundary: 500,
			MaxWorkspaceDeclarations: 1_000_000,
			MaxMatchesPerDeclaration: 50_000,
			MaxDependencies:          5_000_000,
			MaxTargetsPerDependency:  500,
			MaxNestedRepositories:    25_000,
			MaxIssues:                5_000,
			MaxValueBytes:            256 * 1024,
			Timeout:                  5 * time.Minute,
		},
		CodeAnalysis:       codeanalysis.EnterpriseLimits(),
		DevOpsAnalysis:     devopsanalysis.EnterpriseLimits(),
		SystemMap:          systemmap.EnterpriseLimits(),
		RemoteAnalysis:     remoteanalysis.EnterpriseLimits(),
		ChangeImpact:       changeimpact.EnterpriseLimits(),
		ArtifactValidation: artifactvalidation.EnterpriseLimits(),
		Doctor:             doctor.EnterpriseLimits(),
		Findings:           finding.EnterpriseLimits(),
	}
}

// ScalingProfileForName returns the canonical built-in profile for name.
func ScalingProfileForName(name ScalingProfileName) (ScalingProfile, error) {
	switch name {
	case ScalingProfileSmall:
		return SmallScalingProfile(), nil
	case ScalingProfileMonorepo:
		return MonorepoScalingProfile(), nil
	case ScalingProfileEnterprise:
		return EnterpriseScalingProfile(), nil
	default:
		return ScalingProfile{}, fmt.Errorf("%w: unknown name %q", ErrInvalidScalingProfile, name)
	}
}

// Validate checks every component limit and the cross-component retention contract.
func (p ScalingProfile) Validate() error {
	if !validScalingProfileName(p.Name) {
		return fmt.Errorf("%w: unknown name %q", ErrInvalidScalingProfile, p.Name)
	}
	components := []struct {
		name string
		err  error
	}{
		{name: "discovery", err: p.Discovery.Validate()},
		{name: "detection", err: p.Detection.Validate()},
		{name: "project", err: p.Project.Validate()},
		{name: "manifest", err: p.Manifest.Validate()},
		{name: "topology", err: p.Topology.Validate()},
		{name: "code analysis", err: p.CodeAnalysis.Validate()},
		{name: "devops analysis", err: p.DevOpsAnalysis.Validate()},
		{name: "system map", err: p.SystemMap.Validate()},
		{name: "remote analysis", err: p.RemoteAnalysis.Validate()},
		{name: "change impact", err: p.ChangeImpact.Validate()},
		{name: "artifact validation", err: p.ArtifactValidation.Validate()},
		{name: "doctor", err: p.Doctor.Validate()},
		{name: "findings", err: p.Findings.Validate()},
	}
	for _, component := range components {
		if component.err != nil {
			return fmt.Errorf(
				"%w: %s limits: %v",
				ErrInvalidScalingProfile,
				component.name,
				component.err,
			)
		}
	}
	if p.Manifest.MaxFiles > p.Discovery.MaxFiles {
		return fmt.Errorf("%w: manifest files exceed discovered files", ErrInvalidScalingProfile)
	}
	if p.CodeAnalysis.MaxFiles > p.Discovery.MaxFiles {
		return fmt.Errorf("%w: code analysis files exceed discovered files", ErrInvalidScalingProfile)
	}
	if p.DevOpsAnalysis.MaxFiles > p.Discovery.MaxFiles {
		return fmt.Errorf("%w: devops analysis files exceed discovered files", ErrInvalidScalingProfile)
	}
	if p.Topology.MaxManifests < p.Manifest.MaxFiles {
		return fmt.Errorf("%w: topology cannot retain every analyzed manifest", ErrInvalidScalingProfile)
	}
	if p.Topology.MaxValueBytes < p.Manifest.MaxValueBytes {
		return fmt.Errorf("%w: topology values are smaller than manifest values", ErrInvalidScalingProfile)
	}
	if p.Topology.MaxNestedRepositories < p.Discovery.MaxNestedRepositories {
		return fmt.Errorf(
			"%w: topology cannot retain every discovered nested repository",
			ErrInvalidScalingProfile,
		)
	}
	if p.SystemMap.MaxRepositories < p.RemoteAnalysis.MaxRepositories {
		return fmt.Errorf(
			"%w: system map cannot retain every remote repository",
			ErrInvalidScalingProfile,
		)
	}
	if p.SystemMap.MaxRepositories <= p.Discovery.MaxNestedRepositories {
		return fmt.Errorf(
			"%w: system map cannot retain the root and every nested repository",
			ErrInvalidScalingProfile,
		)
	}
	if p.SystemMap.MaxRelationships < p.RemoteAnalysis.MaxRelationships {
		return fmt.Errorf(
			"%w: system map cannot retain every remote repository relationship",
			ErrInvalidScalingProfile,
		)
	}
	if p.SystemMap.MaxEvidencePerFact < p.RemoteAnalysis.MaxEvidencePerFact ||
		p.SystemMap.MaxDiagnostics < p.RemoteAnalysis.MaxDiagnostics ||
		p.SystemMap.MaxTextBytes < p.RemoteAnalysis.MaxTextBytes {
		return fmt.Errorf(
			"%w: system map cannot preserve normalized remote-analysis facts",
			ErrInvalidScalingProfile,
		)
	}
	if p.ChangeImpact.MaxServices > p.SystemMap.MaxServices ||
		p.ChangeImpact.MaxEnvironments > p.SystemMap.MaxEnvironments ||
		p.ChangeImpact.MaxConfigurations > p.SystemMap.MaxInfrastructure ||
		p.ChangeImpact.MaxRelationshipTraversals < p.SystemMap.MaxRelationships {
		return fmt.Errorf(
			"%w: change impact cannot process the retained system map",
			ErrInvalidScalingProfile,
		)
	}
	if p.ArtifactValidation.MaxArtifactBytes > p.DevOpsAnalysis.MaxFileBytes ||
		p.ArtifactValidation.MaxTotalBytes > p.DevOpsAnalysis.MaxTotalBytes {
		return fmt.Errorf(
			"%w: artifact validation exceeds DevOps configuration byte budgets",
			ErrInvalidScalingProfile,
		)
	}
	if p.Doctor.MaxRepositoryFindings > p.Findings.MaxFindings ||
		uint64(p.Doctor.MaxInputFacts) < requiredDoctorInputFacts(p) {
		return fmt.Errorf(
			"%w: Doctor cannot retain all normalized profile inputs",
			ErrInvalidScalingProfile,
		)
	}

	return nil
}

func requiredDoctorInputFacts(profile ScalingProfile) uint64 {
	codeFacts := uint64(profile.CodeAnalysis.MaxServices) + uint64(profile.CodeAnalysis.MaxFrameworks) +
		uint64(profile.CodeAnalysis.MaxPortBindings) + uint64(profile.CodeAnalysis.MaxAPIEndpoints) +
		uint64(profile.CodeAnalysis.MaxEnvironmentVariables) + uint64(profile.CodeAnalysis.MaxResourceDependencies) +
		uint64(profile.CodeAnalysis.MaxDiagnostics)
	devOpsFacts := uint64(profile.DevOpsAnalysis.MaxTools) + uint64(profile.DevOpsAnalysis.MaxContainerBuilds) +
		uint64(profile.DevOpsAnalysis.MaxComposeServices) + uint64(profile.DevOpsAnalysis.MaxKubernetesResources) +
		uint64(profile.DevOpsAnalysis.MaxHelmCharts) + uint64(profile.DevOpsAnalysis.MaxTerraformBlocks) +
		uint64(profile.DevOpsAnalysis.MaxPipelines) + uint64(profile.DevOpsAnalysis.MaxGitOpsResources) +
		uint64(profile.DevOpsAnalysis.MaxObservabilityResources) + uint64(profile.DevOpsAnalysis.MaxSecurityControls) +
		uint64(profile.DevOpsAnalysis.MaxDiagnostics)
	systemFacts := uint64(profile.SystemMap.MaxRepositories) + uint64(profile.SystemMap.MaxServices) +
		uint64(profile.SystemMap.MaxLibraries) + uint64(profile.SystemMap.MaxInfrastructure) +
		uint64(profile.SystemMap.MaxEnvironments) + uint64(profile.SystemMap.MaxOwners) +
		uint64(profile.SystemMap.MaxExternalResources) + uint64(profile.SystemMap.MaxRelationships) +
		uint64(profile.SystemMap.MaxDiagnostics)
	validationFacts := uint64(profile.ArtifactValidation.MaxArtifacts) +
		uint64(profile.ArtifactValidation.MaxDiagnostics)
	return codeFacts + devOpsFacts + systemFacts + validationFacts + uint64(profile.Findings.MaxFindings)
}

func validScalingProfileName(name ScalingProfileName) bool {
	return name == ScalingProfileSmall || name == ScalingProfileMonorepo ||
		name == ScalingProfileEnterprise
}

func normalizedScalingProfileName(name ScalingProfileName) ScalingProfileName {
	if name == "" {
		return ScalingProfileSmall
	}

	return name
}
