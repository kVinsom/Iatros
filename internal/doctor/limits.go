package doctor

import (
	"errors"
	"time"
)

const minimumTextBytes = 256

// ErrInvalidLimits indicates an unusable Doctor limit profile.
var ErrInvalidLimits = errors.New("doctor limits are invalid")

// Limits bound rules, input facts, retained diagnostics, text, and audit time.
type Limits struct {
	MaxRules              int
	MaxInputFacts         int
	MaxRepositoryFindings int
	MaxDiagnostics        int
	MaxTextBytes          int
	Timeout               time.Duration
}

// DefaultLimits returns conservative budgets for small and typical repositories.
func DefaultLimits() Limits {
	return Limits{
		MaxRules:              100,
		MaxInputFacts:         100_000,
		MaxRepositoryFindings: 1_000,
		MaxDiagnostics:        200,
		MaxTextBytes:          16 * 1024,
		Timeout:               15 * time.Second,
	}
}

// LargeSystemLimits returns starting budgets for large monorepository and polyrepository systems.
func LargeSystemLimits() Limits {
	return Limits{
		MaxRules:              1_000,
		MaxInputFacts:         6_000_000,
		MaxRepositoryFindings: 100_000,
		MaxDiagnostics:        10_000,
		MaxTextBytes:          64 * 1024,
		Timeout:               2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxRules:              5_000,
		MaxInputFacts:         30_000_000,
		MaxRepositoryFindings: 500_000,
		MaxDiagnostics:        50_000,
		MaxTextBytes:          256 * 1024,
		Timeout:               10 * time.Minute,
	}
}

// Validate checks that every Doctor budget is positive.
func (l Limits) Validate() error {
	if l.MaxRules <= 0 || l.MaxInputFacts <= 0 || l.MaxRepositoryFindings <= 0 ||
		l.MaxDiagnostics <= 0 || l.MaxTextBytes < minimumTextBytes || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
