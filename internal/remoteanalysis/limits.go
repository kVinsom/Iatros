package remoteanalysis

import (
	"errors"
	"math"
	"time"
)

// ErrInvalidLimits indicates an unusable remote-analysis limit profile.
var ErrInvalidLimits = errors.New("remote analysis limits are invalid")

// Limits bound provider access, response bytes, retained relationships, and execution time.
type Limits struct {
	MaxRepositories        int
	MaxRelationships       int
	MaxProviderPages       int
	MaxRepositoriesPerPage int
	MaxProviderRequests    int
	MaxConcurrentRequests  int
	MaxResponseBytes       int64
	MaxTotalResponseBytes  int64
	MaxEvidencePerFact     int
	MaxDiagnostics         int
	MaxTextBytes           int
	Timeout                time.Duration
}

// DefaultLimits returns conservative budgets for a small remote system.
func DefaultLimits() Limits {
	return Limits{
		MaxRepositories:        100,
		MaxRelationships:       1_000,
		MaxProviderPages:       20,
		MaxRepositoriesPerPage: 100,
		MaxProviderRequests:    500,
		MaxConcurrentRequests:  4,
		MaxResponseBytes:       4 * 1024 * 1024,
		MaxTotalResponseBytes:  64 * 1024 * 1024,
		MaxEvidencePerFact:     20,
		MaxDiagnostics:         100,
		MaxTextBytes:           16 * 1024,
		Timeout:                time.Minute,
	}
}

// LargeSystemLimits returns starting budgets for large polyrepository systems.
func LargeSystemLimits() Limits {
	return Limits{
		MaxRepositories:        5_000,
		MaxRelationships:       250_000,
		MaxProviderPages:       500,
		MaxRepositoriesPerPage: 100,
		MaxProviderRequests:    20_000,
		MaxConcurrentRequests:  16,
		MaxResponseBytes:       8 * 1024 * 1024,
		MaxTotalResponseBytes:  2 * 1024 * 1024 * 1024,
		MaxEvidencePerFact:     100,
		MaxDiagnostics:         2_000,
		MaxTextBytes:           64 * 1024,
		Timeout:                15 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxRepositories:        25_000,
		MaxRelationships:       2_000_000,
		MaxProviderPages:       5_000,
		MaxRepositoriesPerPage: 100,
		MaxProviderRequests:    250_000,
		MaxConcurrentRequests:  32,
		MaxResponseBytes:       16 * 1024 * 1024,
		MaxTotalResponseBytes:  16 * 1024 * 1024 * 1024,
		MaxEvidencePerFact:     500,
		MaxDiagnostics:         10_000,
		MaxTextBytes:           256 * 1024,
		Timeout:                time.Hour,
	}
}

// Validate checks that every budget is positive and internally consistent.
func (l Limits) Validate() error {
	if l.MaxRepositories <= 0 || l.MaxRelationships <= 0 || l.MaxProviderPages <= 0 ||
		l.MaxRepositoriesPerPage <= 0 || l.MaxProviderRequests <= 0 ||
		l.MaxConcurrentRequests <= 0 || l.MaxConcurrentRequests > l.MaxProviderRequests ||
		l.MaxResponseBytes <= 0 || l.MaxResponseBytes == math.MaxInt64 ||
		l.MaxTotalResponseBytes < l.MaxResponseBytes ||
		l.MaxTotalResponseBytes == math.MaxInt64 || l.MaxEvidencePerFact <= 0 ||
		l.MaxDiagnostics <= 0 || l.MaxTextBytes <= 0 || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	minimumPages := 1 + (l.MaxRepositories-1)/l.MaxRepositoriesPerPage
	if l.MaxProviderPages < minimumPages || l.MaxProviderRequests < l.MaxProviderPages {
		return ErrInvalidLimits
	}
	return nil
}
