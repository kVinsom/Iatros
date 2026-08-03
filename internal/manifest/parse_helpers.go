package manifest

import (
	"context"
	"errors"
	"io"
	"strings"
)

var errInvalidManifest = errors.New("manifest content is invalid")

func readDocument(ctx context.Context, document Document) ([]byte, error) {
	if document.Reader == nil || document.Size < 0 {
		return nil, errInvalidManifest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(document.Reader)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func appendMapDependencies(
	destination []Dependency,
	values map[string]string,
	scope Scope,
	optional bool,
	skip map[string]struct{},
) []Dependency {
	for name, constraint := range values {
		if _, excluded := skip[name]; excluded {
			continue
		}
		destination = append(destination, Dependency{
			Name:       name,
			Constraint: constraint,
			Scope:      scope,
			Optional:   optional,
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
	scope Scope,
	optional bool,
) ([]Dependency, error) {
	for _, declaration := range declarations {
		name, constraint, err := dependencyNameAndConstraint(declaration)
		if err != nil {
			return nil, err
		}
		destination = append(destination, Dependency{
			Name:       name,
			Constraint: constraint,
			Scope:      scope,
			Optional:   optional,
		})
	}
	return destination, nil
}
