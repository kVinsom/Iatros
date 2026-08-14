# PS-0008: Change Impact Analysis Contract

- **Status:** Approved
- **Implementation status:** Core analyzer implemented; change collection and product-surface integration are not implemented
- **Approved:** 2026-08-14

See also:

- [Unified system map contract](0006-system-map-contract.md)
- [Remote and polyrepository analysis contract](0007-remote-polyrepo-analysis-contract.md)
- [Change impact architecture](../architecture/change-impact.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Problem

A changed source or infrastructure file rarely affects only itself. A library change can affect several services;
an infrastructure change can affect workloads and environments; and a rename can invalidate both the old and
new ownership paths. IATROS needs a deterministic, explainable answer to which services, environments, and
configurations may be affected before planning or deployment.

## 2. Implemented boundary

`internal/changeimpact` accepts two already-normalized inputs:

1. a validated provider-neutral system map; and
2. a change set containing stable change IDs, repository IDs, operation kinds, and safe repository-relative paths.

The package performs no Git command execution, provider access, filesystem traversal, network access, or target
mutation. Git working-tree, commit-range, pull-request, and provider-webhook collectors remain separate future
adapters. They must translate their external representations into this contract rather than add provider behavior
to the analyzer.

## 3. Change contract

Supported operations are `added`, `modified`, `deleted`, and `renamed`. Added, modified, and deleted entries have
one current path. A rename has distinct previous and current paths; both are analyzed because either side may map
to a system entity. Paths are normalized repository-relative file paths and cannot be absolute, traverse a parent,
or use platform-specific separators.

Change sets and changes have stable IDs. Input order is not semantic: the analyzer normalizes by change ID before
validation and processing.

## 4. Impact result

The schema-versioned result contains bounded, sorted collections of:

- affected services with their repository identities;
- affected environments;
- affected infrastructure configurations with their repository identities;
- evidence-bearing causes; and
- diagnostics and a `partial` flag.

Each cause preserves the originating change ID, changed path, and ordered system-map relationship IDs that led to
the impact. Impact kinds are:

| Kind | Meaning |
| --- | --- |
| `direct` | The changed path is inside an entity root or exactly matches retained entity evidence. |
| `dependent` | A supported dependency-like relationship reaches the entity from a directly or transitively affected target. |
| `associated` | An environment is attached to an affected service, configuration, or relationship. |

Direct impact takes precedence over dependent impact, which takes precedence over associated environment impact.

## 5. Propagation rules

The analyzer propagates from a changed target to entities that depend on that target. The allowlisted relationship
kinds are `calls`, `configured_by`, `consumes`, `depends_on`, `deploys`, `publishes_to`, `reads_from`, `runs_on`,
and `uses`. Ownership, descriptive, unknown, and vendor-specific relationship kinds do not silently propagate
impact. Supporting another kind requires a documented normalized semantic and test cases.

Cycles terminate through bounded visited-state tracking. Multiple changes remain independently attributable. A
changed path with no mapped entity produces an informational diagnostic rather than inventing an affected target.

## 6. Completeness and limits

The analyzer enforces injectable limits for changes, direct entity-match work, retained services, environments,
configurations, causes, relationship-chain length, traversal work, diagnostics, text, and execution time. `small`, `monorepo`, and
`enterprise` starting profiles are implemented.

A retention, depth, or work limit produces a warning, sets `partial`, and preserves the trustworthy retained
prefix. A partial input system map also makes the result partial. Cancellation or invalid input returns an error and
does not misrepresent an empty impact as a completed answer.

## 7. Acceptance criteria

The implemented core contract must:

- identify direct service, environment, and configuration impact;
- analyze both sides of a rename;
- propagate supported dependency relationships in the correct direction;
- preserve the changed path and relationship chain for every retained cause;
- terminate deterministically for cycles and repeated paths;
- reject unsafe paths, unknown repositories, invalid operations, and malformed models;
- produce explicit partial diagnostics when any configured retention or traversal limit omits data;
- honor cancellation and timeouts; and
- return the same normalized result for semantically equivalent input ordering.

## 8. Deferred product work

- collect changes from a local Git worktree, staged index, commit range, or explicit patch;
- collect pull-request and merge-request changes through source-control adapters;
- build complete local and remote system maps through implemented translators;
- expose impact through CLI, API, MCP, or dashboard surfaces;
- map findings, policies, approvals, and deployment gates onto impact results; and
- add language-aware symbol impact where file-level evidence is insufficient.
