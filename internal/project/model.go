// Package project owns canonical operational project contracts and repository boundaries.
package project

import (
	"context"
	"errors"
	"path"
	"slices"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

const (
	// KindCode identifies a boundary established by a code or package manifest.
	KindCode Kind = "code"
	// KindInfrastructure identifies a boundary established by an infrastructure manifest.
	KindInfrastructure Kind = "infrastructure"
	// KindMixed identifies a boundary containing both code and infrastructure manifests.
	KindMixed Kind = "mixed"
)

var (
	// ErrInvalidLimits indicates unusable project-boundary limits.
	ErrInvalidLimits = errors.New("project detection limits are invalid")
	// ErrInvalidSnapshot indicates unsafe or malformed project-boundary input.
	ErrInvalidSnapshot = errors.New("project detection snapshot is invalid")
)

// Kind describes the direct marker classes found at a project root.
type Kind string

// Limits bound evidence retained by project-boundary detection.
type Limits struct {
	MaxEvidencePerMarker int
}

// DefaultLimits returns the conservative baseline project-detection profile.
func DefaultLimits() Limits {
	return Limits{MaxEvidencePerMarker: 20}
}

// Validate checks that project-boundary work has a usable evidence bound.
func (l Limits) Validate() error {
	if l.MaxEvidencePerMarker <= 0 {
		return ErrInvalidLimits
	}
	return nil
}

// Snapshot contains the bounded file evidence used to identify boundaries.
type Snapshot struct {
	Files   []string
	Partial bool
}

// Marker records one boundary signal and its direct file evidence.
type Marker struct {
	ID                string
	Evidence          []string
	EvidenceTruncated bool
}

// Boundary identifies one code, infrastructure, or mixed project root.
type Boundary struct {
	Root          string
	Kind          Kind
	WorkspaceRoot string
	Markers       []Marker
}

// Workspace identifies a repository location that coordinates nested projects.
type Workspace struct {
	Root    string
	Markers []Marker
}

// Model contains deterministic project and workspace boundaries.
type Model struct {
	Projects   []Boundary
	Workspaces []Workspace
	Partial    bool
}

// Detector identifies project and workspace roots from strong filename markers.
type Detector struct {
	limits Limits
}

// NewDetector creates a project-boundary detector with validated limits.
func NewDetector(limits Limits) (Detector, error) {
	if err := limits.Validate(); err != nil {
		return Detector{}, err
	}
	return Detector{limits: limits}, nil
}

// Detect returns boundaries without reading file contents or mutating the snapshot.
func (d Detector) Detect(ctx context.Context, snapshot Snapshot) (Model, error) {
	empty := emptyModel(snapshot.Partial)
	if err := d.limits.Validate(); err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	for _, file := range snapshot.Files {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if !repositorypath.IsValidFile(file) {
			return empty, ErrInvalidSnapshot
		}
	}

	files := slices.Clone(snapshot.Files)
	slices.Sort(files)
	files = slices.Compact(files)

	projects := make(map[string]*projectBuilder)
	workspaces := make(map[string]*boundaryBuilder)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if ignoredBoundaryEvidence(file) {
			continue
		}

		root := path.Dir(file)
		for _, rule := range projectMarkerRules {
			if rule.matches(file) {
				addProjectMarker(projects, root, rule, file, d.limits.MaxEvidencePerMarker)
			}
		}
		for _, rule := range workspaceMarkerRules {
			if rule.matches(file) {
				addBoundaryMarker(workspaces, root, rule.id, file, d.limits.MaxEvidencePerMarker)
			}
		}
	}

	model, err := buildModel(ctx, projects, workspaces, snapshot.Partial)
	if err != nil {
		return empty, err
	}
	return model, nil
}

func emptyModel(partial bool) Model {
	return Model{
		Projects:   make([]Boundary, 0),
		Workspaces: make([]Workspace, 0),
		Partial:    partial,
	}
}
