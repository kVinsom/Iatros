// Package detection identifies repository technologies from direct local evidence.
package detection

import (
	"context"
	"errors"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

const initialEvidenceCapacity = 4

const (
	// CategoryLanguage identifies a programming language.
	CategoryLanguage Category = "language"
	// CategoryRuntime identifies an application runtime.
	CategoryRuntime Category = "runtime"
	// CategoryDependencyManager identifies dependency-management tooling.
	CategoryDependencyManager Category = "dependency_manager"
	// CategoryBuildSystem identifies build orchestration tooling.
	CategoryBuildSystem Category = "build_system"
	// CategoryContainer identifies container build or development tooling.
	CategoryContainer Category = "container"
	// CategoryOrchestration identifies workload orchestration tooling.
	CategoryOrchestration Category = "orchestration"
	// CategoryInfrastructureAsCode identifies infrastructure provisioning tooling.
	CategoryInfrastructureAsCode Category = "infrastructure_as_code"
	// CategoryConfigurationManagement identifies host configuration tooling.
	CategoryConfigurationManagement Category = "configuration_management"
	// CategoryCICD identifies continuous integration or delivery tooling.
	CategoryCICD Category = "ci_cd"
	// CategoryGitOps identifies Git-driven deployment tooling.
	CategoryGitOps Category = "gitops"
	// CategoryObservability identifies telemetry collection or analysis tooling.
	CategoryObservability Category = "observability"
	// CategoryNetworking identifies proxies, gateways, or service networking tooling.
	CategoryNetworking Category = "networking"
	// CategorySecurity identifies security and supply-chain tooling.
	CategorySecurity Category = "security"
	// CategorySecrets identifies secrets-management tooling.
	CategorySecrets Category = "secrets"
	// CategoryCloudPlatform identifies cloud-specific project tooling.
	CategoryCloudPlatform Category = "cloud_platform"
)

// ErrInvalidLimits indicates unusable marker-detection limits.
var ErrInvalidLimits = errors.New("detection limits are invalid")

// ErrInvalidEvidencePath indicates an unsafe or non-relative evidence path.
var ErrInvalidEvidencePath = errors.New("detection evidence path is invalid")

// Category groups detected technologies by their primary repository role.
type Category string

// Limits bound evidence retained by marker detection.
type Limits struct {
	MaxEvidencePerTechnology int
}

// DefaultLimits returns the conservative baseline marker-detection profile.
func DefaultLimits() Limits {
	return Limits{MaxEvidencePerTechnology: 20}
}

// Validate checks that marker-detection work has a usable evidence bound.
func (l Limits) Validate() error {
	if l.MaxEvidencePerTechnology <= 0 {
		return ErrInvalidLimits
	}
	return nil
}

// Technology records one detected technology and its direct file evidence.
type Technology struct {
	ID                string
	Category          Category
	Evidence          []string
	EvidenceTruncated bool
}

// MarkerDetector identifies technologies using the built-in filename catalog.
type MarkerDetector struct {
	limits Limits
	rules  []markerRule
}

// NewMarkerDetector creates a detector with the built-in technology catalog.
func NewMarkerDetector(limits Limits) (MarkerDetector, error) {
	if err := limits.Validate(); err != nil {
		return MarkerDetector{}, err
	}
	return MarkerDetector{limits: limits, rules: defaultMarkerRules}, nil
}

// Detect returns deterministic technology evidence without reading file content.
func (d MarkerDetector) Detect(ctx context.Context, files []string) ([]Technology, error) {
	results := make([]Technology, 0)
	if err := d.limits.Validate(); err != nil {
		return results, err
	}
	if err := ctx.Err(); err != nil {
		return results, err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if !validEvidencePath(file) {
			return results, ErrInvalidEvidencePath
		}
	}

	files = normalizedPaths(files)
	detected := make(map[string]*Technology)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return make([]Technology, 0), err
		}
		if repositorypath.IsExcludedFile(file) {
			continue
		}
		for _, rule := range d.rules {
			if !rule.matches(file) {
				continue
			}

			technology, exists := detected[rule.id]
			if !exists {
				technology = &Technology{
					ID:       rule.id,
					Category: rule.category,
					Evidence: make([]string, 0, min(
						d.limits.MaxEvidencePerTechnology,
						initialEvidenceCapacity,
					)),
				}
				detected[rule.id] = technology
			}
			if len(technology.Evidence) < d.limits.MaxEvidencePerTechnology {
				technology.Evidence = append(technology.Evidence, file)
			} else {
				technology.EvidenceTruncated = true
			}
		}
	}

	results = make([]Technology, 0, len(detected))
	for _, technology := range detected {
		results = append(results, *technology)
	}
	slices.SortFunc(results, func(left, right Technology) int {
		return strings.Compare(left.ID, right.ID)
	})
	return results, nil
}

func normalizedPaths(paths []string) []string {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	return slices.Compact(paths)
}

func validEvidencePath(value string) bool {
	if !validEvidenceText(value) || strings.Contains(value, "\\") || path.IsAbs(value) ||
		looksLikeWindowsPath(value) {
		return false
	}

	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." &&
		!strings.HasPrefix(cleaned, "../")
}

func validEvidenceText(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func looksLikeWindowsPath(value string) bool {
	return len(value) >= 2 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':'
}
