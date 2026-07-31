# ADR-0002: Single Community core with optional overlays

- **Status:** Accepted
- **Date:** 2026-07-31
- **Supersedes:** None
- **Superseded by:** None

## Context

The repository reserves space for optional Commercial and Enterprise capabilities. Separate edition trees containing copied core packages would create divergent behavior, duplicated fixes, inconsistent security posture, and permanent merge overhead.

Community must remain a complete base rather than an implementation detail required by paid overlays.

## Decision

IATROS maintains one provider-neutral Community core:

1. Core domain packages exist once under `internal/`.
2. Optional capabilities live under `extensions/commercial` or `extensions/enterprise`.
3. Extensions integrate through `sdk/extension` or `sdk/plugin` contracts.
4. Community must build, test, and operate without either extension directory being present.
5. Extensions may compose or add capabilities but do not copy or fork core packages.
6. The core does not import optional extensions.
7. Extension packaging and licensing remain undecided until extension code and distribution requirements exist.

## Consequences

### Positive

- Core behavior and fixes have one source of truth.
- Community receives the same foundational correctness and security improvements.
- Optional capabilities exercise stable extension points instead of private imports.
- Edition drift is structurally discouraged.

### Trade-offs

- Extension contracts require careful design and compatibility tests.
- Some composition and packaging work moves to executable or distribution boundaries.
- Community-only CI and tests are required once implementation exists.
- Commercial and Enterprise distribution, version compatibility, and licensing remain unresolved.

## Alternatives considered

### Separate Community, Commercial, and Enterprise source trees

Rejected because duplicated packages would diverge and make fixes difficult to propagate safely.

### Compile-time flags inside core packages

Rejected as the primary extension mechanism because paid behavior would remain entangled with Community domain code and would be difficult to reason about or test independently.

### Private imports from extensions into core

Rejected because it reverses dependency direction and makes Community incomplete without optional code.

## Validation

Future automation should prove:

- Community builds and passes tests without optional extensions;
- core packages do not import `extensions/`;
- extensions depend only on published contracts;
- the same core tests run for every supported distribution;
- compatibility is checked between an extension and the contracts it implements.

## References

- [IATROS target architecture](../README.md)
- [ADR-0001: Capability boundaries and dependency direction](0001-capability-boundaries-and-dependency-direction.md)
