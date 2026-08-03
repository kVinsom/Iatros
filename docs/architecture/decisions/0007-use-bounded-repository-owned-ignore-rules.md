# ADR 0007: Use bounded repository-owned ignore rules

- Status: Accepted
- Date: 2026-08-03

## Context

Local analysis previously skipped only fixed generated and VCS directories. Root and nested `.gitignore` files were visible but did not affect traversal, and initialized submodules could be mistaken for projects belonging to their parent repository.

Correct traversal must honor ordered nested rules without making the core depend on an installed Git executable, machine-specific user configuration, or an unnecessarily broad Git client dependency. Malformed or oversized control files must not cause unbounded reads or silent evidence loss.

## Decision

1. Implement a small provider-neutral Git-style matcher under `internal/repositoryignore`.
2. Apply repository-owned root and nested `.gitignore` files in one shared discovery pipeline.
3. Support rule scope, last-match precedence, negation, anchoring, directory-only rules, escaping, standard wildcards, ranges, and defined globstar forms.
4. Keep fixed safety exclusions independent from repository rules.
5. Bound ignore-file count, bytes, retained rules, nested repository boundaries, issues, and traversal time through the active scaling profile.
6. When a complete ignore file cannot be applied safely, prefer an explicit partial wider inventory over applying an incomplete order-sensitive prefix.
7. Parse bounded root `.gitmodules` declarations and detect nested `.git` worktrees. Record their roots but do not traverse them as part of the parent repository.
8. Do not read global Git excludes, Git configuration, `.git/info/exclude`, or the Git index in the default deterministic local contract.
9. Advance analysis and topology report schemas to `0.3` so skipped nested-repository boundaries are reviewable.

## Consequences

- Small repositories and monorepositories use identical ignore semantics with different validated capacities.
- Technology and topology results no longer include files from ignored trees or separate nested repositories.
- Discovery opens only bounded `.gitignore` and `.gitmodules` control files; ordinary files remain metadata-only.
- Reports can explain that nested repositories were deliberately excluded.
- Applying `.gitignore` independently of index state may exclude a tracked file matching a rule. This is an explicit deterministic analysis policy, not a claim to reproduce `git status`.
- Supporting additional local or user-specific ignore sources requires an explicit future configuration and reporting decision.

## Alternatives considered

- **Invoke the local Git executable.** Rejected because it adds an installation dependency, process execution, version variance, and repository-state coupling to core discovery.
- **Add a complete Git client dependency.** Rejected because the required surface is small and the dependency would materially expand the module and maintenance boundary.
- **Support only root patterns or a wildcard subset.** Rejected because incomplete precedence and negation can silently hide the wrong evidence.
- **Ignore all rules after any malformed line.** Rejected because Git treats a non-matching malformed pattern independently; valid bounded rules remain useful.

