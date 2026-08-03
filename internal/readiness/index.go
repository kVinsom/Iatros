package readiness

import (
	"context"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

type repositoryIndex struct {
	hasRootReadme        bool
	hasRootLicense       bool
	hasRootGitignore     bool
	hasTestMarker        bool
	hasProjectTechnology bool
	hasCICD              bool
	codeTechnologies     []string
}

func buildRepositoryIndex(ctx context.Context, snapshot Snapshot) (repositoryIndex, error) {
	index := repositoryIndex{codeTechnologies: make([]string, 0)}
	for _, directory := range snapshot.Directories {
		if err := ctx.Err(); err != nil {
			return repositoryIndex{}, err
		}
		if !validRepositoryPath(directory, true) {
			return repositoryIndex{}, ErrInvalidSnapshot
		}
		if repositorypath.IsExcludedDirectory(directory) {
			continue
		}
		if isTestDirectory(directory) {
			index.hasTestMarker = true
		}
	}

	for _, file := range snapshot.Files {
		if err := ctx.Err(); err != nil {
			return repositoryIndex{}, err
		}
		if !validRepositoryPath(file, false) {
			return repositoryIndex{}, ErrInvalidSnapshot
		}
		if repositorypath.IsExcludedFile(file) {
			continue
		}
		if !strings.ContainsRune(file, '/') {
			indexRootFile(&index, file)
		}
		if isTestFile(file) {
			index.hasTestMarker = true
		}
	}

	codeTechnologies := make(map[string]struct{})
	for _, technology := range snapshot.Technologies {
		if err := ctx.Err(); err != nil {
			return repositoryIndex{}, err
		}
		if !validIdentifier(technology.ID, "-") ||
			!validIdentifier(string(technology.Category), "_") {
			return repositoryIndex{}, ErrInvalidSnapshot
		}

		index.hasProjectTechnology = true
		switch technology.Category {
		case TechnologyCategoryLanguage, TechnologyCategoryRuntime:
			codeTechnologies[technology.ID] = struct{}{}
		case TechnologyCategoryCICD:
			index.hasCICD = true
		}
	}

	index.codeTechnologies = make([]string, 0, len(codeTechnologies))
	for technologyID := range codeTechnologies {
		index.codeTechnologies = append(index.codeTechnologies, technologyID)
	}
	slices.Sort(index.codeTechnologies)
	return index, nil
}

func indexRootFile(index *repositoryIndex, file string) {
	lowerName := strings.ToLower(file)
	if recognizedReadme(lowerName) {
		index.hasRootReadme = true
	}
	if recognizedLicense(lowerName) {
		index.hasRootLicense = true
	}
	if file == ".gitignore" {
		index.hasRootGitignore = true
	}
}

func recognizedReadme(lowerName string) bool {
	switch lowerName {
	case "readme", "readme.adoc", "readme.markdown", "readme.md", "readme.rst", "readme.txt":
		return true
	default:
		return false
	}
}

func recognizedLicense(lowerName string) bool {
	for _, base := range []string{"copying", "licence", "license", "unlicense"} {
		if lowerName == base || strings.HasPrefix(lowerName, base+"-") {
			return true
		}
		for _, extension := range []string{".markdown", ".md", ".rst", ".txt"} {
			if lowerName == base+extension {
				return true
			}
		}
	}
	return false
}

func validRepositoryPath(value string, rootAllowed bool) bool {
	if !validText(value) || strings.Contains(value, "\\") || path.IsAbs(value) ||
		looksLikeWindowsPath(value) {
		return false
	}

	cleaned := path.Clean(value)
	if cleaned != value || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	return rootAllowed || cleaned != "."
}

func validIdentifier(value, separators string) bool {
	if !validText(value) || !lowerAlphaNumeric(value[0]) ||
		!lowerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	previousSeparator := false
	for index := range len(value) {
		character := value[index]
		if lowerAlphaNumeric(character) {
			previousSeparator = false
			continue
		}
		if !strings.ContainsRune(separators, rune(character)) || previousSeparator {
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

func validText(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
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
	return len(value) >= 2 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':'
}
