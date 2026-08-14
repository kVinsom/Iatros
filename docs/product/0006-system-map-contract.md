# PS-0006: Unified System Map Contract

- **Product status:** Approved
- **Implementation status:** In progress
- **Date:** 2026-08-12
- **Plan:** Basic foundation shared by every plan
- **Scope:** Provider-neutral, evidence-backed representation of one software system
- **Model schema version:** `1.0`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [PS-0002: Stable Core Domain Contracts](0002-core-domain-contracts.md)
- [System map architecture](../architecture/system-map.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Purpose

Repository, source, dependency, and DevOps analysis produce different facts about the same software system.
IATROS needs one normalized map that can represent those facts without collapsing them into an untyped
property bag or coupling the core to a source-control, cloud, or deployment provider.

The implemented stage establishes the private model, normalization, validation, evidence, and resource
contracts. Translators from every analyzer and a user-visible report remain later stages. Until those
translators exist, the CLI does not claim to produce a complete system map.

## 2. System boundary

One map represents one explicitly selected software system and contains separate collections for:

- local and remote source repositories;
- deployable services and reusable libraries scoped to a repository;
- repository-owned infrastructure definitions;
- operational environments;
- explicitly evidenced people, teams, and organizations;
- credential-free external resources outside the system ownership boundary;
- directed relationships between any of those entity kinds;
- bounded diagnostics and an explicit partial-result state.

The model does not treat every package dependency as a system-owned library. An analyzer must have evidence
that the library is part of the selected system. Databases, APIs, queues, clusters, SaaS systems, and similar
dependencies are external resources when ownership is not established by explicit evidence.

## 3. Identity and relationships

Every entity has a stable normalized identifier within its entity kind. Relationship endpoints contain both
the entity kind and identifier, so equal text cannot accidentally join unrelated concepts. A relationship is
directed, has its own identifier and normalized kind, and may be restricted to an ordered set of known
environments.

Services, libraries, and infrastructure identify their source repository. Ownership is represented by an
`owns` relationship from an owner entity rather than a special field repeated on every entity. IATROS records
owners only from explicit declarations such as CODEOWNERS or an approved catalog; it does not infer a person
from commit history or an email address.

## 4. Evidence and privacy

Every entity and relationship requires ordered evidence. Local evidence contains a safe repository-relative
path. Remote evidence contains a sanitized provider reference and no path. Every evidence record names the
known repository from which it originated.

Remote repository locators use a credential-free `host/namespace/repository` form. URL schemes, user
information, query strings, fragments, backslashes, absolute paths, control characters, and escaping paths
are rejected. Remote locators are lowercase, require a sanitized immutable revision and provider evidence,
and cannot repeat one physical source under another repository ID. Local repositories require target-root
evidence. The model has no field for access tokens, passwords, connection strings, resolved environment values,
or provider SDK objects.

Diagnostics identify an optional repository, a safe path, severity, stable code, and message. Warning or error
diagnostics require `partial: true`; a partial result must contain at least one warning or error that explains
the omitted or ambiguous mapping.

## 5. Determinism and validation

Normalization returns detached, non-nil collections; orders every entity collection and relationship by its
identity; and orders and deduplicates evidence and environment references. Validation rejects:

- unsupported schema versions and malformed system or entity identities;
- missing repositories, duplicate or unordered identities, and unknown references;
- provider metadata on a local repository or an unsafe remote locator;
- unsafe paths and evidence that references an unknown repository;
- self-relationships and unknown relationship endpoints;
- duplicate, unordered, or unknown environment references;
- invalid ownership kinds and inconsistent partial outcomes.

## 6. Resource profiles

The shared `small`, `monorepo`, and `enterprise` profiles include bounded counts for every entity collection,
relationships, environments per entity, evidence per fact, diagnostics, retained text, and stage duration.
The limits increase monotonically and represent maximum retained work, not memory reservations.

Reaching a limit must produce an explicit partial result or fail safely. It must never silently omit facts.
Remote or distributed execution cannot weaken the same normalized contract or its limits.

## 7. Acceptance criteria for this contract stage

This stage is complete when:

- every required system concept has a concrete typed collection;
- all cross-entity and repository references are validated;
- normalization is deterministic and does not retain mutable caller-owned collections;
- local paths and remote references cannot carry credentials or escape repository scope;
- all built-in profiles contain valid, monotonic system-map limits;
- success, invalid reference, ordering, safety, partial-result, JSON, and limit tests pass;
- architecture documentation distinguishes the implemented contract from deferred mapping and reporting.

## 8. Deferred implementation stages

Later stages will add bounded translators from repository topology, code analysis, dependency analysis,
DevOps analysis, ownership files, catalogs, and remote-provider metadata. They will define conflict resolution,
incremental refresh, user-visible reports, persistence, and change detection without changing the meaning of
the schema `1.0` entities.
