package finding

import (
	"errors"
	"testing"
	"time"
)

func TestBuiltInLimitsAreValidAndMonotonic(t *testing.T) {
	t.Parallel()

	profiles := []Limits{DefaultLimits(), LargeSystemLimits(), EnterpriseLimits()}
	for index, limits := range profiles {
		if err := limits.Validate(); err != nil {
			t.Fatalf("profile %d Validate() error = %v", index, err)
		}
	}
	for index := 1; index < len(profiles); index++ {
		previous := profiles[index-1]
		current := profiles[index]
		if current.MaxFindings <= previous.MaxFindings ||
			current.MaxSubjectsPerFinding <= previous.MaxSubjectsPerFinding ||
			current.MaxEvidencePerFinding <= previous.MaxEvidencePerFinding ||
			current.MaxActionsPerFinding <= previous.MaxActionsPerFinding ||
			current.MaxExclusions <= previous.MaxExclusions ||
			current.MaxTextBytes <= previous.MaxTextBytes || current.Timeout <= previous.Timeout {
			t.Fatalf("profile %d does not exceed profile %d", index, index-1)
		}
	}
}

func TestLimitsValidateRejectsEveryInvalidField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "findings", mutate: func(limits *Limits) { limits.MaxFindings = 0 }},
		{name: "subjects", mutate: func(limits *Limits) { limits.MaxSubjectsPerFinding = 0 }},
		{name: "evidence", mutate: func(limits *Limits) { limits.MaxEvidencePerFinding = 0 }},
		{name: "actions", mutate: func(limits *Limits) { limits.MaxActionsPerFinding = 0 }},
		{name: "exclusions", mutate: func(limits *Limits) { limits.MaxExclusions = 0 }},
		{name: "text", mutate: func(limits *Limits) { limits.MaxTextBytes = 0 }},
		{name: "timeout", mutate: func(limits *Limits) { limits.Timeout = 0 * time.Second }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			test.mutate(&limits)
			if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
			}
		})
	}
}

func TestValidateWithinEnforcesEveryCollectionAndTextLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Finding, *Limits)
	}{
		{name: "subjects", mutate: func(finding *Finding, limits *Limits) {
			limits.MaxSubjectsPerFinding = 1
			finding.Subjects = append(finding.Subjects, Subject{Kind: "service", ID: "api"})
		}},
		{name: "evidence", mutate: func(finding *Finding, limits *Limits) {
			limits.MaxEvidencePerFinding = 1
			finding.Evidence = append(finding.Evidence, Evidence{Kind: EvidenceObservation, Description: "Another observation."})
		}},
		{name: "actions", mutate: func(finding *Finding, limits *Limits) {
			limits.MaxActionsPerFinding = 1
			finding.Recommendation.Actions = append(finding.Recommendation.Actions, "Write tests.")
		}},
		{name: "text", mutate: func(finding *Finding, limits *Limits) { limits.MaxTextBytes = 4 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			finding := testFinding().Normalized()
			limits := DefaultLimits()
			test.mutate(&finding, &limits)
			if err := finding.ValidateWithin(limits); !errors.Is(err, ErrInvalidFinding) {
				t.Fatalf("ValidateWithin() error = %v, want ErrInvalidFinding", err)
			}
		})
	}
}
