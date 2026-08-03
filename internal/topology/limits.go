package topology

import (
	"errors"
	"time"
)

var (
	// ErrInvalidLimits indicates an unusable topology limit profile.
	ErrInvalidLimits = errors.New("topology limits are invalid")
	// ErrInvalidSnapshot indicates malformed boundary or manifest input.
	ErrInvalidSnapshot = errors.New("topology snapshot is invalid")
)

// Limits bound retained topology facts and association work.
type Limits struct {
	MaxProjects              int
	MaxWorkspaces            int
	MaxManifests             int
	MaxComponentsPerBoundary int
	MaxWorkspaceDeclarations int
	MaxMatchesPerDeclaration int
	MaxDependencies          int
	MaxTargetsPerDependency  int
	MaxIssues                int
	MaxValueBytes            int
	Timeout                  time.Duration
}

// DefaultLimits returns a conservative topology profile for local analysis.
func DefaultLimits() Limits {
	return Limits{
		MaxProjects:              2_000,
		MaxWorkspaces:            500,
		MaxManifests:             100,
		MaxComponentsPerBoundary: 20,
		MaxWorkspaceDeclarations: 2_000,
		MaxMatchesPerDeclaration: 500,
		MaxDependencies:          20_000,
		MaxTargetsPerDependency:  20,
		MaxIssues:                100,
		MaxValueBytes:            4 * 1024,
		Timeout:                  3 * time.Second,
	}
}

// LargeRepositoryLimits returns a starting profile for large company monorepositories.
func LargeRepositoryLimits() Limits {
	return Limits{
		MaxProjects:              100_000,
		MaxWorkspaces:            10_000,
		MaxManifests:             2_000,
		MaxComponentsPerBoundary: 100,
		MaxWorkspaceDeclarations: 100_000,
		MaxMatchesPerDeclaration: 10_000,
		MaxDependencies:          1_000_000,
		MaxTargetsPerDependency:  100,
		MaxIssues:                1_000,
		MaxValueBytes:            64 * 1024,
		Timeout:                  60 * time.Second,
	}
}

// Validate checks that every topology limit is positive.
func (l Limits) Validate() error {
	if l.MaxProjects <= 0 || l.MaxWorkspaces <= 0 || l.MaxManifests <= 0 ||
		l.MaxComponentsPerBoundary <= 0 || l.MaxWorkspaceDeclarations <= 0 ||
		l.MaxMatchesPerDeclaration <= 0 || l.MaxDependencies <= 0 ||
		l.MaxTargetsPerDependency <= 0 || l.MaxIssues <= 0 || l.MaxValueBytes <= 0 ||
		l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
