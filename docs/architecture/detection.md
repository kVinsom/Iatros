# Technology Detection Architecture

> [!IMPORTANT]
> **Status: implemented internally; analysis-report and CLI integration are deferred.** The current detector consumes only the bounded, root-relative file inventory. It does not read file content, execute detected tools, install dependencies, or use the network.

See also:

- [IATROS target architecture](README.md)
- [Security architecture](security.md)
- [Testing strategy](testing.md)
- [PS-0001: Local Repository Analysis](../product/0001-local-repository-analysis.md)

## 1. Boundary

Technology detection is a provider-neutral capability under `internal/detection`. It does not depend on Cobra, report rendering, external SDKs, or vendor APIs.

```text
bounded file inventory
        |
        v
internal/detection.MarkerDetector
        |
        v
sorted private Technology records
        |
        v
future analysis-report adapter
```

The private result records a stable technology ID, its primary category, sorted root-relative evidence paths, and whether the evidence list was truncated. The existing public report contract is unchanged in this increment.

## 2. Detection behavior

The built-in detector:

- validates every input as a safe root-relative slash-separated path;
- sorts and deduplicates input paths before matching;
- matches filename patterns against every path suffix so nested projects are supported;
- performs case-sensitive matching for consistent behavior across operating systems;
- returns technologies sorted by stable ID;
- retains at most 20 lexicographically earliest evidence paths per technology;
- marks bounded evidence with `EvidenceTruncated` instead of silently implying completeness;
- returns cancellation without exposing partially accumulated results.

The evidence limit bounds report growth in large monorepositories. It does not limit the number of files inspected; that boundary remains owned by filesystem discovery.

## 3. Built-in catalog

The broad baseline spans 15 technology categories.

| Category | Supported technology IDs |
| --- | --- |
| Languages | `c`, `clojure`, `cpp`, `csharp`, `elixir`, `erlang`, `fsharp`, `go`, `java`, `javascript`, `kotlin`, `php`, `python`, `ruby`, `rust`, `scala`, `typescript`, `visual-basic-dotnet` |
| Runtimes | `bun-runtime`, `deno`, `nodejs` |
| Dependency managers | `bun`, `cargo`, `clojure-cli`, `composer`, `conan`, `conda`, `go-modules`, `gradle`, `leiningen`, `maven`, `mix`, `npm`, `nuget`, `pdm`, `pip`, `pipenv`, `pnpm`, `poetry`, `rebar3`, `sbt`, `uv`, `vcpkg`, `yarn` |
| Build systems | `bazel`, `cmake`, `make`, `meson`, `msbuild` |
| Containers | `cloud-native-buildpacks`, `dev-container`, `docker`, `docker-compose`, `podman` |
| Orchestration | `docker-swarm`, `helm`, `kubernetes`, `kustomize`, `nomad`, `skaffold`, `tilt` |
| Infrastructure as code | `aws-cdk`, `aws-cloudformation`, `aws-sam`, `azure-bicep`, `cdktf`, `pulumi`, `serverless-framework`, `terraform-compatible` |
| Configuration management | `ansible`, `chef`, `puppet`, `salt` |
| CI/CD | `appveyor`, `aws-codebuild`, `azure-pipelines`, `bitbucket-pipelines`, `buildkite`, `circleci`, `drone`, `github-actions`, `gitlab-ci`, `google-cloud-build`, `jenkins`, `teamcity`, `travis-ci`, `woodpecker` |
| GitOps | `argo-cd`, `flux` |
| Observability | `elasticsearch`, `fluent-bit`, `grafana`, `jaeger`, `loki`, `opensearch`, `opentelemetry-collector`, `prometheus`, `tempo`, `vector` |
| Networking | `caddy`, `envoy`, `haproxy`, `nginx`, `traefik` |
| Security | `checkov`, `codeql`, `dependabot`, `gitleaks`, `grype`, `hadolint`, `opa-conftest`, `renovate`, `semgrep`, `snyk`, `sonarqube`, `syft`, `trivy` |
| Secrets | `sops`, `vault` |
| Cloud platforms | `aws`, `azure`, `cloudflare`, `firebase`, `google-cloud` |

## 4. Evidence policy

A filename marker proves only that matching repository evidence exists. It does not prove that the tool is installed, configured correctly, secure, used in production, or applicable to every subproject.

Generic names such as `config.yaml`, `deployment.yaml`, `manifest.json`, and `template.yaml` are intentionally not treated as evidence. Raw Kubernetes resources, service-mesh resources, database services, and many cloud resources require bounded allowlisted content parsing before they can be identified honestly.

Terraform and OpenTofu use compatible `.tf` configuration markers, so filename-only detection reports `terraform-compatible` rather than claiming a specific implementation. A future allowlisted parser may refine that result when direct distinguishing evidence exists.

## 5. Extension rules

A catalog addition must include:

1. a stable lowercase ID;
2. one primary category;
3. a documented direct filename marker used by the tool;
4. a false-positive assessment;
5. deterministic tests;
6. no requirement to read arbitrary content, execute a command, or contact a provider.

Content-aware detectors will be separate implementations behind the same capability boundary. They must use explicit file and byte limits, allowlisted formats, and the same root-relative evidence contract.

## References

- [Go module file reference](https://go.dev/doc/modules/gomod-ref)
- [npm package metadata](https://docs.npmjs.com/cli/configuring-npm/package-json/)
- [Python `uv` project files](https://docs.astral.sh/uv/guides/projects/)
- [Maven project object model](https://maven.apache.org/guides/introduction/introduction-to-the-pom.html)
- [Gradle dependency management](https://docs.gradle.org/current/userguide/core_dependency_management.html)
- [NuGet PackageReference and lock files](https://learn.microsoft.com/en-us/nuget/consume-packages/package-references-in-project-files)
- [Cargo manifests and lock files](https://doc.rust-lang.org/cargo/guide/cargo-toml-vs-cargo-lock.html)
- [Kubernetes Kustomize files](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/)
- [Terraform configuration files](https://developer.hashicorp.com/terraform/language/files)
