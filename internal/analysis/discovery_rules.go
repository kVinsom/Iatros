package analysis

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositoryignore"
)

var (
	errControlFileTooLarge = errors.New("repository control file is too large")
	errControlFileUnsafe   = errors.New("repository control file is unsafe")
)

func (w *discoveryWalker) rulesForDirectory(
	ctx context.Context,
	directory string,
	entries []fs.DirEntry,
	inherited []repositoryignore.Rule,
) ([]repositoryignore.Rule, error) {
	entry := namedEntry(entries, ".gitignore")
	if entry == nil || entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return inherited, nil
	}
	ignorePath := path.Join(directory, entry.Name())
	if w.ignoreFiles >= w.limits.MaxIgnoreFiles {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueIgnoreFileLimit,
			Path:    ignorePath,
			Message: "The ignore-file limit was reached; this file was not applied.",
		})
		return inherited, nil
	}
	w.ignoreFiles++

	content, err := readBoundedControlFile(
		ctx,
		w.filesystem,
		ignorePath,
		entry,
		w.limits.MaxControlFileBytes,
	)
	if err != nil {
		code := DiscoveryIssueIgnoreReadFailed
		message := "The ignore file could not be read safely and was not applied."
		if errors.Is(err, errControlFileTooLarge) {
			code = DiscoveryIssueIgnoreFileTooLarge
			message = "The ignore file exceeded its byte limit and was not applied."
		}
		w.addIssue(DiscoveryIssue{Code: code, Path: ignorePath, Message: message})
		return inherited, nil
	}

	remaining := w.limits.MaxIgnoreRules - w.ignoreRules
	if remaining <= 0 {
		w.addIssue(ignoreRuleLimitIssue(ignorePath))
		return inherited, nil
	}
	parsed := repositoryignore.Parse(
		directory,
		content,
		remaining,
		w.limits.MaxIgnorePatternBytes,
	)
	if parsed.Invalid > 0 {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueIgnoreRuleInvalid,
			Path:    ignorePath,
			Message: "One or more invalid ignore patterns were not applied.",
		})
	}
	if parsed.Truncated {
		w.addIssue(ignoreRuleLimitIssue(ignorePath))
		return inherited, nil
	}
	if len(parsed.Rules) == 0 {
		return inherited, nil
	}

	rules := slices.Clone(inherited)
	rules = append(rules, parsed.Rules...)
	w.ignoreRules += len(parsed.Rules)
	return rules, nil
}

func ignoreRuleLimitIssue(ignorePath string) DiscoveryIssue {
	return DiscoveryIssue{
		Code:    DiscoveryIssueIgnoreRuleLimit,
		Path:    ignorePath,
		Message: "The ignore-rule limit was reached; this file was not applied.",
	}
}

func (w *discoveryWalker) loadDeclaredSubmodules(
	ctx context.Context,
	entries []fs.DirEntry,
) error {
	entry := namedEntry(entries, ".gitmodules")
	if entry == nil || entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return nil
	}
	content, err := readBoundedControlFile(
		ctx,
		w.filesystem,
		".gitmodules",
		entry,
		w.limits.MaxControlFileBytes,
	)
	if err != nil {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueGitmodulesInvalid,
			Path:    ".gitmodules",
			Message: "Submodule declarations could not be read safely and were not applied.",
		})
		return nil
	}

	parsed := parseGitmodules(content, w.limits.MaxNestedRepositories)
	if parsed.invalid {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueGitmodulesInvalid,
			Path:    ".gitmodules",
			Message: "One or more unsafe or malformed submodule declarations were not applied.",
		})
	}
	if parsed.truncated {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueNestedRepositoryLimit,
			Path:    ".",
			Message: "Additional submodule boundaries were omitted by the configured limit.",
		})
	}
	for _, submodulePath := range parsed.paths {
		w.declaredSubmodules[submodulePath] = struct{}{}
		for ancestor := path.Dir(submodulePath); ancestor != "."; ancestor = path.Dir(ancestor) {
			w.submoduleAncestors[ancestor] = struct{}{}
		}
	}
	return nil
}

type gitmodulesParseResult struct {
	paths     []string
	invalid   bool
	truncated bool
}

func parseGitmodules(content []byte, maximum int) gitmodulesParseResult {
	result := gitmodulesParseResult{paths: make([]string, 0)}
	if maximum <= 0 || !utf8.Valid(content) {
		result.invalid = true
		return result
	}
	result.paths = make([]string, 0, min(maximum, 16))
	seen := make(map[string]struct{}, min(maximum, 16))
	inSubmodule := false
	sectionPath := ""
	flushSection := func() {
		if !inSubmodule {
			return
		}
		if sectionPath == "" {
			result.invalid = true
			return
		}
		if _, exists := seen[sectionPath]; exists {
			return
		}
		if len(result.paths) >= maximum {
			result.truncated = true
			return
		}
		seen[sectionPath] = struct{}{}
		result.paths = append(result.paths, sectionPath)
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			flushSection()
			lower := strings.ToLower(line)
			inSubmodule = strings.HasPrefix(lower, "[submodule ") && strings.HasSuffix(line, "]")
			sectionPath = ""
			if !strings.HasSuffix(line, "]") {
				result.invalid = true
			}
			continue
		}
		if !inSubmodule {
			continue
		}
		key, rawValue, found := splitGitConfigAssignment(line)
		if !found || !strings.EqualFold(key, "path") {
			continue
		}
		submodulePath, isValid := parseGitConfigValue(rawValue)
		if !isValid || !validRelativePath(submodulePath) {
			result.invalid = true
			continue
		}
		sectionPath = submodulePath
	}
	flushSection()
	slices.Sort(result.paths)
	return result
}

func parseGitConfigValue(raw string) (string, bool) {
	configurationValue := strings.TrimSpace(raw)
	if configurationValue == "" {
		return "", false
	}
	if configurationValue[0] == '"' {
		end := quotedValueEnd(configurationValue)
		if end < 0 {
			return "", false
		}
		unquoted, err := strconv.Unquote(configurationValue[:end])
		remainder := strings.TrimSpace(configurationValue[end:])
		return unquoted, err == nil &&
			(remainder == "" || strings.HasPrefix(remainder, "#") || strings.HasPrefix(remainder, ";"))
	}
	for index, character := range configurationValue {
		if (character == '#' || character == ';') && index > 0 &&
			(configurationValue[index-1] == ' ' || configurationValue[index-1] == '\t') {
			configurationValue = strings.TrimSpace(configurationValue[:index])
			break
		}
	}
	return configurationValue, configurationValue != ""
}

func splitGitConfigAssignment(line string) (string, string, bool) {
	separator := strings.IndexAny(line, "= \t")
	if separator <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:separator])
	configurationValue := strings.TrimSpace(line[separator:])
	configurationValue = strings.TrimSpace(strings.TrimPrefix(configurationValue, "="))
	return key, configurationValue, key != "" && configurationValue != ""
}

func quotedValueEnd(configurationValue string) int {
	escaped := false
	for index := 1; index < len(configurationValue); index++ {
		switch {
		case escaped:
			escaped = false
		case configurationValue[index] == '\\':
			escaped = true
		case configurationValue[index] == '"':
			return index + 1
		}
	}
	return -1
}

func readBoundedControlFile(
	ctx context.Context,
	filesystem fs.FS,
	filePath string,
	entry fs.DirEntry,
	maximum int64,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return nil, errControlFileUnsafe
	}
	if info.Size() > maximum {
		return nil, errControlFileTooLarge
	}

	file, err := filesystem.Open(filePath)
	if err != nil {
		return nil, err
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil {
		return nil, errors.Join(statErr, file.Close())
	}
	if !openedInfo.Mode().IsRegular() || openedInfo.Size() != info.Size() {
		return nil, errors.Join(errControlFileUnsafe, file.Close())
	}
	reader := &io.LimitedReader{R: &discoveryContextReader{ctx: ctx, reader: file}, N: maximum + 1}
	content, readErr := io.ReadAll(reader)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(content)) > maximum {
		return nil, errControlFileTooLarge
	}
	if int64(len(content)) != info.Size() {
		return nil, errControlFileUnsafe
	}
	return content, ctx.Err()
}

func namedEntry(entries []fs.DirEntry, name string) fs.DirEntry {
	for _, entry := range entries {
		if entry.Name() == name {
			return entry
		}
	}
	return nil
}

type discoveryContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *discoveryContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
