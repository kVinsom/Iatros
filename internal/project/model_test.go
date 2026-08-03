package project

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDefaultLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	if limits.MaxEvidencePerMarker != 20 {
		t.Fatalf("MaxEvidencePerMarker = %d, want 20", limits.MaxEvidencePerMarker)
	}
	if err := limits.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDetectorRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	for _, value := range []int{-1, 0} {
		limits := Limits{MaxEvidencePerMarker: value}
		if err := limits.Validate(); !errors.Is(err, ErrInvalidLimits) {
			t.Fatalf("Validate() error = %v, want ErrInvalidLimits", err)
		}
		if _, err := NewDetector(limits); !errors.Is(err, ErrInvalidLimits) {
			t.Fatalf("NewDetector() error = %v, want ErrInvalidLimits", err)
		}
	}

	model, err := (Detector{}).Detect(t.Context(), Snapshot{})
	if !errors.Is(err, ErrInvalidLimits) {
		t.Fatalf("Detect() error = %v, want ErrInvalidLimits", err)
	}
	assertEmptyModel(t, model, false)
}

func TestDetectorModelsMonorepository(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(t.Context(), Snapshot{Files: []string{
		"README.md",
		"go.work",
		"services/api/go.mod",
		"services/worker/go.mod",
		"web/package.json",
		"infrastructure/Pulumi.yaml",
	}})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if model.Partial {
		t.Fatal("Partial = true, want false")
	}
	if len(model.Workspaces) != 1 || model.Workspaces[0].Root != "." ||
		!hasMarker(model.Workspaces[0].Markers, "go-workspace") {
		t.Fatalf("Workspaces = %#v, want root Go workspace", model.Workspaces)
	}
	wantRoots := []string{
		"infrastructure",
		"services/api",
		"services/worker",
		"web",
	}
	if len(model.Projects) != len(wantRoots) {
		t.Fatalf("Projects = %#v, want roots %#v", model.Projects, wantRoots)
	}
	for index, project := range model.Projects {
		if project.Root != wantRoots[index] || project.WorkspaceRoot != "." {
			t.Fatalf("Project[%d] = %#v, want root %q in root workspace", index, project, wantRoots[index])
		}
	}
	if model.Projects[0].Kind != KindInfrastructure ||
		!hasMarker(model.Projects[0].Markers, "pulumi-project") {
		t.Fatalf("infrastructure project = %#v, want Pulumi infrastructure", model.Projects[0])
	}
	for _, project := range model.Projects[1:] {
		if project.Kind != KindCode {
			t.Fatalf("project = %#v, want code kind", project)
		}
	}
}

func TestDetectorUsesNearestWorkspaceAndMixedKind(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(t.Context(), Snapshot{Files: []string{
		"go.mod",
		"go.work",
		"platform/go.work",
		"platform/pnpm-workspace.yaml",
		"platform/service/go.mod",
		"platform/service/serverless.yml",
		"web/package.json",
	}})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(model.Workspaces) != 2 || model.Workspaces[0].Root != "." ||
		model.Workspaces[1].Root != "platform" {
		t.Fatalf("Workspaces = %#v, want root and platform", model.Workspaces)
	}
	if len(model.Workspaces[1].Markers) != 2 ||
		model.Workspaces[1].Markers[0].ID != "go-workspace" ||
		model.Workspaces[1].Markers[1].ID != "pnpm-workspace" {
		t.Fatalf("platform markers = %#v, want sorted Go and pnpm markers", model.Workspaces[1].Markers)
	}

	service := findProject(t, model, "platform/service")
	if service.Kind != KindMixed || service.WorkspaceRoot != "platform" ||
		!hasMarker(service.Markers, "go-module") ||
		!hasMarker(service.Markers, "serverless-project") {
		t.Fatalf("service = %#v, want mixed project in nearest workspace", service)
	}
	root := findProject(t, model, ".")
	if root.Kind != KindCode || root.WorkspaceRoot != "." {
		t.Fatalf("root project = %#v, want code project in root workspace", root)
	}
	web := findProject(t, model, "web")
	if web.WorkspaceRoot != "." {
		t.Fatalf("web project = %#v, want root workspace", web)
	}
}

func TestDetectorBoundsMarkerEvidence(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, Limits{MaxEvidencePerMarker: 2})
	model, err := detector.Detect(t.Context(), Snapshot{Files: []string{
		"infra/c.tf",
		"infra/a.tf",
		"infra/b.tf",
	}})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	project := findProject(t, model, "infra")
	marker := findMarker(t, project.Markers, "terraform-module")
	if !slices.Equal(marker.Evidence, []string{"infra/a.tf", "infra/b.tf"}) ||
		!marker.EvidenceTruncated {
		t.Fatalf("marker = %#v, want first two sorted paths and truncation", marker)
	}
}

func TestDetectorIgnoresWeakAndGeneratedEvidence(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(t.Context(), Snapshot{Files: []string{
		"Dockerfile",
		"Makefile",
		"deployment.yaml",
		"requirements.txt",
		"template.yaml",
		".git/generated/go.mod",
		".hg/generated/package.json",
		".svn/generated/main.tf",
		"node_modules/dependency/package.json",
		".terraform/modules/network/main.tf",
		".venv/library/package.json",
	}})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	assertEmptyModel(t, model, false)
}

func TestProjectMarkerCatalog(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	seen := make(map[string]struct{}, len(projectMarkerRules))
	for _, rule := range projectMarkerRules {
		if _, duplicate := seen[rule.id]; duplicate {
			t.Fatalf("duplicate project marker ID %q", rule.id)
		}
		seen[rule.id] = struct{}{}
		if !validTestMarkerID(rule.id) {
			t.Fatalf("project marker ID %q is invalid", rule.id)
		}
		if rule.kind != KindCode && rule.kind != KindInfrastructure {
			t.Fatalf("project marker %q has invalid kind %q", rule.id, rule.kind)
		}
		if len(rule.patterns) == 0 {
			t.Fatalf("project marker %q has no patterns", rule.id)
		}
		assertValidPatterns(t, rule.id, rule.patterns)

		t.Run(rule.id, func(t *testing.T) {
			t.Parallel()

			fixture := patternFixture(rule.patterns[0])
			model, err := detector.Detect(t.Context(), Snapshot{
				Files: []string{"fixture/" + fixture},
			})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			project := findProject(t, model, "fixture")
			if !hasMarker(project.Markers, rule.id) {
				t.Fatalf("Markers = %#v, want %q for %q", project.Markers, rule.id, fixture)
			}
		})
	}
}

func TestWorkspaceMarkerCatalog(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	seen := make(map[string]struct{}, len(workspaceMarkerRules))
	for _, rule := range workspaceMarkerRules {
		if _, duplicate := seen[rule.id]; duplicate {
			t.Fatalf("duplicate workspace marker ID %q", rule.id)
		}
		seen[rule.id] = struct{}{}
		if !validTestMarkerID(rule.id) {
			t.Fatalf("workspace marker ID %q is invalid", rule.id)
		}
		if len(rule.patterns) == 0 {
			t.Fatalf("workspace marker %q has no patterns", rule.id)
		}
		assertValidPatterns(t, rule.id, rule.patterns)

		t.Run(rule.id, func(t *testing.T) {
			t.Parallel()

			fixture := patternFixture(rule.patterns[0])
			model, err := detector.Detect(t.Context(), Snapshot{
				Files: []string{"fixture/" + fixture},
			})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if len(model.Workspaces) != 1 || model.Workspaces[0].Root != "fixture" ||
				!hasMarker(model.Workspaces[0].Markers, rule.id) {
				t.Fatalf("Workspaces = %#v, want marker %q", model.Workspaces, rule.id)
			}
		})
	}
}

func TestDetectorIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	files := []string{
		"services/api/serverless.yml",
		"go.work",
		"services/api/go.mod",
		"services/api/go.mod",
		"web/package.json",
	}
	original := slices.Clone(files)
	reversed := slices.Clone(files)
	slices.Reverse(reversed)
	detector := newTestDetector(t, DefaultLimits())

	first, err := detector.Detect(t.Context(), Snapshot{Files: files})
	if err != nil {
		t.Fatalf("first Detect() error = %v", err)
	}
	second, err := detector.Detect(t.Context(), Snapshot{Files: reversed})
	if err != nil {
		t.Fatalf("second Detect() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("models differ:\nfirst: %#v\nsecond: %#v", first, second)
	}
	if !slices.Equal(files, original) {
		t.Fatalf("input mutated: got %#v, want %#v", files, original)
	}
}

func TestDetectorPreservesPartialState(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(t.Context(), Snapshot{
		Files:   []string{"service/go.mod"},
		Partial: true,
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !model.Partial || len(model.Projects) != 1 {
		t.Fatalf("model = %#v, want partial model with detected boundary", model)
	}
}

func TestDetectorRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	detector := newTestDetector(t, DefaultLimits())
	for _, unsafePath := range []string{
		"",
		".",
		"../go.mod",
		"service/../go.mod",
		"/private/go.mod",
		`C:\private\go.mod`,
		`service\go.mod`,
		" go.mod",
		"go.mod\nforged",
		string([]byte{0xff}),
	} {
		model, err := detector.Detect(t.Context(), Snapshot{Files: []string{unsafePath}})
		if !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("Detect(%q) error = %v, want ErrInvalidSnapshot", unsafePath, err)
		}
		assertEmptyModel(t, model, false)
	}
}

func TestDetectorHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(ctx, Snapshot{Files: []string{"go.mod"}, Partial: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Detect() error = %v, want context.Canceled", err)
	}
	assertEmptyModel(t, model, true)
}

func TestDetectorDiscardsResultsAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx := &cancelAfterChecksContext{
		Context:         t.Context(),
		checksRemaining: 5,
	}
	detector := newTestDetector(t, DefaultLimits())
	model, err := detector.Detect(ctx, Snapshot{Files: []string{
		"go.work",
		"service/go.mod",
		"web/package.json",
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Detect() error = %v, want context.Canceled", err)
	}
	assertEmptyModel(t, model, false)
}

func BenchmarkDetectorTwoThousandFiles(b *testing.B) {
	files := make([]string, 0, 2_000)
	for index := range 500 {
		files = append(files,
			fmt.Sprintf("services/service-%03d/go.mod", index),
			fmt.Sprintf("services/service-%03d/main.go", index),
			fmt.Sprintf("infrastructure/module-%03d/main.tf", index),
			fmt.Sprintf("documentation/page-%03d.md", index),
		)
	}
	detector := newTestDetector(b, DefaultLimits())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := detector.Detect(b.Context(), Snapshot{Files: files}); err != nil {
			b.Fatal(err)
		}
	}
}

func newTestDetector(t testing.TB, limits Limits) Detector {
	t.Helper()

	detector, err := NewDetector(limits)
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}
	return detector
}

func findProject(t testing.TB, model Model, root string) Project {
	t.Helper()

	for _, project := range model.Projects {
		if project.Root == root {
			return project
		}
	}
	t.Fatalf("project root %q not found in %#v", root, model.Projects)
	return Project{}
}

func findMarker(t testing.TB, markers []Marker, id string) Marker {
	t.Helper()

	for _, marker := range markers {
		if marker.ID == id {
			return marker
		}
	}
	t.Fatalf("marker %q not found in %#v", id, markers)
	return Marker{}
}

func hasMarker(markers []Marker, id string) bool {
	for _, marker := range markers {
		if marker.ID == id {
			return true
		}
	}
	return false
}

func patternFixture(pattern string) string {
	return strings.ReplaceAll(pattern, "*", "sample")
}

func assertValidPatterns(t testing.TB, markerID string, patterns []string) {
	t.Helper()

	for _, pattern := range patterns {
		fixture := patternFixture(pattern)
		matched, err := path.Match(pattern, fixture)
		if err != nil || !matched {
			t.Fatalf("marker %q pattern %q does not match fixture %q: %v", markerID, pattern, fixture, err)
		}
	}
}

func validTestMarkerID(id string) bool {
	if id == "" || id != strings.ToLower(id) || strings.Trim(id, "-") != id ||
		strings.Contains(id, "--") {
		return false
	}
	for _, character := range id {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func assertEmptyModel(t testing.TB, model Model, partial bool) {
	t.Helper()

	if model.Projects == nil || model.Workspaces == nil ||
		len(model.Projects) != 0 || len(model.Workspaces) != 0 || model.Partial != partial {
		t.Fatalf("model = %#v, want non-nil empty model with Partial %t", model, partial)
	}
}

type cancelAfterChecksContext struct {
	context.Context
	checksRemaining int
}

func (ctx *cancelAfterChecksContext) Err() error {
	if ctx.checksRemaining == 0 {
		return context.Canceled
	}
	ctx.checksRemaining--
	return nil
}
