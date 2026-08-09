// Package repositoryignore evaluates repository-local Git-style ignore rules.
package repositoryignore

import (
	"context"
	"errors"
	"path"
	"strings"
	"unicode/utf8"
)

var errInvalidEntryKind = errors.New("repository ignore entry kind is invalid")

// EntryKind identifies whether ignore rules are evaluated for a file or directory.
type EntryKind uint8

const (
	// EntryFile identifies a regular repository file.
	EntryFile EntryKind = iota + 1
	// EntryDirectory identifies a repository directory.
	EntryDirectory
)

// Rule is one compiled .gitignore pattern scoped to its containing directory.
// Its fields are intentionally private so callers cannot construct invalid rules.
type Rule struct {
	base          string
	pattern       string
	segments      []string
	negated       bool
	directoryOnly bool
	hasSlash      bool
	hasGlobstar   bool
}

// ParseResult contains valid rules and bounded parser diagnostics.
type ParseResult struct {
	Rules     []Rule
	Invalid   int
	Truncated bool
}

// Parse compiles bounded patterns relative to base.
func Parse(base string, content []byte, maxRules, maxPatternBytes int) ParseResult {
	result := ParseResult{Rules: make([]Rule, 0)}
	if maxRules <= 0 || maxPatternBytes <= 0 || !validBase(base) || !utf8.Valid(content) {
		result.Invalid = 1
		return result
	}
	result.Rules = make([]Rule, 0, min(maxRules, 16))

	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = trimUnescapedTrailingSpaces(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if len(line) > maxPatternBytes {
			result.Invalid++
			continue
		}

		rule, ok := parseRule(base, line)
		if !ok {
			result.Invalid++
			continue
		}
		if len(result.Rules) >= maxRules {
			result.Truncated = true
			continue
		}
		result.Rules = append(result.Rules, rule)
	}

	return result
}

// Ignored reports the last matching rule's decision for a repository-relative path.
func Ignored(rules []Rule, repositoryPath string, entryKind EntryKind) bool {
	ignored, _ := IgnoredContext(context.Background(), rules, repositoryPath, entryKind)
	return ignored
}

// IgnoredContext evaluates the last matching rule and remains cancelable for large rule sets.
func IgnoredContext(
	ctx context.Context,
	rules []Rule,
	repositoryPath string,
	entryKind EntryKind,
) (bool, error) {
	if entryKind != EntryFile && entryKind != EntryDirectory {
		return false, errInvalidEntryKind
	}
	ignored := false
	for index, rule := range rules {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		matched, err := rule.matches(ctx, repositoryPath, entryKind)
		if err != nil {
			return false, err
		}
		if matched {
			ignored = !rule.negated
		}
	}
	return ignored, ctx.Err()
}

func parseRule(base, line string) (Rule, bool) {
	rule := Rule{base: base}
	if line[0] == '!' {
		rule.negated = true
		line = line[1:]
		if line == "" {
			return Rule{}, false
		}
	}

	if strings.HasSuffix(line, "/") && !escapedAt(line, len(line)-1) {
		rule.directoryOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		line = strings.TrimPrefix(line, "/")
		rule.hasSlash = true
	}
	if line == "" || invalidPatternText(line) {
		return Rule{}, false
	}
	if strings.Contains(line, "/") {
		rule.hasSlash = true
	}

	rule.pattern = line
	rule.segments = strings.Split(line, "/")
	for _, segment := range rule.segments {
		if segment == "" {
			return Rule{}, false
		}
		if segment == "**" {
			rule.hasGlobstar = true
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return Rule{}, false
		}
	}
	return rule, true
}

func (r Rule) matches(ctx context.Context, repositoryPath string, entryKind EntryKind) (bool, error) {
	if r.directoryOnly && entryKind != EntryDirectory {
		return false, nil
	}
	relative, ok := relativeToBase(r.base, repositoryPath)
	if !ok {
		return false, nil
	}
	if !r.hasSlash {
		matched, err := path.Match(r.pattern, path.Base(relative))
		return err == nil && matched, nil
	}
	if !r.hasGlobstar {
		matched, err := path.Match(r.pattern, relative)
		return err == nil && matched, nil
	}
	return matchSegments(ctx, r.segments, strings.Split(relative, "/"))
}

func matchSegments(ctx context.Context, patternSegments, pathSegments []string) (bool, error) {
	previous := make([]bool, len(pathSegments)+1)
	current := make([]bool, len(pathSegments)+1)
	previous[0] = true
	for patternIndex, pattern := range patternSegments {
		if patternIndex%128 == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		clear(current)
		if pattern == "**" {
			trailing := patternIndex == len(patternSegments)-1
			if !trailing {
				current[0] = previous[0]
			}
			for pathIndex := 1; pathIndex <= len(pathSegments); pathIndex++ {
				if trailing {
					current[pathIndex] = previous[pathIndex-1] || current[pathIndex-1]
					continue
				}
				current[pathIndex] = previous[pathIndex] || current[pathIndex-1]
			}
			previous, current = current, previous
			continue
		}
		for pathIndex := 1; pathIndex <= len(pathSegments); pathIndex++ {
			matched, err := path.Match(pattern, pathSegments[pathIndex-1])
			current[pathIndex] = err == nil && previous[pathIndex-1] && matched
		}
		previous, current = current, previous
	}
	return previous[len(pathSegments)], ctx.Err()
}

func relativeToBase(base, repositoryPath string) (string, bool) {
	if base == "." {
		return repositoryPath, repositoryPath != "."
	}
	prefix := base + "/"
	if !strings.HasPrefix(repositoryPath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(repositoryPath, prefix), true
}

func trimUnescapedTrailingSpaces(line string) string {
	for len(line) > 0 && line[len(line)-1] == ' ' && !escapedAt(line, len(line)-1) {
		line = line[:len(line)-1]
	}
	return line
}

func escapedAt(pattern string, index int) bool {
	backslashes := 0
	for index--; index >= 0 && pattern[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func invalidPatternText(pattern string) bool {
	if !utf8.ValidString(pattern) || strings.HasSuffix(pattern, "\\") {
		return true
	}
	for _, character := range pattern {
		if character == 0 || character == '\n' || character == '\r' {
			return true
		}
	}
	return false
}

func validBase(base string) bool {
	if base == "." {
		return true
	}
	return base != "" && path.Clean(base) == base && !path.IsAbs(base) &&
		!strings.HasPrefix(base, "../") && !strings.Contains(base, "\\")
}
