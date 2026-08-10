package codeanalysis

import (
	"errors"
	"math"
	"time"
)

// ErrInvalidLimits indicates an unusable static code-analysis limit profile.
var ErrInvalidLimits = errors.New("code analysis limits are invalid")

// Limits bound source I/O, syntax processing, retained facts, and diagnostics.
type Limits struct {
	MaxFiles                int
	MaxFileBytes            int64
	MaxTotalBytes           int64
	MaxLineBytes            int
	MaxNestingDepth         int
	MaxSyntaxNodesPerFile   int
	MaxServices             int
	MaxFrameworks           int
	MaxPortBindings         int
	MaxAPIEndpoints         int
	MaxEnvironmentVariables int
	MaxResourceDependencies int
	MaxEvidencePerFact      int
	MaxDiagnostics          int
	MaxTextBytes            int
	Timeout                 time.Duration
}

// DefaultLimits returns conservative budgets for small and typical local repositories.
func DefaultLimits() Limits {
	return Limits{
		MaxFiles:                1_000,
		MaxFileBytes:            1024 * 1024,
		MaxTotalBytes:           64 * 1024 * 1024,
		MaxLineBytes:            64 * 1024,
		MaxNestingDepth:         256,
		MaxSyntaxNodesPerFile:   250_000,
		MaxServices:             500,
		MaxFrameworks:           500,
		MaxPortBindings:         2_000,
		MaxAPIEndpoints:         10_000,
		MaxEnvironmentVariables: 5_000,
		MaxResourceDependencies: 5_000,
		MaxEvidencePerFact:      20,
		MaxDiagnostics:          100,
		MaxTextBytes:            8 * 1024,
		Timeout:                 10 * time.Second,
	}
}

// LargeRepositoryLimits returns starting budgets for large monorepositories.
func LargeRepositoryLimits() Limits {
	return Limits{
		MaxFiles:                50_000,
		MaxFileBytes:            4 * 1024 * 1024,
		MaxTotalBytes:           2 * 1024 * 1024 * 1024,
		MaxLineBytes:            256 * 1024,
		MaxNestingDepth:         512,
		MaxSyntaxNodesPerFile:   1_000_000,
		MaxServices:             25_000,
		MaxFrameworks:           25_000,
		MaxPortBindings:         100_000,
		MaxAPIEndpoints:         500_000,
		MaxEnvironmentVariables: 250_000,
		MaxResourceDependencies: 250_000,
		MaxEvidencePerFact:      100,
		MaxDiagnostics:          2_000,
		MaxTextBytes:            64 * 1024,
		Timeout:                 2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxFiles:                250_000,
		MaxFileBytes:            8 * 1024 * 1024,
		MaxTotalBytes:           16 * 1024 * 1024 * 1024,
		MaxLineBytes:            1024 * 1024,
		MaxNestingDepth:         1_024,
		MaxSyntaxNodesPerFile:   4_000_000,
		MaxServices:             100_000,
		MaxFrameworks:           100_000,
		MaxPortBindings:         500_000,
		MaxAPIEndpoints:         5_000_000,
		MaxEnvironmentVariables: 1_000_000,
		MaxResourceDependencies: 1_000_000,
		MaxEvidencePerFact:      500,
		MaxDiagnostics:          10_000,
		MaxTextBytes:            256 * 1024,
		Timeout:                 10 * time.Minute,
	}
}

// Validate checks that every budget is positive and internally consistent.
func (l Limits) Validate() error {
	if l.MaxFiles <= 0 || l.MaxFileBytes <= 0 || l.MaxFileBytes == math.MaxInt64 ||
		l.MaxTotalBytes < l.MaxFileBytes || l.MaxTotalBytes == math.MaxInt64 ||
		l.MaxLineBytes <= 0 ||
		int64(l.MaxLineBytes) > l.MaxFileBytes || l.MaxNestingDepth <= 0 ||
		l.MaxSyntaxNodesPerFile <= 0 || l.MaxServices <= 0 || l.MaxFrameworks <= 0 ||
		l.MaxPortBindings <= 0 || l.MaxAPIEndpoints <= 0 ||
		l.MaxEnvironmentVariables <= 0 || l.MaxResourceDependencies <= 0 ||
		l.MaxEvidencePerFact <= 0 || l.MaxDiagnostics <= 0 || l.MaxTextBytes <= 0 ||
		l.MaxTextBytes > l.MaxLineBytes || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
