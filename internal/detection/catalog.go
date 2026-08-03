package detection

import (
	"path"
	"strings"
)

type markerRule struct {
	id       string
	category Category
	patterns []string
}

func (r markerRule) matches(file string) bool {
	for _, pattern := range r.patterns {
		if matchesPathSuffix(pattern, file) {
			return true
		}
	}
	return false
}

func matchesPathSuffix(pattern, file string) bool {
	if !strings.ContainsAny(pattern, "*?[\\") {
		return hasPathSegmentSuffix(file, pattern)
	}
	if !strings.ContainsRune(pattern, '/') {
		matched, err := path.Match(pattern, path.Base(file))
		return err == nil && matched
	}

	for candidate := file; ; {
		matched, err := path.Match(pattern, candidate)
		if err == nil && matched {
			return true
		}
		separator := strings.IndexByte(candidate, '/')
		if separator < 0 {
			return false
		}
		candidate = candidate[separator+1:]
	}
}

func hasPathSegmentSuffix(file, suffix string) bool {
	if len(file) < len(suffix) {
		return false
	}
	offset := len(file) - len(suffix)
	return file[offset:] == suffix && (offset == 0 || file[offset-1] == '/')
}

var defaultMarkerRules = []markerRule{
	// Programming languages and runtimes.
	{id: "c", category: CategoryLanguage, patterns: []string{"*.c"}},
	{id: "clojure", category: CategoryLanguage, patterns: []string{"*.clj", "*.cljc", "*.cljs", "deps.edn", "project.clj"}},
	{id: "cpp", category: CategoryLanguage, patterns: []string{"*.cc", "*.cpp", "*.cxx", "*.hh", "*.hpp", "*.hxx"}},
	{id: "csharp", category: CategoryLanguage, patterns: []string{"*.cs", "*.csproj"}},
	{id: "elixir", category: CategoryLanguage, patterns: []string{"*.ex", "*.exs"}},
	{id: "erlang", category: CategoryLanguage, patterns: []string{"*.erl", "*.hrl", "rebar.config"}},
	{id: "fsharp", category: CategoryLanguage, patterns: []string{"*.fs", "*.fsproj", "*.fsx"}},
	{id: "go", category: CategoryLanguage, patterns: []string{"*.go", "go.mod", "go.work"}},
	{id: "java", category: CategoryLanguage, patterns: []string{"*.java"}},
	{id: "javascript", category: CategoryLanguage, patterns: []string{"*.cjs", "*.js", "*.mjs"}},
	{id: "kotlin", category: CategoryLanguage, patterns: []string{"*.kt"}},
	{id: "php", category: CategoryLanguage, patterns: []string{"*.php", "composer.json"}},
	{id: "python", category: CategoryLanguage, patterns: []string{"*.py", "Pipfile", "pyproject.toml", "requirements*.txt", "setup.cfg", "setup.py"}},
	{id: "ruby", category: CategoryLanguage, patterns: []string{"*.gemspec", "*.rb", "Gemfile"}},
	{id: "rust", category: CategoryLanguage, patterns: []string{"*.rs", "Cargo.toml"}},
	{id: "scala", category: CategoryLanguage, patterns: []string{"*.scala", "build.sbt"}},
	{id: "typescript", category: CategoryLanguage, patterns: []string{"*.cts", "*.mts", "*.ts"}},
	{id: "visual-basic-dotnet", category: CategoryLanguage, patterns: []string{"*.vb", "*.vbproj"}},
	{id: "bun-runtime", category: CategoryRuntime, patterns: []string{"bun.lock", "bun.lockb", "bunfig.toml"}},
	{id: "deno", category: CategoryRuntime, patterns: []string{"deno.json", "deno.jsonc", "deno.lock"}},
	{id: "nodejs", category: CategoryRuntime, patterns: []string{"package.json"}},

	// Dependency managers.
	{id: "bun", category: CategoryDependencyManager, patterns: []string{"bun.lock", "bun.lockb"}},
	{id: "cargo", category: CategoryDependencyManager, patterns: []string{"Cargo.lock", "Cargo.toml"}},
	{id: "clojure-cli", category: CategoryDependencyManager, patterns: []string{"deps.edn"}},
	{id: "composer", category: CategoryDependencyManager, patterns: []string{"composer.json", "composer.lock"}},
	{id: "conan", category: CategoryDependencyManager, patterns: []string{"conan.lock", "conanfile.py", "conanfile.txt"}},
	{id: "conda", category: CategoryDependencyManager, patterns: []string{".condarc", "conda-lock.yml", "conda-lock.yaml", "environment.yml", "environment.yaml"}},
	{id: "go-modules", category: CategoryDependencyManager, patterns: []string{"go.mod", "go.sum", "go.work", "go.work.sum"}},
	{id: "gradle", category: CategoryDependencyManager, patterns: []string{"build.gradle", "build.gradle.kts", "gradle.properties", "gradle/wrapper/gradle-wrapper.properties", "settings.gradle", "settings.gradle.kts"}},
	{id: "leiningen", category: CategoryDependencyManager, patterns: []string{"project.clj"}},
	{id: "maven", category: CategoryDependencyManager, patterns: []string{".mvn/maven.config", "pom.xml"}},
	{id: "mix", category: CategoryDependencyManager, patterns: []string{"mix.exs", "mix.lock"}},
	{id: "npm", category: CategoryDependencyManager, patterns: []string{"npm-shrinkwrap.json", "package-lock.json"}},
	{id: "nuget", category: CategoryDependencyManager, patterns: []string{"Directory.Packages.props", "NuGet.Config", "packages.config", "packages.lock.json"}},
	{id: "pdm", category: CategoryDependencyManager, patterns: []string{"pdm.lock", "pdm.toml"}},
	{id: "pip", category: CategoryDependencyManager, patterns: []string{"constraints*.txt", "requirements*.txt"}},
	{id: "pipenv", category: CategoryDependencyManager, patterns: []string{"Pipfile", "Pipfile.lock"}},
	{id: "pnpm", category: CategoryDependencyManager, patterns: []string{"pnpm-lock.yaml", "pnpm-workspace.yaml"}},
	{id: "poetry", category: CategoryDependencyManager, patterns: []string{"poetry.lock"}},
	{id: "rebar3", category: CategoryDependencyManager, patterns: []string{"rebar.config", "rebar.lock"}},
	{id: "sbt", category: CategoryDependencyManager, patterns: []string{"build.sbt", "project/build.properties"}},
	{id: "uv", category: CategoryDependencyManager, patterns: []string{"uv.lock", "uv.toml"}},
	{id: "vcpkg", category: CategoryDependencyManager, patterns: []string{"vcpkg-configuration.json", "vcpkg.json"}},
	{id: "yarn", category: CategoryDependencyManager, patterns: []string{"yarn.lock"}},

	// Build and container tooling.
	{id: "bazel", category: CategoryBuildSystem, patterns: []string{"BUILD", "BUILD.bazel", "MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel"}},
	{id: "cmake", category: CategoryBuildSystem, patterns: []string{"*.cmake", "CMakeLists.txt"}},
	{id: "make", category: CategoryBuildSystem, patterns: []string{"GNUmakefile", "Makefile"}},
	{id: "meson", category: CategoryBuildSystem, patterns: []string{"meson.build", "meson_options.txt"}},
	{id: "msbuild", category: CategoryBuildSystem, patterns: []string{"*.csproj", "*.fsproj", "*.sln", "*.slnx", "*.vbproj", "Directory.Build.props", "Directory.Build.targets"}},
	{id: "cloud-native-buildpacks", category: CategoryContainer, patterns: []string{"project.toml"}},
	{id: "dev-container", category: CategoryContainer, patterns: []string{".devcontainer.json", ".devcontainer/devcontainer.json"}},
	{id: "docker", category: CategoryContainer, patterns: []string{"*.Dockerfile", ".dockerignore", "Dockerfile", "Dockerfile.*", "docker-bake.hcl", "docker-bake.json"}},
	{id: "docker-compose", category: CategoryContainer, patterns: []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"}},
	{id: "podman", category: CategoryContainer, patterns: []string{"Containerfile", "Containerfile.*", "podman-compose.yml", "podman-compose.yaml"}},

	// Orchestration and infrastructure as code.
	{id: "docker-swarm", category: CategoryOrchestration, patterns: []string{"docker-stack.yml", "docker-stack.yaml"}},
	{id: "helm", category: CategoryOrchestration, patterns: []string{"Chart.lock", "Chart.yaml"}},
	{id: "kubernetes", category: CategoryOrchestration, patterns: []string{"Chart.yaml", "kustomization.yml", "kustomization.yaml", "skaffold.yml", "skaffold.yaml"}},
	{id: "kustomize", category: CategoryOrchestration, patterns: []string{"kustomization.yml", "kustomization.yaml"}},
	{id: "nomad", category: CategoryOrchestration, patterns: []string{"*.nomad", "*.nomad.hcl"}},
	{id: "skaffold", category: CategoryOrchestration, patterns: []string{"skaffold.yml", "skaffold.yaml"}},
	{id: "tilt", category: CategoryOrchestration, patterns: []string{"Tiltfile"}},
	{id: "aws-cdk", category: CategoryInfrastructureAsCode, patterns: []string{"cdk.context.json", "cdk.json"}},
	{id: "aws-cloudformation", category: CategoryInfrastructureAsCode, patterns: []string{".cfnlintrc", "*.template.yml", "*.template.yaml", "cloudformation*.yml", "cloudformation*.yaml"}},
	{id: "aws-sam", category: CategoryInfrastructureAsCode, patterns: []string{"samconfig.toml"}},
	{id: "azure-bicep", category: CategoryInfrastructureAsCode, patterns: []string{"*.bicep"}},
	{id: "cdktf", category: CategoryInfrastructureAsCode, patterns: []string{"cdktf.json"}},
	{id: "pulumi", category: CategoryInfrastructureAsCode, patterns: []string{"Pulumi.*.yaml", "Pulumi.*.yml", "Pulumi.yaml", "Pulumi.yml"}},
	{id: "serverless-framework", category: CategoryInfrastructureAsCode, patterns: []string{"serverless.yml", "serverless.yaml"}},
	{id: "terraform-compatible", category: CategoryInfrastructureAsCode, patterns: []string{"*.tf", "*.tf.json", "*.tfvars", ".terraform.lock.hcl"}},

	// Host configuration and delivery automation.
	{id: "ansible", category: CategoryConfigurationManagement, patterns: []string{".ansible-lint", ".ansible-lint.yml", ".ansible-lint.yaml", "ansible.cfg"}},
	{id: "chef", category: CategoryConfigurationManagement, patterns: []string{"Berksfile", "Cheffile", "Policyfile.rb"}},
	{id: "puppet", category: CategoryConfigurationManagement, patterns: []string{"*.pp", "Puppetfile"}},
	{id: "salt", category: CategoryConfigurationManagement, patterns: []string{"*.sls", "top.sls"}},
	{id: "appveyor", category: CategoryCICD, patterns: []string{"appveyor.yml", "appveyor.yaml"}},
	{id: "aws-codebuild", category: CategoryCICD, patterns: []string{"buildspec.yml", "buildspec.yaml"}},
	{id: "azure-pipelines", category: CategoryCICD, patterns: []string{"azure-pipelines.yml", "azure-pipelines.yaml"}},
	{id: "bitbucket-pipelines", category: CategoryCICD, patterns: []string{"bitbucket-pipelines.yml", "bitbucket-pipelines.yaml"}},
	{id: "buildkite", category: CategoryCICD, patterns: []string{".buildkite/pipeline.yml", ".buildkite/pipeline.yaml"}},
	{id: "circleci", category: CategoryCICD, patterns: []string{".circleci/config.yml", ".circleci/config.yaml"}},
	{id: "drone", category: CategoryCICD, patterns: []string{".drone.yml", ".drone.yaml"}},
	{id: "github-actions", category: CategoryCICD, patterns: []string{".github/workflows/*.yml", ".github/workflows/*.yaml"}},
	{id: "gitlab-ci", category: CategoryCICD, patterns: []string{".gitlab-ci.yml", ".gitlab-ci.yaml"}},
	{id: "google-cloud-build", category: CategoryCICD, patterns: []string{"cloudbuild.yml", "cloudbuild.yaml"}},
	{id: "jenkins", category: CategoryCICD, patterns: []string{"*.Jenkinsfile", "Jenkinsfile"}},
	{id: "teamcity", category: CategoryCICD, patterns: []string{".teamcity/settings.kts"}},
	{id: "travis-ci", category: CategoryCICD, patterns: []string{".travis.yml", ".travis.yaml"}},
	{id: "woodpecker", category: CategoryCICD, patterns: []string{".woodpecker.yml", ".woodpecker.yaml", ".woodpecker/*.yml", ".woodpecker/*.yaml"}},
	{id: "argo-cd", category: CategoryGitOps, patterns: []string{".argocd-source*.yaml", ".argocd-source*.yml"}},
	{id: "flux", category: CategoryGitOps, patterns: []string{"gotk-components.yaml", "gotk-sync.yaml"}},

	// Observability and networking.
	{id: "elasticsearch", category: CategoryObservability, patterns: []string{"elasticsearch.yml", "elasticsearch.yaml"}},
	{id: "fluent-bit", category: CategoryObservability, patterns: []string{"fluent-bit.conf"}},
	{id: "grafana", category: CategoryObservability, patterns: []string{"grafana.ini"}},
	{id: "jaeger", category: CategoryObservability, patterns: []string{"jaeger.yml", "jaeger.yaml"}},
	{id: "loki", category: CategoryObservability, patterns: []string{"loki-config.yml", "loki-config.yaml", "loki.yml", "loki.yaml"}},
	{id: "opensearch", category: CategoryObservability, patterns: []string{"opensearch.yml", "opensearch.yaml"}},
	{id: "opentelemetry-collector", category: CategoryObservability, patterns: []string{"otel-collector*.yml", "otel-collector*.yaml", "otelcol*.yml", "otelcol*.yaml"}},
	{id: "prometheus", category: CategoryObservability, patterns: []string{"prometheus.yml", "prometheus.yaml"}},
	{id: "tempo", category: CategoryObservability, patterns: []string{"tempo.yml", "tempo.yaml"}},
	{id: "vector", category: CategoryObservability, patterns: []string{"vector.toml", "vector.yml", "vector.yaml"}},
	{id: "caddy", category: CategoryNetworking, patterns: []string{"Caddyfile"}},
	{id: "envoy", category: CategoryNetworking, patterns: []string{"envoy.yml", "envoy.yaml"}},
	{id: "haproxy", category: CategoryNetworking, patterns: []string{"haproxy.cfg"}},
	{id: "nginx", category: CategoryNetworking, patterns: []string{"nginx.conf"}},
	{id: "traefik", category: CategoryNetworking, patterns: []string{"traefik.toml", "traefik.yml", "traefik.yaml"}},

	// Security, secrets, and cloud platform tooling.
	{id: "checkov", category: CategorySecurity, patterns: []string{".checkov.yml", ".checkov.yaml"}},
	{id: "codeql", category: CategorySecurity, patterns: []string{".github/codeql/*.yml", ".github/codeql/*.yaml", "codeql-config.yml", "codeql-config.yaml"}},
	{id: "dependabot", category: CategorySecurity, patterns: []string{".github/dependabot.yml", ".github/dependabot.yaml"}},
	{id: "gitleaks", category: CategorySecurity, patterns: []string{".gitleaks.toml"}},
	{id: "grype", category: CategorySecurity, patterns: []string{".grype.yml", ".grype.yaml"}},
	{id: "hadolint", category: CategorySecurity, patterns: []string{".hadolint.yaml", ".hadolint.yml"}},
	{id: "opa-conftest", category: CategorySecurity, patterns: []string{".conftest.toml", "*.rego", "conftest.toml"}},
	{id: "renovate", category: CategorySecurity, patterns: []string{".renovaterc", ".renovaterc.json", "renovate.json", "renovate.json5"}},
	{id: "semgrep", category: CategorySecurity, patterns: []string{".semgrep.yml", ".semgrep.yaml", ".semgrepignore"}},
	{id: "snyk", category: CategorySecurity, patterns: []string{".snyk"}},
	{id: "sonarqube", category: CategorySecurity, patterns: []string{"sonar-project.properties"}},
	{id: "syft", category: CategorySecurity, patterns: []string{".syft.yml", ".syft.yaml"}},
	{id: "trivy", category: CategorySecurity, patterns: []string{".trivyignore", "trivy.yml", "trivy.yaml"}},
	{id: "sops", category: CategorySecrets, patterns: []string{".sops.yml", ".sops.yaml"}},
	{id: "vault", category: CategorySecrets, patterns: []string{"vault.hcl", "vault.json"}},
	{id: "aws", category: CategoryCloudPlatform, patterns: []string{".cfnlintrc", "cdk.json", "samconfig.toml"}},
	{id: "azure", category: CategoryCloudPlatform, patterns: []string{"*.bicep", "azure.yaml", "azure.yml"}},
	{id: "cloudflare", category: CategoryCloudPlatform, patterns: []string{"wrangler.json", "wrangler.jsonc", "wrangler.toml"}},
	{id: "firebase", category: CategoryCloudPlatform, patterns: []string{".firebaserc", "firebase.json"}},
	{id: "google-cloud", category: CategoryCloudPlatform, patterns: []string{"app.yaml", "cloudbuild.yml", "cloudbuild.yaml"}},
}
