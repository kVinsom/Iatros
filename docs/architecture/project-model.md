# Project and Workspace Boundary Model

> [!IMPORTANT]
> **Status: implemented and consumed by the topology CLI workflow.** The detector identifies code, infrastructure, mixed-project, and workspace roots from strong filename markers in the bounded discovery inventory. The topology stage associates these boundaries with bounded manifest facts and maps them to the separate `repository_topology` report without executing tools, accessing the network, or changing the repository.

See also:

- [IATROS target architecture](README.md)
- [Technology detection architecture](detection.md)
- [Repository readiness architecture](readiness.md)
- [Manifest analysis architecture](manifest-analysis.md)
- [Repository topology architecture](topology.md)
- [Security architecture](security.md)
- [PS-0001: Local Repository Analysis](../product/0001-local-repository-analysis.md)

## 1. Boundary and purpose

Project-boundary detection is a provider-neutral capability under `internal/project`. It turns a flat, root-relative file inventory into a deterministic internal model that can represent a simple repository or a nested monorepository.

```text
bounded root-relative file inventory
                 |
                 v
       internal/project.Detector
                 |
                 v
  projects + workspaces + partial state
                 |
                 v
 internal topology association (implemented)
                 |
                 v
 validated topology CLI reporting
```

This stage deliberately remains independent from `internal/analysis`, Cobra, JSON rendering, concrete providers, and external SDKs. `internal/topology` consumes its output and the bounded manifest model through a separate association boundary. `internal/analysis` then maps the associated model into the separate topology schema, leaving the readiness-analysis schema unchanged.

## 2. Model

The internal model contains:

- `Workspace`: a root coordinated by a strong workspace marker;
- `Project`: a root established by a strong code or infrastructure marker;
- `Marker`: a stable marker ID, sorted root-relative evidence, and an evidence-truncation flag;
- `Partial`: a copy of the discovery completeness state.

Every project is classified from direct markers:

| Kind | Meaning |
| --- | --- |
| `code` | At least one code, build, or package manifest exists at the root. |
| `infrastructure` | At least one strong infrastructure manifest exists at the root. |
| `mixed` | Both marker classes exist at the same root. |

`Root` is `.` for the selected repository root or a slash-separated relative directory. `WorkspaceRoot` identifies the nearest containing workspace, including a workspace at the same root. It is empty when no containing workspace is known.

A filename marker establishes only a boundary candidate. It does not prove that the project builds, that a workspace declaration includes every nested project, or that infrastructure is deployed. Those conclusions require bounded content parsing or provider evidence.

## 3. Detection behavior

The detector:

- validates every input as a safe root-relative slash-separated file path;
- sorts and deduplicates a cloned input without mutating caller-owned data;
- matches filenames case-sensitively for consistent cross-platform behavior;
- merges all markers found in the same directory into one project or workspace;
- selects the nearest workspace by walking project ancestors rather than comparing every project with every workspace;
- sorts projects and workspaces by root and markers by stable ID;
- retains at most 20 lexicographically earliest evidence paths per marker;
- sets `EvidenceTruncated` when additional matching paths were omitted;
- preserves trusted detected boundaries when discovery is partial while marking the complete model as partial;
- returns non-nil empty collections and discards accumulated output on invalid input or cancellation.

The upstream discovery limit of 2,000 files bounds matching work. The project detector does not add a separate project-count limit because every reported boundary requires at least one already bounded file. A dedicated evidence limit prevents large Terraform or similar directories from expanding the retained model unnecessarily.

## 4. Code project markers

| Marker ID | Recognized filenames |
| --- | --- |
| `clojure-deps` | `deps.edn` |
| `cmake-project` | `CMakeLists.txt` |
| `conan-project` | `conanfile.py`, `conanfile.txt` |
| `dotnet-project` | `*.csproj`, `*.fsproj`, `*.vbproj` |
| `elixir-mix` | `mix.exs` |
| `erlang-rebar3` | `rebar.config` |
| `go-module` | `go.mod` |
| `gradle-project` | `build.gradle`, `build.gradle.kts` |
| `leiningen-project` | `project.clj` |
| `maven-project` | `pom.xml` |
| `meson-project` | `meson.build` |
| `node-package` | `package.json` |
| `php-composer` | `composer.json` |
| `python-project` | `pyproject.toml`, `setup.py` |
| `ruby-bundler` | `Gemfile` |
| `rust-package` | `Cargo.toml` |
| `scala-sbt` | `build.sbt` |
| `vcpkg-project` | `vcpkg.json` |

Weak dependency lists such as `requirements.txt`, generic `Makefile` files, and source filenames do not create boundaries by themselves.

## 5. Infrastructure project markers

| Marker ID | Recognized filenames |
| --- | --- |
| `ansible-project` | `ansible.cfg` |
| `aws-cdk-project` | `cdk.json` |
| `azure-bicep-project` | `*.bicep` |
| `cdktf-project` | `cdktf.json` |
| `chef-project` | `Berksfile`, `Policyfile.rb` |
| `cloudformation-project` | `*.template.yml`, `*.template.yaml`, `cloudformation*.yml`, `cloudformation*.yaml` |
| `compose-project` | Compose and Docker Compose YAML filenames |
| `helm-chart` | `Chart.yaml` |
| `kustomize-project` | `kustomization.yml`, `kustomization.yaml` |
| `nomad-project` | `*.nomad`, `*.nomad.hcl` |
| `pulumi-project` | `Pulumi.yaml`, `Pulumi.yml` |
| `salt-project` | `top.sls` |
| `serverless-project` | `serverless.yml`, `serverless.yaml` |
| `terraform-module` | `*.tf`, `*.tf.json` |
| `terragrunt-project` | `terragrunt.hcl` |

A standalone `Dockerfile`, generic `template.yaml`, or generic Kubernetes YAML does not create an infrastructure project. Filename-only evidence is not specific enough. Content-aware recognition remains deferred.

## 6. Workspace markers

| Marker ID | Recognized filenames |
| --- | --- |
| `bazel-workspace` | `MODULE.bazel`, `WORKSPACE`, `WORKSPACE.bazel` |
| `dotnet-solution` | `*.sln`, `*.slnx` |
| `go-workspace` | `go.work` |
| `gradle-workspace` | `settings.gradle`, `settings.gradle.kts` |
| `lerna-workspace` | `lerna.json` |
| `nx-workspace` | `nx.json` |
| `pnpm-workspace` | `pnpm-workspace.yaml`, `pnpm-workspace.yml` |
| `rush-workspace` | `rush.json` |

Cargo workspaces, npm workspaces, and Maven aggregator modules are read by the manifest stage and resolved conservatively against repository-confined project roots by topology. Yarn-specific configuration and Elixir umbrella membership are not parsed yet.

## 7. Excluded metadata and dependency evidence

Project markers below the following known generated or dependency directories are ignored by this model:

- `.gradle`;
- `.git`, `.hg`, and `.svn`;
- `.pnpm`;
- `.terraform`;
- `.venv` and `venv`;
- `.yarn`;
- `node_modules`.

This is a boundary-model policy, not a discovery exclusion. Discovery remains responsible for its documented traversal behavior. Avoiding these paths here prevents version-control metadata, a vendored `package.json`, downloaded Terraform module, or local environment from being represented as a user-owned project.

## 8. Partial snapshots and safety

A partial discovery snapshot can still contain trustworthy positive boundary evidence, so the detector returns found projects and workspaces with `Model.Partial` set to `true`. Consumers must not interpret the returned list as complete. The topology report maps this state to `status: partial` and always includes a warning diagnostic.

The detector performs no filesystem calls. It receives only the safe inventory, does not interpret `.gitignore`, does not read manifest values, and has no network or command-execution capability.

## 9. Deferred enrichment

The following decisions remain for richer classification:

- additional manifest formats such as `mix.exs` and Yarn-specific workspace configuration;
- service, library, job, application, and deployable-role classification;
- relationships between code projects and infrastructure projects;
- ordered Yarn-style exclusions and other workspace semantics not represented by the current manifest model;
- user overrides for ambiguous roots;
- per-project readiness and CI/test coverage.

Future enrichment must preserve the implemented safe relative paths, explicit limits, cancellation, deterministic ordering, partial-state propagation, and distinction between observed declarations and inferred relationships.

## 10. Scalable manifest-processing requirement

IATROS must support both small standalone repositories and very large company-scale monorepositories. Conservative defaults are a safety profile, not a permanent domain maximum.

The implemented manifest-processing architecture uses:

- validated injectable limits for file count, bytes per file, total bytes, nesting, retained entries, memory, and duration;
- code-level conservative and large-repository profiles, with user-selectable and Enterprise-calibrated profiles deferred;
- a provider-neutral parser contract that accepts bounded input and returns the same normalized manifest facts and diagnostics regardless of implementation;
- lightweight standard-library JSON and XML implementations as the initial default;
- one-document-at-a-time processing, bounded built-in buffering, and a reader contract that permits specialized streaming parsers;
- replaceable parser backends selected by format and capability rather than leaking a concrete library into the domain model;
- central conformance rules plus contract tests for default and optional implementations;
- explicit partial or failed outcomes when the active profile cannot process an input safely.

Third-party, generated, SIMD-assisted, schema-aware, or specialized streaming parsers may be added when file size, throughput, format fidelity, or profiling data demonstrates a real benefit. Such an implementation must receive a license, maintenance, vulnerability, cross-platform, determinism, and resource-use review. It must not bypass input budgets, cancellation, path confinement, normalized output, or diagnostics.

This design permits a small installation to retain minimal dependencies while a large deployment can select higher limits or specialized implementations without forking the Community core or changing public semantics.
