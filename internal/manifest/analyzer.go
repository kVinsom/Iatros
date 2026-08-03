package manifest

import (
	"context"
	"errors"
	"io"
	"path"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

const (
	// IssueFileLimit identifies manifest files omitted by the configured file limit.
	IssueFileLimit = "IATROS_MANIFEST_FILE_LIMIT"
	// IssueFileTooLarge identifies a manifest omitted by the per-file byte limit.
	IssueFileTooLarge = "IATROS_MANIFEST_FILE_TOO_LARGE"
	// IssueTotalBytesLimit identifies remaining files omitted by the total byte limit.
	IssueTotalBytesLimit = "IATROS_MANIFEST_TOTAL_BYTES_LIMIT"
	// IssueReadFailed identifies a manifest that could not be read safely and completely.
	IssueReadFailed = "IATROS_MANIFEST_READ_FAILED"
	// IssueParseFailed identifies malformed, ambiguous, or unsupported manifest content.
	IssueParseFailed = "IATROS_MANIFEST_PARSE_FAILED"
	// IssueParserUnavailable identifies a recognized format without a configured backend.
	IssueParserUnavailable = "IATROS_MANIFEST_PARSER_UNAVAILABLE"
	// IssueDependencyLimit identifies omitted direct dependencies.
	IssueDependencyLimit = "IATROS_MANIFEST_DEPENDENCY_LIMIT"
	// IssueConstraintLimit identifies omitted runtime or toolchain constraints.
	IssueConstraintLimit = "IATROS_MANIFEST_CONSTRAINT_LIMIT"
	// IssueWorkspaceLimit identifies omitted workspace member declarations.
	IssueWorkspaceLimit = "IATROS_MANIFEST_WORKSPACE_MEMBER_LIMIT"
	// IssueWorkspaceExcludeLimit identifies omitted workspace exclusion declarations.
	IssueWorkspaceExcludeLimit = "IATROS_MANIFEST_WORKSPACE_EXCLUDE_LIMIT"
	// IssueLimit identifies omitted manifest diagnostics.
	IssueLimit = "IATROS_MANIFEST_ISSUE_LIMIT"
)

var errInvalidParserResult = errors.New("manifest parser result is invalid")

// Analyzer coordinates bounded document loading, parser selection, and normalization.
type Analyzer struct {
	limits            Limits
	parsers           map[Format]Parser
	formatsByFilename map[string]Format
}

// NewAnalyzer creates an analyzer from a validated profile and replaceable parser backends.
func NewAnalyzer(limits Limits, parsers ...Parser) (Analyzer, error) {
	if err := limits.Validate(); err != nil {
		return Analyzer{}, err
	}

	registry := make(map[Format]Parser, len(parsers))
	formatsByFilename := defaultFormatsByFilename()
	for _, parser := range parsers {
		if parserIsNil(parser) {
			return Analyzer{}, ErrInvalidParser
		}
		format := parser.Format()
		filenames := parser.Filenames()
		if !validFormat(format) || len(filenames) == 0 {
			return Analyzer{}, ErrInvalidParser
		}
		if _, exists := registry[format]; exists {
			return Analyzer{}, ErrInvalidParser
		}
		requiredFilename := canonicalFilename(format)
		hasRequiredFilename := requiredFilename == ""
		for _, filename := range filenames {
			if !validManifestFilename(filename) {
				return Analyzer{}, ErrInvalidParser
			}
			if existing, exists := formatsByFilename[filename]; exists && existing != format {
				return Analyzer{}, ErrInvalidParser
			}
			formatsByFilename[filename] = format
			hasRequiredFilename = hasRequiredFilename || filename == requiredFilename
		}
		if !hasRequiredFilename {
			return Analyzer{}, ErrInvalidParser
		}
		registry[format] = parser
	}
	return Analyzer{limits: limits, parsers: registry, formatsByFilename: formatsByFilename}, nil
}

func canonicalFilename(format Format) string {
	for filename, candidate := range defaultFormatsByFilename() {
		if candidate == format {
			return filename
		}
	}
	return ""
}

func parserIsNil(parser Parser) bool {
	if parser == nil {
		return true
	}
	value := reflect.ValueOf(parser)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// NewDefaultAnalyzer creates an analyzer with all built-in parser backends.
func NewDefaultAnalyzer(limits Limits) (Analyzer, error) {
	return NewAnalyzer(limits, DefaultParsers()...)
}

// Analyze parses supported manifest paths without retaining raw file content.
func (a Analyzer) Analyze(ctx context.Context, source Source, snapshot Snapshot) (Result, error) {
	result := emptyResult(snapshot.Partial)
	if err := a.limits.Validate(); err != nil {
		return result, err
	}
	if source == nil {
		return result, ErrInvalidSource
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	analysisCtx, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()
	ctx = analysisCtx

	files := slices.Clone(snapshot.Files)
	for _, file := range files {
		if !validRepositoryPath(file) {
			return result, ErrInvalidSnapshot
		}
	}
	slices.Sort(files)
	files = slices.Compact(files)

	candidates := a.manifestCandidates(files)
	if len(candidates) > a.limits.MaxFiles {
		result.addIssue(Issue{
			Code:    IssueFileLimit,
			Path:    candidates[a.limits.MaxFiles].path,
			Message: "additional manifest files were omitted by the configured limit",
		}, a.limits.MaxIssues)
		candidates = candidates[:a.limits.MaxFiles]
	}

	var totalBytes int64
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return emptyResult(snapshot.Partial), err
		}

		parser, exists := a.parsers[candidate.format]
		if !exists {
			result.addIssue(Issue{
				Code:    IssueParserUnavailable,
				Path:    candidate.path,
				Message: "no parser backend is configured for this manifest format",
			}, a.limits.MaxIssues)
			continue
		}

		manifest, bytesRead, issue, err := a.parse(ctx, source, parser, candidate, totalBytes)
		totalBytes = boundedByteTotal(totalBytes, bytesRead, a.limits.MaxTotalBytes)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return emptyResult(snapshot.Partial), err
			}
			result.addIssue(*issue, a.limits.MaxIssues)
			if issue.Code == IssueTotalBytesLimit {
				break
			}
			continue
		}

		normalized, normalizationIssues, err := normalizeManifest(manifest, candidate, a.limits)
		if err != nil {
			result.addIssue(Issue{
				Code:    IssueParseFailed,
				Path:    candidate.path,
				Message: "the manifest parser returned invalid normalized data",
			}, a.limits.MaxIssues)
			continue
		}
		result.Manifests = append(result.Manifests, normalized)
		for _, normalizationIssue := range normalizationIssues {
			result.addIssue(normalizationIssue, a.limits.MaxIssues)
		}
	}

	slices.SortFunc(result.Issues, compareIssues)
	return result, nil
}

func (a Analyzer) parse(
	ctx context.Context,
	source Source,
	parser Parser,
	candidate manifestCandidate,
	totalBytes int64,
) (Manifest, int64, *Issue, error) {
	reader, size, err := source.Open(ctx, candidate.path)
	if err != nil {
		issue := Issue{IssueReadFailed, candidate.path, "the manifest could not be opened safely"}
		return Manifest{}, 0, &issue, err
	}
	if reader == nil || size < 0 {
		if reader != nil {
			_ = reader.Close()
		}
		issue := Issue{IssueReadFailed, candidate.path, "the manifest source returned invalid metadata"}
		return Manifest{}, 0, &issue, ErrInvalidSource
	}

	if size > a.limits.MaxFileBytes {
		_ = reader.Close()
		issue := Issue{IssueFileTooLarge, candidate.path, "the manifest exceeded the configured per-file byte limit"}
		return Manifest{}, 0, &issue, errInvalidParserResult
	}
	if size > a.limits.MaxTotalBytes-totalBytes {
		_ = reader.Close()
		issue := Issue{IssueTotalBytesLimit, candidate.path, "remaining manifests were omitted by the configured total byte limit"}
		return Manifest{}, 0, &issue, errInvalidParserResult
	}

	allowed := min(a.limits.MaxFileBytes, a.limits.MaxTotalBytes-totalBytes)
	counter := &countingReader{reader: &contextReader{ctx: ctx, reader: reader}}
	limited := &io.LimitedReader{R: counter, N: allowed + 1}
	document := Document{Path: candidate.path, Size: size, Reader: limited}
	manifest, parseErr := parser.Parse(ctx, document, a.limits)
	drained, drainErr := io.Copy(io.Discard, limited)
	closeErr := reader.Close()

	if err := ctx.Err(); err != nil {
		return Manifest{}, counter.read, nil, err
	}
	if counter.read > a.limits.MaxFileBytes {
		issue := Issue{IssueFileTooLarge, candidate.path, "the manifest exceeded the configured per-file byte limit"}
		return Manifest{}, counter.read, &issue, errInvalidParserResult
	}
	if counter.read > a.limits.MaxTotalBytes-totalBytes {
		issue := Issue{IssueTotalBytesLimit, candidate.path, "remaining manifests were omitted by the configured total byte limit"}
		return Manifest{}, counter.read, &issue, errInvalidParserResult
	}
	if counter.read != size {
		issue := Issue{IssueReadFailed, candidate.path, "the manifest size changed while it was being read"}
		return Manifest{}, counter.read, &issue, errInvalidParserResult
	}
	if drainErr != nil || closeErr != nil {
		issue := Issue{IssueReadFailed, candidate.path, "the manifest could not be read completely"}
		return Manifest{}, counter.read, &issue, errors.Join(drainErr, closeErr)
	}
	if parseErr != nil {
		issue := Issue{IssueParseFailed, candidate.path, "the manifest content is invalid or unsupported"}
		return Manifest{}, counter.read, &issue, parseErr
	}
	if drained != 0 {
		issue := Issue{IssueParseFailed, candidate.path, "the manifest parser did not consume the complete document"}
		return Manifest{}, counter.read, &issue, errInvalidParserResult
	}
	return manifest, counter.read, nil, nil
}

type manifestCandidate struct {
	path   string
	format Format
}

func (a Analyzer) manifestCandidates(files []string) []manifestCandidate {
	candidates := make([]manifestCandidate, 0, min(len(files), 16))
	for _, file := range files {
		if ignoredManifestPath(file) {
			continue
		}
		if format, exists := a.formatsByFilename[path.Base(file)]; exists {
			candidates = append(candidates, manifestCandidate{path: file, format: format})
		}
	}
	return candidates
}

func defaultFormatsByFilename() map[string]Format {
	return map[string]Format{
		"go.mod":         FormatGoModule,
		"go.work":        FormatGoWorkspace,
		"package.json":   FormatNodePackage,
		"pyproject.toml": FormatPythonProject,
		"Cargo.toml":     FormatRustPackage,
		"composer.json":  FormatPHPComposer,
		"pom.xml":        FormatMavenProject,
	}
}

func validFormat(format Format) bool {
	value := string(format)
	if value == "" || !lowerAlphaNumeric(value[0]) || !lowerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
		if lowerAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if (character != '-' && character != '_' && character != '.') || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func lowerAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= '0' && character <= '9')
}

func validManifestFilename(filename string) bool {
	return validValue(filename, 255) && path.Base(filename) == filename &&
		filename != "." && filename != ".." && !strings.Contains(filename, "\\")
}

func ignoredManifestPath(file string) bool {
	return repositorypath.IsExcludedFile(file)
}

func validRepositoryPath(value string) bool {
	if !validValue(value, len(value)) || strings.Contains(value, "\\") || path.IsAbs(value) ||
		looksLikeWindowsPath(value) {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func validValue(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func looksLikeWindowsPath(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') ||
		(value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func boundedByteTotal(current, added, maximum int64) int64 {
	if added >= maximum-current {
		return maximum
	}
	return current + added
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.read += int64(count)
	return count, err
}

func emptyResult(partial bool) Result {
	return Result{Manifests: make([]Manifest, 0), Issues: make([]Issue, 0), Partial: partial}
}

func (r *Result) addIssue(issue Issue, maxIssues int) {
	r.Partial = true
	if len(r.Issues) < maxIssues {
		r.Issues = append(r.Issues, issue)
		return
	}
	r.Issues[maxIssues-1] = Issue{
		Code:    IssueLimit,
		Path:    ".",
		Message: "additional manifest issues were omitted by the configured limit",
	}
}

func compareIssues(left, right Issue) int {
	if compared := strings.Compare(left.Path, right.Path); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Code, right.Code); compared != 0 {
		return compared
	}
	return strings.Compare(left.Message, right.Message)
}
