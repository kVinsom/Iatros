package artifactvalidation

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
)

type composeShapeValidator struct{}

func (composeShapeValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.structure.compose", Version: "1.0"}
}

func (composeShapeValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Kind == KindCompose
}

func (composeShapeValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	documents, syntaxDiagnostic, err := decodeBoundedYAML(ctx, document, limits)
	if err != nil {
		return nil, err
	}
	if syntaxDiagnostic != nil || len(documents) == 0 {
		return make([]Diagnostic, 0), nil
	}
	if len(documents) != 1 {
		return []Diagnostic{{
			Code: "COMPOSE_DOCUMENT_COUNT_INVALID", Level: DiagnosticError,
			Message: "A Compose artifact must contain exactly one document.",
		}}, nil
	}
	root := documentRoot(documents[0])
	services := mappingValue(root, "services")
	if root == nil || root.Kind != yaml.MappingNode || services == nil ||
		services.Kind != yaml.MappingNode || len(services.Content) == 0 {
		return []Diagnostic{{
			Code: "COMPOSE_SERVICES_MISSING", Level: DiagnosticError,
			Message: "A Compose artifact must define a non-empty services mapping.",
		}}, nil
	}
	return make([]Diagnostic, 0), nil
}

type kubernetesShapeValidator struct{}

func (kubernetesShapeValidator) Descriptor() ValidatorDescriptor {
	return ValidatorDescriptor{ID: "iatros.validation.structure.kubernetes", Version: "1.0"}
}

func (kubernetesShapeValidator) AppliesTo(artifact Artifact) bool {
	return artifact.Kind == KindKubernetes
}

func (kubernetesShapeValidator) Validate(
	ctx context.Context,
	document Document,
	limits Limits,
) ([]Diagnostic, error) {
	documents, syntaxDiagnostic, err := decodeBoundedYAML(ctx, document, limits)
	if err != nil {
		return nil, err
	}
	if syntaxDiagnostic != nil {
		return make([]Diagnostic, 0), nil
	}
	diagnostics := make([]Diagnostic, 0)
	for index, yamlDocument := range documents {
		if len(diagnostics) > limits.MaxDiagnosticsPerArtifact {
			break
		}
		root := documentRoot(yamlDocument)
		if root == nil || root.Kind != yaml.MappingNode {
			diagnostics = append(diagnostics, Diagnostic{
				Code: "KUBERNETES_ROOT_INVALID", Level: DiagnosticError,
				Message: fmt.Sprintf("Kubernetes document %d must be a mapping.", index+1),
			})
			continue
		}
		apiVersion := scalarMappingValue(root, "apiVersion")
		kind := scalarMappingValue(root, "kind")
		if apiVersion == "" || kind == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Code: "KUBERNETES_TYPE_MISSING", Level: DiagnosticError,
				Message: fmt.Sprintf("Kubernetes document %d must define apiVersion and kind.", index+1),
			})
			continue
		}
		if kind == "List" {
			items := mappingValue(root, "items")
			if items == nil || items.Kind != yaml.SequenceNode {
				diagnostics = append(diagnostics, Diagnostic{
					Code: "KUBERNETES_LIST_ITEMS_MISSING", Level: DiagnosticError,
					Message: fmt.Sprintf("Kubernetes List document %d must define an items sequence.", index+1),
				})
			}
			continue
		}
		metadata := mappingValue(root, "metadata")
		if metadata == nil || metadata.Kind != yaml.MappingNode || scalarMappingValue(metadata, "name") == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Code: "KUBERNETES_NAME_MISSING", Level: DiagnosticError,
				Message: fmt.Sprintf("Kubernetes document %d must define metadata.name.", index+1),
			})
		}
	}
	return diagnostics, nil
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	return document.Content[0]
}

func mappingValue(mapping *yaml.Node, name string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Kind == yaml.ScalarNode && key.Value == name {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func scalarMappingValue(mapping *yaml.Node, name string) string {
	value := mappingValue(mapping, name)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
}
