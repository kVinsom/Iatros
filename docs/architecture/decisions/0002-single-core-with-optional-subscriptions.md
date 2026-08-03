# ADR-0002: Single provider-neutral core with optional subscriptions

- **Status:** Accepted
- **Date:** 2026-07-31
- **Updated:** 2026-08-03
- **Supersedes:** None
- **Superseded by:** None

## Context

The product provides a free local Basic plan without AI, a paid Pro plan with cloud AI, and a paid Enterprise plan with cloud or local AI and closed plugins. Separate subscription trees containing copied core packages would create divergent behavior, duplicated fixes, inconsistent security posture, and permanent merge overhead.

Basic must remain a complete deterministic base rather than an implementation detail required by Pro or Enterprise.

## Decision

IATROS maintains one provider-neutral core:

1. Core domain packages exist once under `internal/`.
2. Optional subscription capabilities live under `extensions/pro` or `extensions/enterprise` when a concrete implementation requires them.
3. Extensions integrate through `sdk/extension` or `sdk/plugin` contracts.
4. Basic must build, test, and operate without either extension directory being present.
5. Extensions may compose or add capabilities but do not copy or fork core packages.
6. The core does not import optional extensions.
7. Basic is free and provides local deterministic DevOps architecture generation and open plugin development without AI or a paid entitlement.
8. Pro includes Basic, provides cloud AI for DevOps architecture creation, and supports open plugins.
9. Enterprise includes Pro, provides cloud or local company-controlled AI, and supports closed private plugins in addition to open plugins.
10. Extension packaging and licensing remain undecided until extension code and distribution requirements exist.
11. Pro and Enterprise entitlement is enforced at composition and capability boundaries; provider-neutral domain behavior does not depend on plan names, pricing, or a billing provider.
12. Pro or Enterprise downgrade and expiry return the product to free Basic and preserve direct access to local project artifacts and lifecycle evidence.
13. Subscription expiry denies new plan-gated invocations but does not abandon an already-started external effect before it reaches a verified safe terminal outcome.

## Consequences

### Positive

- Core behavior and fixes have one source of truth.
- Basic receives the same foundational correctness and security improvements.
- Optional capabilities exercise stable extension points instead of private imports.
- Subscription drift is structurally discouraged.
- Downgrade to free Basic preserves Basic behavior, and entitlement loss cannot lock local project artifacts inside IATROS.

### Trade-offs

- Extension contracts require careful design and compatibility tests.
- Some composition and packaging work moves to executable or distribution boundaries.
- Basic-only CI and tests are required once implementation exists.
- Pro and Enterprise distribution, version compatibility, and licensing remain unresolved.
- Entitlement caching, offline Enterprise use, grace periods, and safe expiry require explicit implementation and tests.

## Alternatives considered

### Separate Basic, Pro, and Enterprise source trees

Rejected because duplicated packages would diverge and make fixes difficult to propagate safely.

### Compile-time flags inside core packages

Rejected as the primary extension mechanism because subscription behavior would remain entangled with Basic domain code and would be difficult to reason about or test independently.

### Private imports from extensions into core

Rejected because it reverses dependency direction and makes Basic incomplete without optional code.

## Validation

Future automation should prove:

- Basic builds, passes tests, and runs without a paid entitlement or Pro or Enterprise extensions;
- core packages do not import `extensions/`;
- extensions depend only on published contracts;
- the same core tests run for every supported distribution;
- compatibility is checked between an extension and the contracts it implements.
- expired or invalid entitlement cannot enable a paid capability;
- downgrade or paid subscription expiry preserves free Basic behavior and direct local artifact access;
- subscription expiry and security revocation have distinct, tested behavior.

## References

- [IATROS Product Contract](../../product/product-contract.md)
- [IATROS target architecture](../README.md)
- [ADR-0001: Capability boundaries and dependency direction](0001-capability-boundaries-and-dependency-direction.md)
