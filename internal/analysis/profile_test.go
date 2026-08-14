package analysis

import (
	"errors"
	"testing"
)

func TestBuiltInScalingProfilesAreValidAndMonotonic(t *testing.T) {
	t.Parallel()

	profiles := []ScalingProfile{
		SmallScalingProfile(),
		MonorepoScalingProfile(),
		EnterpriseScalingProfile(),
	}
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			t.Fatalf("%s Validate() error = %v", profile.Name, err)
		}
	}

	for index := 1; index < len(profiles); index++ {
		smaller := profiles[index-1]
		larger := profiles[index]
		if larger.Discovery.MaxFiles <= smaller.Discovery.MaxFiles ||
			larger.Discovery.MaxDirectories <= smaller.Discovery.MaxDirectories ||
			larger.Discovery.MaxIgnoreFiles <= smaller.Discovery.MaxIgnoreFiles ||
			larger.Discovery.MaxControlFileBytes <= smaller.Discovery.MaxControlFileBytes ||
			larger.Discovery.MaxIgnorePatternBytes <= smaller.Discovery.MaxIgnorePatternBytes ||
			larger.Discovery.MaxIgnoreRules <= smaller.Discovery.MaxIgnoreRules ||
			larger.Discovery.MaxNestedRepositories <= smaller.Discovery.MaxNestedRepositories ||
			larger.Detection.MaxEvidencePerTechnology <= smaller.Detection.MaxEvidencePerTechnology ||
			larger.Project.MaxEvidencePerMarker <= smaller.Project.MaxEvidencePerMarker ||
			larger.Manifest.MaxFiles <= smaller.Manifest.MaxFiles ||
			larger.Manifest.MaxTotalBytes <= smaller.Manifest.MaxTotalBytes ||
			larger.Topology.MaxDependencies <= smaller.Topology.MaxDependencies ||
			larger.Topology.MaxNestedRepositories <= smaller.Topology.MaxNestedRepositories ||
			larger.CodeAnalysis.MaxFiles <= smaller.CodeAnalysis.MaxFiles ||
			larger.CodeAnalysis.MaxTotalBytes <= smaller.CodeAnalysis.MaxTotalBytes ||
			larger.CodeAnalysis.MaxAPIEndpoints <= smaller.CodeAnalysis.MaxAPIEndpoints ||
			larger.DevOpsAnalysis.MaxFiles <= smaller.DevOpsAnalysis.MaxFiles ||
			larger.DevOpsAnalysis.MaxTotalBytes <= smaller.DevOpsAnalysis.MaxTotalBytes ||
			larger.DevOpsAnalysis.MaxKubernetesResources <= smaller.DevOpsAnalysis.MaxKubernetesResources ||
			larger.SystemMap.MaxServices <= smaller.SystemMap.MaxServices ||
			larger.RemoteAnalysis.MaxRepositories <= smaller.RemoteAnalysis.MaxRepositories ||
			larger.ChangeImpact.MaxChanges <= smaller.ChangeImpact.MaxChanges ||
			larger.ArtifactValidation.MaxArtifacts <= smaller.ArtifactValidation.MaxArtifacts ||
			larger.Doctor.MaxInputFacts <= smaller.Doctor.MaxInputFacts ||
			larger.Findings.MaxFindings <= smaller.Findings.MaxFindings {
			t.Fatalf("profile %q does not exceed %q in every primary capacity", larger.Name, smaller.Name)
		}
	}
}

func TestScalingProfileForNameReturnsCanonicalProfiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name ScalingProfileName
		want ScalingProfile
	}{
		{name: ScalingProfileSmall, want: SmallScalingProfile()},
		{name: ScalingProfileMonorepo, want: MonorepoScalingProfile()},
		{name: ScalingProfileEnterprise, want: EnterpriseScalingProfile()},
	}
	for _, testCase := range testCases {
		t.Run(string(testCase.name), func(t *testing.T) {
			t.Parallel()

			got, err := ScalingProfileForName(testCase.name)
			if err != nil {
				t.Fatalf("ScalingProfileForName() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("ScalingProfileForName() = %+v, want %+v", got, testCase.want)
			}
		})
	}
}

func TestScalingProfileForNameRejectsUnknownName(t *testing.T) {
	t.Parallel()

	if _, err := ScalingProfileForName("large"); !errors.Is(err, ErrInvalidScalingProfile) {
		t.Fatalf("ScalingProfileForName() error = %v, want ErrInvalidScalingProfile", err)
	}
}

func TestScalingProfileValidateRejectsInvalidComponentsAndRelationships(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*ScalingProfile)
	}{
		{name: "name", mutate: func(profile *ScalingProfile) { profile.Name = "large" }},
		{name: "discovery", mutate: func(profile *ScalingProfile) {
			profile.Discovery.MaxFiles = 0
		}},
		{name: "detection", mutate: func(profile *ScalingProfile) {
			profile.Detection.MaxEvidencePerTechnology = 0
		}},
		{name: "project", mutate: func(profile *ScalingProfile) {
			profile.Project.MaxEvidencePerMarker = 0
		}},
		{name: "manifest", mutate: func(profile *ScalingProfile) {
			profile.Manifest.MaxTotalBytes = 0
		}},
		{name: "topology", mutate: func(profile *ScalingProfile) {
			profile.Topology.MaxDependencies = 0
		}},
		{name: "code analysis", mutate: func(profile *ScalingProfile) {
			profile.CodeAnalysis.MaxTotalBytes = 0
		}},
		{name: "devops analysis", mutate: func(profile *ScalingProfile) {
			profile.DevOpsAnalysis.MaxTotalBytes = 0
		}},
		{name: "system map", mutate: func(profile *ScalingProfile) {
			profile.SystemMap.MaxServices = 0
		}},
		{name: "remote analysis", mutate: func(profile *ScalingProfile) {
			profile.RemoteAnalysis.MaxRepositories = 0
		}},
		{name: "change impact", mutate: func(profile *ScalingProfile) {
			profile.ChangeImpact.MaxChanges = 0
		}},
		{name: "artifact validation", mutate: func(profile *ScalingProfile) {
			profile.ArtifactValidation.MaxArtifacts = 0
		}},
		{name: "Doctor", mutate: func(profile *ScalingProfile) {
			profile.Doctor.MaxRules = 0
		}},
		{name: "findings", mutate: func(profile *ScalingProfile) {
			profile.Findings.MaxFindings = 0
		}},
		{name: "manifest exceeds discovery", mutate: func(profile *ScalingProfile) {
			profile.Manifest.MaxFiles = profile.Discovery.MaxFiles + 1
			profile.Topology.MaxManifests = profile.Manifest.MaxFiles
		}},
		{name: "topology drops manifests", mutate: func(profile *ScalingProfile) {
			profile.Topology.MaxManifests = profile.Manifest.MaxFiles - 1
		}},
		{name: "topology truncates values", mutate: func(profile *ScalingProfile) {
			profile.Topology.MaxValueBytes = profile.Manifest.MaxValueBytes - 1
		}},
		{name: "topology drops nested repositories", mutate: func(profile *ScalingProfile) {
			profile.Topology.MaxNestedRepositories = profile.Discovery.MaxNestedRepositories - 1
		}},
		{name: "code analysis exceeds discovery", mutate: func(profile *ScalingProfile) {
			profile.CodeAnalysis.MaxFiles = profile.Discovery.MaxFiles + 1
		}},
		{name: "devops analysis exceeds discovery", mutate: func(profile *ScalingProfile) {
			profile.DevOpsAnalysis.MaxFiles = profile.Discovery.MaxFiles + 1
		}},
		{name: "impact drops services", mutate: func(profile *ScalingProfile) {
			profile.ChangeImpact.MaxServices = profile.SystemMap.MaxServices + 1
		}},
		{name: "impact drops relationships", mutate: func(profile *ScalingProfile) {
			profile.ChangeImpact.MaxRelationshipTraversals = profile.SystemMap.MaxRelationships - 1
		}},
		{name: "artifact validation exceeds DevOps bytes", mutate: func(profile *ScalingProfile) {
			profile.ArtifactValidation.MaxArtifactBytes = profile.DevOpsAnalysis.MaxFileBytes + 1
			profile.ArtifactValidation.MaxTotalBytes = profile.DevOpsAnalysis.MaxTotalBytes
		}},
		{name: "Doctor drops input facts", mutate: func(profile *ScalingProfile) {
			profile.Doctor.MaxInputFacts = int(requiredDoctorInputFacts(*profile) - 1)
		}},
		{name: "Doctor drops repository findings", mutate: func(profile *ScalingProfile) {
			profile.Doctor.MaxRepositoryFindings = profile.Findings.MaxFindings + 1
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			profile := SmallScalingProfile()
			testCase.mutate(&profile)
			if err := profile.Validate(); !errors.Is(err, ErrInvalidScalingProfile) {
				t.Fatalf("Validate() error = %v, want ErrInvalidScalingProfile", err)
			}
		})
	}
}

func TestNormalizedScalingProfileNameDefaultsToSmall(t *testing.T) {
	t.Parallel()

	if got := normalizedScalingProfileName(""); got != ScalingProfileSmall {
		t.Fatalf("normalizedScalingProfileName() = %q, want %q", got, ScalingProfileSmall)
	}
}
