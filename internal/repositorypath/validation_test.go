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

func TestContainsRepositoryFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		directory string
		candidate string
		contains  bool
	}{
		{name: "root contains file", directory: ".", candidate: "main.go", contains: true},
		{name: "nested directory contains file", directory: "cmd/api", candidate: "cmd/api/main.go", contains: true},
		{name: "similar prefix is outside", directory: "cmd/api", candidate: "cmd/api-v2/main.go"},
		{name: "directory itself is not a file within itself", directory: "cmd/api", candidate: "cmd/api"},
		{name: "parent traversal is rejected", directory: "cmd/api", candidate: "cmd/api/../../../secret"},
		{name: "absolute directory is rejected", directory: "/cmd/api", candidate: "cmd/api/main.go"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if actual := Contains(testCase.directory, testCase.candidate); actual != testCase.contains {
				t.Fatalf("Contains(%q, %q) = %t, want %t", testCase.directory,
					testCase.candidate, actual, testCase.contains)
			}
		})
	}
}
