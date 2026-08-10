# PS-0003: Static Code Analysis Contract

- **Product status:** Approved
- **Implementation status:** In progress
- **Date:** 2026-08-10
- **Plan:** Basic foundation shared by every plan
- **Scope:** Local, deterministic, evidence-backed source analysis
- **Model schema version:** `1.0`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [PS-0001: Local Repository Analysis](0001-local-repository-analysis.md)
- [PS-0002: Stable Core Domain Contracts](0002-core-domain-contracts.md)
- [Static code analysis architecture](../architecture/code-analysis.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Purpose

IATROS must statically recognize frameworks, executable services, ports, protocols, API operations,
environment-variable usage, databases, caches, and message brokers. The result must remain useful for a
small repository and a large company monorepository without executing repository code or introducing a
language-specific model into the core.

This specification establishes the normalized result and resource contracts. Language and configuration
analyzers will be added incrementally and must emit this contract. The current implementation does not yet
claim that any language analyzer is connected to the CLI.

## 2. Normalized facts

`internal/codeanalysis.Model` schema `1.0` owns separate collections for:

- services and their repository roots, with every entrypoint confined to its service root;
- frameworks associated with a service;
- literal or named port bindings, explicit reference kinds, and normalized protocols;
- API endpoints and operations;
- environment-variable metadata without resolved or default values;
- database, cache, and message-broker dependencies without connection strings;
- bounded diagnostics explaining ambiguity, unsupported syntax, failed files, and reached limits.

Every detected fact references a known service and carries one or more ordered evidence locations. Facts
are either `observed` directly or `inferred` deterministically from multiple observations. Inference is not
permission to guess: an analyzer reports a diagnostic when dynamic behavior cannot be resolved safely.

## 3. Evidence and privacy

Evidence contains a safe repository-relative file path, a normalized evidence kind, and either a complete
one-based inclusive source span or no position for whole-file evidence. Absolute paths, escaping paths, invalid UTF-8,
control characters, malformed spans, unresolved service references, duplicate fact identities, and
non-deterministic ordering are rejected.

The contract records environment-variable names, `has_default`, and sensitivity metadata but has no field
for the default or resolved value. Resource dependencies contain a category, technology identifier, and
optional logical name but no credential or connection-string field. Analyzer implementations must redact or
omit sensitive expressions before constructing the model.

## 4. Resource profiles

Static analysis is governed by validated limits for:

- source files, bytes per file, total bytes, and bytes per line;
- parser nesting and syntax nodes per file;
- retained services, frameworks, ports, endpoints, environment variables, and resource dependencies;
- evidence per fact, diagnostics, retained text, and total stage duration.

The `small`, `monorepo`, and `enterprise` profiles increase these capacities monotonically. A code-analysis
profile cannot read more files than discovery retained. Implementations must allocate from observed work,
not reserve configured maximums in advance.

## 5. Safety boundary

Static code analysis:

- reads only bounded files already admitted by repository discovery;
- never executes project code, build tools, package managers, generators, or scripts;
- performs no network access and installs no dependencies;
- resolves no environment, secret-manager, or infrastructure credentials;
- honors ignore rules, nested-repository boundaries, cancellation, and stage timeout;
- returns deterministic normalized facts or an explicit partial result.

Parser backends may use the Go standard library or a maintained third-party parser when language fidelity
requires it. Every backend must preserve the same normalized contract, limits, cancellation, determinism,
privacy rules, cross-platform behavior, diagnostics, and conformance tests.

## 6. Acceptance criteria for this contract stage

This stage is complete when:

- every required fact category has a concrete model rather than a generic property bag;
- validation rejects unsafe or cross-service paths, malformed spans, unknown services, invalid identities,
  contradictory environment-variable metadata, malformed port references, and inconsistent partial outcomes;
- normalization initializes, detaches, orders, and deduplicates nested evidence and entrypoint collections;
- validation against limits rejects excessive collections, evidence, and retained text;
- every built-in scaling profile includes valid monotonic code-analysis budgets;
- JSON round-trip and negative contract tests pass;
- documentation clearly distinguishes the implemented contract from deferred language analyzers and CLI
  integration.

## 7. Deferred implementation stages

The next stages add bounded source access, a consumer-owned analyzer registry, and vertical analyzers for Go,
JavaScript and TypeScript, Python, Java and Kotlin, .NET, PHP, Ruby, and Rust. Configuration and
infrastructure analyzers then correlate Docker Compose, Kubernetes, and infrastructure-as-code declarations
with the same service and resource facts. Aggregation into a versioned CLI report is a separate contract.
