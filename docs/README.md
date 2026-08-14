# IATROS Documentation

> [!IMPORTANT]
> IATROS is in early implementation. A local Cobra-based CLI runs bounded Git-style ignore handling, submodule isolation, metadata discovery, technology detection, readiness evaluation, and deterministic reporting. The same CLI exposes a separate bounded repository-topology report built from project boundaries and safely parsed manifests. Both workflows support explicit `small` and `monorepo` scaling profiles.

## Product specifications

- [Product specification index](product/README.md) — approved user-visible contracts and their implementation status.
- [IATROS Product Contract](product/product-contract.md) — canonical product boundaries, lifecycle, invariants, and subscription responsibilities.
- [PS-0001: Local repository analysis](product/0001-local-repository-analysis.md) — the first local-only, read-only CLI vertical slice.
- [PS-0002: Stable core domain contracts](product/0002-core-domain-contracts.md) — implemented private models and schema-evolution rules shared by future workflows.
- [PS-0003: Static code analysis contract](product/0003-static-code-analysis-contract.md) - normalized local source-analysis facts and resource boundaries.
- [PS-0004: Dependency analysis contract](product/0004-dependency-analysis-contract.md) - approved local lock, build, version, graph, conflict, and license behavior.
- [PS-0005: DevOps stack analysis contract](product/0005-devops-stack-analysis-contract.md) - normalized local Docker, Kubernetes, Helm, Terraform, delivery, GitOps, observability, and security facts.
- [PS-0006: Unified system map contract](product/0006-system-map-contract.md) - the evidence-backed aggregate for services, libraries, infrastructure, environments, owners, resources, and relationships.
- [PS-0007: Remote and polyrepository analysis contract](product/0007-remote-polyrepo-analysis-contract.md) - normalized provider metadata and bounded multi-repository composition.
- [PS-0008: Change impact analysis contract](product/0008-change-impact-analysis-contract.md) - deterministic services, environments, and configurations affected by normalized changes.
- [PS-0009: Unified findings contract](product/0009-unified-findings-contract.md) - required assessment dimensions and controlled exclusion behavior.

## Architecture

- [System map architecture](architecture/system-map.md) - the provider-neutral aggregate model, identity, evidence, relationship, safety, and scaling rules.
- [Remote and polyrepository analysis architecture](architecture/remote-analysis.md) - provider boundaries, immutable repository identity, polyrepository graphs, credentials, safety, and resource budgets.
- [Change impact analysis architecture](architecture/change-impact.md) - direct matching, reverse dependency propagation, causes, limits, and partial-result behavior.
- [Unified findings architecture](architecture/findings.md) - shared assessment, evidence, provenance, risk, recommendations, and controlled exclusions.
- [Manifest analysis architecture](architecture/manifest-analysis.md) - bounded content access, supported formats, normalized facts, parser safety, resource profiles, and backend replacement rules.
- [DevOps stack analysis architecture](architecture/devops-analysis.md) - provider-neutral configuration facts, parser and correlation boundaries, limits, privacy, and staged format coverage.
- [Repository topology architecture](architecture/topology.md) - project/component association, workspace membership, direct dependency resolution, limits, and partial-state rules.
- [Repository discovery architecture](architecture/repository-discovery.md) - bounded Git-style ignore rules, large-file behavior, nested repositories, and submodule isolation.
- [Scaling profiles](architecture/scaling-profiles.md) - unified small-repository, monorepo, and Enterprise per-worker budgets plus selection and validation rules.
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
