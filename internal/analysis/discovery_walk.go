package analysis

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const directoryReadChunkSize = 128

type discoveryWalker struct {
	filesystem fs.FS
	limits     DiscoveryLimits
	inventory  Inventory
	stopped    bool
}

func discoverFilesystem(
	ctx context.Context,
	filesystem fs.FS,
	limits DiscoveryLimits,
) (Inventory, error) {
	walker := discoveryWalker{
		filesystem: filesystem,
		limits:     limits,
		inventory:  emptyInventory(),
	}
	walker.inventory.Directories = append(walker.inventory.Directories, ".")

	err := walker.walk(ctx, ".", 0)
	if err != nil {
		walker.inventory.Partial = true
	}
	finalizeInventory(&walker.inventory)
	return walker.inventory, err
}

func (w *discoveryWalker) walk(ctx context.Context, directory string, depth int) error {
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
			if err := w.walk(ctx, relativePath, depth+1); err != nil {
				return err
			}
			if w.stopped {
				return nil
			}
		case mode.IsRegular():
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

	entries := make([]fs.DirEntry, 0, min(directoryReadChunkSize, w.limits.MaxEntriesPerDirectory+1))
	for {
		remaining := w.limits.MaxEntriesPerDirectory + 1 - len(entries)
		chunk, readErr := reader.ReadDir(min(directoryReadChunkSize, remaining))
		entries = append(entries, chunk...)
		if len(entries) > w.limits.MaxEntriesPerDirectory {
			w.addIssue(DiscoveryIssue{
				Code:    DiscoveryIssueDirectoryEntryLimit,
				Path:    directory,
				Message: "The directory entry limit was reached; its entries were skipped.",
			})
			w.stopped = true
			return nil, directoryFile.Close()
		}
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
	switch name {
	case ".git", ".hg", ".svn":
		return true
	default:
		return false
	}
}
