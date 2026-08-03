# ADR-0004: Control state-changing operations

- **Status:** Proposed
- **Date:** 2026-07-31
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS is intended to combine AI-assisted planning with tools that may eventually change repositories, delivery systems, infrastructure, and production environments. Repository content, provider responses, plugin output, and model output are untrusted data. None of them can safely serve as authorization.

Directly executing generated commands or granting ambient provider credentials would create unacceptable risks: scope expansion, prompt injection, confused-deputy behavior, duplicate effects, secret exposure, and false success after partial failure.

## Proposed decision

All state-changing operations use a provider-neutral controlled-effect lifecycle:

```text
plan → validate → policy decision → approval when required
     → apply through a scoped capability → verify → audit outcome
```

The lifecycle has these invariants:

1. Discovery and proposal capabilities are separate from mutation capabilities.
2. Effect classification determines required policy and approval.
3. Authorization is enforced outside AI models and plugins.
4. Approval binds to the exact plan or artifact hash, actor, target, environment, policy version, and expiry.
5. Scope or target drift invalidates stale approval.
6. Execution uses structured capability requests rather than unrestricted model-generated shell strings.
7. Workers, agents, plugins, and extensions receive task-scoped, short-lived authority.
8. Effects use idempotency and explicit cancellation semantics where applicable.
9. Postconditions are verified; an unknown outcome becomes **indeterminate**, not successful.
10. Every denial, approval, effect, failure, cancellation, and verification emits sanitized audit evidence.
11. Irreversible or production-critical operations may require stronger authentication and separation of duties.

## Consequences

### Positive

- AI can assist without becoming an authority source.
- Users and policy can review exact intended effects.
- Retries and partial failures have explicit semantics.
- Audit evidence can connect intent, approval, execution, and result.
- Plugins remain capability implementations instead of unrestricted automation.

### Trade-offs

- State-changing workflows require more steps and may have higher latency.
- Plans and provider state need integrity and drift handling.
- Idempotency, verification, reconciliation, and audit are provider-specific implementation work.
- Non-interactive automation needs pre-approved policy rather than bypassing controls.
- Some provider operations cannot be fully rolled back and must expose that limitation.

## Alternatives considered

### Direct execution of model-generated commands

Rejected because model output is untrusted, difficult to scope, and cannot provide deterministic authorization.

### Plugin-controlled approval

Rejected because the component requesting or executing an effect must not grant itself authority.

### Confirmation based only on a human-readable description

Rejected because an approval must bind to the exact executable plan, target, and expiry to prevent drift or substitution.

## Open questions

- effect-class taxonomy and default approval thresholds;
- policy engine and policy distribution;
- actor and workload identity protocols;
- plan and artifact canonicalization for hashing;
- approval user experience and non-interactive policy;
- rollback, compensation, break-glass, and reconciliation semantics;
- audit schema, storage, integrity, and retention;
- worker and agent trust and isolation model.

This ADR remains **Proposed** until maintainers agree on an initial effect classification, policy and approval scope, identity binding, and indeterminate-outcome semantics. Acceptance should precede implementation. Detailed policy engines, identity protocols, plugin isolation, audit storage, and break-glass mechanisms should be decided in focused follow-up ADRs rather than expanding this record indefinitely.

## Implementation validation

After acceptance, implementation must provide executable tests for:

- policy and authorization at ingress and immediately before effects;
- stale, replayed, altered, or expired approvals;
- idempotent retry and duplicate delivery;
- cancellation and partial provider failure;
- secret redaction and capability scope;
- postcondition verification and indeterminate outcomes;
- complete audit evidence for every terminal path.

## References

- [IATROS Product Contract](../../product/product-contract.md)
- [IATROS target architecture](../README.md)
- [Security architecture](../security.md)
- [Testing strategy](../testing.md)
