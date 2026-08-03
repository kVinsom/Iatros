// Package readiness evaluates evidence-based repository readiness rules.
package readiness

import (
	"context"
	"errors"
	"slices"
	"strings"
)

const (
	// FindingCodeReadmeMissing identifies a missing root README file.
	FindingCodeReadmeMissing = "repository.readme.missing"
	// FindingCodeLicenseMissing identifies a missing root license file.
	FindingCodeLicenseMissing = "repository.license.missing"
	// FindingCodeGitignoreMissing identifies a missing root .gitignore file.
	FindingCodeGitignoreMissing = "repository.gitignore.missing"
	// FindingCodeTestsNotDetected identifies absent supported test markers.
	FindingCodeTestsNotDetected = "quality.tests.not_detected"
	// FindingCodeCINotDetected identifies absent supported CI/CD markers.
	FindingCodeCINotDetected = "delivery.ci.not_detected"
)

const (
	// SeverityInfo identifies an informational readiness observation.
	SeverityInfo Severity = "info"
	// SeverityWarning identifies an actionable readiness concern.
	SeverityWarning Severity = "warning"
	// SeverityCritical identifies a high-impact readiness concern.
	SeverityCritical Severity = "critical"
)

const (
	// TechnologyCategoryLanguage identifies a programming-language result.
	TechnologyCategoryLanguage TechnologyCategory = "language"
	// TechnologyCategoryRuntime identifies an application-runtime result.
	TechnologyCategoryRuntime TechnologyCategory = "runtime"
	// TechnologyCategoryCICD identifies a CI/CD result.
	TechnologyCategoryCICD TechnologyCategory = "ci_cd"
)

// ErrInvalidSnapshot indicates unsafe or malformed readiness input.
var ErrInvalidSnapshot = errors.New("readiness snapshot is invalid")

// Severity describes the importance of a readiness finding.
type Severity string

// TechnologyCategory identifies a technology's primary repository role.
type TechnologyCategory string

// Technology is the minimal detection result required by readiness rules.
type Technology struct {
	ID       string
	Category TechnologyCategory
}

// Snapshot contains the bounded repository evidence used by readiness rules.
type Snapshot struct {
	Directories  []string
	Files        []string
	Technologies []Technology
	Partial      bool
}

// Finding describes one evidence-based repository readiness observation.
type Finding struct {
	Code        string
	Severity    Severity
	Message     string
	Evidence    []string
	Remediation string
}

// Evaluator applies the built-in readiness rules without external side effects.
type Evaluator struct{}

// Evaluate returns deterministic findings for a complete repository snapshot.
func (Evaluator) Evaluate(ctx context.Context, snapshot Snapshot) ([]Finding, error) {
	findings := make([]Finding, 0)
	if err := ctx.Err(); err != nil {
		return findings, err
	}

	index, err := buildRepositoryIndex(ctx, snapshot)
	if err != nil {
		return findings, err
	}
	if snapshot.Partial {
		return findings, nil
	}

	if !index.hasRootReadme {
		findings = append(findings, missingReadmeFinding())
	}
	if !index.hasRootLicense {
		findings = append(findings, missingLicenseFinding())
	}
	if !index.hasRootGitignore {
		findings = append(findings, missingGitignoreFinding())
	}
	if len(index.codeTechnologies) > 0 && !index.hasTestMarker {
		findings = append(findings, missingTestsFinding(index.codeTechnologies))
	}
	if index.hasProjectTechnology && !index.hasCICD {
		findings = append(findings, missingCIFinding())
	}

	slices.SortFunc(findings, compareFindings)
	return findings, nil
}

func missingReadmeFinding() Finding {
	return Finding{
		Code:     FindingCodeReadmeMissing,
		Severity: SeverityWarning,
		Message:  "The repository does not contain a recognized root README file.",
		Evidence: []string{
			"No recognized README file was found at the repository root.",
		},
		Remediation: "Add a root README that explains the project, its status, " +
			"and verified usage.",
	}
}

func missingLicenseFinding() Finding {
	return Finding{
		Code:     FindingCodeLicenseMissing,
		Severity: SeverityInfo,
		Message:  "The repository does not contain a recognized root license file.",
		Evidence: []string{
			"No recognized license file was found at the repository root.",
		},
		Remediation: "Add a root license file that states the terms under which " +
			"the project may be used and distributed.",
	}
}

func missingGitignoreFinding() Finding {
	return Finding{
		Code:     FindingCodeGitignoreMissing,
		Severity: SeverityInfo,
		Message:  "The repository does not contain a root .gitignore file.",
		Evidence: []string{
			"No .gitignore file was found at the repository root.",
		},
		Remediation: "Add a root .gitignore file for generated, local, and " +
			"sensitive artifacts relevant to the detected ecosystems.",
	}
}

func missingTestsFinding(technologyIDs []string) Finding {
	return Finding{
		Code:     FindingCodeTestsNotDetected,
		Severity: SeverityWarning,
		Message:  "Supported test markers were not detected for the identified code ecosystems.",
		Evidence: []string{
			"Detected code ecosystems: " + strings.Join(technologyIDs, ", ") + ".",
		},
		Remediation: "Add automated tests and a recognized test layout for at " +
			"least one identified code ecosystem.",
	}
}

func missingCIFinding() Finding {
	return Finding{
		Code:     FindingCodeCINotDetected,
		Severity: SeverityWarning,
		Message:  "Supported CI/CD configuration was not detected.",
		Evidence: []string{
			"Repository technologies were detected, but no supported CI/CD marker was found.",
		},
		Remediation: "Add a CI/CD configuration that builds and tests the " +
			"repository on every proposed change.",
	}
}

func compareFindings(left, right Finding) int {
	if comparison := severityRank(left.Severity) - severityRank(right.Severity); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Code, right.Code); comparison != 0 {
		return comparison
	}
	return compareEvidence(left.Evidence, right.Evidence)
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func compareEvidence(left, right []string) int {
	for index := range min(len(left), len(right)) {
		if comparison := strings.Compare(left[index], right[index]); comparison != 0 {
			return comparison
		}
	}
	return len(left) - len(right)
}
