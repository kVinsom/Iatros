package finding

import (
	"errors"
	"time"
)

// ErrInvalidLimits indicates an unusable finding limit profile.
var ErrInvalidLimits = errors.New("finding limits are invalid")

// Limits bound retained findings, evidence, exclusions, text, and evaluation time.
type Limits struct {
	MaxFindings           int
	MaxSubjectsPerFinding int
	MaxEvidencePerFinding int
	MaxActionsPerFinding  int
	MaxExclusions         int
	MaxTextBytes          int
	Timeout               time.Duration
}

// DefaultLimits returns conservative budgets for small and typical repositories.
func DefaultLimits() Limits {
	return Limits{
		MaxFindings:           1_000,
		MaxSubjectsPerFinding: 20,
		MaxEvidencePerFinding: 50,
		MaxActionsPerFinding:  20,
		MaxExclusions:         1_000,
		MaxTextBytes:          16 * 1024,
		Timeout:               10 * time.Second,
	}
}

// LargeSystemLimits returns starting budgets for large monorepository and polyrepository systems.
func LargeSystemLimits() Limits {
	return Limits{
		MaxFindings:           100_000,
		MaxSubjectsPerFinding: 100,
		MaxEvidencePerFinding: 500,
		MaxActionsPerFinding:  100,
		MaxExclusions:         100_000,
		MaxTextBytes:          64 * 1024,
		Timeout:               2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxFindings:           500_000,
		MaxSubjectsPerFinding: 500,
		MaxEvidencePerFinding: 2_000,
		MaxActionsPerFinding:  500,
		MaxExclusions:         500_000,
		MaxTextBytes:          256 * 1024,
		Timeout:               10 * time.Minute,
	}
}

// Validate checks that every finding budget is positive.
func (l Limits) Validate() error {
	if l.MaxFindings <= 0 || l.MaxSubjectsPerFinding <= 0 ||
		l.MaxEvidencePerFinding <= 0 || l.MaxActionsPerFinding <= 0 ||
		l.MaxExclusions <= 0 || l.MaxTextBytes <= 0 || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
