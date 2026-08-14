package changeimpact

import (
	"errors"
	"time"
)

const minimumLimitingDiagnostics = 8

// ErrInvalidLimits indicates an unusable change-impact limit profile.
var ErrInvalidLimits = errors.New("change impact limits are invalid")

// Limits bound input changes, graph work, retained impacts, evidence, text, and execution time.
type Limits struct {
	MaxChanges                int
	MaxServices               int
	MaxEnvironments           int
	MaxConfigurations         int
	MaxDirectMatches          int
	MaxTraversalDepth         int
	MaxRelationshipTraversals int
	MaxCausesPerImpact        int
	MaxRelationshipsPerCause  int
	MaxDiagnostics            int
	MaxTextBytes              int
	Timeout                   time.Duration
}

// DefaultLimits returns conservative budgets for small and typical systems.
func DefaultLimits() Limits {
	return Limits{
		MaxChanges:                1_000,
		MaxServices:               1_000,
		MaxEnvironments:           100,
		MaxConfigurations:         2_000,
		MaxDirectMatches:          50_000,
		MaxTraversalDepth:         20,
		MaxRelationshipTraversals: 50_000,
		MaxCausesPerImpact:        100,
		MaxRelationshipsPerCause:  20,
		MaxDiagnostics:            100,
		MaxTextBytes:              16 * 1024,
		Timeout:                   10 * time.Second,
	}
}

// LargeSystemLimits returns starting budgets for large monorepository and polyrepository systems.
func LargeSystemLimits() Limits {
	return Limits{
		MaxChanges:                100_000,
		MaxServices:               50_000,
		MaxEnvironments:           2_000,
		MaxConfigurations:         250_000,
		MaxDirectMatches:          5_000_000,
		MaxTraversalDepth:         100,
		MaxRelationshipTraversals: 5_000_000,
		MaxCausesPerImpact:        2_000,
		MaxRelationshipsPerCause:  100,
		MaxDiagnostics:            2_000,
		MaxTextBytes:              64 * 1024,
		Timeout:                   2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxChanges:                500_000,
		MaxServices:               250_000,
		MaxEnvironments:           10_000,
		MaxConfigurations:         1_000_000,
		MaxDirectMatches:          25_000_000,
		MaxTraversalDepth:         500,
		MaxRelationshipTraversals: 25_000_000,
		MaxCausesPerImpact:        10_000,
		MaxRelationshipsPerCause:  500,
		MaxDiagnostics:            10_000,
		MaxTextBytes:              256 * 1024,
		Timeout:                   10 * time.Minute,
	}
}

// Validate checks that every budget is positive and internally consistent.
func (l Limits) Validate() error {
	if l.MaxChanges <= 0 || l.MaxServices <= 0 || l.MaxEnvironments <= 0 ||
		l.MaxConfigurations <= 0 || l.MaxDirectMatches <= 0 || l.MaxTraversalDepth <= 0 ||
		l.MaxRelationshipTraversals <= 0 || l.MaxCausesPerImpact <= 0 ||
		l.MaxRelationshipsPerCause <= 0 ||
		l.MaxRelationshipsPerCause > l.MaxTraversalDepth ||
		l.MaxDiagnostics < minimumLimitingDiagnostics ||
		l.MaxTextBytes <= 0 || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
