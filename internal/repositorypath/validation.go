package repositorypath

import (
	"path"
	"strings"
	"unicode/utf8"
)

// IsValidFile reports whether a path is a normalized, safe repository-relative file path.
func IsValidFile(filePath string) bool {
	return filePath != "." && isValidRelativePath(filePath)
}

// IsValidDirectory reports whether a path is a normalized, safe repository-relative directory path.
// The repository root is represented by a single dot.
func IsValidDirectory(directoryPath string) bool {
	return isValidRelativePath(directoryPath)
}

// Contains reports whether candidate is a safe file contained by directory.
func Contains(directory, candidate string) bool {
	if !IsValidDirectory(directory) || !IsValidFile(candidate) {
		return false
	}
	if directory == "." {
		return true
	}
	return strings.HasPrefix(candidate, directory+"/")
}

func isValidRelativePath(repositoryPath string) bool {
	if !validPathText(repositoryPath) || strings.ContainsRune(repositoryPath, '\\') ||
		path.IsAbs(repositoryPath) || hasWindowsDrivePrefix(repositoryPath) {
		return false
	}

	cleanedPath := path.Clean(repositoryPath)
	if cleanedPath != repositoryPath || cleanedPath == ".." ||
		strings.HasPrefix(cleanedPath, "../") {
		return false
	}
	return true
}

func validPathText(repositoryPath string) bool {
	if repositoryPath == "" || !utf8.ValidString(repositoryPath) ||
		strings.TrimSpace(repositoryPath) != repositoryPath {
		return false
	}
	for _, character := range repositoryPath {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}

func hasWindowsDrivePrefix(repositoryPath string) bool {
	if len(repositoryPath) < 2 || repositoryPath[1] != ':' {
		return false
	}
	firstCharacter := repositoryPath[0]
	return (firstCharacter >= 'A' && firstCharacter <= 'Z') ||
		(firstCharacter >= 'a' && firstCharacter <= 'z')
}
