# PS-0002: Stable Core Domain Contracts

- **Status:** Implemented
- **Date:** 2026-08-03
- **Plan:** Basic foundation shared by every plan
- **Scope:** Private provider-neutral core models
- **Core schema version:** `1.0`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [IATROS target architecture](../architecture/README.md)
- [ADR-0001: Capability boundaries and dependency direction](../architecture/decisions/0001-capability-boundaries-and-dependency-direction.md)
- [ADR-0006: Version private core contracts independently from public APIs](../architecture/decisions/0006-version-private-core-contracts.md)

## 1. Purpose

This specification establishes the first stable provider-neutral contracts for project topology and lifecycle results. They give future Analyze, Plan, Generate, Validate, Deploy, Monitor, and Fix implementations one vocabulary without coupling the core to a CLI, provider SDK, plugin, subscription, storage engine, or public transport.

The contracts are implemented as private Go packages. Their JSON field names support deterministic persistence and tests inside IATROS but are not a released public API. A future API or SDK must define and version its own wire schema and translate at the boundary.

## 2. Ownership

| Contract | Owner | Current schema |
| --- | --- | --- |
| Schema version parsing and compatibility | `internal/schema` | Not applicable |
| Project, environment, service, dependency, and configuration model | `internal/project` | `1.0` |
| Common lifecycle-stage result envelope | `internal/workflow` | `1.0` |
| Local repository analysis report | `internal/analysis` | `0.1`, unchanged by this specification |

These packages are capability owners, not a generic shared-model layer. Feature-specific evidence, plans, candidates, validation details, effects, signals, and remediation data remain with the feature that owns their meaning and are carried as typed workflow result data.

## 3. Project model

`Project` is the aggregate root. It contains:

- a schema version, stable project identifier, and display name;
- project-wide configuration defaults;
- ordered environments;
- ordered services and their environment bindings;
- ordered directed dependency edges.

Identifiers owned by a project use lowercase letters, digits, `.`, `_`, and `-`, cannot start or end with a separator, and cannot contain adjacent separators. Display names remain separate and may use human-readable Unicode text.

Collections are strictly ordered by their documented identity. Duplicate identifiers, keys, bindings, references, and dependency scopes are invalid. `Normalized` creates detached slices, initializes empty collections, and applies canonical ordering before validation or persistence.

## 4. Environments and services

An `Environment` has an ID, display name, provider-neutral kind, and configuration. Common kinds may include `development`, `test`, `staging`, and `production`, but the contract accepts any valid provider-neutral identifier so later capabilities do not require a schema change for every organization-specific environment class.

A `Service` has an ID, display name, provider-neutral kind, optional normalized project-relative source path, base configuration, and ordered environment bindings. A service environment binding references an existing environment and carries only that service's overrides for that environment.

The model does not equate one service with one process, container, deployment, repository, or provider resource. A capability translates its provider-specific topology into this contract and retains detailed evidence in its own result data.

## 5. Dependencies

A `Dependency` is a stable directed edge with:

- its own ID;
- a source and target resource reference;
- a provider-neutral relationship kind;
- required or optional semantics;
- an optional ordered environment scope.

Project, environment, and service references must resolve inside the aggregate. External references may use opaque provider or package identities but remain plain data and grant no network, credential, plugin, or effect authority. Self-dependencies and unknown internal references are invalid.

An empty environment scope means that the relationship applies wherever both endpoints are applicable. A non-empty scope must reference existing environments.

## 6. Configuration

Configuration is represented as ordered entries instead of a map so serialized results, diffs, tests, and hashes are deterministic. An entry declares one of four sources:

| Source | Stored data | Safety rule |
| --- | --- | --- |
| `literal` | A non-sensitive literal value. | Sensitive literals are rejected. |
| `environment` | An environment-variable name. | No resolved value is retained. |
| `file` | A normalized project-relative path. | Absolute and escaping paths are rejected. |
| `secret` | An opaque secret-manager reference. | It must be marked sensitive; secret material is never stored. |

Configuration precedence from lowest to highest is:

1. project configuration;
2. environment configuration;
3. service configuration;
4. service-environment configuration.

Resolution is a future use-case responsibility. The core model records declarations and precedence inputs but does not read environment variables, files, or secret providers during validation.

## 7. Workflow result

Every completed stage invocation produces an immutable `workflow.Result[T]` envelope containing:

- schema, result, and workflow-run identities;
- one canonical lifecycle stage;
- one canonical terminal outcome;
- UTC start and finish timestamps;
- ordered immutable input and output references;
- ordered diagnostics;
- feature-owned typed data.

The supported stages are `analyze`, `plan`, `generate`, `validate`, `deploy`, `monitor`, and `fix`. The supported outcomes are `completed`, `partial`, `blocked`, `unavailable`, `failed`, `cancelled`, `denied`, `rolled_back`, `indeterminate`, and `skipped`.

Every non-`completed` outcome requires an explanatory diagnostic. A `completed` result cannot contain an error-level diagnostic. Artifact references may carry their own schema version and a canonical algorithm-prefixed lowercase hexadecimal digest.

Envelope validation does not validate `T`. The capability that owns the typed result data validates it before the complete result is accepted.

## 8. Schema versioning

Schema versions use canonical `major.minor` notation:

- a major version changes when a field is removed, renamed, changes meaning, becomes more restrictive, or otherwise requires migration;
- a minor version changes for backward-compatible optional data or semantics that an older document does not require;
- a stable reader accepts the same major version at its own or an older minor version;
- versions below `1.0` require an exact `major.minor` match;
- leading zeros, labels, patch components, signs, whitespace, and values above `65535` are invalid;
- stored data retains its original schema version and is never silently rewritten during validation;
- a migration must be explicit, tested, preserve provenance, and produce a newly validated value.

The current project and workflow contracts start at `1.0`. The analysis report remains at `0.1` because changing its existing CLI envelope is outside this specification.

## 9. Conformance

The implementation conforms when tests prove that:

- valid empty and populated projects normalize and validate;
- duplicate, unsorted, malformed, escaping, unknown, and unsafe references are rejected;
- raw sensitive configuration cannot be represented as a literal or resolved secret value;
- every lifecycle stage and terminal outcome uses one canonical identifier;
- non-completed outcomes remain explainable;
- normalization does not retain mutable collection storage from its input;
- stable and pre-`1.0` schema compatibility rules are enforced;
- the current analysis CLI contract remains unchanged.

## 10. Deferred public contracts

This specification does not publish an API, SDK, plugin schema, database schema, or network protocol. Those contracts require a concrete consumer, independent compatibility review, security review, conformance tests, and boundary translation. They must not expose `internal/` packages directly.
