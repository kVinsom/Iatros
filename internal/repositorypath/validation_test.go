package repositorypath

import "testing"

func TestRepositoryPathValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		repositoryPath   string
		isValidFile      bool
		isValidDirectory bool
	}{
		{name: "root", repositoryPath: ".", isValidDirectory: true},
		{name: "file", repositoryPath: "services/api/main.go", isValidFile: true, isValidDirectory: true},
		{name: "dotfile", repositoryPath: ".github/workflows/ci.yml", isValidFile: true, isValidDirectory: true},
		{name: "Unicode", repositoryPath: "services/платежі/main.go", isValidFile: true, isValidDirectory: true},
		{name: "empty"},
		{name: "parent", repositoryPath: "../outside"},
		{name: "noncanonical", repositoryPath: "services/../api"},
		{name: "current directory segment", repositoryPath: "services/./api"},
		{name: "repeated separator", repositoryPath: "services//api"},
		{name: "trailing separator", repositoryPath: "services/api/"},
		{name: "POSIX absolute", repositoryPath: "/outside"},
		{name: "Windows absolute", repositoryPath: `C:\outside`},
		{name: "Windows drive relative", repositoryPath: "c:outside"},
		{name: "Windows drive with slash", repositoryPath: "D:/outside"},
		{name: "backslash", repositoryPath: `services\api`},
		{name: "surrounding whitespace", repositoryPath: " services/api"},
		{name: "control character", repositoryPath: "services/\x00api"},
		{name: "C1 control character", repositoryPath: "services/\u0085api"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if actual := IsValidFile(testCase.repositoryPath); actual != testCase.isValidFile {
				t.Fatalf("IsValidFile(%q) = %t, want %t", testCase.repositoryPath, actual, testCase.isValidFile)
			}
			if actual := IsValidDirectory(testCase.repositoryPath); actual != testCase.isValidDirectory {
				t.Fatalf(
					"IsValidDirectory(%q) = %t, want %t",
					testCase.repositoryPath,
					actual,
					testCase.isValidDirectory,
				)
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
		{name: "nested descendant", directoryPath: "services/api", candidatePath: "services/api/main.go", isContained: true},
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

func TestHasWindowsDrivePrefix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		repositoryPath string
		hasPrefix      bool
	}{
		{repositoryPath: "C:relative", hasPrefix: true},
		{repositoryPath: "d:/absolute", hasPrefix: true},
		{repositoryPath: `E:\absolute`, hasPrefix: true},
		{repositoryPath: "services/api"},
		{repositoryPath: "1:relative"},
		{repositoryPath: ":relative"},
		{repositoryPath: "Ä:relative"},
	}

	for _, testCase := range testCases {
		if actual := HasWindowsDrivePrefix(testCase.repositoryPath); actual != testCase.hasPrefix {
			t.Errorf(
				"HasWindowsDrivePrefix(%q) = %t, want %t",
				testCase.repositoryPath,
				actual,
				testCase.hasPrefix,
			)
		}
	}
}
