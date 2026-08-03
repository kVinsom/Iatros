# IATROS Documentation

> [!IMPORTANT]
> IATROS is in early implementation. A local Cobra-based CLI runs bounded metadata discovery, technology detection, readiness evaluation, and deterministic reporting. The same CLI exposes a separate bounded repository-topology report built from project boundaries and safely parsed manifests.

## Product specifications

- [Product specification index](product/README.md) — approved user-visible contracts and their implementation status.
- [PS-0001: Local repository analysis](product/0001-local-repository-analysis.md) — the first local-only, read-only CLI vertical slice.

## Architecture

- [Manifest analysis architecture](architecture/manifest-analysis.md) - bounded content access, supported formats, normalized facts, parser safety, resource profiles, and backend replacement rules.
- [Repository topology architecture](architecture/topology.md) - project/component association, workspace membership, direct dependency resolution, limits, and partial-state rules.
- [Technology detection architecture](architecture/detection.md) — internal marker catalog, evidence rules, categories, limits, and extension requirements.
- [Project and workspace boundary model](architecture/project-model.md) — internal monorepository boundaries, strong markers, workspace membership, limits, and deferred enrichment.
- [Repository readiness architecture](architecture/readiness.md) — internal absence rules, suppression policy, supported test evidence, and deferred checks.
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
| **Implemented** | Verified behavior present in source and tests. Currently includes CLI local-analysis and repository-topology workflows plus their internal project, manifest, and topology pipelines. |

## Documentation rules

1. Describe planned behavior in future or target language.
2. Do not present an empty directory as an implemented feature.
3. Record consequential and durable choices as architecture decision records.
4. Keep uncertain protocols, vendors, deployment topology, and storage choices explicitly marked as TBD.
5. Update architecture documentation and tests together with implementation changes that affect system boundaries.
