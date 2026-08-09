package manifest

import (
	"context"
	"errors"
	"io"
	"strings"
)

var errInvalidManifest = errors.New("manifest content is invalid")

type dependencyDeclarationOptions struct {
	scope         Scope
	isOptional    bool
	excludedNames map[string]struct{}
}

func readDocument(ctx context.Context, document Document) ([]byte, error) {
	if document.Reader == nil || document.Size < 0 {
		return nil, errInvalidManifest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := io.ReadAll(document.Reader)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return content, nil
}

func appendMapDependencies(
	destination []Dependency,
	declarations map[string]string,
	options dependencyDeclarationOptions,
) []Dependency {
	for name, constraint := range declarations {
		if _, isExcluded := options.excludedNames[name]; isExcluded {
			continue
		}
		destination = append(destination, Dependency{
			Name:       name,
			Constraint: constraint,
			Scope:      options.scope,
			Optional:   options.isOptional,
		})
	}
	return destination
}

func dependencyNameAndConstraint(declaration string) (string, string, error) {
	declaration = strings.TrimSpace(declaration)
	if declaration == "" {
		return "", "", errInvalidManifest
	}

	boundary := strings.IndexAny(declaration, "[<>=!~;@ ")
	if boundary < 0 {
		return declaration, "", nil
	}
	name := strings.TrimSpace(declaration[:boundary])
	if name == "" {
		return "", "", errInvalidManifest
	}
	return name, strings.TrimSpace(declaration[boundary:]), nil
}

func appendRequirementList(
	destination []Dependency,
	declarations []string,
	options dependencyDeclarationOptions,
) ([]Dependency, error) {
	for _, declaration := range declarations {
		name, constraint, err := dependencyNameAndConstraint(declaration)
		if err != nil {
			return nil, err
		}
		destination = append(destination, Dependency{
			Name:       name,
			Constraint: constraint,
			Scope:      options.scope,
			Optional:   options.isOptional,
		})
	}
	return destination, nil
}
