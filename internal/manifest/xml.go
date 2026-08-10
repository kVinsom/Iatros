package manifest

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func decodeStrictXML(content []byte, maximumDepth int, destination any) error {
	if !utf8.Valid(content) {
		return errInvalidManifest
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	depth := 0
	rootElements := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			if depth == 0 {
				rootElements++
				if rootElements > 1 {
					return errInvalidManifest
				}
			}
			depth++
			if depth > maximumDepth {
				return errInvalidManifest
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			// DTDs and entity declarations are unnecessary for supported manifests.
			return errInvalidManifest
		}
	}
	if depth != 0 || rootElements != 1 {
		return errInvalidManifest
	}

	typedDecoder := xml.NewDecoder(bytes.NewReader(content))
	typedDecoder.Strict = true
	if err := typedDecoder.Decode(destination); err != nil {
		return err
	}
	return nil
}

type mavenParser struct{}

func (mavenParser) Format() Format { return FormatMavenProject }

func (mavenParser) Filenames() []string { return []string{"pom.xml"} }

type mavenProject struct {
	XMLName    xml.Name `xml:"project"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Version    string   `xml:"version"`
	Parent     struct {
		GroupID string `xml:"groupId"`
		Version string `xml:"version"`
	} `xml:"parent"`
	Properties struct {
		CompilerRelease string `xml:"maven.compiler.release"`
		CompilerSource  string `xml:"maven.compiler.source"`
		CompilerTarget  string `xml:"maven.compiler.target"`
	} `xml:"properties"`
	Modules      []string          `xml:"modules>module"`
	Dependencies []mavenDependency `xml:"dependencies>dependency"`
}

type mavenDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Optional   string `xml:"optional"`
}

func (mavenParser) Parse(ctx context.Context, document Document, limits Limits) (Manifest, error) {
	content, err := readDocument(ctx, document)
	if err != nil {
		return Manifest{}, err
	}
	var parsedDocument mavenProject
	if err := decodeStrictXML(content, limits.MaxNestingDepth, &parsedDocument); err != nil {
		return Manifest{}, err
	}

	groupID := firstNonEmpty(
		strings.TrimSpace(parsedDocument.GroupID),
		strings.TrimSpace(parsedDocument.Parent.GroupID),
	)
	artifactID := strings.TrimSpace(parsedDocument.ArtifactID)
	version := firstNonEmpty(
		strings.TrimSpace(parsedDocument.Version),
		strings.TrimSpace(parsedDocument.Parent.Version),
	)
	name := artifactID
	if groupID != "" && artifactID != "" {
		name = groupID + ":" + artifactID
	}
	manifest := Manifest{
		Name: name, Version: version, WorkspaceDeclared: len(parsedDocument.Modules) != 0,
	}
	for _, member := range parsedDocument.Modules {
		manifest.WorkspaceMembers = append(manifest.WorkspaceMembers, strings.TrimSpace(member))
	}
	if release := strings.TrimSpace(parsedDocument.Properties.CompilerRelease); release != "" {
		manifest.Constraints = append(manifest.Constraints, Constraint{
			Name: "java", Value: release, Scope: ScopeBuild,
		})
	} else {
		if source := strings.TrimSpace(parsedDocument.Properties.CompilerSource); source != "" {
			manifest.Constraints = append(manifest.Constraints, Constraint{
				Name: "java-source", Value: source, Scope: ScopeBuild,
			})
		}
		if target := strings.TrimSpace(parsedDocument.Properties.CompilerTarget); target != "" {
			manifest.Constraints = append(manifest.Constraints, Constraint{
				Name: "java-target", Value: target, Scope: ScopeBuild,
			})
		}
	}
	for _, dependency := range parsedDocument.Dependencies {
		group := strings.TrimSpace(dependency.GroupID)
		artifact := strings.TrimSpace(dependency.ArtifactID)
		if group == "" || artifact == "" {
			return Manifest{}, errInvalidManifest
		}
		scope, err := mavenScope(strings.TrimSpace(dependency.Scope))
		if err != nil {
			return Manifest{}, err
		}
		optional := false
		if text := strings.TrimSpace(dependency.Optional); text != "" {
			optional, err = strconv.ParseBool(text)
			if err != nil {
				return Manifest{}, errInvalidManifest
			}
		}
		manifest.Dependencies = append(manifest.Dependencies, Dependency{
			Name:       group + ":" + artifact,
			Constraint: strings.TrimSpace(dependency.Version),
			Scope:      scope,
			Optional:   optional,
		})
	}
	return manifest, nil
}

func mavenScope(scope string) (Scope, error) {
	switch scope {
	case "", "compile", "runtime":
		return ScopeRuntime, nil
	case "test":
		return ScopeDevelopment, nil
	case "provided", "system", "import":
		return ScopeBuild, nil
	default:
		return "", errInvalidManifest
	}
}
