# IATROS Testing Strategy

> [!IMPORTANT]
> **Status: target strategy with unit and local integration baselines.** Package tests cover unified scaling profiles, bounded Git-style ignore rules, submodule and nested-worktree isolation, large-file behavior, local discovery, broad filename-based technology detection, project/workspace boundary modeling, conservative readiness evaluation, manifest parsing, topology association, report validation, and the Cobra CLI. Cross-package tests exercise both real local analyzers through JSON CLI output. CI, external end-to-end, dedicated security, and resilience suites are not implemented yet.

See also:

- [IATROS Product Contract](../product/product-contract.md)
- [Target architecture](README.md)
- [Security architecture](security.md)
- [Scaling profiles](scaling-profiles.md)
- [Repository discovery architecture](repository-discovery.md)
- [Architecture decision records](decisions/README.md)

## 1. Objectives

Testing must provide evidence that:

- domain behavior is correct and deterministic where required;
- public API, SDK, plugin, and extension contracts remain compatible;
- provider adapters normalize external behavior without leaking vendor details into the core;
- state-changing work obeys policy, approval, scope, idempotency, verification, and audit requirements;
- AI-assisted behavior remains bounded by deterministic controls;
- failures, retries, cancellation, and recovery produce explicit outcomes;
- Basic remains functional without Pro, Enterprise, AI, or closed plugins.
- Basic performs no AI or hosted-service request and does not access a plugin destination until an explicit capability is invoked.
- Pro sends only explicitly permitted context across the cloud AI boundary.
- Enterprise local AI has no undeclared cloud egress, and closed plugins remain isolated to entitled company environments.

Test quantity is not a substitute for testing the correct boundary.

### Current baseline

- `internal/repositoryignore` tests cover root and nested scope, last-match precedence, negation, anchored and directory-only rules, escapes, character patterns, all defined globstar positions, malformed input, and rule bounds.
- `internal/analysis` tests cover all three unified profile contracts, enabled and unavailable profile selection, bounded filesystem discovery, root and nested ignore files, conservative limit failures, declared submodules, nested Git worktrees, oversized manifests, mixed nested projects, ignored VCS and generated dependency paths, access issues, cancellation, target validation, orchestration across discovery, detection, readiness, manifests, and topology, partial-result diagnostics, both report contracts, safe paths, and deterministic normalization.
- `internal/detection` tests cover the language and dependency-manager baseline, all technology categories, nested marker matching, deterministic evidence bounds, generated dependency filtering, unsafe paths, cancellation, catalog invariants, and conservative handling of generic filenames.
- `internal/project` tests cover simple repositories, nested monorepositories, nearest-workspace selection, code/infrastructure/mixed classification, all marker rules, generated dependency paths, weak-marker rejection, evidence bounds, partial snapshots, unsafe paths, cancellation, determinism, and bounded-inventory performance.
- `internal/readiness` tests cover all five rules, root-only foundations, supported test conventions, empty and infrastructure-only repositories, partial-snapshot suppression, deterministic ordering, unsafe input, cancellation, and bounded-snapshot performance.
- `internal/changeimpact` tests cover direct service, environment, and configuration matches; rename pairs; reverse dependency chains; unsupported relationships; partial maps; cancellation; and every traversal or retention limit.
- `internal/finding` tests cover every required finding dimension, evidence representations and source ranges, normalization ownership, profile limits, exact and scoped exclusion precedence, expiration, audit requirements, cancellation, and JSON round trips.
- `internal/manifest` tests cover all built-in formats, explicit empty workspaces, parser replacement and ownership isolation, strict structured-input guards, local and remote reference redaction, normal and maximum limit arithmetic, cancellation, deterministic normalization, and bounded performance.
- `internal/topology` tests cover component and workspace association, nested-repository retention and overlap rejection, empty and overlapping workspaces, manifest-scoped exclusions, local dependency resolution, malformed patterns, deduplicated safe issues, input-order-independent limits, cancellation during bounded selection, and large-snapshot performance.
- `internal/cli` tests cover small and monorepo selection, Enterprise rejection before service invocation, real local readiness and topology analysis, text and JSON rendering, output parity contracts, deep non-null collections, status-to-exit mapping, invalid targets, cancellation, malformed analyzer results, privacy, determinism, usage, version output, and writer failures.
- The verified local commands are `go test ./...`, `go vet ./...`, and `go build ./...`.
- No CI workflow or external-system integration suite exists yet.

## 2. Test placement

| Test type | Location | Purpose |
| --- | --- | --- |
| Package unit tests | Beside the package under test | Domain rules, parsing, validation, state transitions, and edge cases. |
| Component tests | Beside or within the owning internal domain | Use-case behavior with in-memory or controlled boundary substitutes. |
| Public contract tests | Beside `api/` and `sdk/` packages | Compatibility, serialization, validation, and error semantics. |
| Plugin conformance tests | `sdk/plugintest` | Reusable contract suite for every plugin implementation. |
| Integration tests | `test/integration` | Real boundaries such as storage, queues, plugin hosts, and provider emulators. |
| End-to-end tests | `test/e2e` | Complete user workflows across packaged product surfaces. |
| Security and adversarial tests | Closest owning layer plus integration/E2E suites | Authorization, isolation, injection, redaction, and audit guarantees. |
| Performance and resilience tests | Dedicated suites selected when runtime exists | Load, backpressure, retries, recovery, and resource limits. |

Do not place every test under `test/`. Package-level tests remain close to the code so ownership and failure context stay clear.

## 3. Test layers

### 3.1 Domain and use-case tests

Each internal capability should verify:

- valid behavior and explicit error semantics;
- invariants and state transitions;
- boundary conditions and malformed input;
- cancellation and deadline propagation;
- deterministic ordering and output where users review diffs;
- ownership isolation when a replaceable implementation returns mutable collections;
- minimum, exact-limit, just-over-limit, and maximum representable configurations where arithmetic or allocation safety depends on the boundary;
- absence of provider-specific assumptions;
- evidence and provenance propagation;
- authorization and effect-class requirements for state-changing use cases.

Core tests use consumer-owned interfaces and small fakes. They must not import concrete first-party plugins.

### 3.2 Public contract tests

Once public contracts exist, tests must cover:

- forward and backward compatibility policy;
- required and optional fields;
- unknown-field and unknown-enum behavior;
- stable error categories;
- version negotiation where introduced;
- canonical serialization where hashes or approvals depend on bytes;
- size and recursion limits;
- malformed, adversarial, and fuzz-generated payloads.

A public contract is not stable merely because it compiles.

### 3.3 Plugin and extension conformance

`sdk/plugintest` should provide reusable suites that validate:

- manifest identity, version, capability, and compatibility metadata;
- declared versus exercised capabilities;
- scope and effect classification;
- normalized inputs, outputs, errors, and provenance;
- timeout, cancellation, retry, and rate-limit behavior;
- secret redaction and data-class restrictions;
- isolation from private `internal/` packages;
- deterministic behavior for equivalent provider responses where feasible;
- failure containment for crashes and malformed output.

Every first-party plugin should run the same conformance suite plus provider-specific tests. Pro and Enterprise capabilities require Basic-only tests proving they are not mandatory. Closed Enterprise plugins additionally require entitlement, isolation, and private-distribution tests.

The conformance suite must apply the same capability, input, output, resource, cancellation, and effect rules to open and closed plugins. Distribution visibility and source licensing are not trust signals.

### 3.4 Subscription and entitlement tests

Plan-level suites should verify:

- Pro includes every released Basic capability and Enterprise includes every released Pro capability without replacing core behavior;
- Basic core behavior and access build and operate without a paid entitlement or Pro, Enterprise, AI-provider, billing-provider, or closed-plugin dependencies;
- missing, expired, stale, altered, wrong-audience, or wrong-capability entitlement data returns an explicit unavailable or denied result;
- Pro expiry disables new cloud AI requests without rewriting or hiding existing local artifacts and returns the product to free Basic;
- Enterprise expiry disables new closed-plugin invocations unless an explicit tested grace or offline policy applies;
- subscription expiry during an external effect reaches a verified safe terminal outcome rather than abandoning unknown state;
- downgrade never deletes data, hides local artifacts, weakens validation, or silently substitutes another AI, plugin, or generator;
- renewal does not revive stale validation, approval, target state, or capability authority;
- security or administrator revocation remains distinguishable from subscription expiry and follows the capability's cancellation and quarantine semantics.

### 3.5 Integration tests

`test/integration` should verify applicable boundaries that package tests cannot prove as those boundaries are introduced:

- contract-to-domain translation;
- plugin-host registration and capability negotiation;
- storage, queue, lease, and idempotency semantics when persistent or distributed execution exists;
- agent or worker communication when those runtime roles are introduced;
- authorization propagation across hops;
- audit-event delivery and telemetry correlation;
- provider emulator behavior and partial failures;
- migration and compatibility behavior when persistent schemas exist.

Prefer disposable local dependencies, emulators, or isolated test containers. Tests must never depend on production credentials or mutable shared infrastructure.

### 3.6 End-to-end tests

`test/e2e` should cover a small number of critical user journeys:

1. discover and analyze a repository;
2. produce a plan and candidate artifact;
3. validate and present a diff with evidence;
4. satisfy policy and any required approval, then execute a bounded reversible change;
5. verify, monitor, and report the outcome;
6. diagnose a failure and return its remediation through a new controlled cycle;
7. deny or safely stop an unauthorized or stale action.

End-to-end tests validate packaging and integration, not every domain edge case.

### 3.7 Lifecycle contract tests

Lifecycle suites should test the stage contracts independently from a particular CLI, API, AI provider, or plugin:

- every permitted entry point and transition in the standard workflow profiles;
- rejection of Generate without a selected plan and Deploy without current validation and authorization;
- immutable stage results and invalidation after evidence, scope, plan, candidate, capability, policy, approval, or target drift;
- exact semantics for `completed`, `partial`, `blocked`, `unavailable`, `failed`, `cancelled`, `denied`, `rolled_back`, `indeterminate`, and `skipped`;
- acceptance and rejection of partial analysis according to an explicit downstream completeness rule;
- retry attempts that retain workflow identity while using new attempt and stable idempotency identities;
- cancellation before work, during read-only work, before an effect, after a possible effect, and during verification;
- verified rollback and reconciliation after partial, unknown, or externally changed state;
- equivalence of stage gates across Basic, Pro, and Enterprise, including AI-disabled and plugin-unavailable paths;
- external candidates entering Validate with normalized provenance and no inherited execution authority.

Property and model-based tests should generate legal and illegal state sequences and prove that no sequence reaches Deploy without every required gate bound to the same immutable inputs.

## 4. Safety and security test matrix

The future test suite must include negative cases for:

| Risk | Required evidence |
| --- | --- |
| Default-deny behavior | Missing, unknown, stale, or unavailable policy and capability data cannot grant access. |
| Cross-project or cross-tenant access | Requests, queues, persistence, caches, indexes, artifacts, telemetry and audit exports, backups, restores, and reused workers cannot escape or retain data outside their bound scope. |
| Confused deputy | Downstream capabilities retain the initiating identity and exact target. |
| Identity and token misuse | Audience, expiry, nonce, rotation, revocation, and delegated scope are enforced at ingress and execution boundaries. |
| Forged or stale work | Workers and agents reject forged, replayed, expired, revoked, wrong-audience, stale-policy, or stale-capability tasks and updates. |
| Prompt injection | Repository or provider content cannot change policy or grant tools. |
| Unsafe tool use | Model output cannot bypass schemas, capability grants, or approval. |
| Shell and argument injection | Structured execution preserves arguments and rejects unsafe forms. |
| Path traversal and link escape | Safe path resolution plus no-follow or handle-based access and/or execution isolation prevents `..`, symlink, junction, archive, and time-of-check/time-of-use escapes; the exact OS mechanism remains TBD. |
| Network egress abuse | Egress policy blocks SSRF, DNS rebinding, cloud-metadata access, and undeclared destinations. |
| Secret exfiltration | Secrets do not enter prompts, logs, traces, errors, artifacts, or audit events. |
| Approval drift, replay, or self-approval | A changed plan, target, target-state precondition, policy, actor, or expiry invalidates approval; proposer self-approval and required separation-of-duties violations are rejected. |
| Policy distribution failure | Missing or stale policy versions fail closed and cannot be bypassed by cached work. |
| Duplicate execution | Idempotency prevents repeated external effects after retries or redelivery. |
| Partial provider failure | Outcome becomes failed or indeterminate and enters reconciliation. |
| Malicious or revoked plugin | Package integrity, provenance, compatibility, capability, egress, resource, filesystem, data, and revocation limits are enforced. |
| Sandbox escape or cross-job residue | A task cannot observe prior-job data, survive cancellation, exceed resource bounds, or escape its execution boundary. |
| Payload exhaustion | Size, recursion, decompression, output, process, memory, and time limits fail safely. |
| Malicious manifest | Traversal, links, file replacement, exact or case-fold-equivalent duplicate JSON keys, trailing data, DTD/entity declarations, excessive nesting, oversized values, local or credential-bearing references, and malformed parser output fail safely or are redacted. |
| Ambiguous topology | Empty or overlapping workspaces, manifest-scoped exclusions, unmatched members, unsupported glob semantics, duplicate package identities or issues, partial evidence, and relationship limits remain explicit, bounded, and deterministic rather than being guessed. |
| Audit tampering or outage | Alteration, deletion, reordering, and delivery failure are detectable; effects fail closed or become indeterminate when durable evidence cannot be guaranteed. |
| Break-glass misuse | Strong identity, narrow scope, expiry, reason, alerting, post-review, and uninterrupted audit are enforced. |

Fuzz and property tests should target parsers, schema validation, policy evaluation, safe path resolution, artifact normalization, and state machines.

## 5. AI evaluation

AI evaluation is separate from deterministic software tests. A versioned evaluation corpus should eventually measure:

- grounding in supplied repository and operational evidence;
- unsupported or fabricated claims;
- plan completeness and internal consistency;
- risk identification and uncertainty reporting;
- safe refusal when scope or evidence is insufficient;
- resistance to prompt injection and authority escalation;
- tool and capability selection;
- secret and sensitive-data handling;
- measured variance and regression stability across versioned prompt, policy, tool, and provider changes using repeated trials;
- cost and latency within defined budgets.

Evaluation results cannot replace deterministic policy, validation, authorization, or contract tests. Provider, model, prompt-policy, and tool versions must be recorded without exposing sensitive context.

The initial corpus, scoring method, thresholds, reviewer process, and release gates remain TBD.

## 6. Determinism and fixtures

Tests should:

- use fixed clocks, random seeds, identifiers, and stable ordering when output is reviewed or hashed;
- reverse and duplicate over-limit inputs to prove bounded selection is independent of caller order;
- keep golden artifacts explicit and reviewable;
- use sanitized, minimal fixtures with documented provenance;
- avoid real credentials, personal data, and copied production logs;
- isolate filesystem roots and verify platform-appropriate safe path access rather than relying on string canonicalization alone;
- avoid network access in unit tests;
- make integration-network dependencies explicit and bounded;
- reset external emulators and persistent state between cases.

Golden files are appropriate for generated plans and artifacts only when semantic assertions accompany them.

## 7. Resilience and performance verification

The current local analyzers have focused tests that cancel before work, between composed stages, and during bounded topology selection. A canceled or expired context must prevent the next stage from running and must discard accumulated output rather than returning a partial success.

When distributed runtime components exist, tests should cover:

- worker crash, restart, lease expiry, and fencing;
- duplicate and out-of-order delivery;
- provider timeouts, throttling, malformed responses, and partial success;
- cancellation during planning, execution, and verification;
- queue backpressure, quotas, load shedding, and graceful shutdown;
- circuit-breaker and retry behavior;
- backup and restore;
- reconciliation after indeterminate effects;
- load, soak, latency, and bounded-resource behavior;
- telemetry degradation without loss of required audit evidence.

Destructive and chaos tests run only in disposable, explicitly isolated environments.

## 8. Future quality gates

Exact CI tooling is TBD, but a change should eventually pass the relevant gates:

1. formatting and static analysis;
2. unit and component tests;
3. race and concurrency checks where applicable;
4. public contract compatibility checks;
5. plugin conformance tests;
6. dependency, vulnerability, secret, license, and source analysis;
7. reproducible build, SBOM, provenance, and artifact-signing checks;
8. integration tests for changed boundaries;
9. selected end-to-end and security tests;
10. architecture dependency checks;
11. documentation and local-link validation.

Expensive provider, load, chaos, and broad compatibility suites may run on protected or scheduled pipelines once they exist.

## 9. Definition of done

An implementation change is complete only when the applicable requirements below are satisfied:

- behavior and non-behavior are documented accurately;
- tests prove domain behavior at the lowest useful layer;
- changes to public or plugin contracts have compatibility evidence;
- applicable failure, cancellation, retry, and indeterminate outcomes are covered;
- authorization, secret, audit, and telemetry requirements are tested where relevant;
- no test depends on production data or credentials;
- architectural boundaries remain valid;
- the threat model is updated when a trust boundary, effect class, identity path, or sensitive data flow changes;
- any changed durable decision has an ADR;
- user-visible claims in the README match implemented behavior.

## 10. Open testing decisions

- additional test frameworks or assertion libraries beyond the Go standard library;
- provider emulator and contract-fixture strategy;
- CI platform and protected-branch gates;
- coverage measurement and whether thresholds are useful;
- AI evaluation corpus, scoring, and review ownership;
- compatibility matrix and supported platforms;
- performance environments and service-level targets;
- chaos and disaster-recovery cadence;
- dependency, SBOM, provenance, and artifact-signing tooling.
