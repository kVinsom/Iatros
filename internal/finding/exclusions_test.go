package finding

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestApplyExclusionsUsesMostSpecificActiveRecord(t *testing.T) {
	t.Parallel()

	evaluatedAt := testEvaluationTime()
	findings := []Finding{testFinding()}
	exclusions := []Exclusion{
		testExclusion("rule.app", "", "repository.readme.missing", ExclusionScope{RepositoryID: "app"}),
		testExclusion("finding.exact", findings[0].ID, "", ExclusionScope{}),
		testExclusion("rule.other", "", "repository.readme.missing", ExclusionScope{RepositoryID: "other"}),
	}

	model, err := ApplyExclusions(t.Context(), findings, exclusions, evaluatedAt, DefaultLimits())
	if err != nil {
		t.Fatalf("ApplyExclusions() error = %v", err)
	}
	if len(model.Findings) != 1 || model.Findings[0].Disposition != DispositionExcluded ||
		model.Findings[0].Exclusion == nil || model.Findings[0].Exclusion.ID != "finding.exact" {
		t.Fatalf("finding = %#v, want exact exclusion", model.Findings)
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyExclusionsMatchesAllScopeDimensions(t *testing.T) {
	t.Parallel()

	finding := testFinding()
	finding.Subjects = append(finding.Subjects, Subject{
		Kind: "service", ID: "api", RepositoryID: "app", EnvironmentID: "production",
	})
	finding.Evidence = append(finding.Evidence, Evidence{
		Kind: EvidenceRepositoryFile, Description: "The insecure option is enabled.",
		RepositoryID: "app", Path: "deploy/production/app.yaml",
	})
	exclusion := testExclusion("rule.scoped", "", finding.RuleID, ExclusionScope{
		RepositoryID: "app", EnvironmentID: "production", SubjectKind: "service",
		SubjectID: "api", PathPrefix: "deploy/production",
	})

	model, err := ApplyExclusions(
		t.Context(), []Finding{finding}, []Exclusion{exclusion}, testEvaluationTime(), DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("ApplyExclusions() error = %v", err)
	}
	if model.Findings[0].Disposition != DispositionExcluded {
		t.Fatalf("finding = %#v, want matching scoped exclusion", model.Findings[0])
	}
}

func TestApplyExclusionsDoesNotApplyInactiveOrMismatchedRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Exclusion)
	}{
		{name: "expired", mutate: func(exclusion *Exclusion) { exclusion.ExpiresAt = testEvaluationTime() }},
		{name: "future", mutate: func(exclusion *Exclusion) {
			exclusion.CreatedAt = testEvaluationTime().Add(time.Hour)
			exclusion.ExpiresAt = testEvaluationTime().Add(2 * time.Hour)
		}},
		{name: "repository", mutate: func(exclusion *Exclusion) { exclusion.Scope.RepositoryID = "other" }},
		{name: "environment", mutate: func(exclusion *Exclusion) { exclusion.Scope.EnvironmentID = "production" }},
		{name: "subject", mutate: func(exclusion *Exclusion) { exclusion.Scope.SubjectKind = "service"; exclusion.Scope.SubjectID = "api" }},
		{name: "path", mutate: func(exclusion *Exclusion) { exclusion.Scope.PathPrefix = "deploy" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exclusion := testExclusion("finding.exact", testFinding().ID, "", ExclusionScope{})
			test.mutate(&exclusion)
			model, err := ApplyExclusions(
				t.Context(), []Finding{testFinding()}, []Exclusion{exclusion},
				testEvaluationTime(), DefaultLimits(),
			)
			if err != nil {
				t.Fatalf("ApplyExclusions() error = %v", err)
			}
			if model.Findings[0].Disposition != DispositionActive || model.Findings[0].Exclusion != nil {
				t.Fatalf("finding = %#v, want active", model.Findings[0])
			}
		})
	}
}

func TestExclusionValidateRejectsUncontrolledRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Exclusion)
	}{
		{name: "id", mutate: func(exclusion *Exclusion) { exclusion.ID = "" }},
		{name: "no target", mutate: func(exclusion *Exclusion) { exclusion.FindingID = "" }},
		{name: "two targets", mutate: func(exclusion *Exclusion) { exclusion.RuleID = "repository.readme.missing" }},
		{name: "global rule", mutate: func(exclusion *Exclusion) { exclusion.FindingID = ""; exclusion.RuleID = "repository.readme.missing" }},
		{name: "partial subject", mutate: func(exclusion *Exclusion) { exclusion.Scope.SubjectKind = "service" }},
		{name: "unsafe path", mutate: func(exclusion *Exclusion) { exclusion.Scope.PathPrefix = "../deploy" }},
		{name: "reason", mutate: func(exclusion *Exclusion) { exclusion.Reason = "" }},
		{name: "requester", mutate: func(exclusion *Exclusion) { exclusion.RequestedBy = "" }},
		{name: "approver", mutate: func(exclusion *Exclusion) { exclusion.ApprovedBy = "" }},
		{name: "created", mutate: func(exclusion *Exclusion) { exclusion.CreatedAt = time.Time{} }},
		{name: "unbounded", mutate: func(exclusion *Exclusion) { exclusion.ExpiresAt = time.Time{} }},
		{name: "reversed lifetime", mutate: func(exclusion *Exclusion) { exclusion.ExpiresAt = exclusion.CreatedAt.Add(-time.Minute) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exclusion := testExclusion("finding.exact", testFinding().ID, "", ExclusionScope{})
			test.mutate(&exclusion)
			if err := exclusion.Validate(); !errors.Is(err, ErrInvalidExclusion) {
				t.Fatalf("Validate() error = %v, want ErrInvalidExclusion", err)
			}
		})
	}
}

func TestApplyExclusionsRejectsDuplicateRecordsAndPreExcludedInput(t *testing.T) {
	t.Parallel()

	exclusion := testExclusion("finding.exact", testFinding().ID, "", ExclusionScope{})
	_, err := ApplyExclusions(
		t.Context(), []Finding{testFinding()}, []Exclusion{exclusion, exclusion},
		testEvaluationTime(), DefaultLimits(),
	)
	if !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("duplicate ApplyExclusions() error = %v, want ErrInvalidModel", err)
	}

	preExcluded := testFinding()
	preExcluded.Disposition = DispositionExcluded
	preExcluded.Exclusion = &AppliedExclusion{
		ID: exclusion.ID, Reason: exclusion.Reason, RequestedBy: exclusion.RequestedBy,
		ApprovedBy: exclusion.ApprovedBy, AppliedAt: testEvaluationTime(), ExpiresAt: exclusion.ExpiresAt,
	}
	_, err = ApplyExclusions(
		t.Context(), []Finding{preExcluded}, []Exclusion{exclusion},
		testEvaluationTime(), DefaultLimits(),
	)
	if !errors.Is(err, ErrInvalidFinding) {
		t.Fatalf("pre-excluded ApplyExclusions() error = %v, want ErrInvalidFinding", err)
	}
}

func TestModelValidateRequiresDeterministicallySelectedExclusion(t *testing.T) {
	t.Parallel()

	evaluatedAt := testEvaluationTime()
	exact := testExclusion("finding.exact", testFinding().ID, "", ExclusionScope{})
	rule := testExclusion(
		"rule.app", "", testFinding().RuleID, ExclusionScope{RepositoryID: "app"},
	)
	model, err := ApplyExclusions(
		t.Context(), []Finding{testFinding()}, []Exclusion{rule, exact},
		evaluatedAt, DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("ApplyExclusions() error = %v", err)
	}

	active := model
	active.Findings = cloneSlice(model.Findings)
	active.Findings[0].Disposition = DispositionActive
	active.Findings[0].Exclusion = nil
	if err := active.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("active Validate() error = %v, want ErrInvalidModel", err)
	}

	lessSpecific := model
	lessSpecific.Findings = cloneSlice(model.Findings)
	lessSpecific.Findings[0].Exclusion = &AppliedExclusion{
		ID: rule.ID, Reason: rule.Reason, RequestedBy: rule.RequestedBy,
		ApprovedBy: rule.ApprovedBy, AppliedAt: evaluatedAt, ExpiresAt: rule.ExpiresAt,
	}
	if err := lessSpecific.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("less-specific Validate() error = %v, want ErrInvalidModel", err)
	}
}

func TestApplyExclusionsNormalizesZeroOffsetTimestampsToUTC(t *testing.T) {
	t.Parallel()

	zeroOffset := time.FixedZone("zero", 0)
	evaluatedAt := time.Date(2026, time.August, 14, 12, 0, 0, 0, zeroOffset)
	exclusion := testExclusion("finding.exact", testFinding().ID, "", ExclusionScope{})
	exclusion.CreatedAt = exclusion.CreatedAt.In(zeroOffset)
	exclusion.ExpiresAt = exclusion.ExpiresAt.In(zeroOffset)

	model, err := ApplyExclusions(
		t.Context(), []Finding{testFinding()}, []Exclusion{exclusion},
		evaluatedAt, DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("ApplyExclusions() error = %v", err)
	}
	if model.EvaluatedAt.Location() != time.UTC || model.Exclusions[0].CreatedAt.Location() != time.UTC ||
		model.Findings[0].Exclusion.AppliedAt.Location() != time.UTC {
		t.Fatalf("model = %#v, want canonical UTC timestamps", model)
	}
}

func TestApplyExclusionsHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	model, err := ApplyExclusions(ctx, []Finding{testFinding()}, nil, testEvaluationTime(), DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyExclusions() error = %v, want context.Canceled", err)
	}
	if model.Findings == nil || model.Exclusions == nil {
		t.Fatalf("model = %#v, want normalized empty collections", model)
	}
}

func BenchmarkApplyExclusions(b *testing.B) {
	evaluatedAt := testEvaluationTime()
	findings := make([]Finding, 100)
	exclusions := make([]Exclusion, 100)
	for index := range findings {
		findings[index] = testFinding()
		findings[index].ID = fmt.Sprintf("repository.readme.missing.repo-%03d", index)
		exclusions[index] = testExclusion(
			fmt.Sprintf("finding.exact-%03d", index),
			findings[index].ID,
			"",
			ExclusionScope{},
		)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ApplyExclusions(
			context.Background(), findings, exclusions, evaluatedAt, DefaultLimits(),
		); err != nil {
			b.Fatalf("ApplyExclusions() error = %v", err)
		}
	}
}

func testExclusion(
	id string,
	findingID string,
	ruleID string,
	scope ExclusionScope,
) Exclusion {
	return Exclusion{
		ID: id, FindingID: findingID, RuleID: ruleID, Scope: scope,
		Reason:      "The accepted migration window is still active.",
		RequestedBy: "developer@example.test", ApprovedBy: "owner@example.test",
		CreatedAt: testEvaluationTime().Add(-time.Hour),
		ExpiresAt: testEvaluationTime().Add(24 * time.Hour),
	}
}

func testEvaluationTime() time.Time {
	return time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
}
