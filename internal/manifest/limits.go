package manifest

import (
	"errors"
	"math"
	"time"
)

var (
	// ErrInvalidLimits indicates an unusable manifest-analysis limit profile.
	ErrInvalidLimits = errors.New("manifest limits are invalid")
	// ErrInvalidSnapshot indicates unsafe repository-relative input paths.
	ErrInvalidSnapshot = errors.New("manifest snapshot is invalid")
	// ErrInvalidParser indicates an incomplete or conflicting parser registry.
	ErrInvalidParser = errors.New("manifest parser is invalid")
	// ErrInvalidSource indicates that no document source was provided.
	ErrInvalidSource = errors.New("manifest source is invalid")
)

// Limits bound manifest I/O, retained values, parser nesting, and diagnostics.
type Limits struct {
	MaxFiles                        int
	MaxFileBytes                    int64
	MaxTotalBytes                   int64
	MaxDependenciesPerManifest      int
	MaxConstraintsPerManifest       int
	MaxWorkspaceMembersPerManifest  int
	MaxWorkspaceExcludesPerManifest int
	MaxIssues                       int
	MaxNestingDepth                 int
	MaxValueBytes                   int
	Timeout                         time.Duration
}

// DefaultLimits returns a conservative profile suitable for local analysis.
func DefaultLimits() Limits {
	return Limits{
		MaxFiles:                        100,
		MaxFileBytes:                    256 * 1024,
		MaxTotalBytes:                   4 * 1024 * 1024,
		MaxDependenciesPerManifest:      1_000,
		MaxConstraintsPerManifest:       100,
		MaxWorkspaceMembersPerManifest:  500,
		MaxWorkspaceExcludesPerManifest: 500,
		MaxIssues:                       50,
		MaxNestingDepth:                 64,
		MaxValueBytes:                   4 * 1024,
		Timeout:                         3 * time.Second,
	}
}

// LargeRepositoryLimits returns a validated starting profile for large monorepositories.
// Callers may supply a different validated profile when measured workloads require it.
func LargeRepositoryLimits() Limits {
	return Limits{
		MaxFiles:                        2_000,
		MaxFileBytes:                    8 * 1024 * 1024,
		MaxTotalBytes:                   256 * 1024 * 1024,
		MaxDependenciesPerManifest:      10_000,
		MaxConstraintsPerManifest:       1_000,
		MaxWorkspaceMembersPerManifest:  5_000,
		MaxWorkspaceExcludesPerManifest: 5_000,
		MaxIssues:                       200,
		MaxNestingDepth:                 128,
		MaxValueBytes:                   64 * 1024,
		Timeout:                         30 * time.Second,
	}
}

// Validate checks that the profile has positive, internally consistent bounds.
func (l Limits) Validate() error {
	if l.MaxFiles <= 0 || l.MaxFileBytes <= 0 || l.MaxTotalBytes < l.MaxFileBytes ||
		l.MaxFileBytes == math.MaxInt64 ||
		l.MaxDependenciesPerManifest <= 0 || l.MaxConstraintsPerManifest <= 0 ||
		l.MaxWorkspaceMembersPerManifest <= 0 || l.MaxIssues <= 0 ||
		l.MaxWorkspaceExcludesPerManifest <= 0 ||
		l.MaxNestingDepth <= 0 || l.MaxValueBytes <= 0 || int64(l.MaxValueBytes) > l.MaxFileBytes ||
		l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
