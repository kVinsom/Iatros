# PS-0004: Dependency Analysis Contract

- **Product status:** Approved
- **Implementation status:** Not started
- **Approved:** 2026-08-10
- **Plan:** Basic foundation shared by every plan
- **Scope:** Local, deterministic, evidence-backed dependency analysis
- **Model schema version:** `1.0`
- **Planned CLI report schema version:** `0.1`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [PS-0001: Local Repository Analysis](0001-local-repository-analysis.md)
- [PS-0002: Stable Core Domain Contracts](0002-core-domain-contracts.md)
- [Manifest analysis architecture](../architecture/manifest-analysis.md)
- [Repository topology architecture](../architecture/topology.md)
- [Scaling profiles](../architecture/scaling-profiles.md)
- [Security architecture](../architecture/security.md)

## 1. Purpose and current status

IATROS must explain which dependencies a repository declares, which versions its local evidence selects,
why each transitive package is required, which build and toolchain versions influence resolution, and
which conflicts or license facts need attention. The result must serve both a small project and a large
company monorepository through the same provider-neutral contract.

This specification approves the product behavior before implementation. The current manifest and topology
pipelines retain direct declarations and repository-local targets, but they do not parse lock files, build a
transitive graph, evaluate dependency conflicts, or inventory dependency licenses. Until source, tests, and
the planned CLI report satisfy the implementation acceptance criteria, those capabilities remain unavailable.

## 2. Product boundary

The dependency-analysis capability will:

- associate dependency evidence with known project and workspace boundaries;
- parse supported local manifests, lock files, vendored metadata, and build configuration;
- retain declared constraints, locally selected versions, checksums, sources, and dependency scopes;
- distinguish direct, transitive, workspace-local, vendored, and unresolved packages;
- construct only graph edges supported by repository evidence;
- identify required transitive paths, unreachable lock entries, and ecosystem-aware conflicts;
- inventory locally evidenced license expressions and license files;
- return deterministic complete or explicitly partial results under the active scaling profile.

The capability will not:

- execute package managers, compilers, build scripts, generators, hooks, or repository code;
- install, update, remove, or repair dependencies or lock files;
- contact registries, source-control hosts, license services, vulnerability services, or other networks;
- inspect an ambient package cache or installed environment as authoritative repository state;
- follow dependency references outside the selected repository;
- treat a locked version as proof that a package is installed or that the project builds;
- provide legal advice or infer a license without local evidence;
- use AI in the Basic implementation.

"Full analysis" means complete processing of every admitted and supported local evidence source within the
selected repository snapshot and configured limits. It does not mean that unavailable registry metadata,
unrecorded dynamic build behavior, or omitted upstream dependency metadata has been discovered. Such gaps
must remain explicit rather than being filled by assumptions.

## 3. Canonical terminology

| Term | Meaning |
| --- | --- |
| **Declared dependency** | A direct requirement recorded by a project manifest or supported build declaration. |
| **Declared constraint** | The exact local version, range, tag, branch, source, platform, or workspace constraint attached to a declaration. |
| **Locked package** | A package identity and version selected by a supported lock or vendored metadata format. |
| **Resolved package** | A locked package whose source format also provides enough identity and graph evidence to place it in the normalized resolution graph. It is not proof of installation. |
| **Direct dependency** | A normalized graph edge originating from a project or workspace root declaration. |
| **Transitive dependency** | A normalized graph edge originating from another resolved package. |
| **Required transitive dependency** | A resolved package reachable from at least one retained direct dependency through edges applicable to the selected scope. |
| **Unreachable package** | A retained lock entry that has no evidenced path from a retained project root. It may be stale, platform-specific, optional, or omitted from an incomplete graph. |
| **Conflict** | A deterministic contradiction under the rules of the relevant ecosystem, supported by local evidence. |
| **Unknown fact** | A fact the admitted local evidence does not establish. Unknown is a valid explicit state, not a conflict or an inferred value. |

The model must keep declared constraints, locked versions, and observed vendored versions separate. It must
not overwrite one source with another or use the word `installed` unless a future capability explicitly and
safely inspects an installation target.

## 4. Normalized dependency facts

The private model will use concrete fields rather than an unbounded property map. It must represent:

- project and workspace roots that own the analysis;
- dependency ecosystem and dependency-manager identity;
- package name, normalized identity, version, source kind, and safe source identity;
- declared constraint and selected lock version as independent optional values;
- dependency scope: runtime, development, build, peer, or ecosystem-specific normalized scope;
- optional dependency state independently from scope;
- direct or transitive relationship and the local evidence that proves it;
- workspace-local target, vendored target, external target, ambiguous target, or unresolved target state;
- checksum or integrity metadata when the local format records it;
- build system, package-manager, language, runtime, and toolchain version constraints;
- license evidence and its known, unknown, or ambiguous state;
- conflict category, severity, affected identities, and evidence;
- independent truncation and partial-state markers;
- bounded diagnostics for unsupported, malformed, unreadable, unsafe, ambiguous, or omitted evidence.

Every retained fact must identify safe repository-relative evidence. Collections must be non-nil,
deterministically ordered, deduplicated, detached from parser-owned memory, and validated before they leave
the capability.

## 5. Evidence and source semantics

Repository evidence is additive rather than silently authoritative. The analyzer will preserve provenance
from these source classes:

1. direct declarations in supported manifests and build configuration;
2. package selections and graph edges in supported lock files;
3. workspace and repository-local identity from the topology model;
4. vendored dependency metadata and safe local license files;
5. wrapper, toolchain, and build configuration containing literal version evidence.

A lock file may select versions without containing a complete graph. For example, a checksum inventory must
not be interpreted as proof that every retained entry is reachable or that edges exist between entries.
Likewise, dynamic build code may expose a literal observation without establishing the complete effective
configuration.

Remote URLs, credentials, signed query strings, private registry locations, local absolute paths, and paths
outside the repository must be redacted or represented by a fixed safe source classification. Raw file
content and raw parser or operating-system errors must not enter the model, report, logs, caches, or future
AI context.

## 6. Format coverage

The architecture must allow independently registered, replaceable parsers. The complete feature increment
targets the following commonly used evidence families:

| Ecosystem | Required local evidence families |
| --- | --- |
| Go | `go.mod`, `go.work`, `go.sum`, and `vendor/modules.txt`. |
| Node.js and TypeScript | `package.json`, npm lock and shrinkwrap files, pnpm lock files, Yarn lock files, and supported text Bun locks. |
| Python | `pyproject.toml`, Poetry, uv, PDM, and Pipenv lock files, plus explicitly supported requirements lock files. |
| Rust | `Cargo.toml`, `Cargo.lock`, and literal Rust toolchain configuration. |
| PHP | `composer.json` and `composer.lock`. |
| Ruby | `Gemfile`, `Gemfile.lock`, and literal Ruby version evidence. |
| JVM | Maven project and wrapper files, Gradle settings, wrappers and dependency locks, and literal SBT tool versions. |
| .NET | project and central package declarations, `packages.lock.json`, `global.json`, and literal SDK constraints. |
| Elixir and Erlang | Mix and Rebar declarations, lock files, and literal toolchain constraints. |
| C and C++ | Conan and vcpkg manifests, configuration, and lock evidence. |
| Infrastructure | Terraform/OpenTofu provider locks and Helm chart dependency locks. |

Binary formats without a safe and deterministic parser must produce an unsupported-format diagnostic rather
than be interpreted heuristically. Additional ecosystems may be registered without changing the normalized
contract when they satisfy the same safety, limits, cancellation, determinism, and conformance requirements.

## 7. Build and toolchain configuration

Build analysis is limited to dependency and resolution semantics. It may retain literal evidence for:

- package-manager and wrapper versions;
- language, runtime, SDK, compiler, and toolchain constraints;
- dependency catalogs, repositories, substitutions, overrides, and workspace declarations;
- build-only dependencies and platform- or target-specific dependency scopes.

Executable or Turing-complete build formats must be treated as untrusted source code. A parser may extract
bounded literal declarations, but it must not execute or partially emulate arbitrary Groovy, Kotlin, Ruby,
Python, JavaScript, shell, Make, MSBuild task, or plugin behavior. A dynamic expression that cannot be
resolved from supported local evidence becomes an explicit unknown or partial diagnostic.

## 8. Transitive graph rules

The normalized graph must:

- identify project roots independently from package nodes;
- preserve every evidenced edge with its scope, optional state, and source;
- retain cycles without recursive traversal or duplicate nodes;
- support ecosystems that legitimately install more than one version of a package;
- derive required transitive packages only through evidenced reachability;
- retain at least one bounded explanatory path from a project root to each required transitive package;
- mark graph completeness independently for each project or workspace;
- distinguish an unreachable package from a definite stale-lock conflict when evidence is insufficient;
- avoid inventing edges for formats that contain versions or checksums but no dependency relationships.

Graph traversal must be iterative or otherwise bounded, cancellation-aware, deterministic, and safe for
large monorepositories.

## 9. Conflict semantics

A conflict requires an ecosystem-specific rule and positive local evidence. Initial conflict categories are:

- a locked selection that does not satisfy a retained direct constraint;
- a retained direct requirement missing from an otherwise complete lock graph;
- a required transitive edge whose target is absent from an otherwise complete lock graph;
- contradictory source identities or integrity records for the same normalized package selection;
- an ambiguous workspace-local package identity;
- incompatible runtime, SDK, compiler, package-manager, or toolchain constraints;
- manifest and lock evidence that are demonstrably out of sync;
- dependency-manager files that are mutually exclusive within the same project boundary.

Multiple versions are not automatically a conflict. Multiple package managers are not automatically a
conflict in a monorepository. Unknown registry metadata, unsupported dynamic build behavior, optional paths,
and incomplete graphs must not be upgraded from uncertainty to a contradiction.

Every conflict must carry stable English wording, severity, safe evidence, and the affected project and
package identities. The model may recommend inspection but must not rewrite a lock file or claim a repair
without a later generation and validation contract.

## 10. License evidence

The capability will inventory licenses from local, attributable evidence such as:

- an SPDX expression or license field in a supported manifest or lock file;
- package metadata stored with a workspace-local or vendored dependency;
- recognized license, copying, or notice files within a safely bounded vendored package root.

A license fact must identify the package, the original normalized expression or recognized identifier, the
evidence kind, and the repository-relative evidence path. Conflicting expressions become `ambiguous`.
Missing evidence becomes `unknown`. The analyzer must not infer a license from a package name, remembered
registry metadata, or similarity to another license text.

License compatibility, allow or deny policy, notice generation, and legal conclusions require an explicit
future policy contract. IATROS can present attributable facts and unresolved risk, but it is not a source of
legal advice.

## 11. Completeness, diagnostics, and outcomes

A successful model is `complete` only when every admitted supported evidence file was read and parsed, every
retained relationship was processed within limits, and no diagnostic makes the relevant graph indeterminate.
Completeness is relative to the selected local snapshot, supported formats, and active resource profile.

The model is `partial` when trustworthy retained facts exist but any relevant evidence is unreadable,
malformed, unsupported, truncated, omitted by a limit, dynamically unresolved, ambiguous, or graph-incomplete.
Each partial result requires at least one bounded diagnostic. Unknown per-package facts may exist in an
otherwise structurally complete graph when the source format does not promise that metadata.

Cancellation, an invalid root, an invalid scaling profile, or an invalid normalized model returns a failed
result without exposing an unvalidated graph. Positive evidence retained before a recoverable omission
remains available only through a validated partial result.

## 12. Resource and extension contract

Dependency analysis will receive validated injectable limits from the unified `small`, `monorepo`, or
Enterprise per-worker profile. Limits must cover at least:

- evidence files, bytes per file, total bytes, and retained text;
- packages, graph edges, roots, graph depth, and explanatory paths;
- conflicts, licenses, integrity records, and diagnostics;
- parser nesting or syntax complexity where applicable;
- total stage duration.

Configured maxima are budgets, not allocation targets or product ceilings. Implementations must allocate
from observed work, avoid recursion proportional to untrusted graph depth, check cancellation throughout
large operations, and select retained prefixes deterministically when limits are reached.

Every built-in or third-party parser backend must preserve the same normalized contract, path confinement,
redaction, limits, cancellation, deterministic behavior, diagnostics, cross-platform behavior, and shared
conformance tests. A third-party dependency additionally requires maintenance, license, vulnerability,
format-fidelity, and measured resource-use review.

## 13. Planned CLI contract

The feature will use a separate command and report rather than silently changing the existing readiness or
topology schemas:

```text
iatros dependencies [path] [--format text|json] [--profile small|monorepo]
```

The first report will use schema `0.1` and `report_type: repository_dependencies`. Text and JSON will map the
same validated model. The report will include project and manager summaries, build and toolchain constraints,
direct and transitive packages, dependency paths, declared and locked versions, integrity metadata, conflicts,
license facts, completeness, and diagnostics.

Empty collections must remain arrays, paths must remain repository-relative and slash-separated, ordering
must be deterministic, and the report must not contain timestamps, raw content, absolute paths, credentials,
or unredacted remote references.

## 14. Contract-stage acceptance criteria

This contract stage is complete when:

- direct, transitive, required, unreachable, locked, resolved, conflict, and unknown terms are unambiguous;
- local and offline safety boundaries are explicit;
- build, version, graph, conflict, license, completeness, and scaling behavior have testable requirements;
- commonly used dependency ecosystems have an explicit coverage target;
- the product specification index identifies the contract as approved but not implemented;
- documentation does not claim that lock parsing or dependency analysis already runs.

## 15. Feature implementation acceptance criteria

The dependency-analysis feature is implemented only when:

- the private schema-versioned model, normalization, validation, and limits satisfy this contract;
- safe local evidence access and replaceable parser registries are implemented;
- every format claimed as supported has malformed-input, cancellation, limit, and deterministic-output tests;
- transitive graph tests cover cycles, optional paths, multiple versions, missing nodes, unreachable entries,
  incomplete formats, and workspace-local targets;
- conflict tests distinguish ecosystem-permitted layouts from positive contradictions;
- license tests cover known, unknown, ambiguous, vendored, and redacted evidence;
- `small` and `monorepo` profiles configure the complete pipeline and Enterprise limits validate monotonically;
- the versioned text and JSON reports are semantically equivalent and validated before rendering;
- `go test ./...`, `go vet ./...`, and `go build ./...` pass;
- product and architecture documentation names exactly which formats are implemented and which remain target
  behavior.

## 16. Deferred capabilities

The following remain separate future capabilities unless a later approved specification changes the boundary:

- explicit registry or source-control metadata resolution through a declared network plugin;
- inspection of a concrete installed environment, container image, artifact, or deployment target;
- vulnerability, end-of-life, update-availability, or package-maintenance intelligence;
- organization license policy, legal compatibility decisions, and generated notice bundles;
- SBOM publication formats, signing, attestation, and external supply-chain exchange;
- automated lock-file generation, dependency upgrades, conflict repair, and build execution.
