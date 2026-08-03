package cli

import (
	"fmt"

	"github.com/kVinsom/Iatros/internal/analysis"
)

const defaultScalingProfile = string(analysis.ScalingProfileSmall)

func basicScalingProfile(value string) (analysis.ScalingProfileName, error) {
	profile := analysis.ScalingProfileName(value)
	switch profile {
	case analysis.ScalingProfileSmall, analysis.ScalingProfileMonorepo:
		return profile, nil
	default:
		return "", fmt.Errorf(
			"unsupported profile %q; supported profiles are small and monorepo",
			value,
		)
	}
}
