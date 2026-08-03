package manifest

import (
	"context"

	"golang.org/x/mod/modfile"
)

type goModParser struct{}

func (goModParser) Format() Format { return FormatGoModule }

func (goModParser) Filenames() []string { return []string{"go.mod"} }

func (goModParser) Parse(ctx context.Context, document Document, _ Limits) (Manifest, error) {
	data, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	file, err := modfile.ParseLax(document.Path, data, nil)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{}
	if file.Module != nil {
		manifest.Name = file.Module.Mod.Path
		manifest.Module = file.Module.Mod.Path
	}
	if file.Go != nil {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "go", Value: file.Go.Version, Scope: ScopeBuild,
		})
	}
	if file.Toolchain != nil {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "go-toolchain", Value: file.Toolchain.Name, Scope: ScopeBuild,
		})
	}
	for _, requirement := range file.Require {
		manifest.Dependencies = append(manifest.Dependencies, Dependency{
			Name:       requirement.Mod.Path,
			Constraint: requirement.Mod.Version,
			Scope:      ScopeRuntime,
			Indirect:   requirement.Indirect,
		})
	}
	return manifest, nil
}

type goWorkParser struct{}

func (goWorkParser) Format() Format { return FormatGoWorkspace }

func (goWorkParser) Filenames() []string { return []string{"go.work"} }

func (goWorkParser) Parse(ctx context.Context, document Document, _ Limits) (Manifest, error) {
	data, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	file, err := modfile.ParseWork(document.Path, data, nil)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{WorkspaceDeclared: true}
	if file.Go != nil {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "go", Value: file.Go.Version, Scope: ScopeBuild,
		})
	}
	if file.Toolchain != nil {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "go-toolchain", Value: file.Toolchain.Name, Scope: ScopeBuild,
		})
	}
	for _, use := range file.Use {
		manifest.WorkspaceMembers = append(manifest.WorkspaceMembers, use.Path)
	}
	return manifest, nil
}
