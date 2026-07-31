# ADR-0001: Capability boundaries and dependency direction

- **Status:** Accepted
- **Date:** 2026-07-31
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS is intended to span project discovery, AI-assisted planning, generation, validation, deployment, observability, security, cost analysis, collaboration, and external integrations. Without explicit boundaries, domain behavior, product entry points, public contracts, and vendor adapters would become mutually coupled.

Coupling private domain types to public wire formats or provider SDKs would make integrations difficult to replace and public contracts difficult to evolve. Generic horizontal folders such as `common`, `shared`, or `utils` would also obscure capability ownership.

## Decision

IATROS uses capability-oriented boundaries with dependencies directed toward provider-neutral behavior:

1. Provider-neutral application and domain logic belongs under `internal/<capability>`.
2. `cmd/*` packages are composition roots. They select and wire concrete implementations but contain no business logic.
3. `api/` owns public wire contracts, not service implementation.
4. `sdk/` owns public client, plugin, extension, and plugin-test contracts.
5. `api/` and `sdk/` do not import `internal/`.
6. Core domains do not import concrete `plugins/` or optional `extensions/`.
7. Plugins and extensions do not import private `internal/` packages.
8. Concrete provider integrations belong under the relevant `plugins/<capability>` category and depend on stable public contracts.
9. Internal interfaces remain close to their consumers; boundary adapters translate between public and private models.
10. Cross-domain dependencies remain acyclic, minimal, and justified by a use case.
11. Global `interfaces`, `models`, `services`, `repositories`, `common`, `shared`, `helpers`, and `utils` packages are not introduced as generic dumping grounds.

## Consequences

### Positive

- Capabilities have clear ownership.
- Vendor integrations can be replaced without rewriting the core.
- Public contracts can be versioned independently from private domain representation.
- Composition and domain behavior remain testable at different layers.
- Architecture dependency checks can enforce the direction once code exists.

### Trade-offs

- Boundary translation code is expected and should not be hidden in shared models.
- Public contracts require compatibility discipline once released.
- Some concepts may have separate public and private representations.
- Cross-capability use cases require deliberate orchestration instead of unrestricted imports.

## Alternatives considered

### Technical-layer folders

Organizing the repository primarily into controllers, services, repositories, and models was rejected because ownership would be spread across unrelated folders and capability changes would touch every layer.

### Provider SDKs inside the core

This was rejected because cloud, source-control, AI, observability, and other vendor dependencies would leak into provider-neutral behavior.

### One shared model for API, plugins, and core

This was rejected because public compatibility and private domain evolution have different lifecycles.

## Validation

When source exists, automated checks should verify:

- forbidden imports between public, private, plugin, and extension areas;
- absence of cycles between internal domains;
- composition logic remains in `cmd/*`;
- every concrete provider has contract and conformance tests;
- no generic shared package accumulates unrelated capability logic.

## References

- [IATROS target architecture](../README.md)
- [Testing strategy](../testing.md)
