package repositoryignore

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestIgnoredUsesGitStyleScopesAndPrecedence(t *testing.T) {
	t.Parallel()

	root := Parse(".", []byte("*.log\n/build/\nservices/**/generated/\n!important.log\n"), 20, 4_096)
	nested := Parse("services/api", []byte("/tmp/\n!debug.log\n"), 20, 4_096)
	rules := append(slices.Clone(root.Rules), nested.Rules...)

	tests := []struct {
		path      string
		directory bool
		ignored   bool
	}{
		{path: "application.log", ignored: true},
		{path: "important.log"},
		{path: "build", directory: true, ignored: true},
		{path: "nested/build", directory: true},
		{path: "services/api/generated", directory: true, ignored: true},
		{path: "services/api/tmp", directory: true, ignored: true},
		{path: "services/web/tmp", directory: true},
		{path: "services/api/debug.log"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			if got := Ignored(rules, test.path, test.directory); got != test.ignored {
				t.Fatalf("Ignored() = %t, want %t", got, test.ignored)
			}
		})
	}
}

func TestIgnoredSupportsGlobstarPositions(t *testing.T) {
	t.Parallel()

	rules := Parse(".", []byte("**/cache\na/**/b\nlogs/**\n"), 20, 4_096).Rules
	tests := []struct {
		path      string
		directory bool
		ignored   bool
	}{
		{path: "cache", directory: true, ignored: true},
		{path: "one/two/cache", directory: true, ignored: true},
		{path: "a/b", ignored: true},
		{path: "a/x/y/b", ignored: true},
		{path: "logs", directory: true},
		{path: "logs/current", directory: true, ignored: true},
	}
	for _, test := range tests {
		if got := Ignored(rules, test.path, test.directory); got != test.ignored {
			t.Fatalf("Ignored(%q) = %t, want %t", test.path, got, test.ignored)
		}
	}
}

func TestParseHandlesCommentsEscapesAndBounds(t *testing.T) {
	t.Parallel()

	result := Parse(".", []byte("# comment\n\\#literal\n\\!important\nname\\ \n[invalid\nlast\n"), 3, 4_096)
	if result.Invalid != 1 || !result.Truncated || len(result.Rules) != 3 {
		t.Fatalf("Parse() = %#v, want one invalid and three bounded rules", result)
	}
	for _, value := range []string{"#literal", "!important", "name "} {
		if !Ignored(result.Rules, value, false) {
			t.Fatalf("%q was not matched as a literal pattern", value)
		}
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	for _, result := range []ParseResult{
		Parse("../outside", []byte("value"), 1, 4_096),
		Parse(".", []byte{0xff}, 1, 4_096),
		Parse(".", []byte("value"), 0, 4_096),
		Parse(".", []byte("value"), -1, 4_096),
		Parse(".", []byte("value"), 1, 0),
	} {
		if result.Invalid == 0 || len(result.Rules) != 0 {
			t.Fatalf("Parse() = %#v, want rejected input", result)
		}
	}
}

func TestIgnoredContextHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := IgnoredContext(ctx, Parse(".", []byte("*.log\n"), 1, 4_096).Rules, "app.log", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("IgnoredContext() error = %v, want context.Canceled", err)
	}
}

func TestParseRejectsOversizedPattern(t *testing.T) {
	t.Parallel()

	result := Parse(".", []byte("12345\nok\n"), 10, 4)
	if result.Invalid != 1 || result.Truncated || len(result.Rules) != 1 {
		t.Fatalf("Parse() = %#v, want one invalid oversized pattern", result)
	}
}
