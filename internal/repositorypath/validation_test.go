package repositorypath

import "testing"

func TestValidRepositoryPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		repositoryPath string
		validFile      bool
		validDirectory bool
	}{
		{name: "root", repositoryPath: ".", validDirectory: true},
		{name: "file", repositoryPath: "services/api/go.mod", validFile: true, validDirectory: true},
		{name: "directory", repositoryPath: "services/api", validFile: true, validDirectory: true},
		{name: "empty", repositoryPath: ""},
		{name: "parent", repositoryPath: "../outside"},
		{name: "embedded parent", repositoryPath: "services/../outside"},
		{name: "absolute POSIX", repositoryPath: "/outside"},
		{name: "absolute Windows", repositoryPath: `C:\outside`},
		{name: "backslash", repositoryPath: `services\api`},
		{name: "surrounding whitespace", repositoryPath: " services/api"},
		{name: "control character", repositoryPath: "services/\x00api"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if actual := IsValidFile(testCase.repositoryPath); actual != testCase.validFile {
				t.Fatalf("IsValidFile(%q) = %t, want %t", testCase.repositoryPath, actual, testCase.validFile)
			}
			if actual := IsValidDirectory(testCase.repositoryPath); actual != testCase.validDirectory {
				t.Fatalf("IsValidDirectory(%q) = %t, want %t", testCase.repositoryPath, actual, testCase.validDirectory)
			}
		})
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		directoryPath string
		candidatePath string
		isContained   bool
	}{
		{name: "root itself", directoryPath: ".", candidatePath: ".", isContained: true},
		{name: "root descendant", directoryPath: ".", candidatePath: "services/api/main.go", isContained: true},
		{name: "directory itself", directoryPath: "services/api", candidatePath: "services/api", isContained: true},
		{name: "nested file", directoryPath: "services/api", candidatePath: "services/api/main.go", isContained: true},
		{name: "sibling", directoryPath: "services/api", candidatePath: "services/worker/main.go"},
		{name: "prefix collision", directoryPath: "services/api", candidatePath: "services/api-v2/main.go"},
		{name: "invalid directory", directoryPath: "../services", candidatePath: "services/api/main.go"},
		{name: "invalid candidate", directoryPath: "services", candidatePath: "../main.go"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if actual := Contains(testCase.directoryPath, testCase.candidatePath); actual != testCase.isContained {
				t.Fatalf(
					"Contains(%q, %q) = %t, want %t",
					testCase.directoryPath,
					testCase.candidatePath,
					actual,
					testCase.isContained,
				)
			}
		})
	}
}
