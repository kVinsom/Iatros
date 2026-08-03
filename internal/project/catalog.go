package project

import (
	"path"

	"github.com/kVinsom/Iatros/internal/repositorypath"
)

type markerRule struct {
	id       string
	kind     Kind
	patterns []string
}

func (r markerRule) matches(file string) bool {
	name := path.Base(file)
	for _, pattern := range r.patterns {
		matched, err := path.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

var projectMarkerRules = []markerRule{
	{id: "clojure-deps", kind: KindCode, patterns: []string{"deps.edn"}},
	{id: "cmake-project", kind: KindCode, patterns: []string{"CMakeLists.txt"}},
	{id: "conan-project", kind: KindCode, patterns: []string{"conanfile.py", "conanfile.txt"}},
	{id: "dotnet-project", kind: KindCode, patterns: []string{"*.csproj", "*.fsproj", "*.vbproj"}},
	{id: "elixir-mix", kind: KindCode, patterns: []string{"mix.exs"}},
	{id: "erlang-rebar3", kind: KindCode, patterns: []string{"rebar.config"}},
	{id: "go-module", kind: KindCode, patterns: []string{"go.mod"}},
	{id: "gradle-project", kind: KindCode, patterns: []string{"build.gradle", "build.gradle.kts"}},
	{id: "leiningen-project", kind: KindCode, patterns: []string{"project.clj"}},
	{id: "maven-project", kind: KindCode, patterns: []string{"pom.xml"}},
	{id: "meson-project", kind: KindCode, patterns: []string{"meson.build"}},
	{id: "node-package", kind: KindCode, patterns: []string{"package.json"}},
	{id: "php-composer", kind: KindCode, patterns: []string{"composer.json"}},
	{id: "python-project", kind: KindCode, patterns: []string{"pyproject.toml", "setup.py"}},
	{id: "ruby-bundler", kind: KindCode, patterns: []string{"Gemfile"}},
	{id: "rust-package", kind: KindCode, patterns: []string{"Cargo.toml"}},
	{id: "scala-sbt", kind: KindCode, patterns: []string{"build.sbt"}},
	{id: "vcpkg-project", kind: KindCode, patterns: []string{"vcpkg.json"}},

	{id: "ansible-project", kind: KindInfrastructure, patterns: []string{"ansible.cfg"}},
	{id: "aws-cdk-project", kind: KindInfrastructure, patterns: []string{"cdk.json"}},
	{id: "azure-bicep-project", kind: KindInfrastructure, patterns: []string{"*.bicep"}},
	{id: "cdktf-project", kind: KindInfrastructure, patterns: []string{"cdktf.json"}},
	{id: "chef-project", kind: KindInfrastructure, patterns: []string{"Berksfile", "Policyfile.rb"}},
	{id: "cloudformation-project", kind: KindInfrastructure, patterns: []string{"*.template.yml", "*.template.yaml", "cloudformation*.yml", "cloudformation*.yaml"}},
	{id: "compose-project", kind: KindInfrastructure, patterns: []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"}},
	{id: "helm-chart", kind: KindInfrastructure, patterns: []string{"Chart.yaml"}},
	{id: "kustomize-project", kind: KindInfrastructure, patterns: []string{"kustomization.yml", "kustomization.yaml"}},
	{id: "nomad-project", kind: KindInfrastructure, patterns: []string{"*.nomad", "*.nomad.hcl"}},
	{id: "pulumi-project", kind: KindInfrastructure, patterns: []string{"Pulumi.yaml", "Pulumi.yml"}},
	{id: "salt-project", kind: KindInfrastructure, patterns: []string{"top.sls"}},
	{id: "serverless-project", kind: KindInfrastructure, patterns: []string{"serverless.yml", "serverless.yaml"}},
	{id: "terraform-module", kind: KindInfrastructure, patterns: []string{"*.tf", "*.tf.json"}},
	{id: "terragrunt-project", kind: KindInfrastructure, patterns: []string{"terragrunt.hcl"}},
}

var workspaceMarkerRules = []markerRule{
	{id: "bazel-workspace", patterns: []string{"MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel"}},
	{id: "dotnet-solution", patterns: []string{"*.sln", "*.slnx"}},
	{id: "go-workspace", patterns: []string{"go.work"}},
	{id: "gradle-workspace", patterns: []string{"settings.gradle", "settings.gradle.kts"}},
	{id: "lerna-workspace", patterns: []string{"lerna.json"}},
	{id: "nx-workspace", patterns: []string{"nx.json"}},
	{id: "pnpm-workspace", patterns: []string{"pnpm-workspace.yaml", "pnpm-workspace.yml"}},
	{id: "rush-workspace", patterns: []string{"rush.json"}},
}

func ignoredBoundaryEvidence(file string) bool {
	return repositorypath.IsExcludedFile(file)
}
