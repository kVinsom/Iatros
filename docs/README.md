# IATROS Documentation

> [!IMPORTANT]
> IATROS is in early implementation. The MVP uses the free local Basic plan without AI; its current user-facing slice is read-only repository analysis. A local Cobra-based CLI contract stub is runnable, while bounded filesystem discovery, broad filename-based technology detection, and stable private project and workflow contracts exist internally without CLI integration. Deterministic architecture generation and broader target capabilities remain unimplemented.

## Product specifications

- [Product specification index](product/README.md) — approved user-visible contracts and their implementation status.
- [IATROS Product Contract](product/product-contract.md) — canonical product boundaries, lifecycle, invariants, and subscription responsibilities.
- [PS-0001: Local repository analysis](product/0001-local-repository-analysis.md) — the first local-only, read-only CLI vertical slice.
- [PS-0002: Stable core domain contracts](product/0002-core-domain-contracts.md) — implemented private models and schema-evolution rules shared by future workflows.

## Architecture

- [Technology detection architecture](architecture/detection.md) — internal marker catalog, evidence rules, categories, limits, and extension requirements.
- [Target architecture](architecture/README.md) — system context, component boundaries, domain map, dependency rules, planned runtime roles, conceptual flows, and open decisions.
- [Security architecture](architecture/security.md) — trust boundaries, controlled effects, identity, secrets, plugin and AI isolation, auditability, and resilience requirements.
- [Testing strategy](architecture/testing.md) — test layers, contract verification, AI evaluation, security cases, and future quality gates.
- [Architecture decision records](architecture/decisions/README.md) — accepted and proposed decisions with their context and consequences.

## Document status

Architecture documents use the following labels:

| Label | Meaning |
| --- | --- |
| **Established** | A boundary or rule already represented by the canonical repository structure. |
| **Target** | Intended behavior or responsibility that has not been implemented yet. |
| **Proposed** | A concrete direction that still requires an accepted decision. |
| **TBD** | An intentionally open choice that must not be silently assumed. |
| **Implemented** | Verified behavior present in source and tests. Currently limited to the CLI contract stub, internal bounded discovery and technology detection documented in PS-0001, and private core contracts documented in PS-0002. |

## Documentation rules

1. Describe planned behavior in future or target language.
2. Do not present an empty directory as an implemented feature.
3. Record consequential and durable choices as architecture decision records.
4. Keep uncertain protocols, vendors, deployment topology, and storage choices explicitly marked as TBD.
5. Update architecture documentation and tests together with implementation changes that affect system boundaries.
