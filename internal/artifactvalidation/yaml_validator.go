package artifactvalidation

import (
	"context"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

type yamlSyntaxValidator struct{}

func (yamlSyntaxValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.syntax.yaml", Version: "1.0"}
}

func (yamlSyntaxValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Format == FormatYAML
}

func (yamlSyntaxValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	documents, diagnostic, err := decodeBoundedYAML(ctx, document, limits)
	if err != nil {
		return nil, err
	}
	if diagnostic != nil {
		return []Diagnostic{*diagnostic}, nil
	}
	if len(documents) == 0 {
		return []Diagnostic{{
			Code: "YAML_EMPTY", Level: DiagnosticError,
			Message: "YAML does not contain a document.",
		}}, nil
	}
	return make([]Diagnostic, 0), nil
}

type yamlVisit struct {
	node  *yaml.Node
	depth int
}

func decodeBoundedYAML(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]*yaml.Node, *Diagnostic, error) {
	decoder := yaml.NewDecoder(document.Reader())
	documents := make([]*yaml.Node, 0)
	nodes := 0
	aliases := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		root := &yaml.Node{}
		err := decoder.Decode(root)
		if err == io.EOF {
			return documents, nil, nil
		}
		if err != nil {
			return nil, &Diagnostic{
				Code: "YAML_SYNTAX_INVALID", Level: DiagnosticError,
				Message: fmt.Sprintf("YAML syntax is invalid: %v", err),
			}, nil
		}
		if diagnostic, err := validateYAMLTree(ctx, root, limits, &nodes, &aliases); diagnostic != nil || err != nil {
			return nil, diagnostic, err
		}
		documents = append(documents, root)
	}
}

func validateYAMLTree(
	ctx context.Context,
	root *yaml.Node,
	limits Limits,
	nodes *int,
	aliases *int,
) (*Diagnostic, error) {
	stack := []yamlVisit{{node: root, depth: 1}}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		visit := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		(*nodes)++
		if *nodes > limits.MaxSyntaxNodes {
			return &Diagnostic{
				Code: "YAML_NODES_EXCEEDED", Level: DiagnosticError,
				Message: "YAML syntax exceeds the configured node limit.",
			}, nil
		}
		if visit.depth > limits.MaxNestingDepth {
			return &Diagnostic{
				Code: "YAML_DEPTH_EXCEEDED", Level: DiagnosticError,
				Message: "YAML nesting exceeds the configured depth limit.",
			}, nil
		}
		if visit.node.Kind == yaml.AliasNode {
			(*aliases)++
			if *aliases > limits.MaxAliases {
				return &Diagnostic{
					Code: "YAML_ALIASES_EXCEEDED", Level: DiagnosticError,
					Message: "YAML aliases exceed the configured limit.",
				}, nil
			}
		}
		if visit.node.Kind == yaml.MappingNode {
			if duplicate := duplicateYAMLKey(visit.node); duplicate != nil {
				return duplicate, nil
			}
		}
		for index := len(visit.node.Content) - 1; index >= 0; index-- {
			stack = append(stack, yamlVisit{node: visit.node.Content[index], depth: visit.depth + 1})
		}
	}
	return nil, nil
}

func duplicateYAMLKey(mapping *yaml.Node) *Diagnostic {
	keys := make(map[string]struct{}, len(mapping.Content)/2)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		if _, exists := keys[key.Value]; exists {
			return &Diagnostic{
				Code: "YAML_DUPLICATE_KEY", Level: DiagnosticError,
				Message:  "YAML contains a duplicate mapping key.",
				Location: Location{StartLine: key.Line, StartColumn: key.Column},
			}
		}
		keys[key.Value] = struct{}{}
	}
	return nil
}
