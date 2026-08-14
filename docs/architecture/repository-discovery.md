# Repository Discovery Architecture

> [!IMPORTANT]
> **Status: implemented for local analysis.** The shared discovery pipeline applies bounded repository-owned ignore rules, isolates Git submodules and nested worktrees, retains deterministic root-relative metadata, and feeds both `iatros analyze` and `iatros topology`.

## 1. Responsibility

Repository discovery produces one bounded inventory for every downstream local-analysis stage. It owns:

- safe traversal below one validated local directory;
- fixed exclusions for VCS metadata, downloaded dependencies, and generated tool state;
- root and nested `.gitignore` evaluation;
- declared submodule and nested Git-worktree boundaries;
- file, directory, depth, control-file, issue, and time limits;
- deterministic paths and partial-result diagnostics.

Discovery does not identify technologies, parse application content, execute tools, resolve dependencies, or contact Git remotes.

## 2. Ignore contract

IATROS reads repository-owned `.gitignore` files from the selected root and every entered child directory. Rules follow the repository pattern format defined by the [official Git ignore contract](https://git-scm.com/docs/gitignore). They are scoped to the directory containing the file. Parent rules are evaluated before child rules, and the last matching rule decides the result.

The matcher supports:

- blank lines and `#` comments;
- escaped leading `#` and `!` characters;
- ignored and escaped trailing spaces;
- `!` negation;
- leading-slash anchoring and patterns containing directory separators;
- directory-only patterns ending in `/`;
- `*`, `?`, character ranges, and the defined leading, middle, and trailing `**` forms;
- nested `.gitignore` precedence.

An ignored directory is not entered. Consequently, a later rule cannot re-include a file below an excluded parent unless the parent itself remains traversable. This matches Git's directory traversal rule.

The `.gitignore` and `.gitmodules` control files remain visible in inventory even if a rule would otherwise match their filenames. Fixed safety exclusions such as `.git`, `.hg`, `.svn`, `node_modules`, `.terraform`, and virtual-environment directories take precedence over repository rules.

### 2.1 Deliberate source boundary

The current local contract applies repository-owned `.gitignore` files to the filesystem snapshot. It does not read a user's global excludes file, Git configuration, or `.git/info/exclude`, because those sources are machine-specific and would make an identical repository produce different reports. It also does not parse the Git index, so the scanner does not distinguish tracked from untracked files when applying a pattern.

Additional ignore sources can be introduced later only as explicit configuration. They must remain bounded and identify their active policy in the report contract.

## 3. Bounded control-file handling

Discovery normally reads directory entries and regular-file metadata only. Two control-file exceptions are allowed:

- `.gitignore`, to decide traversal;
- root `.gitmodules`, to establish repository boundaries.

Both are read through the confined filesystem root, checked as regular non-link files, and limited before and during reading. Discovery never reserves a profile's full capacity in advance.

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Ignore files | 100 | 2,000 | 10,000 |
| Bytes per ignore/control file | 256 KiB | 1 MiB | 4 MiB |
| Bytes per ignore pattern | 4 KiB | 32 KiB | 64 KiB |
| Retained ignore rules | 10,000 | 100,000 | 500,000 |
| Nested repository boundaries | 100 | 5,000 | 25,000 |

If an ignore file cannot be read or exceeds its byte limit, its rules are not applied and the inventory becomes `partial`. If applying a file would exceed the rule limit, the complete file is left unapplied rather than applying only an order-sensitive prefix. The scan therefore favors a wider inventory with an explicit diagnostic over silently hiding relevant evidence.

Malformed patterns are not applied and produce a bounded diagnostic. Valid patterns from the same file remain active. All diagnostics use repository-relative paths and stable codes.

## 4. Submodules and nested Git worktrees

The root `.gitmodules` file is parsed for safe repository-relative `path` declarations. A declared submodule is retained as a nested-repository boundary, but its contents are not traversed or analyzed as part of the parent repository. Ancestor directories remain traversable when necessary to reach a declared boundary, even if a broad ignore rule matches an ancestor.

Discovery also treats a child directory containing its own regular `.git` file or `.git` directory as a nested Git worktree. This covers initialized submodules and embedded repositories that are not declared by the parent `.gitmodules` file.

The behavior prevents languages, manifests, dependencies, readiness markers, and infrastructure from a separate repository being attributed to the selected parent. `iatros analyze` reports the skipped-boundary count. `iatros topology` reports the count and sorted root-relative boundary paths. A nested repository is a deliberate exclusion and does not by itself make a report partial.

## 5. Large files

Ordinary regular files are never opened by discovery, regardless of size. Their metadata and paths are retained only within the selected discovery profile.

The topology pipeline opens only registered manifest filenames. It checks the reported size before parsing, wraps every read with per-file and total-byte limits, validates that the size stayed stable, processes one document at a time, and returns an explicit partial diagnostic for an oversized or changing manifest. Parser nesting, retained values, collections, issues, cancellation, and duration have independent limits.

This separation keeps large binaries, archives, datasets, logs, and generated assets from consuming content memory while still allowing their paths to contribute to a bounded inventory when they are not ignored.

## 6. Nested and complex repositories

The resulting flat inventory preserves every retained root-relative file. Project detection can therefore recognize independent code, infrastructure, mixed, and workspace roots at any retained depth. Topology then associates allowlisted manifests with the nearest exact project and workspace boundaries.

Ignored trees and nested repositories are removed before technology, project, readiness, manifest, and topology analysis. All consumers receive the same snapshot, so `analyze` and `topology` cannot disagree because of separate traversal implementations.

## 7. Public report contract

Analysis uses schema `1.0` with unified findings. The independent topology schema remains version `0.3`.

- Analysis summary adds `nested_repositories_skipped`.
- Topology summary adds `nested_repositories_skipped`.
- Topology adds the always-present `nested_repositories` array.

Every path is slash-separated, sorted, unique, root-relative, and validated not to contain a reported project or workspace. Reports contain no absolute submodule target, remote URL, `.git` metadata, or raw control-file content.

## 8. Verification baseline

Tests cover root and nested rule scope, ordering, negation, anchoring, directory-only patterns, globstars, escapes, malformed patterns, control-file byte and rule limits, declared and detected nested repositories, ignore/submodule interaction, oversized manifests, mixed nested projects, deterministic output, cancellation, and confined link-safe access.
