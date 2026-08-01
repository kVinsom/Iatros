# ADR-0003: Start with one Go module

- **Status:** Accepted
- **Date:** 2026-07-31
- **Accepted:** 2026-08-01
- **Supersedes:** None
- **Superseded by:** None

## Context

Go is the intended language for the backend, CLI, SDK, and first-party plugins, but the repository does not yet contain `go.mod`, source code, a canonical module path, or independently released packages.

Starting with multiple modules would introduce workspace configuration, version coordination, release automation, compatibility policy, and cross-module refactoring costs before independent lifecycles are demonstrated.

## Decision

Begin first-party Go implementation with one `go.mod` at the repository root.

- Use `github.com/kVinsom/Iatros` as the canonical module path.
- Backend executables, internal domains, public Go contracts, and first-party Go plugins initially share the root module and release cadence.
- Do not introduce nested modules merely to mirror directory boundaries.
- Split an SDK, plugin family, or other component only when it has a demonstrated need for independent consumption, versioning, compatibility, and release.
- A module split requires a new ADR and migration plan.

## Consequences

### Positive

- Simple local builds, tests, refactoring, and dependency management.
- Atomic changes across early boundaries.
- No premature cross-module version coordination.
- Architecture boundaries remain package and dependency rules rather than release machinery.

### Trade-offs

- All first-party Go packages initially share one release cadence.
- External consumers cannot version the SDK independently at first.
- Later extraction may require import-path migration and compatibility work.
- CI must still enforce architectural boundaries within the single module.

## Alternatives considered

### Separate modules for SDK and every plugin category

Deferred because there is no implementation or independent release evidence.

### A Go workspace from the beginning

Deferred because a workspace coordinates modules but does not justify why those modules should exist.

## Implementation validation

The initial implementation must continue to:

- keep the canonical module path documented in the root `go.mod`;
- verify all initial first-party packages build and test from the root;
- ensure no nested `go.mod` is introduced without a superseding ADR.

## References

- [IATROS target architecture](../README.md)
- [ADR-0001: Capability boundaries and dependency direction](0001-capability-boundaries-and-dependency-direction.md)
