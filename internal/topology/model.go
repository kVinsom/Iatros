package topology

import (
	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
)

const (
	// DependencyInternal identifies one unambiguous dependency target in the repository.
	DependencyInternal DependencyResolution = "internal"
	// DependencyUnresolved identifies a dependency with no known repository target.
	DependencyUnresolved DependencyResolution = "unresolved"
	// DependencyAmbiguous identifies a dependency matching multiple repository targets.
	DependencyAmbiguous DependencyResolution = "ambiguous"
)

const (
	// MemberMatched identifies a workspace declaration with at least one known project match.
	MemberMatched MemberResolution = "matched"
	// MemberUnmatched identifies a declaration with no match in a complete project snapshot.
	MemberUnmatched MemberResolution = "unmatched"
	// MemberIndeterminate identifies a declaration with no match in a partial project snapshot.
	MemberIndeterminate MemberResolution = "indeterminate"
	// MemberUnsupported identifies a declaration using unsupported pattern semantics.
	MemberUnsupported MemberResolution = "unsupported"
	// MemberOutsideRoot identifies a declaration that could escape the selected repository.
	MemberOutsideRoot MemberResolution = "outside_root"
)

// DependencyResolution describes how a direct dependency maps to repository projects.
type DependencyResolution string

// MemberResolution describes how a workspace declaration maps to repository projects.
type MemberResolution string

// Snapshot contains the normalized boundary and manifest results to associate.
type Snapshot struct {
	Projects           project.Model
	Manifests          manifest.Result
	NestedRepositories []string
	Issues             []Issue
}

// Marker records one project or workspace boundary signal.
type Marker struct {
	ID                string
	Evidence          []string
	EvidenceTruncated bool
}

// Constraint records a runtime, platform, or toolchain requirement.
type Constraint struct {
	Name  string
	Value string
	Scope string
}

// Component identifies one parsed manifest associated with a project or workspace.
type Component struct {
	ManifestPath               string
	Format                     string
	Name                       string
	Version                    string
	Module                     string
	Constraints                []Constraint
	WorkspaceDeclared          bool
	DependenciesTruncated      bool
	ConstraintsTruncated       bool
	WorkspaceMembersTruncated  bool
	WorkspaceExcludesTruncated bool
}

// Project is one code, infrastructure, or mixed repository boundary.
type Project struct {
	Root                 string
	Kind                 string
	PrimaryWorkspaceRoot string
	WorkspaceRoots       []string
	Markers              []Marker
	Components           []Component
}

// WorkspaceDeclaration records one safe manifest declaration and its known project matches.
type WorkspaceDeclaration struct {
	ManifestPath     string
	Pattern          string
	Exclude          bool
	Resolution       MemberResolution
	ProjectRoots     []string
	MatchesTruncated bool
}

// Workspace coordinates contained or explicitly declared projects.
type Workspace struct {
	Root              string
	Markers           []Marker
	Components        []Component
	ContainedProjects []string
	DeclaredProjects  []string
	ExcludedProjects  []string
	Declarations      []WorkspaceDeclaration
}

// Dependency records one direct dependency declaration and any repository-local targets.
type Dependency struct {
	FromProject      string
	ManifestPath     string
	Ecosystem        string
	Name             string
	Constraint       string
	Scope            string
	Indirect         bool
	Optional         bool
	Resolution       DependencyResolution
	TargetProjects   []string
	TargetsTruncated bool
}

// Issue describes a bounded topology omission or ambiguous relationship.
type Issue struct {
	Code    string
	Path    string
	Message string
}

// Model is the deterministic provider-neutral topology of one repository snapshot.
type Model struct {
	Projects           []Project
	Workspaces         []Workspace
	Dependencies       []Dependency
	NestedRepositories []string
	Issues             []Issue
	Partial            bool
}
