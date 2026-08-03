# Manifest Analysis Architecture

> [!IMPORTANT]
> **Status: implemented and consumed by the topology CLI workflow.** The current `internal/manifest` capability safely reads an explicit set of repository manifests, normalizes direct dependencies and workspace declarations, and returns bounded diagnostics. `iatros topology` exposes mapped facts through the separate `repository_topology` schema while `iatros analyze` remains metadata-only.

See also:

- [Project and workspace boundary model](project-model.md)
- [Repository topology architecture](topology.md)
- [Security architecture](security.md)
- [Testing strategy](testing.md)
- [PS-0001: Local Repository Analysis](../product/0001-local-repository-analysis.md)

## 1. Purpose and boundary

Manifest analysis is the content-aware stage between filename-based project boundaries and the public topology report mapping:

```text
bounded root-relative inventory
              |
              v
exact manifest filename registry
              |
              v
confined local source -> per-document byte guard -> parser backend
                                                   |
                                                   v
                                      central normalization and limits
                                                   |
                                                   v
                         manifests + direct dependencies + constraints
                         + workspace members/exclusions + partial diagnostics
```

The package does not execute repositories, package managers, build tools, hooks, plugins, or generated code. It does not resolve parents, registries, remote URLs, dependency versions, workspace globs, or transitive graphs. It performs no network access and does not modify the target.

The capability is provider-neutral and independent from Cobra, report rendering, concrete DevOps providers, and public SDK contracts.

## 2. Supported built-in formats

| Exact filename | Normalized format | Implemented facts |
| --- | --- | --- |
| `go.mod` | `go_module` | Module path, Go/toolchain constraints, direct requirements, indirect flag. |
| `go.work` | `go_workspace` | Go/toolchain constraints and `use` workspace members. |
| `package.json` | `node_package` | Name, version, runtime/development/optional/peer dependencies, engines, workspace packages. |
| `pyproject.toml` | `python_project` | PEP 621 and common Poetry name/version, Python constraint, direct/optional/build/group dependencies. |
| `Cargo.toml` | `rust_package` | Package identity, Rust constraint, runtime/development/build and target-specific dependencies, workspace members and exclusions. |
| `composer.json` | `php_composer` | Package identity, runtime/development dependencies, PHP and Composer platform constraints. |
| `pom.xml` | `maven_project` | Maven coordinates, direct dependencies and scopes, optional flag, modules, compiler release/source/target. |

The first increment deliberately does not parse lock files. It also does not resolve Go replacements, Maven parents, properties, dependency management or plugins, Cargo workspace inheritance, Python environment markers, package registries, or transitive dependencies. These require a separate resolution policy because they can change the meaning of direct declarations or require additional files and external state.

## 3. Normalized model

Each retained manifest contains:

- a root-relative evidence path and stable format identifier;
- optional name, version, and module identity;
- sorted direct dependencies with normalized scope, declared constraint, indirect flag, and optional flag;
- sorted runtime, platform, or toolchain constraints with a scope;
- sorted workspace member and exclusion declarations;
- `WorkspaceDeclared`, which distinguishes an explicit workspace with zero retained members from no workspace declaration;
- independent truncation flags for dependencies, constraints, workspace members, and workspace exclusions.

Dependency scopes are `runtime`, `development`, `build`, and `peer`. Parsers may produce values in any order, but the analyzer validates, sorts, deduplicates, and truncates them centrally. It overwrites parser-supplied paths and formats with the trusted registry values.

`WorkspaceDeclared` is true for `go.work`, a present non-null Node.js `workspaces` value, or a present Cargo `[workspace]` table, including an empty array or table. Empty declarations derive workspace metadata but do not fabricate a member declaration.

Remote and local dependency references are normalized to `[redacted-reference]`. The check covers URI schemes, SCP-style remotes, `link:` and `portal:` values, UNC and drive paths, and direct current-, parent-, or home-relative references, including PEP direct references. Non-location version protocols such as a safe `workspace:`, npm alias, or catalog constraint remain intact. This preserves the dependency relationship without retaining a credential-bearing URL, signed query string, or local path for later reports, logs, caches, or model context.

## 4. Safe local file access

`internal/analysis.LocalManifestSource` opens one validated local directory through `os.Root`. For each requested repository-relative path it:

1. rejects unsafe, absolute, Windows drive, and parent-traversal paths;
2. performs `Lstat` relative to the confined root;
3. rejects symbolic links and non-regular files;
4. opens the file through the same confined root;
5. verifies the opened file is still the same regular file;
6. returns only a reader and byte size, never the host absolute path.

The analyzer selects only exact registered basenames from the already bounded inventory. It ignores manifests below `.git`, `.hg`, `.svn`, dependency caches, generated package directories, virtual environments, and local Terraform state directories.

Raw parser errors and operating-system errors are not copied into diagnostics. Raw file content is processed one document at a time and is not retained in `Result`.

## 5. Parser safety

The built-in parsers use these implementations:

- `golang.org/x/mod/modfile` for the official Go module and workspace grammars;
- Go `encoding/json` for JSON manifests;
- Go `encoding/xml` for Maven XML;
- `github.com/pelletier/go-toml/v2` for TOML 1.x manifests.

The JSON guard requires UTF-8, rejects exact and Unicode case-fold-equivalent duplicate object keys, rejects trailing values, and enforces a nesting limit before typed decoding. Case-fold rejection prevents two spellings from resolving ambiguously through Go's case-insensitive struct-field matching. The XML guard requires UTF-8 and one root element, uses strict tokenization, rejects directives including DTD and entity declarations, and enforces a nesting limit. TOML input is decoded according to the TOML grammar and its decoded structure is checked against the same configured nesting budget.

Built-in parsers currently buffer one already byte-bounded document because the Go module grammar and typed validation benefit from a complete input. The analyzer never requires all repository manifests in memory together. It checks cancellation before each document, through reads, and after parser execution. A specialized parser may stream through the supplied reader when profiling demonstrates that a larger supported file requires it.

## 6. Resource profiles

Limits are validated, injectable configuration rather than domain constants. Two code-level profiles provide safe starting points:

| Limit | Conservative default | Large-repository profile |
| --- | ---: | ---: |
| Manifest files | 100 | 2,000 |
| Bytes per file | 256 KiB | 8 MiB |
| Total manifest bytes | 4 MiB | 256 MiB |
| Direct dependencies per manifest | 1,000 | 10,000 |
| Constraints per manifest | 100 | 1,000 |
| Workspace members per manifest | 500 | 5,000 |
| Workspace exclusions per manifest | 500 | 5,000 |
| Structured issues | 50 | 200 |
| Structured nesting depth | 64 | 128 |
| Bytes in one retained value | 4 KiB | 64 KiB |
| Analysis duration | 3 seconds | 30 seconds |

The CLI does not expose profile selection yet. Large deployments may compose a different validated profile; no repository-size ceiling appears in the normalized model or parser interface. Limit validation rejects impossible byte profiles that would overflow the one-byte size sentinel, and total-byte accounting saturates at its configured maximum. Production or Enterprise defaults require measurements from representative repositories before release.

## 7. Replaceable parser backends

The consumer-owned `Parser` interface declares a stable format, exact filenames, and a parse operation over a bounded `Document`. A composition root can replace a built-in backend or register an additional exact manifest filename without changing the normalized model. The analyzer clones every mutable collection returned by a backend before normalization, so sorting, redaction, deduplication, and truncation cannot mutate parser-owned storage; truncated retained prefixes are detached from omitted backing storage.

Every backend, including a third-party, generated, SIMD-assisted, schema-aware, or streaming implementation, must:

- consume the complete document and honor context cancellation;
- perform no command execution, network access, external entity resolution, or unrestricted file access;
- return only normalized facts, never raw content or parser errors intended for users;
- accept central path, byte, value, nesting, collection, and issue limits;
- tolerate central sorting, deduplication, validation, redaction, and truncation;
- produce deterministic results on Windows, Linux, and macOS;
- pass the shared contract, malformed-input, cancellation, and resource-limit tests.

Registration accepts lowercase ASCII alphanumeric format segments separated by single `-`, `_`, or `.` characters and rejects malformed identifiers, unsafe filenames, duplicate formats, and filename collisions. The central analyzer rejects a backend that leaves unread content or returns invalid values. A new dependency still requires license, maintenance, vulnerability, cross-platform, determinism, and measured resource-use review.

## 8. Partial results and diagnostics

Positive facts parsed before a recoverable omission remain trustworthy. `Result.Partial` becomes `true` when the input inventory is partial or when any manifest issue occurs. Bounded diagnostic codes distinguish:

- file-count, per-file-byte, total-byte, dependency, constraint, workspace-member, workspace-exclusion, and issue limits;
- safe-open or complete-read failure;
- malformed or unsupported content;
- a recognized format without a configured backend.

A cancellation or invalid snapshot returns an error instead of a successful partial result. Diagnostics contain only stable English messages and root-relative evidence paths.

## 9. Verification

Tests cover all built-in formats, explicit empty workspaces, deterministic normalization, optional and scoped dependencies, workspace members and exclusions, conservative and large profiles, custom backends, backend ownership isolation, backend collisions, incomplete and malformed backend results, cancellation, traversal rejection, symlink rejection, exact and case-fold-equivalent duplicate JSON keys, trailing JSON, XML directives, multiple XML roots, nesting limits, collection limits, normal and maximum byte-limit arithmetic, diagnostic limits, and local and remote reference redaction.

The 100-document local benchmark exists to detect large allocation or throughput regressions. It is evidence for engineering comparisons, not a release service-level objective.

## 10. Topology and public-report integration

`internal/topology` associates normalized manifests with project boundaries, resolves only repository-confined workspace members, identifies direct local dependency targets, and propagates manifest diagnostics. `internal/analysis.LocalTopologyAnalyzer` composes the complete local pipeline. The `iatros topology` adapter maps this model into validated schema `0.1` text or JSON. `iatros analyze` has its own unchanged schema `0.1` and continues to perform metadata-only discovery.
