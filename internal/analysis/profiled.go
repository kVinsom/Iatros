package analysis

import (
	"context"
	"fmt"

	"github.com/kVinsom/Iatros/internal/topology"
)

// ProfiledLocalAnalyzer selects one prevalidated local-analysis pipeline per request.
type ProfiledLocalAnalyzer struct {
	analyzers map[ScalingProfileName]LocalAnalyzer
}

// NewProfiledLocalAnalyzer creates a selector containing only explicitly enabled profiles.
func NewProfiledLocalAnalyzer(profiles ...ScalingProfile) (ProfiledLocalAnalyzer, error) {
	if err := validateScalingProfiles(profiles); err != nil {
		return ProfiledLocalAnalyzer{}, err
	}

	analyzers := make(map[ScalingProfileName]LocalAnalyzer, len(profiles))
	for _, profile := range profiles {
		analyzer, err := NewLocalAnalyzerWithProfile(profile)
		if err != nil {
			return ProfiledLocalAnalyzer{}, err
		}
		analyzers[profile.Name] = analyzer
	}

	return ProfiledLocalAnalyzer{analyzers: analyzers}, nil
}

// Analyze selects the requested enabled profile and runs its local-analysis pipeline.
func (a ProfiledLocalAnalyzer) Analyze(ctx context.Context, request Request) (Report, error) {
	profile := normalizedScalingProfileName(request.Profile)
	if !validScalingProfileName(profile) {
		return failedProfileReport(
			ScalingProfileSmall,
			DiagnosticCodeScalingProfileUnavailable,
			"The requested scaling profile is invalid.",
		), ErrInvalidScalingProfile
	}
	analyzer, exists := a.analyzers[profile]
	if !exists {
		return failedProfileReport(
			profile,
			DiagnosticCodeScalingProfileUnavailable,
			"The requested scaling profile is unavailable in this installation.",
		), ErrScalingProfileUnavailable
	}

	request.Profile = profile
	return analyzer.Analyze(ctx, request)
}

// ProfiledLocalTopologyAnalyzer selects one prevalidated topology pipeline per request.
type ProfiledLocalTopologyAnalyzer struct {
	analyzers map[ScalingProfileName]LocalTopologyAnalyzer
}

// NewProfiledLocalTopologyAnalyzer creates a selector containing explicitly enabled profiles.
func NewProfiledLocalTopologyAnalyzer(
	profiles ...ScalingProfile,
) (ProfiledLocalTopologyAnalyzer, error) {
	if err := validateScalingProfiles(profiles); err != nil {
		return ProfiledLocalTopologyAnalyzer{}, err
	}

	analyzers := make(map[ScalingProfileName]LocalTopologyAnalyzer, len(profiles))
	for _, profile := range profiles {
		analyzer, err := NewLocalTopologyAnalyzerWithProfile(profile)
		if err != nil {
			return ProfiledLocalTopologyAnalyzer{}, err
		}
		analyzers[profile.Name] = analyzer
	}

	return ProfiledLocalTopologyAnalyzer{analyzers: analyzers}, nil
}

// Analyze selects the requested enabled profile and runs its topology pipeline.
func (a ProfiledLocalTopologyAnalyzer) Analyze(
	ctx context.Context,
	request Request,
) (topology.Model, error) {
	profile := normalizedScalingProfileName(request.Profile)
	if !validScalingProfileName(profile) {
		return topology.Model{}, ErrInvalidScalingProfile
	}
	analyzer, exists := a.analyzers[profile]
	if !exists {
		return topology.Model{}, ErrScalingProfileUnavailable
	}

	request.Profile = profile
	return analyzer.Analyze(ctx, request)
}

func validateScalingProfiles(profiles []ScalingProfile) error {
	if len(profiles) == 0 {
		return fmt.Errorf("%w: no profiles are enabled", ErrInvalidScalingProfile)
	}
	seen := make(map[ScalingProfileName]struct{}, len(profiles))
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return err
		}
		if _, exists := seen[profile.Name]; exists {
			return fmt.Errorf(
				"%w: profile %q is enabled more than once",
				ErrInvalidScalingProfile,
				profile.Name,
			)
		}
		seen[profile.Name] = struct{}{}
	}

	return nil
}
