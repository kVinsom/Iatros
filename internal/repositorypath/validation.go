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

// Contains reports whether candidatePath is the directory itself or one of its descendants.
func Contains(directoryPath, candidatePath string) bool {
	if !IsValidDirectory(directoryPath) || !IsValidDirectory(candidatePath) {
		return false
	}
	return directoryPath == "." || candidatePath == directoryPath ||
		strings.HasPrefix(candidatePath, directoryPath+"/")
}

func isValidRelativePath(repositoryPath string) bool {
	if !validPathText(repositoryPath) || strings.ContainsRune(repositoryPath, '\\') ||
		path.IsAbs(repositoryPath) || HasWindowsDrivePrefix(repositoryPath) {
		return false
	}

	cleanedPath := path.Clean(repositoryPath)
	return cleanedPath == repositoryPath && cleanedPath != ".." &&
		!strings.HasPrefix(cleanedPath, "../")
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

// HasWindowsDrivePrefix reports whether a value begins with an ASCII drive designator.
func HasWindowsDrivePrefix(repositoryPath string) bool {
	if len(repositoryPath) < 2 || repositoryPath[1] != ':' {
		return false
	}
	firstCharacter := repositoryPath[0]
	return (firstCharacter >= 'A' && firstCharacter <= 'Z') ||
		(firstCharacter >= 'a' && firstCharacter <= 'z')
}
