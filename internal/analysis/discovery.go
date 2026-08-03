package analysis

import (
	"context"
	"errors"
	"math"
	"time"
)

const (
	// DiscoveryIssuePathUnreadable identifies a path that could not be inspected.
	DiscoveryIssuePathUnreadable = "IATROS_DISCOVERY_PATH_UNREADABLE"
	// DiscoveryIssueUnsafePath identifies a path that cannot be represented safely.
	DiscoveryIssueUnsafePath = "IATROS_DISCOVERY_UNSAFE_PATH"
	// DiscoveryIssueDepthLimit identifies a directory skipped at the configured depth limit.
	DiscoveryIssueDepthLimit = "IATROS_DISCOVERY_DEPTH_LIMIT"
	// DiscoveryIssueFileLimit identifies a scan stopped at the configured file limit.
	DiscoveryIssueFileLimit = "IATROS_DISCOVERY_FILE_LIMIT"
	// DiscoveryIssueDirectoryLimit identifies a scan stopped at the configured directory limit.
	DiscoveryIssueDirectoryLimit = "IATROS_DISCOVERY_DIRECTORY_LIMIT"
	// DiscoveryIssueDirectoryEntryLimit identifies a directory that exceeded its entry limit.
	DiscoveryIssueDirectoryEntryLimit = "IATROS_DISCOVERY_DIRECTORY_ENTRY_LIMIT"
	// DiscoveryIssueIgnoreFileLimit identifies ignore files omitted by the configured limit.
	DiscoveryIssueIgnoreFileLimit = "IATROS_DISCOVERY_IGNORE_FILE_LIMIT"
	// DiscoveryIssueIgnoreFileTooLarge identifies an ignore file skipped before reading.
	DiscoveryIssueIgnoreFileTooLarge = "IATROS_DISCOVERY_IGNORE_FILE_TOO_LARGE"
	// DiscoveryIssueIgnoreReadFailed identifies an ignore file that could not be read safely.
	DiscoveryIssueIgnoreReadFailed = "IATROS_DISCOVERY_IGNORE_READ_FAILED"
	// DiscoveryIssueIgnoreRuleInvalid identifies malformed ignore patterns that were not applied.
	DiscoveryIssueIgnoreRuleInvalid = "IATROS_DISCOVERY_IGNORE_RULE_INVALID"
	// DiscoveryIssueIgnoreRuleLimit identifies rules omitted by the configured limit.
	DiscoveryIssueIgnoreRuleLimit = "IATROS_DISCOVERY_IGNORE_RULE_LIMIT"
	// DiscoveryIssueGitmodulesInvalid identifies an unsafe or malformed submodule declaration.
	DiscoveryIssueGitmodulesInvalid = "IATROS_DISCOVERY_GITMODULES_INVALID"
	// DiscoveryIssueNestedRepositoryLimit identifies nested repository boundaries omitted by limits.
	DiscoveryIssueNestedRepositoryLimit = "IATROS_DISCOVERY_NESTED_REPOSITORY_LIMIT"
	// DiscoveryIssueTimeout identifies a scan stopped at its internal time limit.
	DiscoveryIssueTimeout = "IATROS_DISCOVERY_TIMEOUT"
	// DiscoveryIssueLimit identifies omitted discovery issues.
	DiscoveryIssueLimit = "IATROS_DISCOVERY_ISSUE_LIMIT"
)

// ErrInvalidDiscoveryLimits indicates unusable local discovery limits.
var ErrInvalidDiscoveryLimits = errors.New("discovery limits are invalid")

// DiscoveryLimits bound local filesystem work and retained results.
type DiscoveryLimits struct {
	MaxFiles               int
	MaxDirectories         int
	MaxDepth               int
	MaxIssues              int
	MaxEntriesPerDirectory int
	MaxIgnoreFiles         int
	MaxControlFileBytes    int64
	MaxIgnorePatternBytes  int
	MaxIgnoreRules         int
	MaxNestedRepositories  int
	Timeout                time.Duration
}

// DefaultDiscoveryLimits returns the conservative baseline discovery profile.
func DefaultDiscoveryLimits() DiscoveryLimits {
	return DiscoveryLimits{
		MaxFiles:               2_000,
		MaxDirectories:         500,
		MaxDepth:               20,
		MaxIssues:              50,
		MaxEntriesPerDirectory: 2_500,
		MaxIgnoreFiles:         100,
		MaxControlFileBytes:    256 * 1024,
		MaxIgnorePatternBytes:  4 * 1024,
		MaxIgnoreRules:         10_000,
		MaxNestedRepositories:  100,
		Timeout:                5 * time.Second,
	}
}

// Validate checks that every discovery limit can safely bound work.
func (l DiscoveryLimits) Validate() error {
	if l.MaxFiles <= 0 ||
		l.MaxDirectories <= 0 ||
		l.MaxDepth < 0 ||
		l.MaxIssues <= 0 ||
		l.MaxEntriesPerDirectory <= 0 ||
		l.MaxIgnoreFiles <= 0 || l.MaxIgnoreFiles > l.MaxDirectories ||
		l.MaxControlFileBytes <= 0 || l.MaxControlFileBytes == math.MaxInt64 ||
		l.MaxIgnorePatternBytes <= 0 ||
		int64(l.MaxIgnorePatternBytes) > l.MaxControlFileBytes ||
		l.MaxIgnoreRules <= 0 ||
		l.MaxNestedRepositories <= 0 || l.MaxNestedRepositories > l.MaxDirectories ||
		l.Timeout <= 0 {
		return ErrInvalidDiscoveryLimits
	}
	return nil
}

// DiscoveryIssue describes a skipped path or a reached safety limit.
type DiscoveryIssue struct {
	Code    string
	Path    string
	Message string
}

// Inventory is a bounded, root-relative filesystem snapshot without file content.
type Inventory struct {
	Directories        []string
	Files              []string
	NestedRepositories []string
	Issues             []DiscoveryIssue
	Partial            bool
}

// LocalDiscovery inventories a selected local directory within explicit limits.
type LocalDiscovery struct {
	limits DiscoveryLimits
}

// NewLocalDiscovery creates a local discovery service with validated limits.
func NewLocalDiscovery(limits DiscoveryLimits) (LocalDiscovery, error) {
	if err := limits.Validate(); err != nil {
		return LocalDiscovery{}, err
	}
	return LocalDiscovery{limits: limits}, nil
}

// Discover inventories one local root and reads only bounded repository control files.
func (d LocalDiscovery) Discover(ctx context.Context, rootPath string) (Inventory, error) {
	inventory := emptyInventory()
	if err := d.limits.Validate(); err != nil {
		return inventory, err
	}
	if err := ctx.Err(); err != nil {
		return inventory, err
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, d.limits.Timeout)
	defer cancel()

	root, err := openLocalRoot(rootPath)
	if err != nil {
		return resolveDiscoveryFailure(
			ctx,
			discoveryCtx,
			inventory,
			d.limits.MaxIssues,
			err,
		)
	}

	inventory, discoveryErr := discoverFilesystem(discoveryCtx, root.FS(), d.limits)
	closeErr := root.Close()
	if discoveryErr != nil {
		inventory, err = resolveDiscoveryFailure(
			ctx,
			discoveryCtx,
			inventory,
			d.limits.MaxIssues,
			discoveryErr,
		)
		if err == nil && closeErr != nil {
			return inventory, closeErr
		}
		return inventory, err
	}
	if closeErr != nil {
		return inventory, closeErr
	}

	return inventory, nil
}

func emptyInventory() Inventory {
	return Inventory{
		Directories:        make([]string, 0),
		Files:              make([]string, 0),
		NestedRepositories: make([]string, 0),
		Issues:             make([]DiscoveryIssue, 0),
	}
}

func timeoutIssue() DiscoveryIssue {
	return DiscoveryIssue{
		Code:    DiscoveryIssueTimeout,
		Path:    ".",
		Message: "The discovery time limit was reached; remaining entries were skipped.",
	}
}

func resolveDiscoveryFailure(
	parent context.Context,
	discovery context.Context,
	inventory Inventory,
	maxIssues int,
	fallback error,
) (Inventory, error) {
	contextErr := discovery.Err()
	if contextErr == nil {
		return inventory, fallback
	}
	if parentErr := parent.Err(); parentErr != nil {
		return inventory, parentErr
	}
	if errors.Is(contextErr, context.DeadlineExceeded) {
		recordDiscoveryIssue(&inventory, maxIssues, timeoutIssue())
		finalizeInventory(&inventory)
		return inventory, nil
	}
	return inventory, contextErr
}
