package artifactvalidation

import (
	"errors"
	"math"
	"time"
)

const minimumTextBytes = 256

// ErrInvalidLimits indicates an unusable artifact-validation limit profile.
var ErrInvalidLimits = errors.New("artifact validation limits are invalid")

// Limits bound artifact I/O, parser complexity, validators, diagnostics, and execution time.
type Limits struct {
	MaxArtifacts              int
	MaxArtifactBytes          int64
	MaxTotalBytes             int64
	MaxValidators             int
	MaxDiagnosticsPerArtifact int
	MaxDiagnostics            int
	MaxSyntaxNodes            int
	MaxNestingDepth           int
	MaxAliases                int
	MaxTextBytes              int
	Timeout                   time.Duration
}

// DefaultLimits returns conservative budgets for small and typical repositories.
func DefaultLimits() Limits {
	return Limits{
		MaxArtifacts:              500,
		MaxArtifactBytes:          2 * 1024 * 1024,
		MaxTotalBytes:             32 * 1024 * 1024,
		MaxValidators:             100,
		MaxDiagnosticsPerArtifact: 100,
		MaxDiagnostics:            1_000,
		MaxSyntaxNodes:            250_000,
		MaxNestingDepth:           128,
		MaxAliases:                1_000,
		MaxTextBytes:              16 * 1024,
		Timeout:                   15 * time.Second,
	}
}

// LargeRepositoryLimits returns starting budgets for large monorepositories.
func LargeRepositoryLimits() Limits {
	return Limits{
		MaxArtifacts:              25_000,
		MaxArtifactBytes:          8 * 1024 * 1024,
		MaxTotalBytes:             2 * 1024 * 1024 * 1024,
		MaxValidators:             1_000,
		MaxDiagnosticsPerArtifact: 1_000,
		MaxDiagnostics:            100_000,
		MaxSyntaxNodes:            1_000_000,
		MaxNestingDepth:           256,
		MaxAliases:                10_000,
		MaxTextBytes:              64 * 1024,
		Timeout:                   2 * time.Minute,
	}
}

// EnterpriseLimits returns per-worker starting budgets for calibrated company installations.
func EnterpriseLimits() Limits {
	return Limits{
		MaxArtifacts:              100_000,
		MaxArtifactBytes:          16 * 1024 * 1024,
		MaxTotalBytes:             16 * 1024 * 1024 * 1024,
		MaxValidators:             5_000,
		MaxDiagnosticsPerArtifact: 5_000,
		MaxDiagnostics:            500_000,
		MaxSyntaxNodes:            4_000_000,
		MaxNestingDepth:           512,
		MaxAliases:                50_000,
		MaxTextBytes:              256 * 1024,
		Timeout:                   10 * time.Minute,
	}
}

// Validate checks that every budget is positive and internally consistent.
func (l Limits) Validate() error {
	if l.MaxArtifacts <= 0 || l.MaxArtifactBytes <= 0 || l.MaxArtifactBytes == math.MaxInt64 ||
		l.MaxTotalBytes < l.MaxArtifactBytes || l.MaxTotalBytes == math.MaxInt64 ||
		l.MaxValidators <= 0 || l.MaxDiagnosticsPerArtifact <= 0 || l.MaxDiagnostics <= 0 ||
		l.MaxDiagnostics < l.MaxDiagnosticsPerArtifact || l.MaxSyntaxNodes <= 0 ||
		l.MaxNestingDepth <= 0 || l.MaxAliases <= 0 || l.MaxTextBytes < minimumTextBytes ||
		int64(l.MaxTextBytes) > l.MaxArtifactBytes || l.Timeout <= 0 {
		return ErrInvalidLimits
	}
	return nil
}
