# ADR-0006: Version private core contracts independently from public APIs

- **Status:** Accepted
- **Date:** 2026-08-03
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS needs stable representations for projects, environments, services, dependencies, configuration, and lifecycle results before broader Analyze, Plan, Generate, Validate, Deploy, Monitor, and Fix features can share data safely.

Using CLI reports, public API messages, provider SDK types, or one global shared model as the core representation would couple private domain evolution to external compatibility. Publishing a public schema before an API, SDK, or plugin consumer exists would also create irreversible compatibility obligations without evidence that the shape is appropriate.

At the same time, leaving internal schemas unversioned would make persisted results, migrations, provenance, and cross-version tests ambiguous.

## Decision

1. `internal/project` owns the canonical provider-neutral project aggregate, including environments, services, directed dependencies, and configuration declarations.
2. `internal/workflow` owns the common lifecycle-stage result envelope and canonical stages and terminal outcomes.
3. `internal/schema` owns canonical `major.minor` parsing and compatibility checks used by private versioned contracts.
4. The project and workflow schemas start at version `1.0`.
5. Stable major versions accept documents with the same major and an equal or older minor version. Pre-`1.0` versions require an exact match.
6. Each model exposes normalization and validation. Normalization detaches and canonically orders collections; validation rejects invalid identity, ordering, reference, safety, and compatibility state.
7. Configuration stores non-sensitive literals or unresolved environment, file, and secret references. It never treats a secret value as an inline literal.
8. Feature-owned payloads remain with their capability and use the workflow envelope without moving their models into a global shared package.
9. Internal JSON tags support deterministic private persistence and tests but do not create a public wire contract.
10. `api/` and `sdk/` define and version their own contracts only when a concrete external consumer exists. Boundary adapters translate between those contracts and private models.
11. The existing analysis report remains on its independent `0.1` contract until an approved feature specification changes it.

## Consequences

### Positive

- Core workflows share stable vocabulary and compatibility rules.
- Project topology remains provider-neutral and subscription-neutral.
- Deterministic normalization supports repeatable tests, diffs, hashes, and persistence.
- Sensitive configuration is represented by reference instead of embedded secret material.
- Public APIs can evolve around real consumers without exposing private Go packages.
- Feature-specific models keep clear owners instead of accumulating in a shared-model directory.

### Trade-offs

- Public boundaries will require explicit translation code.
- Internal schema changes now require compatibility analysis, tests, and explicit migrations.
- The first contracts intentionally omit provider-specific fields and workflow persistence mechanics.
- Feature payload validation remains separate from workflow-envelope validation.

## Alternatives considered

### Publish the private models directly through API and SDK packages

This was rejected because no public consumer or transport contract exists yet, and private representation has a different evolution lifecycle from external wire messages.

### Use maps and untyped payloads throughout the core

This was rejected because field meaning, ordering, validation, secret safety, reference integrity, and compatibility could not be enforced consistently.

### Create one global shared models package

This was rejected because unrelated capability models would lose ownership and become transitively coupled. Only schema compatibility is cross-cutting; project topology and workflow results remain separately owned.

### Leave private models unversioned

This was rejected because stored state and provenance would not identify the contract required to read or migrate them.

## Validation

- Unit tests cover canonical and malformed schema versions and stable compatibility.
- Project contract tests cover valid, empty, unsorted, duplicate, unsafe, and unresolved models.
- Workflow contract tests cover every lifecycle outcome, envelope invariant, deterministic ordering, and detached normalization.
- Repository verification runs formatting, unit tests, vet, and build across the Go module.
- Architecture review rejects direct public exposure of `internal/` types.

## References

- [Stable Core Domain Contracts](../../product/0002-core-domain-contracts.md)
- [IATROS Product Contract](../../product/product-contract.md)
- [IATROS target architecture](../README.md)
- [ADR-0001: Capability boundaries and dependency direction](0001-capability-boundaries-and-dependency-direction.md)
