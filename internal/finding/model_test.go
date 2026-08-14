package finding

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestFindingValidateAcceptsCompleteNormalizedFinding(t *testing.T) {
	t.Parallel()

	finding := testFinding().Normalized()
	if err := finding.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFindingValidateRejectsMissingOrInvalidDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Finding)
	}{
		{name: "id", mutate: func(finding *Finding) { finding.ID = "" }},
		{name: "unnamespaced id", mutate: func(finding *Finding) { finding.ID = "readme" }},
		{name: "rule", mutate: func(finding *Finding) { finding.RuleID = "Readme" }},
		{name: "unnamespaced rule", mutate: func(finding *Finding) { finding.RuleID = "readme" }},
		{name: "title", mutate: func(finding *Finding) { finding.Title = "" }},
		{name: "description", mutate: func(finding *Finding) { finding.Description = "bad\ntext" }},
		{name: "severity", mutate: func(finding *Finding) { finding.Severity = "warning" }},
		{name: "confidence", mutate: func(finding *Finding) { finding.Confidence = "certain" }},
		{name: "subjects", mutate: func(finding *Finding) { finding.Subjects = nil }},
		{name: "evidence", mutate: func(finding *Finding) { finding.Evidence = nil }},
		{name: "producer", mutate: func(finding *Finding) { finding.Provenance.Producer = "" }},
		{name: "producer version", mutate: func(finding *Finding) { finding.Provenance.ProducerVersion = "1" }},
		{name: "noncanonical version", mutate: func(finding *Finding) { finding.Provenance.RuleVersion = "01.0" }},
		{name: "risk", mutate: func(finding *Finding) { finding.Risk.Level = "severe" }},
		{name: "likelihood", mutate: func(finding *Finding) { finding.Risk.Likelihood = "sometimes" }},
		{name: "recommendation", mutate: func(finding *Finding) { finding.Recommendation.Summary = "" }},
		{name: "actions", mutate: func(finding *Finding) { finding.Recommendation.Actions = nil }},
		{name: "disposition", mutate: func(finding *Finding) { finding.Disposition = "ignored" }},
		{
			name: "active with exclusion",
			mutate: func(finding *Finding) {
				finding.Exclusion = &AppliedExclusion{}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			finding := testFinding().Normalized()
			test.mutate(&finding)
			if err := finding.Validate(); !errors.Is(err, ErrInvalidFinding) {
				t.Fatalf("Validate() error = %v, want ErrInvalidFinding", err)
			}
		})
	}
}

func TestEvidenceValidationCoversEveryRepresentationAndPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		evidence   Evidence
		isAccepted bool
	}{
		{
			name: "observation", isAccepted: true,
			evidence: Evidence{Kind: EvidenceObservation, Description: "A complete scan found no README."},
		},
		{
			name: "repository file", isAccepted: true,
			evidence: Evidence{Kind: EvidenceRepositoryFile, Description: "The setting is enabled.",
				RepositoryID: "app.repo", Path: "deploy/app.yaml", StartLine: 2, StartColumn: 1,
				EndLine: 2, EndColumn: 8},
		},
		{
			name: "runtime", isAccepted: true,
			evidence: Evidence{Kind: EvidenceRuntime, Description: "The probe failed.", Reference: "cluster.prod/probe.api"},
		},
		{
			name: "external", isAccepted: true,
			evidence: Evidence{Kind: EvidenceExternal, Description: "The provider reported a violation.", Reference: "provider.finding-42"},
		},
		{name: "unknown kind", evidence: Evidence{Kind: "unknown", Description: "Unknown."}},
		{name: "file without repository", evidence: Evidence{Kind: EvidenceRepositoryFile, Description: "File.", Path: "go.mod"}},
		{name: "unsafe path", evidence: Evidence{Kind: EvidenceRepositoryFile, Description: "File.", RepositoryID: "app", Path: "../go.mod"}},
		{name: "partial position", evidence: Evidence{Kind: EvidenceRepositoryFile, Description: "File.", RepositoryID: "app", Path: "go.mod", StartLine: 1}},
		{name: "runtime without reference", evidence: Evidence{Kind: EvidenceRuntime, Description: "Runtime."}},
		{name: "external URL reference", evidence: Evidence{Kind: EvidenceExternal, Description: "External.", Reference: "https://provider.test/finding"}},
		{name: "observation with reference", evidence: Evidence{Kind: EvidenceObservation, Description: "Observation.", Reference: "unexpected"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if actual := validEvidence(test.evidence); actual != test.isAccepted {
				t.Fatalf("validEvidence(%#v) = %t, want %t", test.evidence, actual, test.isAccepted)
			}
		})
	}
}

func TestFindingNormalizedDetachesSortsAndDeduplicatesCollections(t *testing.T) {
	t.Parallel()

	input := testFinding()
	input.Subjects = []Subject{
		{Kind: "service", ID: "web", RepositoryID: "app"},
		{Kind: "repository", ID: "local", RepositoryID: "app"},
		{Kind: "service", ID: "web", RepositoryID: "app"},
	}
	input.Evidence = []Evidence{
		{Kind: EvidenceObservation, Description: "Second observation."},
		{Kind: EvidenceObservation, Description: "First observation."},
		{Kind: EvidenceObservation, Description: "First observation."},
	}
	input.Recommendation.Actions = []string{"Write tests.", "Add CI.", "Write tests."}

	normalized := input.Normalized()
	if len(normalized.Subjects) != 2 || len(normalized.Evidence) != 2 ||
		!slices.Equal(normalized.Recommendation.Actions, []string{"Add CI.", "Write tests."}) {
		t.Fatalf("Normalized() = %#v, want sorted unique collections", normalized)
	}
	normalized.Subjects[0].ID = "changed"
	normalized.Evidence[0].Description = "Changed."
	normalized.Recommendation.Actions[0] = "Changed."
	if input.Subjects[0].ID == "changed" || input.Evidence[0].Description == "Changed." ||
		input.Recommendation.Actions[0] == "Changed." {
		t.Fatal("Normalized() mutated or retained aliases to input collections")
	}
}

func TestModelJSONRoundTripPreservesValidatedContract(t *testing.T) {
	t.Parallel()

	evaluatedAt := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	model, err := ApplyExclusions(t.Context(), []Finding{testFinding()}, nil, evaluatedAt, DefaultLimits())
	if err != nil {
		t.Fatalf("ApplyExclusions() error = %v", err)
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Model
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, model) {
		t.Fatalf("round trip changed model:\ngot  %#v\nwant %#v", decoded, model)
	}
}

func testFinding() Finding {
	return Finding{
		ID:          "repository.readme.missing.local",
		RuleID:      "repository.readme.missing",
		Title:       "Root README is missing",
		Description: "The repository does not contain a recognized root README file.",
		Severity:    SeverityMedium,
		Confidence:  ConfidenceHigh,
		Subjects: []Subject{{
			Kind: "repository", ID: "local", RepositoryID: "app",
		}},
		Evidence: []Evidence{{
			Kind: EvidenceObservation, Description: "A complete scan found no recognized root README file.",
		}},
		Provenance: Provenance{
			Producer: "iatros.readiness", ProducerVersion: "1.0", RuleVersion: "1.0", Source: "repository_snapshot",
		},
		Risk: Risk{
			Level: RiskMedium, Likelihood: LikelihoodLikely,
			Summary: "Contributors and operators may not have verified project instructions.",
		},
		Recommendation: Recommendation{
			Summary: "Document the project at the repository root.",
			Actions: []string{"Add a root README with project status and verified usage instructions."},
		},
		Disposition: DispositionActive,
	}
}
