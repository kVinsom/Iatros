# PS-0001: Local Repository Analysis

- **Product status:** Approved
- **Implementation status:** Complete local repository analysis baseline implemented
- **Approved:** 2026-08-01
- **Primary users:** DevOps engineers and software developers
- **Interface language:** English
- **Execution scope:** Local filesystem only

Related documents:

- [IATROS Product Contract](product-contract.md)
- [IATROS target architecture](../architecture/README.md)
- [Security architecture](../architecture/security.md)
- [Testing strategy](../architecture/testing.md)
- [Manifest analysis architecture](../architecture/manifest-analysis.md)
- [Repository discovery architecture](../architecture/repository-discovery.md)
- [ADR-0004: Control state-changing operations](../architecture/decisions/0004-control-state-changing-operations.md)
- [ADR-0005: Use Cobra as the CLI adapter](../architecture/decisions/0005-use-cobra-as-the-cli-adapter.md)
- [ADR-0007: Use bounded repository-owned ignore rules](../architecture/decisions/0007-use-bounded-repository-owned-ignore-rules.md)

## Implementation progress

| Increment | Status |
| --- | --- |
| Go module and Cobra command surface | Implemented |
| Versioned text and JSON contract stub | Implemented |
| Local target validation and documented exit codes | Implemented |
| Safe filesystem discovery | Implemented and integrated |
| Ecosystem marker detection | Implemented and integrated |
| Repository-readiness findings | Five rules implemented and integrated |
| Discovery diagnostics and partial-report policy | Implemented and integrated |
| Deterministic text and JSON output | Implemented and integration-tested |
| Project and workspace boundary model | Implemented and exposed through topology reports |
| Bounded manifest analysis | Seven formats, normalized direct declarations, unified scaling profiles, and replaceable backends implemented |
| Repository topology | Local project/component association, workspace membership, direct dependency edges, and versioned CLI text/JSON implemented |
| Scaling profiles | `small` and `monorepo` selectable in both commands; Enterprise per-worker profile implemented for gated composition |
| Ignore and nested-repository handling | Root and nested Git-style rules, submodules, and nested Git worktrees implemented in shared discovery |

## 1. Summary

The first IATROS vertical slice contains two local, read-only repository commands:

```text
iatros analyze [flags] [path]
iatros topology [flags] [path]
```

`analyze` applies bounded repository ignore rules, isolates nested repositories, detects project ecosystems and operational markers, and produces an evidence-based readiness report. `topology` uses the same inventory, safely reads allowlisted manifests, and produces project, component, workspace, and direct dependency relationships. Both return text or JSON.

The default CLI now runs the implemented analyzer. The stable envelope still permits an explicit `not_implemented` result for a future analyzer implementation that is deliberately unavailable; it must never fabricate ecosystems, findings, scan counts, or successful analysis.

## 2. Problem

Developers and DevOps engineers often need a quick, repeatable answer to basic repository questions:

- What ecosystems and operational tools are present?
- Does the repository contain expected project documentation and metadata?
- Are tests and CI configuration visible?
- Which readiness gaps can be supported by direct filesystem evidence?

The first slice establishes a trustworthy local analysis boundary before adding AI, remote providers, infrastructure access, or state-changing operations.

## 3. Users

### DevOps engineer

Needs a fast inventory and readiness overview before improving delivery, infrastructure, observability, or operational practices.

### Software developer

Needs a clear explanation of missing repository foundations without requiring knowledge of every DevOps tool.

Both users must receive evidence and remediation rather than unsupported conclusions.

## 4. Goals

The first vertical slice will:

1. accept a user-selected local directory;
2. operate without network access or external credentials;
3. inspect the directory without modifying it;
4. detect supported ecosystem and operational marker files;
5. report basic repository-readiness findings;
6. provide semantically equivalent text and JSON output;
7. keep ordering and machine-readable output deterministic;
8. preserve a stable result envelope from the initial stub through real analysis;
9. make every finding traceable to local evidence;
10. fail honestly when analysis is unavailable, invalid, incomplete, or unsuccessful;
11. honor repository-owned ignore rules without unbounded control-file reads;
12. prevent submodule and nested-worktree content from being attributed to the parent repository.

## 5. Non-goals

This slice does not include:

- remote Git repositories or source-control APIs;
- LAN discovery, network shares, remote hosts, or service scanning;
- AI providers, prompts, embeddings, or model calls;
- cloud, cluster, database, CI/CD, or observability credentials;
- dependency installation or execution of repository code;
- shell-command execution;
- file generation, file modification, pull requests, deployment, or rollback;
- deep semantic analysis of source code;
- vulnerability scanning;
- monorepo orchestration across separately configured roots;
- policy-based failure thresholds for findings.

A user-mounted network filesystem may look like a local path, but it is outside the supported and tested scope of this slice. IATROS will not discover or initiate network access.

## 6. CLI contract

### Syntax

```text
iatros analyze [flags] [path]
iatros topology [flags] [path]
```

### Arguments and flags

| Input | Required | Default | Meaning |
| --- | --- | --- | --- |
| `path` | No | `.` | Local directory to analyze. |
| `--format text` | No | Selected | Human-readable report. |
| `--format json` | No | — | Machine-readable report using the versioned result envelope. |
| `--profile small` | No | Selected | Conservative complete-pipeline resource profile. |
| `--profile monorepo` | No | — | Explicit larger profile for multi-project repositories. |

Unknown flags, unsupported formats, or more than one positional path are usage errors.

The commands use separate versioned envelopes. Both current schemas are `0.3` and record the active profile and skipped nested-repository count. `analyze` reads only bounded repository control files in addition to ordinary file metadata.

### Output streams

- `stdout` contains only the selected report format.
- `stderr` contains CLI usage help or diagnostic logging, never a second report.
- JSON output must remain valid JSON even when the analysis status is `not_implemented`, `partial`, or `failed`.
- Topology JSON must remain valid JSON when its status is `partial` or `failed`.
- Output ends with one newline.

### Path behavior

- The default path is the current working directory.
- The implementation resolves and validates the path internally.
- User-visible paths remain relative to the analysis root and use forward slashes.
- The absolute local root is not included in normal report output.
- The target must be an existing directory.
- The selected root itself must not be a symbolic link or junction. Existing links in its parent path are resolved by the operating system while locating the selected root; they do not authorize analysis to follow links below that root.

## 7. Result status

| Status | Meaning |
| --- | --- |
| `not_implemented` | The `analyze` contract exists, but no repository analysis was performed. Topology does not use this status. |
| `completed` | Analysis finished within all configured limits. |
| `partial` | Safe limits or recoverable read failures prevented complete analysis. |
| `failed` | No trustworthy analysis result could be produced. |

A `partial` result must explain what was skipped. It must not silently look complete.

## 8. Exit codes

| Code | Meaning |
| --- | --- |
| `0` | The selected analysis completed or produced an explicitly partial report. Readiness findings do not change the exit code. |
| `1` | Operational or internal failure. |
| `2` | Invalid command usage. |
| `3` | Target path is missing, inaccessible, or not a directory. |
| `4` | The configured readiness analyzer explicitly reports that its implementation is unavailable. The default local analyzer and topology command do not return this code. |

A future opt-in policy such as `--fail-on` may map findings to a non-zero exit code. It is outside this specification and must not change the default behavior.

## 9. JSON envelope

The current analysis schema version is `0.3`. Version `0.2` added the active profile; version `0.3` adds the skipped nested-repository count.

```json
{
  "schema_version": "0.3",
  "profile": "small",
  "status": "completed",
  "target": {
    "kind": "local_directory",
    "path": "."
  },
  "summary": {
    "directories_scanned": 3,
    "files_scanned": 7,
    "nested_repositories_skipped": 0,
    "ecosystems_detected": 3,
    "findings_total": 0
  },
  "ecosystems": [
    {
      "id": "github-actions",
      "category": "ci_cd",
      "evidence": [
        ".github/workflows/ci.yml"
      ],
      "evidence_truncated": false
    },
    {
      "id": "go",
      "category": "language",
      "evidence": [
        "go.mod",
        "main.go",
        "main_test.go"
      ],
      "evidence_truncated": false
    },
    {
      "id": "go-modules",
      "category": "dependency_manager",
      "evidence": [
        "go.mod"
      ],
      "evidence_truncated": false
    }
  ],
  "findings": [],
  "diagnostics": []
}
```

### Envelope rules

- Top-level fields shown above are always present.
- New optional fields may be added when readers are expected to ignore unknown fields.
- Removing a field, renaming a field, or changing its meaning requires a new `schema_version`.
- Reports contain no generation timestamp by default so identical inputs can produce deterministic output.
- Collections use deterministic ordering.
- Counts match the included result collections and actual scan scope.
- Empty or unavailable data is represented honestly; it is not replaced with guessed values.

### Repository topology envelope

`iatros topology` has an independent schema version `0.3`, fixed `report_type: repository_topology`, and required active profile:

```json
{
  "schema_version": "0.3",
  "report_type": "repository_topology",
  "profile": "small",
  "status": "completed",
  "target": {"kind": "local_directory", "path": "."},
  "summary": {
    "projects_total": 0,
    "workspaces_total": 0,
    "components_total": 0,
    "dependencies_total": 0,
    "internal_dependencies": 0,
    "unresolved_dependencies": 0,
    "ambiguous_dependencies": 0,
    "nested_repositories_skipped": 0
  },
  "projects": [],
  "workspaces": [],
  "dependencies": [],
  "nested_repositories": [],
  "diagnostics": []
}
```

All top-level and nested collections are arrays when empty. Projects include roots, kinds, workspace relationships, markers, and parsed components. Workspaces include containment, explicit member and exclusion matches, and declaration resolution. Dependencies include their source, ecosystem, declaration details, resolution, and bounded local targets. Nested repository paths identify submodules or embedded Git worktrees deliberately excluded from the parent analysis. Every path is root-relative; no timestamp, absolute target, remote submodule URL, or raw manifest content is included. The complete field and validation contract is documented in [Repository Topology Architecture](../architecture/topology.md#10-public-cli-contract).

## 10. Ecosystem result

An implemented detector may return:

```json
{
  "id": "go",
  "category": "language",
  "evidence": [
    "go.mod"
  ],
  "evidence_truncated": false
}
```

Rules:

- `id` is a stable lowercase identifier.
- `category` is the technology's stable primary role, such as `language`, `dependency_manager`, or `ci_cd`.
- `evidence` contains sorted paths relative to the analysis root.
- `evidence_truncated` is `true` when the per-technology evidence limit omitted additional matching paths.
- Ecosystem results are sorted by `id`; duplicate identifiers or evidence paths are invalid.
- A marker indicates detected tooling or an ecosystem; it does not prove that the repository builds or runs.
- The detector must not claim an ecosystem without direct evidence.

The internal filename catalog covers common backend languages, runtimes, dependency managers, build systems, containers, orchestration, infrastructure as code, configuration management, CI/CD, GitOps, observability, networking, security, secrets, and cloud-platform tooling. The exact supported IDs and evidence policy are documented in the [technology detection architecture](../architecture/detection.md).

Filename-only results remain private internal `Technology` records. The analysis orchestrator translates them into the versioned `Ecosystem` report contract, preserving category, bounded evidence, and truncation state without exposing detector types to the CLI.

## 11. Finding result

An implemented readiness rule may return:

```json
{
  "code": "repository.readme.missing",
  "severity": "warning",
  "message": "The repository does not contain a recognized README file.",
  "evidence": [
    "README file not found at the repository root"
  ],
  "remediation": "Add a root README that explains the project, its status, and verified usage."
}
```

### Finding rules

- `code` is a stable lowercase dotted identifier.
- `severity` is `info`, `warning`, or `critical`.
- `message` describes the observed condition without exaggeration.
- `evidence` explains which local facts support the finding.
- `remediation` proposes a bounded next action.
- Finding evidence is sorted lexicographically without duplicates.
- Findings are ordered by severity (`critical`, `warning`, then `info`), followed by code and evidence in lexical order; duplicate ordering keys are invalid.
- Findings never imply that an absent marker proves a system is insecure or broken.

The internal readiness evaluator implements:

| Code | Condition | Initial severity |
| --- | --- | --- |
| `repository.readme.missing` | No recognized root README. | `warning` |
| `repository.license.missing` | No recognized root license file. | `info` |
| `repository.gitignore.missing` | No root `.gitignore`. | `info` |
| `quality.tests.not_detected` | No supported test marker is detected. | `warning` |
| `delivery.ci.not_detected` | No supported CI configuration is detected. | `warning` |

These checks report marker presence, not the quality or correctness of the referenced files.

The current rules evaluate only complete discovery snapshots. If discovery is partial, absence-based findings are suppressed because omitted or unreadable paths make absence untrustworthy. An empty complete repository receives only the README, license, and `.gitignore` findings. Test absence applies only when a language or runtime is detected; infrastructure-only repositories do not receive the code-test finding.

## 12. Text output

A completed text report uses the same data as JSON and groups technologies by category:

```text
IATROS Local Repository Analysis
Status: completed
Target: .

Summary:
- Directories scanned: 3
- Files scanned: 7
- Ecosystems detected: 3
- Findings total: 0

Technologies:
- CI/CD:
  - github-actions
    Evidence:
      - .github/workflows/ci.yml
    Evidence truncated: false
- Dependency manager:
  - go-modules
    Evidence:
      - go.mod
    Evidence truncated: false
- Language:
  - go
    Evidence:
      - go.mod
      - main.go
      - main_test.go
    Evidence truncated: false

Findings:
- None
```

An implemented text report must represent the same target, status, summary, ecosystems, findings, and diagnostics as the JSON report. Text is a presentation format, not a separate analysis path.

## 13. Safety and privacy constraints

The implementation must:

- remain read-only and never create, modify, rename, or delete target files;
- avoid network, DNS, remote API, telemetry-export, and provider calls;
- never execute repository files, build scripts, hooks, package managers, or detected tools;
- reject a missing or non-directory target;
- use one explicit analysis root;
- reject a selected root that is a symbolic link or junction, and not follow links encountered below the root in the first scanner implementation;
- prevent traversal outside the root;
- skip VCS internals such as `.git/`;
- never read values from `.env`, credentials, private keys, certificates, tokens, or known secret stores;
- keep the active analysis schema `0.3` report path metadata-only except for bounded `.gitignore` and root `.gitmodules` control files; topology enrichment may additionally read only exact registered manifest filenames through a confined source and the active validated profile;
- use relative evidence paths and avoid exposing local usernames or absolute paths;
- enforce file-count, directory-count, directory-depth, per-directory entry, retained-issue, evidence, and execution-time limits;
- return `partial` or `failed` with diagnostics when safe analysis cannot continue;
- handle cancellation without reporting success.

The internal discovery baseline uses the following conservative defaults:

| Limit | Basic default |
| --- | ---: |
| Files retained | 2,000 |
| Directories retained, including the root | 500 |
| Directory depth below the root | 20 |
| Structured discovery issues retained | 50 |
| Entries accepted from one directory | 2,500 |
| Ignore files | 100 |
| Bytes per ignore/control file | 256 KiB |
| Bytes per ignore pattern | 4 KiB |
| Retained ignore rules | 10,000 |
| Nested repository boundaries | 100 |
| Discovery duration | 5 seconds |

Discovery reads directory entries, file metadata, bounded `.gitignore` files, and bounded root `.gitmodules`. It retains one confined `os.Root`, returns sorted root-relative paths, skips symbolic links, junction-like irregular entries, `.git`, `.hg`, and `.svn`, applies ordered root and nested Git-style rules, and isolates submodules and nested Git worktrees. Other regular files are never opened. Access failures and reached limits produce bounded structured issues that can later support deterministic or AI-assisted remediation comments.

The separate internal manifest stage may open `go.mod`, `go.work`, `package.json`, `pyproject.toml`, `Cargo.toml`, `composer.json`, and `pom.xml` after discovery. It processes one document at a time and uses independent exact-filename, identity, byte, nesting, retained-value, collection, cancellation, and diagnostic controls. It does not execute tools, resolve external entities, contact registries, or retain raw content and parser errors in its result.

| Manifest limit | `small` | `monorepo` |
| --- | ---: | ---: |
| Files | 100 | 2,000 |
| Bytes per file | 256 KiB | 8 MiB |
| Total bytes | 4 MiB | 256 MiB |
| Dependencies per manifest | 1,000 | 10,000 |
| Constraints per manifest | 100 | 1,000 |
| Workspace members per manifest | 500 | 5,000 |
| Workspace exclusions per manifest | 500 | 5,000 |
| Structured issues | 50 | 200 |
| Nesting depth | 64 | 128 |
| Bytes per retained value | 4 KiB | 64 KiB |
| Analysis duration | 3 seconds | 30 seconds |

These limits are now selected through the complete `small`, `monorepo`, or Enterprise per-worker profile rather than independently. The default CLI exposes `small` and `monorepo`; the complete values and Enterprise activation boundary are defined in [Scaling profiles](../architecture/scaling-profiles.md).

Root and nested `.gitignore` files support scoped last-match precedence, negation, escaping, anchoring, directory-only rules, standard wildcards, ranges, and defined globstar forms. If a complete ignore file cannot be applied within its bounds, IATROS keeps a wider partial inventory instead of applying an order-sensitive prefix. Repository-owned rules are deterministic and intentionally exclude user-global Git settings, `.git/info/exclude`, and index tracking state. The complete contract is documented in [Repository discovery architecture](../architecture/repository-discovery.md).

## 14. Placeholder compatibility contract

The historical placeholder remains available as a tested analyzer implementation for compatibility and future deliberately unavailable configurations. It may:

1. parse `analyze`, `path`, and `--format`;
2. validate that the target exists and is a directory;
3. produce the complete text or JSON envelope with `status: not_implemented`;
4. leave scan counts, ecosystems, and findings empty;
5. return exit code `4`;
6. perform no directory traversal, content reads, network access, or writes.

The placeholder must not:

- return exit code `0`;
- claim that analysis completed;
- populate fake findings or ecosystems;
- make the text and JSON behavior diverge;
- introduce temporary fields that disappear when real analysis is added.

## 15. Acceptance criteria

The product specification is satisfied when:

1. `iatros analyze` accepts zero or one local directory path.
2. `--format text` and `--format json` describe the same result.
3. The default composition root runs real local analysis; the placeholder remains an explicit non-default outcome.
4. Real analysis populates the stable envelope without changing its established meaning.
5. The command performs no network access and no target writes.
6. All user-visible project content and messages are English.
7. Output is deterministic for the same root, files, configuration, and implementation version.
8. Evidence paths are root-relative and do not expose the absolute local path.
9. Findings do not change the default success exit code.
10. Invalid usage, invalid targets, unavailable implementation, and internal failures use distinct exit codes.
11. Unit tests cover parsing, result formatting, ordering, and error mapping.
12. Integration tests prove the local-only, read-only path from CLI input to report output.
13. The README is updated only when runnable behavior exists and uses verified commands.
14. `iatros topology` exposes the associated local model through a separate validated schema; both reports identify the active scaling profile.
15. Root and nested repository-owned ignore rules affect both commands through the same bounded discovery snapshot.
16. Declared submodules and detected nested Git worktrees are reported but their contents are not analyzed as parent projects.
17. Oversized control files and manifests produce explicit bounded partial results without unbounded reads.

## 16. Implementation sequence

1. **Contract stub — complete:** initialize the Go module and return the approved placeholder envelope.
2. **Safe discovery — complete:** add bounded ignore-aware local inventory with explicit control-file exceptions.
3. **Marker detection — complete:** populate ecosystems from direct evidence.
4. **Readiness rules — complete:** populate findings from supported repository markers.
5. **Integration — complete:** compose discovery, detection, readiness, diagnostics, report validation, text/JSON rendering, exit behavior, and documentation.

The working ten-step plan counts the Go module and Cobra foundation separately from the contract stub. In that plan:

- **Step 7, project boundaries — complete internally:** identify nested project and workspace roots through the topology workflow.
- **Step 8, manifest analysis — complete internally:** parse bounded allowlisted manifests into normalized facts behind replaceable backends.
- **Step 9, topology integration — complete internally:** associate boundary and manifest facts, resolve repository-confined workspace membership, and build direct local dependency edges.
- **Step 10, public topology contract — complete:** expose a separate validated `repository_topology` schema through deterministic CLI text and JSON.
- **Scaling profiles — complete:** configure the complete pipeline through explicit `small` or `monorepo` CLI selection and retain an Enterprise per-worker composition profile.
- **Full local repository discovery — complete:** apply bounded ignore rules, handle large files safely, preserve nested project boundaries, and isolate submodules and nested Git worktrees.

## 17. Deferred decisions

- distributed Enterprise admission, worker fleets, quotas, autoscaling, isolation, and production capacity calibration beyond the implemented per-worker profile;
- selection criteria and production composition for optional specialized, third-party, generated, or streaming parser backends; the replacement contract already exists;
- optional machine-specific Git ignore sources and an explicit configuration policy;
- hidden-file policy outside known sensitive paths;
- binary-file detection;
- supported marker versions and confidence model;
- opt-in policy failure thresholds;
- schema stabilization criteria beyond the current `0.3` reports;
- treatment of user-mounted remote filesystems;
- local caching, if any.

Each deferred choice must preserve the local-only, read-only, evidence-based contract.
