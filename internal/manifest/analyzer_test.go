package manifest

import (
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memorySource map[string][]byte

func (s memorySource) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	data, exists := s[path]
	if !exists {
		return nil, 0, errors.New("missing fixture")
	}
	return io.NopCloser(strings.NewReader(string(data))), int64(len(data)), nil
}

type waitingSource struct{}

func (waitingSource) Open(ctx context.Context, _ string) (io.ReadCloser, int64, error) {
	<-ctx.Done()
	return nil, 0, ctx.Err()
}

func TestAnalyzerParsesSupportedManifests(t *testing.T) {
	t.Parallel()

	source := memorySource{
		"go.mod":  []byte("module example.com/service\n\ngo 1.25.0\n\nrequire golang.org/x/text v0.31.0\n"),
		"go.work": []byte("go 1.25.0\n\nuse (\n\t./services/api\n\t./services/worker\n)\n"),
		"web/package.json": []byte(`{
  "name": "web",
  "version": "1.2.3",
  "engines": {"node": ">=22"},
  "dependencies": {"react": "^19.0.0", "sharp": "1.0.0"},
  "optionalDependencies": {"sharp": "2.0.0"},
  "devDependencies": {"typescript": "^6.0.0"},
  "peerDependencies": {"vite": "^7.0.0"},
  "peerDependenciesMeta": {"vite": {"optional": true}},
  "workspaces": {"packages": ["packages/*"]}
}`),
		"python/pyproject.toml": []byte(`[project]
name = "worker"
version = "0.4.0"
requires-python = ">=3.13"
dependencies = ["httpx>=0.28"]

[project.optional-dependencies]
postgres = ["psycopg[binary]>=3.2"]

[build-system]
requires = ["hatchling>=1.27"]

[tool.poetry.group.compat.dependencies]
urllib3 = [
  { version = "<2", python = "<3.10" },
  { version = ">=2", python = ">=3.10" }
]
`),
		"rust/Cargo.toml": []byte(`[package]
name = "agent"
version = "0.1.0"
rust-version = "1.88"

[dependencies]
serde = { version = "1", optional = true }

[workspace]
members = ["crates/*"]
default-members = ["crates/default"]
exclude = ["crates/private"]
`),
		"php/composer.json": []byte(`{
  "name": "example/api",
  "version": "1.0.0",
  "require": {"php": ">=8.4", "symfony/console": "^7.0"},
  "require-dev": {"phpunit/phpunit": "^12.0"}
}`),
		"java/pom.xml": []byte(`<project>
  <groupId>com.example</groupId><artifactId>gateway</artifactId><version>2.0.0</version>
  <properties><maven.compiler.release>25</maven.compiler.release></properties>
  <modules><module>api</module></modules>
  <dependencies>
    <dependency><groupId>org.slf4j</groupId><artifactId>slf4j-api</artifactId><version>2.0.17</version></dependency>
    <dependency><groupId>org.junit.jupiter</groupId><artifactId>junit-jupiter</artifactId><scope>test</scope></dependency>
  </dependencies>
</project>`),
	}
	analyzer := mustDefaultAnalyzer(t, DefaultLimits())
	files := make([]string, 0, len(source))
	for path := range source {
		files = append(files, path)
	}
	result, err := analyzer.Analyze(context.Background(), source, Snapshot{Files: files})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Partial || len(result.Issues) != 0 {
		t.Fatalf("Analyze() partial result = %+v", result)
	}
	if len(result.Manifests) != 7 {
		t.Fatalf("len(Manifests) = %d, want 7", len(result.Manifests))
	}

	assertManifest(t, result, "go.mod", func(manifest Manifest) {
		if manifest.Module != "example.com/service" || len(manifest.Dependencies) != 1 {
			t.Errorf("Go manifest = %+v", manifest)
		}
	})
	assertManifest(t, result, "go.work", func(manifest Manifest) {
		if !manifest.WorkspaceDeclared || len(manifest.WorkspaceMembers) != 2 {
			t.Errorf("Go workspace manifest = %+v", manifest)
		}
	})
	assertManifest(t, result, "web/package.json", func(manifest Manifest) {
		if manifest.Name != "web" || len(manifest.Dependencies) != 4 ||
			!manifest.WorkspaceDeclared ||
			!slices.Equal(manifest.WorkspaceMembers, []string{"packages/*"}) {
			t.Errorf("Node manifest = %+v", manifest)
		}
		for _, dependency := range manifest.Dependencies {
			if dependency.Name == "sharp" && (!dependency.Optional || dependency.Constraint != "2.0.0") {
				t.Errorf("optional dependency = %+v", dependency)
			}
		}
	})
	assertManifest(t, result, "python/pyproject.toml", func(manifest Manifest) {
		if manifest.Name != "worker" || len(manifest.Dependencies) != 5 {
			t.Errorf("Python manifest = %+v", manifest)
		}
	})
	assertManifest(t, result, "rust/Cargo.toml", func(manifest Manifest) {
		if manifest.Name != "agent" || !manifest.Dependencies[0].Optional ||
			!manifest.WorkspaceDeclared ||
			!slices.Equal(manifest.WorkspaceMembers, []string{"crates/*"}) ||
			!slices.Equal(manifest.WorkspaceExcludes, []string{"crates/private"}) {
			t.Errorf("Rust manifest = %+v", manifest)
		}
	})
	assertManifest(t, result, "php/composer.json", func(manifest Manifest) {
		if len(manifest.Dependencies) != 2 || len(manifest.Constraints) != 1 {
			t.Errorf("Composer manifest = %+v", manifest)
		}
	})
	assertManifest(t, result, "java/pom.xml", func(manifest Manifest) {
		if manifest.Name != "com.example:gateway" || len(manifest.Dependencies) != 2 ||
			!manifest.WorkspaceDeclared {
			t.Errorf("Maven manifest = %+v", manifest)
		}
	})
}

func TestAnalyzerPreservesExplicitEmptyWorkspaces(t *testing.T) {
	t.Parallel()

	source := memorySource{
		"package.json": []byte(`{"name":"root","workspaces":[]}`),
		"Cargo.toml":   []byte("[workspace]\n"),
	}
	analyzer := mustDefaultAnalyzer(t, DefaultLimits())
	result, err := analyzer.Analyze(t.Context(), source, Snapshot{
		Files: []string{"Cargo.toml", "package.json"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Partial || len(result.Manifests) != 2 {
		t.Fatalf("Analyze() = %+v, want two complete manifests", result)
	}
	for _, value := range result.Manifests {
		if !value.WorkspaceDeclared || len(value.WorkspaceMembers) != 0 ||
			len(value.WorkspaceExcludes) != 0 {
			t.Fatalf("manifest = %+v, want explicit empty workspace", value)
		}
	}
}

func TestAnalyzerRejectsUnsafeOrAmbiguousContent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path    string
		content string
	}{
		"duplicate JSON key": {
			path:    "package.json",
			content: `{"name":"first","name":"second"}`,
		},
		"case-fold duplicate JSON key": {
			path:    "package.json",
			content: `{"name":"first","Name":"second"}`,
		},
		"Unicode case-fold duplicate JSON key": {
			path:    "package.json",
			content: `{"metadata":{"S":true,"\u017f":false}}`,
		},
		"trailing JSON": {
			path:    "composer.json",
			content: `{} {}`,
		},
		"XML directive": {
			path:    "pom.xml",
			content: `<!DOCTYPE project [<!ENTITY secret SYSTEM "file:///etc/passwd">]><project/>`,
		},
		"multiple XML roots": {
			path:    "pom.xml",
			content: `<project/><project/>`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			analyzer := mustDefaultAnalyzer(t, DefaultLimits())
			result, err := analyzer.Analyze(context.Background(), memorySource{
				test.path: []byte(test.content),
			}, Snapshot{Files: []string{test.path}})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if !result.Partial || len(result.Manifests) != 0 || len(result.Issues) != 1 ||
				result.Issues[0].Code != IssueParseFailed {
				t.Fatalf("Analyze() = %+v", result)
			}
		})
	}
}

func TestAnalyzerEnforcesStructuredNesting(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxNestingDepth = 2
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"package.json": []byte(`{"metadata":{"nested":{"value":true}}}`),
	}, Snapshot{Files: []string{"package.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 0 || result.Issues[0].Code != IssueParseFailed {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerEnforcesCentralLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxFiles = 1
	limits.MaxDependenciesPerManifest = 1
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"a/package.json": []byte(`{"dependencies":{"b":"1","a":"1"}}`),
		"b/package.json": []byte(`{}`),
	}, Snapshot{Files: []string{"b/package.json", "a/package.json"}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !result.Partial || len(result.Manifests) != 1 || len(result.Issues) != 2 {
		t.Fatalf("Analyze() = %+v", result)
	}
	manifest := result.Manifests[0]
	if !manifest.DependenciesTruncated || len(manifest.Dependencies) != 1 ||
		manifest.Dependencies[0].Name != "a" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestAnalyzerEnforcesTotalByteLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxFileBytes = 3
	limits.MaxTotalBytes = 3
	limits.MaxValueBytes = 1
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"a/package.json": []byte(`{}`),
		"b/package.json": []byte(`{}`),
	}, Snapshot{Files: []string{"a/package.json", "b/package.json"}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !result.Partial || len(result.Manifests) != 1 || len(result.Issues) != 1 ||
		result.Issues[0].Code != IssueTotalBytesLimit {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerEnforcesPerFileByteLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxFileBytes = 5
	limits.MaxTotalBytes = 10
	limits.MaxValueBytes = 5
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"package.json": []byte(`{"x":1}`),
	}, Snapshot{Files: []string{"package.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 0 || result.Issues[0].Code != IssueFileTooLarge {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerBoundsWorkspaceExclusions(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxWorkspaceExcludesPerManifest = 1
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"Cargo.toml": []byte(`[workspace]
members = ["crates/*"]
exclude = ["crates/private", "crates/generated"]
`),
	}, Snapshot{Files: []string{"Cargo.toml"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 1 ||
		!result.Manifests[0].WorkspaceExcludesTruncated ||
		len(result.Manifests[0].WorkspaceExcludes) != 1 ||
		result.Issues[0].Code != IssueWorkspaceExcludeLimit {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerBoundsConstraintsAndWorkspaceMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		configure func(*Limits)
		wantCode  string
		assert    func(*testing.T, Manifest)
	}{
		{
			name:    "constraints",
			content: `{"engines":{"z-runtime":"2","a-runtime":"1"}}`,
			configure: func(limits *Limits) {
				limits.MaxConstraintsPerManifest = 1
			},
			wantCode: IssueConstraintLimit,
			assert: func(t *testing.T, value Manifest) {
				if !value.ConstraintsTruncated || len(value.Constraints) != 1 ||
					value.Constraints[0].Name != "a-runtime" {
					t.Fatalf("manifest = %+v, want first bounded constraint", value)
				}
			},
		},
		{
			name:    "workspace members",
			content: `{"workspaces":["z/*","a/*"]}`,
			configure: func(limits *Limits) {
				limits.MaxWorkspaceMembersPerManifest = 1
			},
			wantCode: IssueWorkspaceLimit,
			assert: func(t *testing.T, value Manifest) {
				if !value.WorkspaceMembersTruncated || len(value.WorkspaceMembers) != 1 ||
					value.WorkspaceMembers[0] != "a/*" {
					t.Fatalf("manifest = %+v, want first bounded workspace member", value)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultLimits()
			test.configure(&limits)
			analyzer := mustDefaultAnalyzer(t, limits)
			result, err := analyzer.Analyze(t.Context(), memorySource{
				"package.json": []byte(test.content),
			}, Snapshot{Files: []string{"package.json"}})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if !result.Partial || len(result.Manifests) != 1 || len(result.Issues) != 1 ||
				result.Issues[0].Code != test.wantCode {
				t.Fatalf("Analyze() = %+v, want one %s issue", result, test.wantCode)
			}
			test.assert(t, result.Manifests[0])
		})
	}
}

func TestAnalyzerReportsUnavailableKnownParser(t *testing.T) {
	t.Parallel()

	analyzer, err := NewAnalyzer(DefaultLimits(), nodeParser{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"go.mod": []byte("module example.com/service\n"),
	}, Snapshot{Files: []string{"go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Manifests) != 0 || result.Issues[0].Code != IssueParserUnavailable {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerBoundsIssues(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxIssues = 1
	analyzer := mustDefaultAnalyzer(t, limits)
	result, err := analyzer.Analyze(context.Background(), memorySource{
		"a/package.json": []byte(`{"name":"a","name":"b"}`),
		"b/package.json": []byte(`{"name":"a","name":"b"}`),
	}, Snapshot{Files: []string{"a/package.json", "b/package.json"}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !result.Partial || len(result.Manifests) != 0 || len(result.Issues) != 1 ||
		result.Issues[0].Code != IssueLimit {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzerRejectsInvalidInputsAndHonorsCancellation(t *testing.T) {
	t.Parallel()

	analyzer := mustDefaultAnalyzer(t, DefaultLimits())
	if _, err := analyzer.Analyze(context.Background(), memorySource{}, Snapshot{
		Files: []string{"../package.json"},
	}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Analyze() error = %v, want ErrInvalidSnapshot", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := analyzer.Analyze(ctx, memorySource{}, Snapshot{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze() error = %v, want context.Canceled", err)
	}
}

func TestAnalyzerEnforcesInternalTimeout(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.Timeout = time.Millisecond
	analyzer := mustDefaultAnalyzer(t, limits)
	_, err := analyzer.Analyze(context.Background(), waitingSource{}, Snapshot{
		Files: []string{"package.json"},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Analyze() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestAnalyzerHandlesFaultingSourcesSafely(t *testing.T) {
	t.Parallel()

	readFailure := errors.New("read failed")
	closeFailure := errors.New("close failed")
	tests := []struct {
		name         string
		source       func(*bool) Source
		wantCode     string
		expectClosed bool
	}{
		{
			name: "open failure",
			source: func(*bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return nil, 0, errors.New("open failed")
				})
			},
			wantCode: IssueReadFailed,
		},
		{
			name: "nil reader",
			source: func(*bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return nil, 0, nil
				})
			},
			wantCode: IssueReadFailed,
		},
		{
			name: "negative size",
			source: func(closed *bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return &faultReadCloser{Reader: strings.NewReader(`{}`), closed: closed}, -1, nil
				})
			},
			wantCode: IssueReadFailed, expectClosed: true,
		},
		{
			name: "dishonest size",
			source: func(closed *bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return &faultReadCloser{Reader: strings.NewReader(`{}`), closed: closed}, 1, nil
				})
			},
			wantCode: IssueReadFailed, expectClosed: true,
		},
		{
			name: "read failure",
			source: func(*bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return &faultReadCloser{
						Reader: &stickyErrorReader{data: []byte(`{}`), err: readFailure},
					}, 2, nil
				})
			},
			wantCode: IssueReadFailed,
		},
		{
			name: "close failure",
			source: func(*bool) Source {
				return sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
					return &faultReadCloser{Reader: strings.NewReader(`{}`), closeErr: closeFailure}, 2, nil
				})
			},
			wantCode: IssueReadFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			closed := false
			analyzer := mustDefaultAnalyzer(t, DefaultLimits())
			result, err := analyzer.Analyze(
				t.Context(),
				test.source(&closed),
				Snapshot{Files: []string{"package.json"}},
			)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if !result.Partial || len(result.Manifests) != 0 || len(result.Issues) != 1 ||
				result.Issues[0].Code != test.wantCode {
				t.Fatalf("Analyze() = %+v, want one %s issue", result, test.wantCode)
			}
			if test.expectClosed && !closed {
				t.Fatal("source reader was not closed")
			}
		})
	}
}

func TestAnalyzerHonorsCancellationAfterParserReturns(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	closed := false
	parser := cancelingParser{cancel: cancel}
	analyzer, err := NewAnalyzer(DefaultLimits(), parser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = analyzer.Analyze(ctx, sourceFunc(func(context.Context, string) (io.ReadCloser, int64, error) {
		return &faultReadCloser{Reader: strings.NewReader(`{}`), closed: &closed}, 2, nil
	}), Snapshot{Files: []string{"package.json"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze() error = %v, want context.Canceled", err)
	}
	if !closed {
		t.Fatal("source reader was not closed after cancellation")
	}
}

func assertManifest(t *testing.T, result Result, path string, assertion func(Manifest)) {
	t.Helper()
	for _, manifest := range result.Manifests {
		if manifest.Path == path {
			assertion(manifest)
			return
		}
	}
	t.Errorf("manifest %q not found", path)
}

func mustDefaultAnalyzer(t *testing.T, limits Limits) Analyzer {
	t.Helper()
	analyzer, err := NewDefaultAnalyzer(limits)
	if err != nil {
		t.Fatalf("NewDefaultAnalyzer() error = %v", err)
	}
	return analyzer
}

type sourceFunc func(context.Context, string) (io.ReadCloser, int64, error)

func (function sourceFunc) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	return function(ctx, path)
}

type faultReadCloser struct {
	io.Reader
	closed   *bool
	closeErr error
}

func (reader *faultReadCloser) Close() error {
	if reader.closed != nil {
		*reader.closed = true
	}
	return reader.closeErr
}

type stickyErrorReader struct {
	data []byte
	err  error
}

func (reader *stickyErrorReader) Read(destination []byte) (int, error) {
	if len(reader.data) != 0 {
		count := copy(destination, reader.data)
		reader.data = reader.data[count:]
		return count, nil
	}
	return 0, reader.err
}

type cancelingParser struct {
	cancel context.CancelFunc
}

func (cancelingParser) Format() Format { return FormatNodePackage }

func (cancelingParser) Filenames() []string { return []string{"package.json"} }

func (parser cancelingParser) Parse(
	_ context.Context,
	document Document,
	_ Limits,
) (Manifest, error) {
	_, _ = io.Copy(io.Discard, document.Reader)
	parser.cancel()
	return Manifest{}, nil
}

func BenchmarkAnalyzer(b *testing.B) {
	content := []byte(`{"name":"service","dependencies":{"example":"1.0.0"}}`)
	files := make([]string, 100)
	source := make(memorySource, len(files))
	for index := range files {
		number := strconv.Itoa(index)
		files[index] = "services/service-" + strings.Repeat("0", 3-len(number)) + number + "/package.json"
		source[files[index]] = content
	}
	analyzer, err := NewDefaultAnalyzer(DefaultLimits())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := analyzer.Analyze(context.Background(), source, Snapshot{Files: files}); err != nil {
			b.Fatal(err)
		}
	}
}
