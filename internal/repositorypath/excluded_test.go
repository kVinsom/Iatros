package repositorypath

import "testing"

func TestExcludedPathPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		file      string
		directory string
		excluded  bool
	}{
		{name: "root source", file: "main.go"},
		{name: "ordinary nested source", file: "services/api/main.go"},
		{name: "dependency file", file: "node_modules/pkg/main.go", excluded: true},
		{name: "nested dependency file", file: "web/node_modules/pkg/main.go", excluded: true},
		{name: "terraform state file", file: "infra/.terraform/modules/main.tf", excluded: true},
		{name: "dependency directory", directory: "node_modules", excluded: true},
		{name: "nested virtual environment", directory: "services/api/.venv/lib", excluded: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var got bool
			if test.file != "" {
				got = IsExcludedFile(test.file)
			} else {
				got = IsExcludedDirectory(test.directory)
			}
			if got != test.excluded {
				t.Fatalf("excluded = %t, want %t", got, test.excluded)
			}
		})
	}
}
