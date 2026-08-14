# ADR-0008: Use bounded replaceable artifact parsers

- **Status:** Accepted
- **Date:** 2026-08-14
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS must validate generated and existing DevOps artifacts without executing them or allowing a parser to expand
the repository scope. JSON and XML have suitable streaming support in the Go standard library, while production
YAML and HCL syntax contain enough edge cases that maintaining project-specific parsers would add correctness and
security risk.

Parser choice must not become part of the normalized validation contract. Small repositories, monorepositories,
and Enterprise workers also require explicit byte, depth, node, alias, token, diagnostic, and time limits rather
than parser-specific unbounded behavior.

## Decision

1. `internal/artifactvalidation` owns the consumer-facing validator contract and normalized diagnostic model.
2. JSON and XML use bounded standard-library streaming decoders.
3. YAML uses the maintained YAML Organization v3 Go implementation, with IATROS-owned cumulative document, node,
   depth, alias, and diagnostic limits.
4. HCL uses HashiCorp HCL v2 syntax parsing after IATROS has bounded and read the input.
5. Dockerfile validation remains a conservative IATROS-owned syntax baseline until a specialized backend is
   justified by compatibility tests and benchmarks.
6. Parser libraries receive in-memory bounded content and never open paths, access the network, execute commands,
   or mutate artifacts.
7. Every replacement or specialized backend must preserve cancellation, deterministic ordering, safety limits,
   normalized provenance, diagnostic semantics, and cross-platform behavior.
8. Dependency versions remain explicit in the Go module. Release maintenance includes license, vulnerability, and
   compatibility review for parser dependencies.

## Consequences

### Positive

- YAML and HCL behavior follows maintained language implementations instead of incomplete project-specific parsers.
- A stable IATROS contract isolates callers from parser-specific diagnostics and types.
- Generated and repository-owned artifacts use the same validation path.
- Bounded source access and parser-specific limits keep resource use explicit across scaling profiles.
- Specialized schema and policy validators can be added without replacing the engine or report model.

### Trade-offs

- YAML and HCL add third-party dependencies that require maintenance and release review.
- AST-based parsers can allocate relative to bounded input, so byte limits remain the first safety boundary.
- The built-in Dockerfile validator intentionally reports some unknown instructions as warnings instead of claiming
  complete Docker build compatibility.
- Provider dry runs and external binaries require a separate controlled-execution capability and are not validators
  hidden behind this interface.

## Alternatives considered

### Implement YAML and HCL parsers in IATROS

This was rejected because the formats have complex syntax and compatibility behavior. A custom implementation would
increase maintenance cost and create avoidable correctness and security risk.

### Expose parser-native results

This was rejected because downstream Doctor, workflow, CLI, and future API consumers require one stable diagnostic
contract independent of a selected backend.

### Invoke provider tools during ordinary validation

This was rejected because process execution, provider credentials, network access, and state changes require a
separate trust boundary and explicit user control.

### Use one parser library for every format

This was rejected because no single dependency provides the best fidelity and resource controls for all supported
formats, and it would unnecessarily broaden the dependency surface.

## Validation

- Contract tests exercise syntax success, malformed input, cancellation, size, depth, node, alias, and diagnostic
  limits.
- Structure tests verify Compose and Kubernetes semantics without bypassing syntax limits.
- Registry tests verify stable validator descriptors and reject malformed or duplicate registrations.
- Source tests verify path confinement, regular-file identity, symlink rejection, close behavior, and detached
  generated content.
- Full repository tests, vet, and build remain required.

## References

- [Artifact validation architecture](../artifact-validation.md)
- [Doctor and validation product contract](../../product/0010-doctor-validation-contract.md)
- [Scaling profiles](../scaling-profiles.md)
- [Security architecture](../security.md)
