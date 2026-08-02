# PS-0001: Local Repository Analysis

- **Product status:** Approved
- **Implementation status:** In progress
- **Approved:** 2026-08-01
- **Primary users:** DevOps engineers and software developers
- **Interface language:** English
- **Execution scope:** Local filesystem only

Related documents:

- [IATROS target architecture](../architecture/README.md)
- [Security architecture](../architecture/security.md)
- [Testing strategy](../architecture/testing.md)
- [ADR-0004: Control state-changing operations](../architecture/decisions/0004-control-state-changing-operations.md)
- [ADR-0005: Use Cobra as the CLI adapter](../architecture/decisions/0005-use-cobra-as-the-cli-adapter.md)

## Implementation progress

| Increment | Status |
| --- | --- |
| Go module and Cobra command surface | Implemented |
| Versioned text and JSON contract stub | Implemented |
| Local target validation and documented exit codes | Implemented |
| Safe filesystem discovery | Implemented internally; CLI integration deferred |
| Ecosystem marker detection | Implemented internally; report and CLI integration deferred |
| Repository-readiness findings | Not started |

## 1. Summary

The first IATROS vertical slice is a local, read-only repository analysis command:

```text
iatros analyze [flags] [path]
```

The command will eventually inspect a directory on the same machine, detect project ecosystems and operational markers, and produce an evidence-based readiness report in text or JSON format.

The initial implementation may return an explicit `not_implemented` result through the same stable result envelope. It must never fabricate ecosystems, findings, scan counts, or successful analysis.

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
10. fail honestly when analysis is unavailable, invalid, incomplete, or unsuccessful.

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
```

### Arguments and flags

| Input | Required | Default | Meaning |
| --- | --- | --- | --- |
| `path` | No | `.` | Local directory to analyze. |
| `--format text` | No | Selected | Human-readable report. |
| `--format json` | No | — | Machine-readable report using the versioned result envelope. |

Unknown flags, unsupported formats, or more than one positional path are usage errors.

### Output streams

- `stdout` contains only the selected report format.
- `stderr` contains CLI usage help or diagnostic logging, never a second report.
- JSON output must remain valid JSON even when the analysis status is `not_implemented`, `partial`, or `failed`.
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
| `not_implemented` | The contract exists, but no repository analysis was performed. |
| `completed` | Analysis finished within all configured limits. |
| `partial` | Safe limits or recoverable read failures prevented complete analysis. |
| `failed` | No trustworthy analysis result could be produced. |

A `partial` result must explain what was skipped. It must not silently look complete.

## 8. Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Analysis completed or produced an explicitly partial report. Findings do not change the exit code in this slice. |
| `1` | Operational or internal failure. |
| `2` | Invalid command usage. |
| `3` | Target path is missing, inaccessible, or not a directory. |
| `4` | Analysis is not implemented yet. |

A future opt-in policy such as `--fail-on` may map findings to a non-zero exit code. It is outside this specification and must not change the default behavior.

## 9. JSON envelope

The initial schema version is `0.1`.

```json
{
  "schema_version": "0.1",
  "status": "not_implemented",
  "target": {
    "kind": "local_directory",
    "path": "."
  },
  "summary": {
    "directories_scanned": 0,
    "files_scanned": 0,
    "ecosystems_detected": 0,
    "findings_total": 0
  },
  "ecosystems": [],
  "findings": [],
  "diagnostics": [
    {
      "code": "IATROS_ANALYSIS_NOT_IMPLEMENTED",
      "level": "info",
      "message": "Local repository analysis is not implemented yet."
    }
  ]
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

## 10. Ecosystem result

An implemented detector may return:

```json
{
  "id": "go",
  "evidence": [
    "go.mod"
  ]
}
```

Rules:

- `id` is a stable lowercase identifier.
- `evidence` contains sorted paths relative to the analysis root.
- Ecosystem results are sorted by `id`; duplicate identifiers or evidence paths are invalid.
- A marker indicates detected tooling or an ecosystem; it does not prove that the repository builds or runs.
- The detector must not claim an ecosystem without direct evidence.

The internal filename catalog covers common backend languages, runtimes, dependency managers, build systems, containers, orchestration, infrastructure as code, configuration management, CI/CD, GitOps, observability, networking, security, secrets, and cloud-platform tooling. The exact supported IDs and evidence policy are documented in the [technology detection architecture](../architecture/detection.md).

Filename-only results are private internal `Technology` records in this increment. Mapping them into the versioned `Ecosystem` report contract is deferred until discovery, detection, diagnostics, and bounded-evidence behavior can be integrated together without changing the CLI stub prematurely.

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

Initial readiness rules may include:

| Code | Condition | Initial severity |
| --- | --- | --- |
| `repository.readme.missing` | No recognized root README. | `warning` |
| `repository.license.missing` | No recognized root license file. | `info` |
| `repository.gitignore.missing` | No root `.gitignore`. | `info` |
| `quality.tests.not_detected` | No supported test marker is detected. | `warning` |
| `delivery.ci.not_detected` | No supported CI configuration is detected. | `warning` |

These checks report marker presence, not the quality or correctness of the referenced files.

## 12. Text output

The stub text report is:

```text
IATROS Local Repository Analysis
Status: not implemented
Target: .

No analysis was performed.
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
- limit content reads to an explicit allowlist of marker files required by implemented detectors;
- use relative evidence paths and avoid exposing local usernames or absolute paths;
- enforce file-count, directory-depth, file-size, total-read, and execution-time limits before full scanning is implemented;
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
| Discovery duration | 5 seconds |

Discovery reads directory entries and file metadata only. It retains one confined `os.Root`, returns sorted root-relative paths, skips symbolic links, junction-like irregular entries, `.git`, `.hg`, and `.svn`, and never opens regular files. Access failures and reached limits produce bounded structured issues that can later support deterministic or AI-assisted remediation comments.

Root and nested `.gitignore` rules are not interpreted in this increment. Correct support requires nested rule scope, negation, escaping, and anchored matching; partial support could hide relevant evidence. The `.gitignore` file itself remains visible in inventory, while its patterns do not change traversal.

## 14. Stub implementation contract

The first executable placeholder may:

1. parse `analyze`, `path`, and `--format`;
2. validate that the target exists and is a directory;
3. produce the complete text or JSON envelope with `status: not_implemented`;
4. leave scan counts, ecosystems, and findings empty;
5. return exit code `4`;
6. perform no directory traversal, content reads, network access, or writes.

The stub must not:

- return exit code `0`;
- claim that analysis completed;
- populate fake findings or ecosystems;
- make the text and JSON behavior diverge;
- introduce temporary fields that disappear when real analysis is added.

## 15. Acceptance criteria

The product specification is satisfied when:

1. `iatros analyze` accepts zero or one local directory path.
2. `--format text` and `--format json` describe the same result.
3. The placeholder behavior exactly follows the stub contract until scanning exists.
4. Real analysis replaces empty values without changing the envelope's meaning.
5. The command performs no network access and no target writes.
6. All user-visible project content and messages are English.
7. Output is deterministic for the same root, files, configuration, and implementation version.
8. Evidence paths are root-relative and do not expose the absolute local path.
9. Findings do not change the default success exit code.
10. Invalid usage, invalid targets, unavailable implementation, and internal failures use distinct exit codes.
11. Unit tests cover parsing, result formatting, ordering, and error mapping.
12. Integration tests prove the local-only, read-only path from CLI input to report output.
13. The README is updated only when runnable behavior exists and uses verified commands.

## 16. Implementation sequence

1. **Contract stub:** initialize the Go module and return the approved placeholder envelope.
2. **Safe discovery:** add bounded local filesystem inventory without content analysis.
3. **Marker detection:** populate ecosystems from direct evidence.
4. **Readiness rules:** populate findings from supported repository markers.
5. **Integration:** verify text/JSON parity, deterministic output, safety constraints, and documentation.

## 17. Deferred decisions

- release and Enterprise discovery-limit profiles and their configuration surface;
- complete `.gitignore` semantics and their configuration policy;
- hidden-file policy outside known sensitive paths;
- nested project and monorepo boundaries;
- binary-file detection;
- supported marker versions and confidence model;
- opt-in policy failure thresholds;
- schema stabilization criteria beyond `0.1`;
- treatment of user-mounted remote filesystems;
- local caching, if any.

Each deferred choice must preserve the local-only, read-only, evidence-based contract.
