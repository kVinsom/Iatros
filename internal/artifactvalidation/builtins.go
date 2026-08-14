package artifactvalidation

// BuiltInValidators returns a new provider-neutral validator set.
func BuiltInValidators() []Validator {
	return []Validator{
		contentValidator{},
		composeShapeValidator{},
		dockerfileSyntaxValidator{},
		hclSyntaxValidator{},
		jsonSyntaxValidator{},
		kubernetesShapeValidator{},
		xmlSyntaxValidator{},
		yamlSyntaxValidator{},
	}
}
