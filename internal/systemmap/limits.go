package systemmap

import (
	"errors"
	"time"
)

// ErrInvalidLimits indicates an unusable system-map limit profile.
var ErrInvalidLimits = errors.New("system map limits are invalid")

// Limits bound retained system entities, relationships, evidence, text, and execution time.
type Limits struct {
	MaxRepositories          int
	MaxServices              int
	MaxLibraries             int
	MaxInfrastructure        int
	MaxEnvironments          int
	MaxOwners                int
	MaxExternalResources     int
	MaxRelationships         int
	MaxEnvironmentsPerEntity int
	MaxEvidencePerFact       int
	MaxDiagnostics           int
	MaxTextBytes             int
	Timeout                  time.Duration
}

// DefaultLimits returns conservative budgets for a small or typical local system.
func DefaultLimits() Limits {
	return Limits{
		MaxRepositories:          200,
		MaxServices:              1_000,
		MaxLibraries:             2_000,
		MaxInfrastructure:        2_000,
		MaxEnvironments:          100,
		MaxOwners:                500,
		MaxExternalResources:     2_000,
		MaxRelationships:         10_000,
		MaxEnvironmentsPerEntity: 100,
		MaxEvidencePerFact:       20,
		MaxDiagnostics:           100,
		MaxTextBytes:             16 * 1024,
		Timeout:                  10 * time.Second,
	}
}

// LargeSystemLimits returns starting budgets for large monorepositories and polyrepositories.
func LargeSystemLimits() Limits {
	return Limits{
		MaxRepositories:          10_000,
		MaxServices:              50_000,
		MaxLibraries:             100_000,
		MaxInfrastructure:        250_000,
		MaxEnvironments:          2_000,
		MaxOwners:                25_000,
		MaxExternalResources:     250_000,
		MaxRelationships:         1_000_000,
		MaxEnvironmentsPerEntity: 2_000,
		MaxEvidencePerFact:       100,
		MaxDiagnostics:           2_000,
		MaxTextBytes:             64 * 1024,
		Timeout:                  2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxRepositories:          50_000,
		MaxServices:              250_000,
		MaxLibraries:             500_000,
		MaxInfrastructure:        1_000_000,
		MaxEnvironments:          10_000,
		MaxOwners:                100_000,
		MaxExternalResources:     1_000_000,
		MaxRelationships:         5_000_000,
		MaxEnvironmentsPerEntity: 10_000,
		MaxEvidencePerFact:       500,
		MaxDiagnostics:           10_000,
		MaxTextBytes:             256 * 1024,
		Timeout:                  10 * time.Minute,
	}
}

// Validate checks that every budget is positive.
func (l Limits) Validate() error {
	if l.MaxRepositories <= 0 || l.MaxServices <= 0 || l.MaxLibraries <= 0 ||
		l.MaxInfrastructure <= 0 || l.MaxEnvironments <= 0 || l.MaxOwners <= 0 ||
		l.MaxExternalResources <= 0 || l.MaxRelationships <= 0 ||
		l.MaxEnvironmentsPerEntity <= 0 || l.MaxEvidencePerFact <= 0 ||
		l.MaxDiagnostics <= 0 || l.MaxTextBytes <= 0 || l.Timeout <= 0 {
		return ErrInvalidLimits
	}

	return nil
}
