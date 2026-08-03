package topology

import (
	"errors"
	"testing"
)

func TestLimitProfilesAreValidatedAndScalable(t *testing.T) {
	t.Parallel()

	defaultLimits := DefaultLimits()
	largeLimits := LargeRepositoryLimits()
	if err := defaultLimits.Validate(); err != nil {
		t.Fatalf("DefaultLimits().Validate() error = %v", err)
	}
	if err := largeLimits.Validate(); err != nil {
		t.Fatalf("LargeRepositoryLimits().Validate() error = %v", err)
	}
	if largeLimits.MaxProjects <= defaultLimits.MaxProjects ||
		largeLimits.MaxDependencies <= defaultLimits.MaxDependencies ||
		largeLimits.MaxNestedRepositories <= defaultLimits.MaxNestedRepositories ||
		largeLimits.MaxWorkspaceDeclarations <= defaultLimits.MaxWorkspaceDeclarations {
		t.Fatalf("large limits = %+v, default limits = %+v", largeLimits, defaultLimits)
	}

	invalid := defaultLimits
	invalid.MaxDependencies = 0
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
	}
	invalid = defaultLimits
	invalid.MaxNestedRepositories = 0
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
	}
}

func TestEmptyModelUsesNonNilCollections(t *testing.T) {
	t.Parallel()

	builder := mustBuilder(t, DefaultLimits())
	model, err := builder.Build(t.Context(), Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if model.Projects == nil || model.Workspaces == nil || model.Dependencies == nil ||
		model.NestedRepositories == nil ||
		model.Issues == nil || model.Partial {
		t.Fatalf("Build() = %+v", model)
	}
}
