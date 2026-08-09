package project

import (
	"context"
	"path"
	"slices"
	"strings"
)

type markerBuilder struct {
	evidence          []string
	evidenceTruncated bool
}

type boundaryBuilder struct {
	markers map[string]*markerBuilder
}

type projectBuilder struct {
	boundaryBuilder
	hasCode           bool
	hasInfrastructure bool
}

func addProjectMarker(
	projects map[string]*projectBuilder,
	root string,
	rule markerRule,
	evidence string,
	maxEvidence int,
) {
	builder, exists := projects[root]
	if !exists {
		builder = &projectBuilder{
			boundaryBuilder: boundaryBuilder{markers: make(map[string]*markerBuilder)},
		}
		projects[root] = builder
	}
	switch rule.kind {
	case KindCode:
		builder.hasCode = true
	case KindInfrastructure:
		builder.hasInfrastructure = true
	}
	addMarker(builder.markers, rule.id, evidence, maxEvidence)
}

func addBoundaryMarker(
	boundaries map[string]*boundaryBuilder,
	root string,
	markerID string,
	evidence string,
	maxEvidence int,
) {
	builder, exists := boundaries[root]
	if !exists {
		builder = &boundaryBuilder{markers: make(map[string]*markerBuilder)}
		boundaries[root] = builder
	}
	addMarker(builder.markers, markerID, evidence, maxEvidence)
}

func addMarker(
	markers map[string]*markerBuilder,
	markerID string,
	evidence string,
	maxEvidence int,
) {
	marker, exists := markers[markerID]
	if !exists {
		marker = &markerBuilder{evidence: make([]string, 0, min(maxEvidence, 4))}
		markers[markerID] = marker
	}
	if len(marker.evidence) < maxEvidence {
		marker.evidence = append(marker.evidence, evidence)
		return
	}
	marker.evidenceTruncated = true
}

func buildModel(
	ctx context.Context,
	projects map[string]*projectBuilder,
	workspaces map[string]*boundaryBuilder,
	partial bool,
) (Model, error) {
	model := emptyModel(partial)
	workspaceRoots := make(map[string]struct{}, len(workspaces))
	for root, builder := range workspaces {
		if err := ctx.Err(); err != nil {
			return emptyModel(partial), err
		}
		workspaceRoots[root] = struct{}{}
		model.Workspaces = append(model.Workspaces, Workspace{
			Root:    root,
			Markers: buildMarkers(builder.markers),
		})
	}
	for root, builder := range projects {
		if err := ctx.Err(); err != nil {
			return emptyModel(partial), err
		}
		model.Projects = append(model.Projects, Boundary{
			Root:          root,
			Kind:          projectKind(builder),
			WorkspaceRoot: nearestWorkspace(root, workspaceRoots),
			Markers:       buildMarkers(builder.markers),
		})
	}

	slices.SortFunc(model.Projects, func(left, right Boundary) int {
		return strings.Compare(left.Root, right.Root)
	})
	slices.SortFunc(model.Workspaces, func(left, right Workspace) int {
		return strings.Compare(left.Root, right.Root)
	})
	return model, nil
}

func buildMarkers(builders map[string]*markerBuilder) []Marker {
	markers := make([]Marker, 0, len(builders))
	for id, builder := range builders {
		markers = append(markers, Marker{
			ID:                id,
			Evidence:          builder.evidence,
			EvidenceTruncated: builder.evidenceTruncated,
		})
	}
	slices.SortFunc(markers, func(left, right Marker) int {
		return strings.Compare(left.ID, right.ID)
	})
	return markers
}

func projectKind(builder *projectBuilder) Kind {
	if builder.hasCode && builder.hasInfrastructure {
		return KindMixed
	}
	if builder.hasInfrastructure {
		return KindInfrastructure
	}
	return KindCode
}

func nearestWorkspace(root string, workspaceRoots map[string]struct{}) string {
	for candidate := root; ; candidate = path.Dir(candidate) {
		if _, exists := workspaceRoots[candidate]; exists {
			return candidate
		}
		if candidate == "." {
			return ""
		}
	}
}
