# ADR-0006: Use explicit unified scaling profiles

- **Status:** Accepted
- **Date:** 2026-08-03
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS analyzes repositories through multiple bounded stages. Independent default and large-repository limit functions already existed for some stages, but no single selection configured discovery, detection, project boundaries, manifests, and topology together. A caller could increase one stage and still receive downstream truncation governed by an unrelated conservative default.

Automatically detecting repository size and silently increasing limits would make resource use unpredictable. Treating Enterprise installation capacity as only a larger file count would also ignore tenancy, admission, queueing, isolation, and worker-fleet concerns that do not yet have a runtime.

## Decision

1. `internal/analysis` owns a complete `ScalingProfile` composition contract over all implemented local-analysis stages.
2. The canonical names are `small`, `monorepo`, and `enterprise`.
3. `small` remains the conservative default, and `monorepo` requires explicit selection.
4. `enterprise` is implemented as a validated per-repository, per-worker starting profile. Its availability is chosen by an entitlement-aware composition rather than by provider-neutral domain branches.
5. The default local CLI enables `small` and `monorepo` and rejects `enterprise` before invoking a service.
6. Profiles are explicit; the pipeline does not auto-promote an invocation after inspecting its input.
7. Profile validation covers every component limit and cross-component retention invariants.
8. Profile-aware analyzers prebuild only explicitly enabled pipelines and reject empty or duplicate profile sets.
9. Analysis and topology report schemas advance from `0.1` to `0.2` and include the active profile in text and JSON.
10. A reached limit produces explicit partial or failed semantics; larger profiles do not weaken read-only, path-confinement, cancellation, no-execution, or no-network rules.
11. Distributed Enterprise installation scaling requires a separate specification and does not become implemented merely because the per-worker profile exists.

## Consequences

### Positive

- One selection configures the entire implemented pipeline coherently.
- Results identify the budget under which they were produced.
- Small repositories retain conservative defaults.
- Large monorepositories have an explicit runnable mode.
- Enterprise composition can enable a tested high-capacity worker profile without forking core behavior.
- Future backends can be benchmarked against named, reproducible budgets.

### Trade-offs

- Adding the profile field requires a report schema version change.
- Large profiles can consume substantially more time and memory and therefore remain explicit.
- Built-in numerical values require release measurements and may evolve.
- The Enterprise profile does not solve distributed installation concerns.

## Alternatives considered

### Independent stage profiles

This was rejected because incompatible selections can silently truncate downstream facts and make results difficult to reproduce.

### Automatic profile selection

This was rejected because repository shape is not captured by one size threshold and automatic promotion makes resource consumption unpredictable.

### Unbounded Enterprise mode

This was rejected because every workload must retain explicit file, byte, collection, issue, and time budgets.

### Expose Enterprise in the default local CLI

This was rejected because profile availability is a composition and entitlement decision. Provider-neutral core behavior supports the profile without making the default distribution claim Enterprise activation.

## Validation

- Contract tests validate all built-in profiles and their ordering.
- Constructor tests reject malformed, empty, and duplicate profile configurations.
- Pipeline tests run enabled small, monorepo, and Enterprise profiles against local repositories.
- CLI tests prove Basic profile selection and pre-invocation Enterprise rejection.
- Report tests reject missing or unknown profile metadata.
- Full repository tests, vet, and build remain required.

## References

- [Scaling profiles](../scaling-profiles.md)
- [Target architecture](../README.md)
- [Manifest analysis architecture](../manifest-analysis.md)
- [Repository topology architecture](../topology.md)
