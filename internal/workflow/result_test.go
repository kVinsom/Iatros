package workflow

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestResultValidateAcceptsEveryTerminalOutcome(t *testing.T) {
	t.Parallel()

	outcomes := []Outcome{
		OutcomeCompleted,
		OutcomePartial,
		OutcomeBlocked,
		OutcomeUnavailable,
		OutcomeFailed,
		OutcomeCancelled,
		OutcomeDenied,
		OutcomeRolledBack,
		OutcomeIndeterminate,
		OutcomeSkipped,
	}

	for _, outcome := range outcomes {
		outcome := outcome
		t.Run(string(outcome), func(t *testing.T) {
			t.Parallel()

			result := validResult()
			result.Outcome = outcome
			if outcome != OutcomeCompleted {
				result.Diagnostics = []Diagnostic{
					{Code: "IATROS_STAGE_OUTCOME", Level: DiagnosticLevelWarning, Message: "The stage did not complete."},
				}
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestResultValidateRejectsInvalidEnvelopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Result[string])
	}{
		{name: "schema", mutate: func(result *Result[string]) { result.SchemaVersion = "2.0" }},
		{name: "result id", mutate: func(result *Result[string]) { result.ID = "RESULT" }},
		{name: "run id", mutate: func(result *Result[string]) { result.RunID = "" }},
		{name: "stage", mutate: func(result *Result[string]) { result.Stage = "review" }},
		{name: "outcome", mutate: func(result *Result[string]) { result.Outcome = "success" }},
		{name: "zero start", mutate: func(result *Result[string]) { result.StartedAt = time.Time{} }},
		{name: "zero finish", mutate: func(result *Result[string]) { result.FinishedAt = time.Time{} }},
		{name: "non utc", mutate: func(result *Result[string]) {
			result.StartedAt = result.StartedAt.In(time.FixedZone("EEST", 3*60*60))
		}},
		{name: "reverse time", mutate: func(result *Result[string]) {
			result.FinishedAt = result.StartedAt.Add(-time.Nanosecond)
		}},
		{name: "input order", mutate: func(result *Result[string]) {
			result.Inputs[0], result.Inputs[1] = result.Inputs[1], result.Inputs[0]
		}},
		{name: "input kind", mutate: func(result *Result[string]) { result.Inputs[0].Kind = "Project" }},
		{name: "input id", mutate: func(result *Result[string]) { result.Inputs[0].ID = " project" }},
		{name: "input schema", mutate: func(result *Result[string]) {
			result.Inputs[0].SchemaVersion = "latest"
		}},
		{name: "input digest", mutate: func(result *Result[string]) {
			result.Inputs[0].Digest = "sha256:XYZ"
		}},
		{name: "duplicate output", mutate: func(result *Result[string]) {
			result.Outputs = append(result.Outputs, result.Outputs[0])
		}},
		{name: "missing diagnostic", mutate: func(result *Result[string]) {
			result.Outcome = OutcomePartial
		}},
		{name: "diagnostic code", mutate: func(result *Result[string]) {
			result.Diagnostics = []Diagnostic{
				{Code: "stage.warning", Level: DiagnosticLevelWarning, Message: "A warning."},
			}
		}},
		{name: "diagnostic level", mutate: func(result *Result[string]) {
			result.Diagnostics = []Diagnostic{
				{Code: "IATROS_STAGE_WARNING", Level: "critical", Message: "A warning."},
			}
		}},
		{name: "diagnostic message", mutate: func(result *Result[string]) {
			result.Diagnostics = []Diagnostic{
				{Code: "IATROS_STAGE_WARNING", Level: DiagnosticLevelWarning, Message: " warning"},
			}
		}},
		{name: "completed error", mutate: func(result *Result[string]) {
			result.Diagnostics = []Diagnostic{
				{Code: "IATROS_STAGE_ERROR", Level: DiagnosticLevelError, Message: "An error."},
			}
		}},
		{name: "diagnostic order", mutate: func(result *Result[string]) {
			result.Diagnostics = []Diagnostic{
				{Code: "IATROS_Z_WARNING", Level: DiagnosticLevelWarning, Message: "Last."},
				{Code: "IATROS_A_WARNING", Level: DiagnosticLevelWarning, Message: "First."},
			}
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := validResult()
			test.mutate(&result)
			if err := result.Validate(); !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("Validate() error = %v, want ErrInvalidResult", err)
			}
		})
	}
}

func TestResultNormalizedCanonicalizesAndDetachesEnvelope(t *testing.T) {
	t.Parallel()

	result := validResult()
	zone := time.FixedZone("EEST", 3*60*60)
	result.StartedAt = result.StartedAt.In(zone)
	result.FinishedAt = result.FinishedAt.In(zone)
	result.Inputs[0], result.Inputs[1] = result.Inputs[1], result.Inputs[0]
	result.Diagnostics = []Diagnostic{
		{Code: "IATROS_Z_WARNING", Level: DiagnosticLevelWarning, Message: "Last."},
		{Code: "IATROS_A_WARNING", Level: DiagnosticLevelWarning, Message: "First."},
	}

	original := result
	normalized := result.Normalized()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if normalized.StartedAt.Location() != time.UTC || normalized.FinishedAt.Location() != time.UTC {
		t.Fatal("Normalized() did not convert timestamps to UTC")
	}
	if reflect.DeepEqual(normalized, original) {
		t.Fatal("Normalized() did not canonicalize the envelope")
	}

	normalized.Inputs[0].ID = "changed"
	normalized.Diagnostics[0].Message = "Changed."
	if result.Inputs[0].ID == "changed" || result.Diagnostics[0].Message == "Changed." {
		t.Fatal("Normalized() retained an input collection")
	}
}

func TestResultNormalizedInitializesEmptyCollections(t *testing.T) {
	t.Parallel()

	result := validResult()
	result.Inputs = nil
	result.Outputs = nil
	result.Diagnostics = nil
	result = result.Normalized()

	if result.Inputs == nil || result.Outputs == nil || result.Diagnostics == nil {
		t.Fatal("Normalized() left a collection nil")
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResultJSONRoundTripPreservesEnvelope(t *testing.T) {
	t.Parallel()

	want := validResult().Normalized()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Result[string]
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip result = %#v, want %#v", got, want)
	}
}

func validResult() Result[string] {
	startedAt := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)

	return Result[string]{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "result-0001",
		RunID:         "run-0001",
		Stage:         StageAnalyze,
		Outcome:       OutcomeCompleted,
		StartedAt:     startedAt,
		FinishedAt:    startedAt.Add(time.Second),
		Inputs: []ArtifactReference{
			{
				Kind:          "project",
				ID:            "iatros",
				SchemaVersion: "1.0",
			},
			{
				Kind:   "snapshot",
				ID:     "repository-0001",
				Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
		Outputs: []ArtifactReference{
			{
				Kind:          "analysis",
				ID:            "analysis-0001",
				SchemaVersion: "1.0",
				Digest:        "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			},
		},
		Diagnostics: []Diagnostic{},
		Data:        "complete",
	}
}
