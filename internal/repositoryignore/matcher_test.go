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

	testCases := []struct {
		path      string
		entryKind EntryKind
		ignored   bool
	}{
		{path: "application.log", entryKind: EntryFile, ignored: true},
		{path: "important.log", entryKind: EntryFile},
		{path: "build", entryKind: EntryDirectory, ignored: true},
		{path: "nested/build", entryKind: EntryDirectory},
		{path: "services/api/generated", entryKind: EntryDirectory, ignored: true},
		{path: "services/api/tmp", entryKind: EntryDirectory, ignored: true},
		{path: "services/web/tmp", entryKind: EntryDirectory},
		{path: "services/api/debug.log", entryKind: EntryFile},
	}
	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			t.Parallel()
			if actual := Ignored(rules, testCase.path, testCase.entryKind); actual != testCase.ignored {
				t.Fatalf("Ignored() = %t, want %t", actual, testCase.ignored)
			}
		})
	}
}

func TestIgnoredSupportsGlobstarPositions(t *testing.T) {
	t.Parallel()

	rules := Parse(".", []byte("**/cache\na/**/b\nlogs/**\n"), 20, 4_096).Rules
	testCases := []struct {
		path      string
		entryKind EntryKind
		ignored   bool
	}{
		{path: "cache", entryKind: EntryDirectory, ignored: true},
		{path: "one/two/cache", entryKind: EntryDirectory, ignored: true},
		{path: "a/b", entryKind: EntryFile, ignored: true},
		{path: "a/x/y/b", entryKind: EntryFile, ignored: true},
		{path: "logs", entryKind: EntryDirectory},
		{path: "logs/current", entryKind: EntryDirectory, ignored: true},
	}
	for _, testCase := range testCases {
		if actual := Ignored(rules, testCase.path, testCase.entryKind); actual != testCase.ignored {
			t.Fatalf("Ignored(%q) = %t, want %t", testCase.path, actual, testCase.ignored)
		}
	}
}

func TestParseHandlesCommentsEscapesAndBounds(t *testing.T) {
	t.Parallel()

	result := Parse(".", []byte("# comment\n\\#literal\n\\!important\nname\\ \n[invalid\nlast\n"), 3, 4_096)
	if result.Invalid != 1 || !result.Truncated || len(result.Rules) != 3 {
		t.Fatalf("Parse() = %#v, want one invalid and three bounded rules", result)
	}
	for _, literalPath := range []string{"#literal", "!important", "name "} {
		if !Ignored(result.Rules, literalPath, EntryFile) {
			t.Fatalf("%q was not matched as a literal pattern", literalPath)
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
	_, err := IgnoredContext(
		ctx,
		Parse(".", []byte("*.log\n"), 1, 4_096).Rules,
		"app.log",
		EntryFile,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("IgnoredContext() error = %v, want context.Canceled", err)
	}
}

func TestIgnoredContextRejectsUnknownEntryKind(t *testing.T) {
	t.Parallel()

	_, err := IgnoredContext(
		t.Context(),
		Parse(".", []byte("*.log\n"), 1, 4_096).Rules,
		"app.log",
		EntryKind(255),
	)
	if !errors.Is(err, errInvalidEntryKind) {
		t.Fatalf("IgnoredContext() error = %v, want errInvalidEntryKind", err)
	}
}

func TestParseRejectsOversizedPattern(t *testing.T) {
	t.Parallel()

	result := Parse(".", []byte("12345\nok\n"), 10, 4)
	if result.Invalid != 1 || result.Truncated || len(result.Rules) != 1 {
		t.Fatalf("Parse() = %#v, want one invalid oversized pattern", result)
	}
}
