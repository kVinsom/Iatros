// Package readiness evaluates evidence-based repository readiness rules.
package readiness

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/finding"
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
	SeverityInfo Severity = finding.SeverityInformational
	// SeverityWarning identifies an actionable readiness concern.
	SeverityWarning Severity = finding.SeverityMedium
	// SeverityCritical identifies a high-impact readiness concern.
	SeverityCritical Severity = finding.SeverityCritical
)

const (
	readinessProducer        = "iatros.readiness"
	readinessProducerVersion = "1.0"
	readinessRuleVersion     = "1.0"
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
type Severity = finding.Severity

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

// Finding is the unified evidence, provenance, risk, recommendation, and exclusion contract.
type Finding = finding.Finding

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
	return newFinding(
		FindingCodeReadmeMissing,
		"Root README is missing",
		"The repository does not contain a recognized root README file.",
		SeverityWarning,
		"A complete repository snapshot contains no recognized root README file.",
		finding.RiskMedium,
		finding.LikelihoodLikely,
		"Contributors and operators may not have verified project instructions.",
		"Document the project at the repository root.",
		"Add a root README that explains the project, its status, and verified usage.",
	)
}

func missingLicenseFinding() Finding {
	return newFinding(
		FindingCodeLicenseMissing,
		"Root license is missing",
		"The repository does not contain a recognized root license file.",
		SeverityInfo,
		"A complete repository snapshot contains no recognized root license file.",
		finding.RiskLow,
		finding.LikelihoodPossible,
		"Users may not know the terms under which the project can be used or distributed.",
		"State the project's usage and distribution terms.",
		"Add a root license file with the project's usage and distribution terms.",
	)
}

func missingGitignoreFinding() Finding {
	return newFinding(
		FindingCodeGitignoreMissing,
		"Root .gitignore is missing",
		"The repository does not contain a root .gitignore file.",
		SeverityInfo,
		"A complete repository snapshot contains no root .gitignore file.",
		finding.RiskLow,
		finding.LikelihoodPossible,
		"Generated, local, or sensitive artifacts may be committed unintentionally.",
		"Define repository-wide ignore rules.",
		"Add a root .gitignore for generated, local, and sensitive ecosystem artifacts.",
	)
}

func missingTestsFinding(technologyIDs []string) Finding {
	return newFinding(
		FindingCodeTestsNotDetected,
		"Automated tests were not detected",
		"Supported test markers were not detected for the identified code ecosystems.",
		SeverityWarning,
		"Detected code ecosystems: "+strings.Join(technologyIDs, ", ")+".",
		finding.RiskHigh,
		finding.LikelihoodLikely,
		"Regressions may reach users without automated detection.",
		"Add automated verification for the detected code ecosystems.",
		"Add tests and a recognized test layout for at least one detected code ecosystem.",
	)
}

func missingCIFinding() Finding {
	return newFinding(
		FindingCodeCINotDetected,
		"CI/CD configuration was not detected",
		"Supported CI/CD configuration was not detected.",
		SeverityWarning,
		"Repository technologies were detected, but no supported CI/CD marker was found.",
		finding.RiskHigh,
		finding.LikelihoodLikely,
		"Changes may be integrated without consistent build and test verification.",
		"Verify every proposed change in CI/CD.",
		"Add CI/CD configuration that builds and tests every proposed change.",
	)
}

func newFinding(
	ruleID string,
	title string,
	description string,
	severity Severity,
	evidenceDescription string,
	riskLevel finding.RiskLevel,
	likelihood finding.Likelihood,
	riskSummary string,
	recommendationSummary string,
	recommendationAction string,
) Finding {
	return Finding{
		ID:          ruleID + ".local",
		RuleID:      ruleID,
		Title:       title,
		Description: description,
		Severity:    severity,
		Confidence:  finding.ConfidenceHigh,
		Subjects: []finding.Subject{{
			Kind: "repository", ID: "local",
		}},
		Evidence: []finding.Evidence{{
			Kind: finding.EvidenceObservation, Description: evidenceDescription,
		}},
		Provenance: finding.Provenance{
			Producer: readinessProducer, ProducerVersion: readinessProducerVersion,
			RuleVersion: readinessRuleVersion, Source: "repository_snapshot",
		},
		Risk: finding.Risk{Level: riskLevel, Likelihood: likelihood, Summary: riskSummary},
		Recommendation: finding.Recommendation{
			Summary: recommendationSummary, Actions: []string{recommendationAction},
		},
		Disposition: finding.DispositionActive,
	}.Normalized()
}

func compareFindings(left, right Finding) int {
	if comparison := severityRank(left.Severity) - severityRank(right.Severity); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.RuleID, right.RuleID); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.ID, right.ID)
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
