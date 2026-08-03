package manifest

import (
	"context"
	"io"
)

const (
	// FormatGoModule identifies a go.mod file.
	FormatGoModule Format = "go_module"
	// FormatGoWorkspace identifies a go.work file.
	FormatGoWorkspace Format = "go_workspace"
	// FormatNodePackage identifies a package.json file.
	FormatNodePackage Format = "node_package"
	// FormatPythonProject identifies a pyproject.toml file.
	FormatPythonProject Format = "python_project"
	// FormatRustPackage identifies a Cargo.toml file.
	FormatRustPackage Format = "rust_package"
	// FormatPHPComposer identifies a composer.json file.
	FormatPHPComposer Format = "php_composer"
	// FormatMavenProject identifies a pom.xml file.
	FormatMavenProject Format = "maven_project"
)

const (
	// ScopeRuntime identifies a dependency required by the running application.
	ScopeRuntime Scope = "runtime"
	// ScopeDevelopment identifies a dependency used for development or tests.
	ScopeDevelopment Scope = "development"
	// ScopeBuild identifies a dependency used to build or package a project.
	ScopeBuild Scope = "build"
	// ScopePeer identifies a dependency supplied by a consuming project.
	ScopePeer Scope = "peer"
)

// Format identifies a supported manifest grammar.
type Format string

// Scope is a normalized dependency relationship.
type Scope string

// Dependency records one direct dependency declaration.
type Dependency struct {
	Name       string
	Constraint string
	Scope      Scope
	Indirect   bool
	Optional   bool
}

// Constraint records a runtime, toolchain, or platform requirement.
type Constraint struct {
	Name  string
	Value string
	Scope Scope
}

// Manifest contains normalized declarations from one repository manifest.
type Manifest struct {
	Path                       string
	Format                     Format
	Name                       string
	Version                    string
	Module                     string
	Dependencies               []Dependency
	Constraints                []Constraint
	WorkspaceMembers           []string
	WorkspaceExcludes          []string
	WorkspaceDeclared          bool
	DependenciesTruncated      bool
	ConstraintsTruncated       bool
	WorkspaceMembersTruncated  bool
	WorkspaceExcludesTruncated bool
}

// Snapshot contains bounded repository paths that may include supported manifests.
type Snapshot struct {
	Files   []string
	Partial bool
}

// Document is a single open manifest owned by an Analyzer for the duration of parsing.
type Document struct {
	Path   string
	Size   int64
	Reader io.Reader
}

// Source opens repository-relative documents without exposing a host path to parsers.
type Source interface {
	Open(context.Context, string) (io.ReadCloser, int64, error)
}

// Parser translates one manifest grammar into the normalized model.
//
// Implementations must consume the complete document, honor context cancellation through
// Document.Reader, and must not execute repository code or resolve external resources.
type Parser interface {
	Format() Format
	Filenames() []string
	Parse(context.Context, Document, Limits) (Manifest, error)
}

// Issue describes a bounded, non-fatal omission without exposing file contents.
type Issue struct {
	Code    string
	Path    string
	Message string
}

// Result contains deterministic normalized manifests and bounded issues.
type Result struct {
	Manifests []Manifest
	Issues    []Issue
	Partial   bool
}
