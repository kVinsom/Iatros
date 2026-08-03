package analysis

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/kVinsom/Iatros/internal/repositoryignore"
	"github.com/kVinsom/Iatros/internal/repositorypath"
)

const directoryReadChunkSize = 128

type discoveryWalker struct {
	filesystem         fs.FS
	limits             DiscoveryLimits
	inventory          Inventory
	ignoreFiles        int
	ignoreRules        int
	declaredSubmodules map[string]struct{}
	submoduleAncestors map[string]struct{}
	seenSubmodules     map[string]struct{}
	stopped            bool
}

func discoverFilesystem(
	ctx context.Context,
	filesystem fs.FS,
	limits DiscoveryLimits,
) (Inventory, error) {
	walker := discoveryWalker{
		filesystem:         filesystem,
		limits:             limits,
		inventory:          emptyInventory(),
		declaredSubmodules: make(map[string]struct{}),
		submoduleAncestors: make(map[string]struct{}),
		seenSubmodules:     make(map[string]struct{}),
	}
	walker.inventory.Directories = append(walker.inventory.Directories, ".")

	err := walker.walk(ctx, ".", 0, nil)
	if err != nil {
		walker.inventory.Partial = true
	}
	finalizeInventory(&walker.inventory)
	return walker.inventory, err
}

func (w *discoveryWalker) walk(
	ctx context.Context,
	directory string,
	depth int,
	inheritedRules []repositoryignore.Rule,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := w.readDirectory(directory)
	if err != nil || w.stopped {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if directory != "." && nestedGitBoundary(entries) {
		w.addSubmodule(directory)
		return nil
	}
	if directory == "." {
		if err := w.loadDeclaredSubmodules(ctx, entries); err != nil {
			return err
		}
	}
	rules, err := w.rulesForDirectory(ctx, directory, entries, inheritedRules)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ignoredDiscoveryEntry(entry.Name()) {
			continue
		}

		relativePath := path.Join(directory, entry.Name())
		if !validRelativePath(relativePath) {
			w.addIssue(DiscoveryIssue{
				Code:    DiscoveryIssueUnsafePath,
				Path:    ".",
				Message: "An unsafe path was skipped.",
			})
			continue
		}
		if entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			w.addIssue(unreadablePathIssue(relativePath))
			continue
		}
		mode := info.Mode()
		if mode&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			continue
		}

		switch {
		case mode.IsDir():
			declaredSubmodule := w.isDeclaredSubmodule(relativePath)
			if !declaredSubmodule && !w.containsDeclaredSubmodule(relativePath) {
				ignored, err := repositoryignore.IgnoredContext(ctx, rules, relativePath, true)
				if err != nil {
					return err
				}
				if ignored {
					continue
				}
			}
			if depth+1 > w.limits.MaxDepth {
				w.addIssue(DiscoveryIssue{
					Code:    DiscoveryIssueDepthLimit,
					Path:    relativePath,
					Message: "The directory was skipped at the configured depth limit.",
				})
				continue
			}
			if len(w.inventory.Directories) >= w.limits.MaxDirectories {
				w.addIssue(DiscoveryIssue{
					Code:    DiscoveryIssueDirectoryLimit,
					Path:    relativePath,
					Message: "The directory limit was reached; remaining entries were skipped.",
				})
				w.stopped = true
				return nil
			}

			w.inventory.Directories = append(w.inventory.Directories, relativePath)
			if declaredSubmodule {
				w.addSubmodule(relativePath)
				continue
			}
			if err := w.walk(ctx, relativePath, depth+1, rules); err != nil {
				return err
			}
			if w.stopped {
				return nil
			}
		case mode.IsRegular():
			if !repositoryControlFile(entry.Name()) {
				ignored, err := repositoryignore.IgnoredContext(ctx, rules, relativePath, false)
				if err != nil {
					return err
				}
				if ignored {
					continue
				}
			}
			if len(w.inventory.Files) >= w.limits.MaxFiles {
				w.addIssue(DiscoveryIssue{
					Code:    DiscoveryIssueFileLimit,
					Path:    relativePath,
					Message: "The file limit was reached; remaining entries were skipped.",
				})
				w.stopped = true
				return nil
			}
			w.inventory.Files = append(w.inventory.Files, relativePath)
		}
	}

	return ctx.Err()
}

func (w *discoveryWalker) readDirectory(directory string) ([]fs.DirEntry, error) {
	directoryFile, err := w.filesystem.Open(directory)
	if err != nil {
		w.addIssue(unreadablePathIssue(directory))
		return nil, nil
	}

	reader, ok := directoryFile.(fs.ReadDirFile)
	if !ok {
		w.addIssue(unreadablePathIssue(directory))
		return nil, directoryFile.Close()
	}

	entries := make(
		[]fs.DirEntry,
		0,
		min(directoryReadChunkSize, w.limits.MaxEntriesPerDirectory),
	)
	for {
		remaining := w.limits.MaxEntriesPerDirectory - len(entries)
		readSize := min(directoryReadChunkSize, remaining)
		if readSize == 0 {
			readSize = 1
		}
		chunk, readErr := reader.ReadDir(readSize)
		if len(chunk) > remaining {
			w.addIssue(DiscoveryIssue{
				Code:    DiscoveryIssueDirectoryEntryLimit,
				Path:    directory,
				Message: "The directory entry limit was reached; its entries were skipped.",
			})
			return nil, directoryFile.Close()
		}
		entries = append(entries, chunk...)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil || len(chunk) == 0 {
			w.addIssue(unreadablePathIssue(directory))
			return nil, directoryFile.Close()
		}
	}

	if err := directoryFile.Close(); err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(left, right fs.DirEntry) int {
		return strings.Compare(left.Name(), right.Name())
	})
	return entries, nil
}

func (w *discoveryWalker) addIssue(issue DiscoveryIssue) {
	recordDiscoveryIssue(&w.inventory, w.limits.MaxIssues, issue)
}

func recordDiscoveryIssue(inventory *Inventory, maxIssues int, issue DiscoveryIssue) {
	inventory.Partial = true
	for _, existing := range inventory.Issues {
		if existing.Code == DiscoveryIssueLimit {
			return
		}
	}
	if len(inventory.Issues) < maxIssues {
		inventory.Issues = append(inventory.Issues, issue)
		return
	}

	inventory.Issues[maxIssues-1] = DiscoveryIssue{
		Code:    DiscoveryIssueLimit,
		Path:    ".",
		Message: "Additional discovery issues were omitted.",
	}
}

func finalizeInventory(inventory *Inventory) {
	slices.Sort(inventory.Directories)
	slices.Sort(inventory.Files)
	slices.Sort(inventory.NestedRepositories)
	inventory.NestedRepositories = slices.Compact(inventory.NestedRepositories)
	slices.SortFunc(inventory.Issues, func(left, right DiscoveryIssue) int {
		if comparison := strings.Compare(left.Path, right.Path); comparison != 0 {
			return comparison
		}
		if comparison := strings.Compare(left.Code, right.Code); comparison != 0 {
			return comparison
		}
		return strings.Compare(left.Message, right.Message)
	})
}

func unreadablePathIssue(relativePath string) DiscoveryIssue {
	return DiscoveryIssue{
		Code:    DiscoveryIssuePathUnreadable,
		Path:    relativePath,
		Message: "The path could not be read and was skipped.",
	}
}

func ignoredDiscoveryEntry(name string) bool {
	return repositorypath.IsExcludedDirectoryName(name)
}

func repositoryControlFile(name string) bool {
	return name == ".gitignore" || name == ".gitmodules"
}

func nestedGitBoundary(entries []fs.DirEntry) bool {
	entry := namedEntry(entries, ".git")
	if entry == nil || entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return false
	}
	info, err := entry.Info()
	return err == nil && (info.Mode().IsRegular() || info.IsDir())
}

func (w *discoveryWalker) isDeclaredSubmodule(directory string) bool {
	_, exists := w.declaredSubmodules[directory]
	return exists
}

func (w *discoveryWalker) containsDeclaredSubmodule(directory string) bool {
	_, exists := w.submoduleAncestors[directory]
	return exists
}

func (w *discoveryWalker) addSubmodule(directory string) {
	if _, exists := w.seenSubmodules[directory]; exists {
		return
	}
	w.seenSubmodules[directory] = struct{}{}
	if len(w.inventory.NestedRepositories) >= w.limits.MaxNestedRepositories {
		w.addIssue(DiscoveryIssue{
			Code:    DiscoveryIssueNestedRepositoryLimit,
			Path:    directory,
			Message: "Additional nested repository boundaries were omitted by the configured limit.",
		})
		return
	}
	w.inventory.NestedRepositories = append(w.inventory.NestedRepositories, directory)
}
