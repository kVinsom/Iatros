# IATROS Scaling Profiles

> **Status: implemented for local per-repository analysis, system-map contracts, and remote-analysis contracts.** `small` and `monorepo` are selectable in the local CLI. `enterprise` is a validated per-worker profile that an Enterprise composition may enable; the default local composition does not enable it. Remote adapters, distributed control-plane behavior, and worker-fleet scaling remain target behavior.

See also:

- [Target architecture](README.md)
- [Project and workspace boundary model](project-model.md)
- [Manifest analysis architecture](manifest-analysis.md)
- [Repository topology architecture](topology.md)
- [ADR-0006: Use explicit unified scaling profiles](decisions/0006-use-explicit-unified-scaling-profiles.md)

## 1. Purpose

Repository size affects every analysis stage. Increasing only filesystem discovery while leaving manifest, project, or topology retention at conservative values creates a result that appears complete but is silently truncated later. IATROS therefore selects one validated profile that configures the complete local pipeline:

1. filesystem discovery;
2. technology evidence retention;
3. project and workspace boundary evidence;
4. manifest reads, parsing, and retained declarations;
5. topology association and retained relationships;
6. static source analysis and retained code facts.
7. normalized system-map entities, relationships, evidence, and diagnostics.
8. remote provider requests, response bytes, repositories, relationships, evidence, and diagnostics.

Profiles are bounded resource budgets, not claims that every repository below a file count will complete. Repository shape, directory fan-out, manifest size, dependency density, storage performance, and host capacity also affect the result.

## 2. Profile contract

| Profile | Intended workload | Availability |
| --- | --- | --- |
| `small` | Small and typical standalone repositories; conservative CPU, memory, retained evidence, and time budgets. | Default local CLI profile. |
| `monorepo` | Large multi-project repositories with many manifests, workspaces, and dependency edges. | Explicit local CLI opt-in. |
| `enterprise` | A high-capacity per-worker starting point for measured company repositories. | Implemented and tested in the core; activation belongs to an Enterprise-aware composition. |

Selection is explicit and deterministic. IATROS does not auto-promote a job after observing repository size because a silent promotion could unexpectedly increase memory, I/O, and runtime. Re-running with a larger profile is a new explicit invocation.

An Enterprise product entitlement does not itself grant filesystem, provider, network, or secret authority. It may make the profile available to a composition, while the normal target scope and safety checks remain unchanged.

## 3. Complete resource matrix

### 3.1 Discovery and evidence

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Files retained | 2,000 | 100,000 | 500,000 |
| Directories retained | 500 | 25,000 | 100,000 |
| Directory depth | 20 | 40 | 64 |
| Discovery issues retained | 50 | 500 | 2,000 |
| Entries per directory | 2,500 | 20,000 | 50,000 |
| Ignore files | 100 | 2,000 | 10,000 |
| Bytes per ignore/control file | 256 KiB | 1 MiB | 4 MiB |
| Bytes per ignore pattern | 4 KiB | 32 KiB | 64 KiB |
| Ignore rules | 10,000 | 100,000 | 500,000 |
| Nested repository boundaries | 100 | 5,000 | 25,000 |
| Discovery timeout | 5 seconds | 60 seconds | 5 minutes |
| Evidence paths per technology | 20 | 100 | 500 |
| Evidence paths per project marker | 20 | 100 | 500 |

### 3.2 Manifest analysis

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Manifest files | 100 | 2,000 | 10,000 |
| Bytes per manifest | 256 KiB | 8 MiB | 16 MiB |
| Total manifest bytes | 4 MiB | 256 MiB | 1 GiB |
| Dependencies per manifest | 1,000 | 10,000 | 50,000 |
| Constraints per manifest | 100 | 1,000 | 5,000 |
| Workspace members per manifest | 500 | 5,000 | 25,000 |
| Workspace exclusions per manifest | 500 | 5,000 | 25,000 |
| Manifest issues retained | 50 | 200 | 1,000 |
| Parser nesting depth | 64 | 128 | 256 |
| Bytes per retained value | 4 KiB | 64 KiB | 256 KiB |
| Manifest timeout | 3 seconds | 30 seconds | 2 minutes |

### 3.3 Topology association

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Projects | 2,000 | 100,000 | 500,000 |
| Workspaces | 500 | 10,000 | 50,000 |
| Manifests | 100 | 2,000 | 10,000 |
| Components per boundary | 20 | 100 | 500 |
| Workspace declarations | 2,000 | 100,000 | 1,000,000 |
| Matches per declaration | 500 | 10,000 | 50,000 |
| Dependencies | 20,000 | 1,000,000 | 5,000,000 |
| Targets per dependency | 20 | 100 | 500 |
| Nested repository boundaries | 100 | 5,000 | 25,000 |
| Topology issues retained | 100 | 1,000 | 5,000 |
| Bytes per retained value | 4 KiB | 64 KiB | 256 KiB |
| Topology timeout | 3 seconds | 60 seconds | 5 minutes |

### 3.4 Static code analysis

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Source files | 1,000 | 50,000 | 250,000 |
| Bytes per source file | 1 MiB | 4 MiB | 8 MiB |
| Total source bytes | 64 MiB | 2 GiB | 16 GiB |
| Bytes per source line | 64 KiB | 256 KiB | 1 MiB |
| Parser nesting depth | 256 | 512 | 1,024 |
| Syntax nodes per file | 250,000 | 1,000,000 | 4,000,000 |
| Services | 500 | 25,000 | 100,000 |
| Frameworks | 500 | 25,000 | 100,000 |
| Port bindings | 2,000 | 100,000 | 500,000 |
| API endpoints | 10,000 | 500,000 | 5,000,000 |
| Environment variables | 5,000 | 250,000 | 1,000,000 |
| Resource dependencies | 5,000 | 250,000 | 1,000,000 |
| Evidence locations per fact | 20 | 100 | 500 |
| Diagnostics | 100 | 2,000 | 10,000 |
| Bytes per retained text field | 8 KiB | 64 KiB | 256 KiB |
| Static analysis timeout | 10 seconds | 2 minutes | 10 minutes |

### 3.5 Unified system map

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Repositories | 200 | 10,000 | 50,000 |
| Services | 1,000 | 50,000 | 250,000 |
| Libraries | 2,000 | 100,000 | 500,000 |
| Infrastructure entities | 2,000 | 250,000 | 1,000,000 |
| Environments | 100 | 2,000 | 10,000 |
| Owners | 500 | 25,000 | 100,000 |
| External resources | 2,000 | 250,000 | 1,000,000 |
| Relationships | 10,000 | 1,000,000 | 5,000,000 |
| Environments per entity | 100 | 2,000 | 10,000 |
| Evidence records per fact | 20 | 100 | 500 |
| Diagnostics | 100 | 2,000 | 10,000 |
| Bytes per retained text field | 16 KiB | 64 KiB | 256 KiB |
| System-map stage timeout | 10 seconds | 2 minutes | 10 minutes |

### 3.6 Remote and polyrepository analysis

| Limit | `small` | `monorepo` | `enterprise` |
| --- | ---: | ---: | ---: |
| Repositories | 100 | 5,000 | 25,000 |
| Repository relationships | 1,000 | 250,000 | 2,000,000 |
| Provider pages | 20 | 500 | 5,000 |
| Repositories per page | 100 | 100 | 100 |
| Provider requests | 500 | 20,000 | 250,000 |
| Concurrent provider requests | 4 | 16 | 32 |
| Bytes per response | 4 MiB | 8 MiB | 16 MiB |
| Total response bytes | 64 MiB | 2 GiB | 16 GiB |
| Evidence records per fact | 20 | 100 | 500 |
| Diagnostics | 100 | 2,000 | 10,000 |
| Bytes per retained text field | 16 KiB | 64 KiB | 256 KiB |
| Remote-analysis timeout | 1 minute | 15 minutes | 1 hour |

These values are release starting points, not immutable product ceilings. A later release may change values without changing the semantic meaning of a profile, but every shipped set must remain validated and covered by representative performance measurements.

## 4. Validation invariants

Every profile is rejected during composition unless:

- its name is one of the three canonical identifiers;
- every component-specific limit is positive and internally valid;
- manifest file retention does not exceed discovered file retention;
- static code-analysis file retention does not exceed discovered file retention;
- topology can retain at least every analyzed manifest;
- topology can retain at least every discovered nested repository boundary;
- topology value retention is not smaller than manifest value retention;
- an enabled profile name appears only once in a composition;
- at least one profile is enabled.

Built-in contract tests also prove that primary capacities increase strictly from `small` to `monorepo` to `enterprise`.

## 5. CLI contract

The default local CLI enables `small` and `monorepo`:

```text
iatros analyze [path] [--format text|json] [--profile small|monorepo]
iatros topology [path] [--format text|json] [--profile small|monorepo]
```

`small` is the default. An unknown, differently cased, or unavailable profile is rejected before repository analysis begins. The default CLI rejects `enterprise`; an Enterprise-aware composition must explicitly enable it after entitlement and installation policy checks.

Both report schemas are version `0.3` and include the canonical `profile` field. Analysis summaries expose the skipped nested-repository count; topology additionally exposes their safe root-relative paths. Text reports print the same information. This makes resource selection and deliberate repository boundaries reviewable.

## 6. Limit outcomes

Reaching a safe limit does not cause silent omission:

- recoverable discovery, parsing, and association limits produce `partial` data and bounded diagnostics;
- a stage that cannot produce a trustworthy bounded result fails safely;
- collection truncation remains explicit on fields that support retained prefixes;
- cancellation and stage timeouts are checked independently;
- increasing a profile never permits network access, target writes, code execution, dependency installation, unrestricted file reads, or secret resolution.

The profile selects maximum work. Implementations should not reserve the complete maximum in advance and should retain only bounded data actually encountered.

## 7. Enterprise installation boundary

The implemented `enterprise` values configure one local analysis worker for one explicitly selected repository. They do not define or claim an implemented control plane, queue, multi-tenant scheduler, shared cache, distributed index, autoscaler, admission controller, or worker fleet.

A future distributed Enterprise specification must separately define:

- tenant and repository isolation;
- admission, quotas, queue capacity, fairness, and backpressure;
- worker CPU, memory, disk, and concurrency classes;
- cancellation, retry, lease, idempotency, and recovery semantics;
- cache ownership, invalidation, encryption, and cross-tenant isolation;
- horizontal scaling signals and safe maximums;
- telemetry, service objectives, capacity planning, and cost controls.

Those installation-level controls may schedule workers that use this per-repository profile, but they cannot weaken its path, byte, time, cancellation, diagnostic, and no-network safety rules.

## 8. Replaceable implementations

Measured monorepo or Enterprise workloads may justify streaming, generated, indexed, SIMD-assisted, or third-party implementations. A replacement must use the same consumer-owned interfaces, accept the active limits, preserve normalized results and diagnostics, pass the shared conformance suite, and receive license, maintenance, vulnerability, cross-platform, determinism, and resource-use review.

Profile selection never selects an implementation by itself. Backend selection remains a separate composition decision supported by measurement.

## 9. Verification

Tests cover:

- all built-in values and monotonic primary capacities;
- invalid component and cross-component limits;
- empty and duplicate enabled profile sets;
- selection of every enabled profile by both local pipelines;
- a safe unavailable result for a disabled Enterprise analysis profile;
- rejection of Enterprise by the default CLI before a service is invoked;
- text and JSON profile reporting;
- schema validation for unknown profile names;
- existing discovery, parser, topology, code-analysis contract, readiness, cancellation, and safety behavior under the default profile.
