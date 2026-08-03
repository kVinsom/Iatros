package analysis

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
	"time"
)

func TestDefaultDiscoveryLimits(t *testing.T) {
	t.Parallel()

	limits := DefaultDiscoveryLimits()
	want := DiscoveryLimits{
		MaxFiles:               2_000,
		MaxDirectories:         500,
		MaxDepth:               20,
		MaxIssues:              50,
		MaxEntriesPerDirectory: 2_500,
		Timeout:                5 * time.Second,
	}

	if limits != want {
		t.Fatalf("DefaultDiscoveryLimits() = %#v, want %#v", limits, want)
	}
	if err := limits.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDiscoveryLimitsRejectInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*DiscoveryLimits)
	}{
		{name: "files", mutate: func(limits *DiscoveryLimits) { limits.MaxFiles = 0 }},
		{name: "directories", mutate: func(limits *DiscoveryLimits) { limits.MaxDirectories = 0 }},
		{name: "depth", mutate: func(limits *DiscoveryLimits) { limits.MaxDepth = -1 }},
		{name: "issues", mutate: func(limits *DiscoveryLimits) { limits.MaxIssues = 0 }},
		{
			name: "directory entries",
			mutate: func(limits *DiscoveryLimits) {
				limits.MaxEntriesPerDirectory = 0
			},
		},
		{name: "timeout", mutate: func(limits *DiscoveryLimits) { limits.Timeout = 0 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultDiscoveryLimits()
			test.mutate(&limits)
			if err := limits.Validate(); !errors.Is(err, ErrInvalidDiscoveryLimits) {
				t.Fatalf("Validate() error = %v, want ErrInvalidDiscoveryLimits", err)
			}
			if _, err := NewLocalDiscovery(limits); !errors.Is(err, ErrInvalidDiscoveryLimits) {
				t.Fatalf("NewLocalDiscovery() error = %v, want ErrInvalidDiscoveryLimits", err)
			}
		})
	}
}

func TestDiscoverFilesystemReturnsDeterministicInventory(t *testing.T) {
	t.Parallel()

	filesystem := fstest.MapFS{
		".git/config":                   &fstest.MapFile{Data: []byte("ignored")},
		".gradle/cache.bin":             &fstest.MapFile{Data: []byte("ignored")},
		".gitignore":                    &fstest.MapFile{Data: []byte("ignored/\n")},
		".hg/store":                     &fstest.MapFile{Data: []byte("ignored")},
		".svn/entries":                  &fstest.MapFile{Data: []byte("ignored")},
		".terraform/main.tf":            &fstest.MapFile{Data: []byte("ignored")},
		"cmd/main.go":                   &fstest.MapFile{Data: []byte("package main")},
		"docs/README.md":                &fstest.MapFile{Data: []byte("documentation")},
		"go.mod":                        &fstest.MapFile{Data: []byte("module example")},
		"ignored/kept.txt":              &fstest.MapFile{Data: []byte("kept")},
		"link":                          &fstest.MapFile{Mode: fs.ModeSymlink | 0o777},
		"nested/.git/config":            &fstest.MapFile{Data: []byte("ignored")},
		"nested/app.go":                 &fstest.MapFile{Data: []byte("package nested")},
		"node_modules/pkg/package.json": &fstest.MapFile{Data: []byte("ignored")},
	}

	first, err := discoverFilesystem(t.Context(), filesystem, DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	second, err := discoverFilesystem(t.Context(), filesystem, DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("second discoverFilesystem() error = %v", err)
	}

	wantDirectories := []string{".", "cmd", "docs", "ignored", "nested"}
	wantFiles := []string{
		".gitignore",
		"cmd/main.go",
		"docs/README.md",
		"go.mod",
		"ignored/kept.txt",
		"nested/app.go",
	}
	if !slices.Equal(first.Directories, wantDirectories) {
		t.Fatalf("Directories = %#v, want %#v", first.Directories, wantDirectories)
	}
	if !slices.Equal(first.Files, wantFiles) {
		t.Fatalf("Files = %#v, want %#v", first.Files, wantFiles)
	}
	if first.Partial || len(first.Issues) != 0 {
		t.Fatalf("inventory = %#v, want complete without issues", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("inventory is not deterministic:\nfirst: %#v\nsecond: %#v", first, second)
	}
}

func TestDiscoverFilesystemDoesNotOpenRegularFiles(t *testing.T) {
	t.Parallel()

	filesystem := denyRegularFileOpenFS{
		FS: fstest.MapFS{
			"README.md":   &fstest.MapFile{Data: []byte("documentation")},
			"cmd/main.go": &fstest.MapFile{Data: []byte("package main")},
		},
		denied: map[string]bool{
			"README.md":   true,
			"cmd/main.go": true,
		},
	}

	inventory, err := discoverFilesystem(t.Context(), filesystem, DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}

	wantFiles := []string{"README.md", "cmd/main.go"}
	if !slices.Equal(inventory.Files, wantFiles) {
		t.Fatalf("Files = %#v, want %#v", inventory.Files, wantFiles)
	}
	if inventory.Partial || len(inventory.Issues) != 0 {
		t.Fatalf("inventory = %#v, want complete without issues", inventory)
	}
}

func TestDiscoverFilesystemEnforcesWorkLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		filesystem      fs.FS
		configure       func(*DiscoveryLimits)
		wantDirectories []string
		wantFiles       []string
		wantIssueCode   string
		wantIssuePath   string
	}{
		{
			name: "file limit",
			filesystem: fstest.MapFS{
				"a.txt": &fstest.MapFile{},
				"b.txt": &fstest.MapFile{},
				"c.txt": &fstest.MapFile{},
			},
			configure:       func(limits *DiscoveryLimits) { limits.MaxFiles = 2 },
			wantDirectories: []string{"."},
			wantFiles:       []string{"a.txt", "b.txt"},
			wantIssueCode:   DiscoveryIssueFileLimit,
			wantIssuePath:   "c.txt",
		},
		{
			name: "directory limit",
			filesystem: fstest.MapFS{
				"a/file.txt": &fstest.MapFile{},
				"b/file.txt": &fstest.MapFile{},
			},
			configure:       func(limits *DiscoveryLimits) { limits.MaxDirectories = 2 },
			wantDirectories: []string{".", "a"},
			wantFiles:       []string{"a/file.txt"},
			wantIssueCode:   DiscoveryIssueDirectoryLimit,
			wantIssuePath:   "b",
		},
		{
			name: "depth limit",
			filesystem: fstest.MapFS{
				"a/b/deep.txt": &fstest.MapFile{},
				"a/file.txt":   &fstest.MapFile{},
				"z.txt":        &fstest.MapFile{},
			},
			configure:       func(limits *DiscoveryLimits) { limits.MaxDepth = 1 },
			wantDirectories: []string{".", "a"},
			wantFiles:       []string{"a/file.txt", "z.txt"},
			wantIssueCode:   DiscoveryIssueDepthLimit,
			wantIssuePath:   "a/b",
		},
		{
			name: "directory entry limit",
			filesystem: fstest.MapFS{
				"a.txt": &fstest.MapFile{},
				"b.txt": &fstest.MapFile{},
				"c.txt": &fstest.MapFile{},
			},
			configure: func(limits *DiscoveryLimits) {
				limits.MaxEntriesPerDirectory = 2
			},
			wantDirectories: []string{"."},
			wantFiles:       []string{},
			wantIssueCode:   DiscoveryIssueDirectoryEntryLimit,
			wantIssuePath:   ".",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultDiscoveryLimits()
			test.configure(&limits)
			inventory, err := discoverFilesystem(t.Context(), test.filesystem, limits)
			if err != nil {
				t.Fatalf("discoverFilesystem() error = %v", err)
			}
			if !inventory.Partial {
				t.Fatal("Partial = false, want true")
			}
			if !slices.Equal(inventory.Directories, test.wantDirectories) {
				t.Fatalf(
					"Directories = %#v, want %#v",
					inventory.Directories,
					test.wantDirectories,
				)
			}
			if !slices.Equal(inventory.Files, test.wantFiles) {
				t.Fatalf("Files = %#v, want %#v", inventory.Files, test.wantFiles)
			}
			assertSingleDiscoveryIssue(t, inventory, test.wantIssueCode, test.wantIssuePath)
		})
	}
}

func TestDiscoverFilesystemContinuesAfterNestedDirectoryEntryLimit(t *testing.T) {
	t.Parallel()

	filesystem := fstest.MapFS{
		"a/one.txt":   &fstest.MapFile{},
		"a/three.txt": &fstest.MapFile{},
		"a/two.txt":   &fstest.MapFile{},
		"z.txt":       &fstest.MapFile{},
	}
	limits := DefaultDiscoveryLimits()
	limits.MaxEntriesPerDirectory = 2

	inventory, err := discoverFilesystem(t.Context(), filesystem, limits)
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	if !slices.Equal(inventory.Directories, []string{".", "a"}) {
		t.Fatalf("Directories = %#v, want root and skipped directory", inventory.Directories)
	}
	if !slices.Equal(inventory.Files, []string{"z.txt"}) {
		t.Fatalf("Files = %#v, want safe sibling after skipped directory", inventory.Files)
	}
	assertSingleDiscoveryIssue(t, inventory, DiscoveryIssueDirectoryEntryLimit, "a")
}

func TestDiscoverFilesystemAcceptsMaximumDirectoryEntryLimit(t *testing.T) {
	t.Parallel()

	limits := DefaultDiscoveryLimits()
	limits.MaxEntriesPerDirectory = math.MaxInt
	filesystem := fstest.MapFS{
		"a.txt": &fstest.MapFile{},
		"b.txt": &fstest.MapFile{},
	}

	inventory, err := discoverFilesystem(t.Context(), filesystem, limits)
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	if !slices.Equal(inventory.Files, []string{"a.txt", "b.txt"}) {
		t.Fatalf("Files = %#v, want both files", inventory.Files)
	}
	if inventory.Partial || len(inventory.Issues) != 0 {
		t.Fatalf("inventory = %#v, want complete inventory", inventory)
	}
}

func TestDiscoverFilesystemRecordsAccessIssues(t *testing.T) {
	t.Parallel()

	base := fstest.MapFS{
		"open/file.txt":       &fstest.MapFile{},
		"restricted/file.txt": &fstest.MapFile{},
	}
	filesystem := readDirErrorFS{
		FS:     base,
		denied: map[string]bool{"restricted": true},
	}

	inventory, err := discoverFilesystem(t.Context(), filesystem, DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	if !slices.Equal(inventory.Files, []string{"open/file.txt"}) {
		t.Fatalf("Files = %#v, want only readable file", inventory.Files)
	}
	assertSingleDiscoveryIssue(
		t,
		inventory,
		DiscoveryIssuePathUnreadable,
		"restricted",
	)
}

func TestDiscoverFilesystemBoundsIssues(t *testing.T) {
	t.Parallel()

	base := fstest.MapFS{
		"restricted-a/file.txt": &fstest.MapFile{},
		"restricted-b/file.txt": &fstest.MapFile{},
		"restricted-c/file.txt": &fstest.MapFile{},
	}
	filesystem := readDirErrorFS{
		FS: base,
		denied: map[string]bool{
			"restricted-a": true,
			"restricted-b": true,
			"restricted-c": true,
		},
	}
	limits := DefaultDiscoveryLimits()
	limits.MaxIssues = 2

	inventory, err := discoverFilesystem(t.Context(), filesystem, limits)
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	if len(inventory.Issues) != limits.MaxIssues {
		t.Fatalf("len(Issues) = %d, want %d", len(inventory.Issues), limits.MaxIssues)
	}
	if !containsDiscoveryIssue(inventory.Issues, DiscoveryIssueLimit) {
		t.Fatalf("Issues = %#v, want issue-limit marker", inventory.Issues)
	}
}

func TestDiscoverFilesystemSkipsUnsafePaths(t *testing.T) {
	t.Parallel()

	filesystem := fstest.MapFS{
		"safe.txt":         &fstest.MapFile{},
		"unsafe\nname.txt": &fstest.MapFile{},
	}

	inventory, err := discoverFilesystem(t.Context(), filesystem, DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("discoverFilesystem() error = %v", err)
	}
	if !slices.Equal(inventory.Files, []string{"safe.txt"}) {
		t.Fatalf("Files = %#v, want safe file only", inventory.Files)
	}
	assertSingleDiscoveryIssue(t, inventory, DiscoveryIssueUnsafePath, ".")
}

func TestDiscoverFilesystemHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	inventory, err := discoverFilesystem(ctx, fstest.MapFS{}, DefaultDiscoveryLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("discoverFilesystem() error = %v, want context.Canceled", err)
	}
	if !inventory.Partial {
		t.Fatal("Partial = false, want true")
	}
}

func TestLocalDiscoveryUsesConfinedLocalRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, directory := range []string{".git", "ignored"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
	}
	files := map[string]string{
		".git/config":      "ignored",
		".gitignore":       "ignored/\n",
		"go.mod":           "module example",
		"ignored/kept.txt": "kept",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	linkCreated := os.Symlink(outside, filepath.Join(root, "linked-directory")) == nil

	discovery, err := NewLocalDiscovery(DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("NewLocalDiscovery() error = %v", err)
	}
	inventory, err := discovery.Discover(t.Context(), root)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	wantDirectories := []string{".", "ignored"}
	wantFiles := []string{".gitignore", "go.mod", "ignored/kept.txt"}
	if !slices.Equal(inventory.Directories, wantDirectories) {
		t.Fatalf("Directories = %#v, want %#v", inventory.Directories, wantDirectories)
	}
	if !slices.Equal(inventory.Files, wantFiles) {
		t.Fatalf("Files = %#v, want %#v", inventory.Files, wantFiles)
	}
	if linkCreated && slices.Contains(inventory.Files, "linked-directory/secret.txt") {
		t.Fatal("discovery followed a symbolic link outside the selected root")
	}
	if inventory.Partial || len(inventory.Issues) != 0 {
		t.Fatalf("inventory = %#v, want complete without issues", inventory)
	}
}

func TestResolveDiscoveryFailureConvertsInternalTimeoutToIssue(t *testing.T) {
	t.Parallel()

	discoveryCtx, cancelDiscovery := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancelDiscovery()

	inventory, err := resolveDiscoveryFailure(
		t.Context(),
		discoveryCtx,
		emptyInventory(),
		DefaultDiscoveryLimits().MaxIssues,
		fs.ErrInvalid,
	)
	if err != nil {
		t.Fatalf("resolveDiscoveryFailure() error = %v", err)
	}
	if !inventory.Partial {
		t.Fatal("Partial = false, want true")
	}
	assertSingleDiscoveryIssue(t, inventory, DiscoveryIssueTimeout, ".")
}

func TestResolveDiscoveryFailurePrefersParentCancellation(t *testing.T) {
	t.Parallel()

	parentCtx, cancelParent := context.WithCancel(t.Context())
	discoveryCtx, cancelDiscovery := context.WithCancel(parentCtx)
	cancelParent()
	defer cancelDiscovery()

	inventory, err := resolveDiscoveryFailure(
		parentCtx,
		discoveryCtx,
		emptyInventory(),
		DefaultDiscoveryLimits().MaxIssues,
		fs.ErrInvalid,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveDiscoveryFailure() error = %v, want context.Canceled", err)
	}
	if inventory.Partial || len(inventory.Issues) != 0 {
		t.Fatalf("inventory = %#v, want unchanged inventory", inventory)
	}
}

func TestResolveDiscoveryFailureReturnsFilesystemError(t *testing.T) {
	t.Parallel()

	inventory, err := resolveDiscoveryFailure(
		t.Context(),
		t.Context(),
		emptyInventory(),
		DefaultDiscoveryLimits().MaxIssues,
		fs.ErrInvalid,
	)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("resolveDiscoveryFailure() error = %v, want fs.ErrInvalid", err)
	}
	if inventory.Partial || len(inventory.Issues) != 0 {
		t.Fatalf("inventory = %#v, want unchanged inventory", inventory)
	}
}

func TestLocalDiscoveryHonorsParentCancellation(t *testing.T) {
	t.Parallel()

	discovery, err := NewLocalDiscovery(DefaultDiscoveryLimits())
	if err != nil {
		t.Fatalf("NewLocalDiscovery() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	inventory, err := discovery.Discover(ctx, filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() error = %v, want context.Canceled", err)
	}
	if inventory.Directories == nil || inventory.Files == nil || inventory.Issues == nil {
		t.Fatal("error inventory collections must not be nil")
	}
}

func assertSingleDiscoveryIssue(
	t *testing.T,
	inventory Inventory,
	code string,
	issuePath string,
) {
	t.Helper()

	if len(inventory.Issues) != 1 {
		t.Fatalf("Issues = %#v, want one issue", inventory.Issues)
	}
	issue := inventory.Issues[0]
	if issue.Code != code || issue.Path != issuePath || issue.Message == "" {
		t.Fatalf("issue = %#v, want code %q and path %q", issue, code, issuePath)
	}
}

func containsDiscoveryIssue(issues []DiscoveryIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type denyRegularFileOpenFS struct {
	fs.FS
	denied map[string]bool
}

func (filesystem denyRegularFileOpenFS) Open(name string) (fs.File, error) {
	if filesystem.denied[name] {
		return nil, errors.New("regular file content must not be opened")
	}
	return filesystem.FS.Open(name)
}

type readDirErrorFS struct {
	fs.FS
	denied map[string]bool
}

func (filesystem readDirErrorFS) Open(name string) (fs.File, error) {
	file, err := filesystem.FS.Open(name)
	if err != nil || !filesystem.denied[name] {
		return file, err
	}
	if _, ok := file.(fs.ReadDirFile); !ok {
		return file, nil
	}
	return &readDirErrorFile{File: file}, nil
}

type readDirErrorFile struct {
	fs.File
}

func (*readDirErrorFile) ReadDir(int) ([]fs.DirEntry, error) {
	return nil, fs.ErrPermission
}
