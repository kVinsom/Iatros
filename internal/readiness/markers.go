package readiness

import (
	"path"
	"strings"
)

var testConfigurationFiles = map[string]struct{}{
	".rspec":                {},
	"conftest.py":           {},
	"jest.config.cjs":       {},
	"jest.config.js":        {},
	"jest.config.mjs":       {},
	"jest.config.ts":        {},
	"karma.conf.js":         {},
	"phpunit.xml":           {},
	"phpunit.xml.dist":      {},
	"playwright.config.js":  {},
	"playwright.config.mjs": {},
	"playwright.config.ts":  {},
	"pytest.ini":            {},
	"tox.ini":               {},
	"vitest.config.js":      {},
	"vitest.config.mjs":     {},
	"vitest.config.ts":      {},
}

var testSourceExtensions = map[string]struct{}{
	".c":     {},
	".cc":    {},
	".cjs":   {},
	".clj":   {},
	".cljc":  {},
	".cljs":  {},
	".cpp":   {},
	".cs":    {},
	".cts":   {},
	".cxx":   {},
	".erl":   {},
	".ex":    {},
	".exs":   {},
	".fs":    {},
	".fsx":   {},
	".go":    {},
	".java":  {},
	".js":    {},
	".kt":    {},
	".mjs":   {},
	".mts":   {},
	".php":   {},
	".py":    {},
	".rb":    {},
	".rs":    {},
	".scala": {},
	".ts":    {},
	".vb":    {},
}

func isTestDirectory(directory string) bool {
	if directory == "." {
		return false
	}
	for segment := range strings.SplitSeq(directory, "/") {
		switch strings.ToLower(segment) {
		case "__tests__", "spec", "specs", "test", "tests":
			return true
		}
	}
	return false
}

func isTestFile(file string) bool {
	base := path.Base(file)
	lowerBase := strings.ToLower(base)
	if _, exists := testConfigurationFiles[lowerBase]; exists {
		return true
	}

	extension := strings.ToLower(path.Ext(base))
	if _, exists := testSourceExtensions[extension]; !exists {
		return false
	}
	lowerStem := strings.TrimSuffix(lowerBase, extension)
	if strings.HasPrefix(lowerStem, "test_") ||
		strings.HasSuffix(lowerStem, "_spec") ||
		strings.HasSuffix(lowerStem, "_suite") ||
		strings.HasSuffix(lowerStem, "_test") ||
		strings.HasSuffix(lowerStem, "_tests") ||
		hasTestStemSegment(lowerStem) {
		return true
	}

	stem := strings.TrimSuffix(base, path.Ext(base))
	return strings.HasSuffix(stem, "Spec") || strings.HasSuffix(stem, "Specs") ||
		strings.HasSuffix(stem, "Test") || strings.HasSuffix(stem, "Tests")
}

func hasTestStemSegment(stem string) bool {
	for segment := range strings.SplitSeq(stem, ".") {
		if segment == "spec" || segment == "test" {
			return true
		}
	}
	return false
}
