# Repository Topology Architecture

> [!IMPORTANT]
> **Status: implemented and exposed through a versioned CLI contract.** `internal/topology` associates the project-boundary and manifest models, `internal/analysis.LocalTopologyAnalyzer` runs the complete local read-only pipeline, and `iatros topology` renders the validated model as deterministic text or JSON. The established `iatros analyze` schema remains unchanged.

See also:

- [Project and workspace boundary model](project-model.md)
- [Manifest analysis architecture](manifest-analysis.md)
- [Security architecture](security.md)
- [Testing strategy](testing.md)
- [PS-0001: Local Repository Analysis](../product/0001-local-repository-analysis.md)

## 1. Purpose and boundary

Topology association converts independent positive evidence into one deterministic repository model:

```text
bounded local discovery
      |             |
      v             v
project boundaries  bounded manifest facts
      |             |
      +------v------+
       topology builder
              |
              v
projects + components + workspaces
+ declarations + direct dependency edges
+ partial diagnostics
```

The builder performs no filesystem or network access. It consumes only normalized outputs from `internal/project` and `internal/manifest`. The separate local orchestrator owns confined file access and supplies both results.

Topology is observed or conservatively inferred repository structure, not proof that a project builds, a dependency resolves at install time, a workspace tool accepts the configuration, or a component is deployed.

## 2. Model

The model contains:

- `Project`: a code, infrastructure, or mixed boundary with markers, parsed components, and workspace relationships;
- `Component`: normalized identity, version, module, constraints, explicit workspace-declaration state, and truncation state from one manifest;
- `Workspace`: a detected or manifest-derived coordination root;
- `WorkspaceDeclaration`: one safe member declaration, its resolution state, and matched project roots;
- `Dependency`: one direct declaration and any repository-local target projects;
- `Issue`: a bounded omission, unsafe declaration, or ambiguous relationship;
- `Partial`: an explicit signal that consumers must not treat the model as complete.

Every path is slash-separated and relative to the selected repository. The repository root is `.`. Collections are non-nil, sorted, deduplicated, deterministic, and independent from caller-owned input slices. Mutable nested collections are copied before topology normalization mutates them.

## 3. Project and component association

A parsed code manifest is associated with the project whose root equals the manifest directory. `go.work` is workspace metadata and is not treated as a project component. If a parsed non-workspace manifest has no retained project boundary, the builder keeps the remaining topology, returns `partial`, and records `IATROS_TOPOLOGY_MANIFEST_UNASSIGNED`.

Project components retain no raw manifest content. They contain only normalized identity fields, runtime or toolchain constraints, and manifest truncation flags. Direct dependencies are represented once in the model-level dependency collection rather than duplicated in every component.

## 4. Workspace association

A workspace can originate from:

- a strong filename marker detected by `internal/project`;
- `go.work`;
- a present non-null `package.json` `workspaces` value or Cargo `[workspace]` table, including an explicit declaration with zero members;
- a `pom.xml` with retained modules.

An explicit empty workspace remains a workspace component with `WorkspaceDeclared` set, an empty declaration collection, and no fabricated declared project. This preserves the difference between `declared but empty` and `not declared`.

The model preserves two different relationships:

- `ContainedProjects`: projects whose nearest containing workspace is this root;
- `DeclaredProjects`: projects positively matched by this workspace's manifest declarations.

`Project.PrimaryWorkspaceRoot` is the nearest containing workspace. `Project.WorkspaceRoots` is the sorted union of the primary workspace and every retained explicit declaration that matches the project. `Workspace.ExcludedProjects` records positive exclusion matches, and exclusions are applied after member matches. This supports nested and overlapping monorepositories without forcing one lossy ownership relationship.

## 5. Workspace declaration resolution

Declarations are matched only against already known project roots. Resolution never performs another filesystem walk and never follows a path outside the selected repository.

Supported matching includes:

- exact relative paths;
- parent-relative paths that normalize to a location still inside the repository;
- segment wildcards supported by Go path matching, including `*`, `?`, and character classes;
- `**` as a whole segment matching zero or more path segments.

Cargo `workspace.exclude` entries are represented separately and remove matching projects after positive members from the same manifest are evaluated. An exclusion in one ecosystem therefore cannot erase a membership declared by another manifest at the same root. The first version deliberately does not interpret inline ordered negation, brace expansion, extglob expressions, or backslash-based patterns. A declaration using those semantics is `unsupported`, not partially interpreted. Absolute paths, Windows drive paths, remote references, `file:` references, and normalized parent escapes become the fixed `[outside-root]` marker and are never resolved or exposed verbatim.

Member resolution values are:

| Resolution | Meaning |
| --- | --- |
| `matched` | At least one retained project root matched. |
| `unmatched` | No project matched and both upstream snapshots were complete. |
| `indeterminate` | No project matched, but upstream evidence was partial. |
| `unsupported` | The declaration requires semantics this resolver does not implement. |
| `outside_root` | The declaration could address content outside the repository boundary. |

An unmatched declaration in a partial snapshot is not reported as a definite configuration problem.

## 6. Direct dependency resolution

The builder creates one edge for each retained direct dependency. It never installs packages, reads lock files, contacts registries, selects a version, or builds a transitive graph.

Repository-local resolution uses the dependency ecosystem and normalized component identity:

| Ecosystem | Identity rule |
| --- | --- |
| Go | Exact module path. |
| Node.js | Lowercase package name. |
| Python | PEP-style lowercase normalization of `.`, `_`, and `-`. |
| Rust | Lowercase package identity with `_` and `-` normalized. |
| Composer | Lowercase package name. |
| Maven | Exact `groupId:artifactId`. |
| Custom parser | Exact format ID and declared name. |

Dependency resolutions are:

- `internal`: exactly one retained project matches;
- `unresolved`: no retained project matches;
- `ambiguous`: multiple retained projects publish the same normalized identity.

`unresolved` does not mean definitively external. The target may be external, absent, unsupported, or omitted by a partial snapshot. Ambiguous dependencies retain bounded candidate roots and never select one arbitrarily.

## 7. Partial state and diagnostics

The topology is partial when either upstream result is partial or when association omits or cannot safely resolve data. Diagnostic families cover:

- project, workspace, manifest, component, declaration, member-match, dependency, target, and issue limits;
- unassigned manifests;
- unsupported, outside-root, and unmatched workspace declarations;
- ambiguous local dependency identities;
- inherited discovery and manifest read, parse, access, and resource issues.

Positive projects, components, declarations, and dependency edges found before a recoverable issue remain available. Cancellation, an invalid snapshot, or invalid limits returns an error instead of a successful partial model.

Inherited and generated issues are validated before exposure. Identical issues are retained once, inherited issue selection is deterministic and bounded independently of caller order, and a single `IATROS_TOPOLOGY_ISSUE_LIMIT` sentinel represents additional safe issues that could not be retained. Invalid codes, paths, or messages fail the snapshot instead of entering diagnostics.

## 8. Resource profiles

Limits are validated injectable profiles, not permanent product ceilings:

| Limit | Conservative default | Large-repository profile |
| --- | ---: | ---: |
| Projects | 2,000 | 100,000 |
| Workspaces | 500 | 10,000 |
| Parsed manifests | 100 | 2,000 |
| Components per boundary | 20 | 100 |
| Workspace declarations | 2,000 | 100,000 |
| Matches per declaration | 500 | 10,000 |
| Direct dependencies | 20,000 | 1,000,000 |
| Targets per ambiguous dependency | 20 | 100 |
| Structured issues | 100 | 1,000 |
| Bytes per retained value | 4 KiB | 64 KiB |
| Association duration | 3 seconds | 60 seconds |

The conservative profile aligns with current discovery and manifest defaults. Large-profile values are starting points for explicit opt-in deployments and require representative release measurements before becoming an Enterprise default.

The builder selects the lexicographically first bounded projects, workspaces, manifests, and inherited issues independently of caller order. Selection uses bounded top-N storage rather than cloning an over-limit input. It then indexes component identities once, finds nearest workspaces by ancestor traversal, and resolves declarations only against retained project roots. Context is checked during selection and validation, between association stages, while matching members, and while indexing dependencies. The local benchmark covering 2,000 projects and one globstar declaration exists to detect regressions; it is not a release service-level objective.

## 9. Security properties

Topology association:

- receives no host absolute root and exposes only repository-relative paths;
- does not read files, execute code, load plugins, contact package registries, or resolve remote references;
- preserves reference redaction performed by manifest normalization;
- does not treat repository text as authorization, policy, or executable instructions;
- refuses traversal and ambiguous identities rather than guessing;
- bounds collections, matches, targets, values, duration, and diagnostics;
- checks cancellation at orchestration and topology stage boundaries and periodically inside large bounded loops;
- discards accumulated output on invalid input or cancellation.

The builder and its parser inputs remain trusted in-process code. A future untrusted extension requires process isolation and enforceable memory, CPU, filesystem, and network controls in addition to these contracts.

## 10. Public CLI contract

`internal/analysis.LocalTopologyAnalyzer` composes:

1. bounded local discovery;
2. project and workspace boundary detection;
3. confined allowlisted manifest reading;
4. normalized manifest parsing;
5. topology association.

The topology workflow is deliberately separate from readiness analysis:

```text
iatros topology [path] [--format text|json]
```

`path` defaults to `.` and accepts one existing local directory. Text is the default format. The command uses the conservative discovery, manifest, and topology profiles. User-selectable and release-calibrated Enterprise profiles remain deferred until a configuration contract is approved.

The JSON envelope has schema version `0.1` and fixed `report_type: repository_topology`. It does not alter or supersede the separate `iatros analyze` schema `0.1`.

```json
{
  "schema_version": "0.1",
  "report_type": "repository_topology",
  "status": "completed",
  "target": {"kind": "local_directory", "path": "."},
  "summary": {
    "projects_total": 0,
    "workspaces_total": 0,
    "components_total": 0,
    "dependencies_total": 0,
    "internal_dependencies": 0,
    "unresolved_dependencies": 0,
    "ambiguous_dependencies": 0
  },
  "projects": [],
  "workspaces": [],
  "dependencies": [],
  "diagnostics": []
}
```

Every shown field is always present. Every top-level and nested collection encodes as an array, including empty collections. `components_total` counts unique format-and-manifest-path pairs, so a manifest represented at both a project and workspace boundary is counted once. Dependency resolution counts sum to `dependencies_total`.

### 10.1 Public project fields

Each project exposes:

- `root`, `kind`, `primary_workspace_root`, and `workspace_roots`;
- boundary `markers` with bounded evidence and truncation state;
- `components` with `manifest_path`, `format`, optional normalized name, version, and module values, normalized constraints, `workspace_declared`, and four manifest truncation flags.

An absent optional scalar is the empty string in JSON and `none` in text. This keeps field presence stable without inventing values.

### 10.2 Public workspace fields

Each workspace exposes its root, markers, components, contained, declared, and excluded project roots, and normalized declarations. A declaration includes its manifest path, safe pattern, exclusion flag, member-resolution value, bounded matching project roots, and truncation state. Unsafe original declarations are represented only by the fixed `[outside-root]` marker.

### 10.3 Public dependency fields

Each direct dependency exposes the source project and manifest, ecosystem, declared name and constraint, normalized scope, indirect and optional flags, resolution, bounded candidate target projects, and truncation state. The report does not claim transitive resolution, registry availability, installed versions, or build success.

### 10.4 Status, diagnostics, and exits

- `completed` means every retained stage completed within its configured limits;
- `partial` means the included facts are trustworthy but incomplete, and at least one warning diagnostic explains the limitation;
- `failed` means no topology is exposed and an error diagnostic explains the safe failure category.

Topology issues become bounded warning diagnostics. Root-relative issue paths prefix diagnostic messages; the absolute selected root and raw operating-system or parser errors are never included. If an upstream stage is partial without a more specific retained issue, the report uses `IATROS_TOPOLOGY_PARTIAL` rather than silently appearing complete.

Exit code `0` represents `completed` and `partial`, `1` represents operational or internal failure, `2` represents invalid usage, and `3` represents an invalid target. A partial report is successful because its incompleteness is explicit and machine-readable.

### 10.5 Rendering and validation

Text and JSON render the same validated report rather than running separate analysis paths. Output contains no timestamp, uses deterministic ordering, and exposes only slash-separated repository-relative paths with `.` as the selected root. Before rendering, the adapter verifies envelope identity, counts, statuses, diagnostic structure, ordering, all path fields, workspace and dependency resolution invariants, and the absence of contradictory failed-result data. Unsafe or malformed internal output is replaced with a canonical failed report.
