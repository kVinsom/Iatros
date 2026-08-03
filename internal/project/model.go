// Package project identifies provider-neutral project and workspace boundaries.
package project

import (
	"context"
	"errors"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
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

// Project identifies one code, infrastructure, or mixed project root.
type Project struct {
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
	Projects   []Project
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
		if !validRepositoryFile(file) {
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
		Projects:   make([]Project, 0),
		Workspaces: make([]Workspace, 0),
		Partial:    partial,
	}
}

func validRepositoryFile(value string) bool {
	if !validText(value) || strings.Contains(value, "\\") || path.IsAbs(value) ||
		looksLikeWindowsPath(value) {
		return false
	}

	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." &&
		!strings.HasPrefix(cleaned, "../")
}

func validText(value string) bool {
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
