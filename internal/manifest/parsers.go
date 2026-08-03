package manifest

// DefaultParsers returns the built-in, network-free parser backends.
func DefaultParsers() []Parser {
	return []Parser{
		goModParser{},
		goWorkParser{},
		nodeParser{},
		pythonParser{},
		rustParser{},
		composerParser{},
		mavenParser{},
	}
}
