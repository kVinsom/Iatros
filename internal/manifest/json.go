package manifest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

func decodeStrictJSON(content []byte, maximumDepth int, destination any) error {
	if !utf8.Valid(content) {
		return errInvalidManifest
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, 1, maximumDepth); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errInvalidManifest
	}
	if err := json.Unmarshal(content, destination); err != nil {
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, depth, maximumDepth int) error {
	if depth > maximumDepth {
		return errInvalidManifest
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errInvalidManifest
			}
			foldedKey := foldJSONKey(key)
			if _, duplicate := keys[foldedKey]; duplicate {
				return errInvalidManifest
			}
			keys[foldedKey] = struct{}{}
			if err := scanJSONValue(decoder, depth+1, maximumDepth); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, depth+1, maximumDepth); err != nil {
				return err
			}
		}
	default:
		return errInvalidManifest
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if closing != json.Delim(map[json.Delim]rune{'{': '}', '[': ']'}[delimiter]) {
		return errInvalidManifest
	}
	return nil
}

func foldJSONKey(key string) string {
	ascii := true
	for index := range len(key) {
		if key[index] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(key)
	}

	var folded strings.Builder
	folded.Grow(len(key))
	for _, character := range key {
		canonical := unicode.ToLower(character)
		for candidate := unicode.SimpleFold(character); candidate != character; candidate = unicode.SimpleFold(candidate) {
			lowerCandidate := unicode.ToLower(candidate)
			if lowerCandidate < canonical {
				canonical = lowerCandidate
			}
		}
		folded.WriteRune(canonical)
	}
	return folded.String()
}

type nodeParser struct{}

func (nodeParser) Format() Format { return FormatNodePackage }

func (nodeParser) Filenames() []string { return []string{"package.json"} }

type nodeDocument struct {
	Name                 string                      `json:"name"`
	Version              string                      `json:"version"`
	Dependencies         map[string]string           `json:"dependencies"`
	DevDependencies      map[string]string           `json:"devDependencies"`
	OptionalDependencies map[string]string           `json:"optionalDependencies"`
	PeerDependencies     map[string]string           `json:"peerDependencies"`
	PeerMetadata         map[string]nodePeerMetadata `json:"peerDependenciesMeta"`
	Engines              map[string]string           `json:"engines"`
	Workspaces           json.RawMessage             `json:"workspaces"`
}

type nodePeerMetadata struct {
	Optional bool `json:"optional"`
}

func (nodeParser) Parse(ctx context.Context, document Document, limits Limits) (Manifest, error) {
	content, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	var parsedDocument nodeDocument
	if err := decodeStrictJSON(content, limits.MaxNestingDepth, &parsedDocument); err != nil {
		return Manifest{}, err
	}

	workspaceDeclaration := bytes.TrimSpace(parsedDocument.Workspaces)
	manifest := Manifest{
		Name: parsedDocument.Name, Version: parsedDocument.Version,
		WorkspaceDeclared: len(workspaceDeclaration) != 0 &&
			!bytes.Equal(workspaceDeclaration, []byte("null")),
	}
	optionalNames := make(map[string]struct{}, len(parsedDocument.OptionalDependencies))
	for name := range parsedDocument.OptionalDependencies {
		optionalNames[name] = struct{}{}
	}
	manifest.Dependencies = appendMapDependencies(
		manifest.Dependencies,
		parsedDocument.Dependencies,
		dependencyDeclarationOptions{scope: ScopeRuntime, excludedNames: optionalNames},
	)
	manifest.Dependencies = appendMapDependencies(
		manifest.Dependencies,
		parsedDocument.OptionalDependencies,
		dependencyDeclarationOptions{scope: ScopeRuntime, isOptional: true},
	)
	manifest.Dependencies = appendMapDependencies(
		manifest.Dependencies,
		parsedDocument.DevDependencies,
		dependencyDeclarationOptions{scope: ScopeDevelopment},
	)
	for name, constraint := range parsedDocument.PeerDependencies {
		manifest.Dependencies = append(manifest.Dependencies, Dependency{
			Name:       name,
			Constraint: constraint,
			Scope:      ScopePeer,
			Optional:   parsedDocument.PeerMetadata[name].Optional,
		})
	}
	for name, constraint := range parsedDocument.Engines {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: name, Value: constraint, Scope: ScopeRuntime,
		})
	}
	members, err := nodeWorkspaceMembers(parsedDocument.Workspaces, limits.MaxNestingDepth)
	if err != nil {
		return Manifest{}, err
	}
	manifest.WorkspaceMembers = members
	return manifest, nil
}

func nodeWorkspaceMembers(raw json.RawMessage, maximumDepth int) ([]string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var members []string
	if err := decodeStrictJSON(raw, maximumDepth, &members); err == nil {
		return members, nil
	}
	var workspace struct {
		Packages []string `json:"packages"`
	}
	if err := decodeStrictJSON(raw, maximumDepth, &workspace); err != nil {
		return nil, err
	}
	return workspace.Packages, nil
}

type composerParser struct{}

func (composerParser) Format() Format { return FormatPHPComposer }

func (composerParser) Filenames() []string { return []string{"composer.json"} }

type composerDocument struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

func (composerParser) Parse(ctx context.Context, document Document, limits Limits) (Manifest, error) {
	content, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	var parsedDocument composerDocument
	if err := decodeStrictJSON(content, limits.MaxNestingDepth, &parsedDocument); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{Name: parsedDocument.Name, Version: parsedDocument.Version}
	appendComposerRequirements(&manifest, parsedDocument.Require, ScopeRuntime)
	appendComposerRequirements(&manifest, parsedDocument.RequireDev, ScopeDevelopment)
	return manifest, nil
}

func appendComposerRequirements(
	manifest *Manifest,
	requirements map[string]string,
	scope Scope,
) {
	for name, constraint := range requirements {
		if composerPlatformRequirement(name) {
			manifest.Constraints = append(manifest.Constraints, Constraint{
				Name: name, Value: constraint, Scope: scope,
			})
			continue
		}
		manifest.Dependencies = append(manifest.Dependencies, Dependency{
			Name: name, Constraint: constraint, Scope: scope,
		})
	}
}

func composerPlatformRequirement(name string) bool {
	return name == "php" || strings.HasPrefix(name, "ext-") || strings.HasPrefix(name, "lib-") ||
		name == "composer-plugin-api" || name == "composer-runtime-api"
}
