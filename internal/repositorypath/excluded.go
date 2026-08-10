// Package repositorypath defines shared repository-relative path policies.
package repositorypath

import (
	"path"
	"strings"
)

// IsExcludedDirectoryName reports whether a directory contains VCS metadata,
// downloaded dependencies, or generated tool state that should not be analyzed.
func IsExcludedDirectoryName(name string) bool {
	switch name {
	case ".git", ".gradle", ".hg", ".pnpm", ".svn", ".terraform", ".venv",
		".yarn", "node_modules", "venv":
		return true
	default:
		return false
	}
}

// IsExcludedFile reports whether a repository-relative file is inside an excluded directory.
func IsExcludedFile(file string) bool {
	return containsExcludedDirectory(path.Dir(file))
}

// IsExcludedDirectory reports whether a repository-relative directory is excluded.
func IsExcludedDirectory(directory string) bool {
	return containsExcludedDirectory(directory)
}

func containsExcludedDirectory(repositoryPath string) bool {
	for segment := range strings.SplitSeq(repositoryPath, "/") {
		if IsExcludedDirectoryName(segment) {
			return true
		}
	}
	return false
}
