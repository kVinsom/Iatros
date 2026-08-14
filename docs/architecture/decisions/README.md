# IATROS Architecture Decision Records

Architecture decision records (ADRs) capture consequential choices, their context, and their trade-offs. The [target architecture](../README.md) describes the complete system view; ADRs explain why durable choices were made.

## Index

| ID | Decision | Status | Date | Supersedes |
| --- | --- | --- | --- | --- |
| [ADR-0001](0001-capability-boundaries-and-dependency-direction.md) | Capability boundaries and dependency direction | Accepted | 2026-07-31 | — |
| [ADR-0002](0002-single-core-with-optional-subscriptions.md) | Single provider-neutral core with optional subscriptions | Accepted | 2026-07-31 | — |
| [ADR-0003](0003-start-with-one-go-module.md) | Start with one Go module | Accepted | 2026-07-31 | — |
| [ADR-0004](0004-control-state-changing-operations.md) | Control state-changing operations | Proposed | 2026-07-31 | — |
| [ADR-0005](0005-use-cobra-as-the-cli-adapter.md) | Use Cobra as the CLI adapter | Accepted | 2026-08-01 | — |
| [ADR-0006](0006-version-private-core-contracts.md) | Version private core contracts independently from public APIs | Accepted | 2026-08-03 | — |
| [ADR-0006](0006-use-explicit-unified-scaling-profiles.md) | Use explicit unified scaling profiles | Accepted | 2026-08-03 | — |
| [ADR-0007](0007-use-bounded-repository-owned-ignore-rules.md) | Use bounded repository-owned ignore rules | Accepted | 2026-08-03 | — |
| [ADR-0008](0008-use-bounded-replaceable-artifact-parsers.md) | Use bounded replaceable artifact parsers | Accepted | 2026-08-14 | — |

Two accepted records were assigned `ADR-0006` before this index was reconciled. Their stable filenames and historical
identifiers remain unchanged; new decisions continue from the next unused filename.

## Status lifecycle

| Status | Meaning |
| --- | --- |
| **Proposed** | Concrete decision awaiting architectural agreement. |
| **Accepted** | Current architectural direction. |
| **Rejected** | Considered but intentionally not adopted. |
| **Deprecated** | Still present but no longer recommended for new work. |
| **Superseded** | Replaced by a newer ADR. |

## Process

1. Copy [ADR-0000 template](0000-template.md) to the next unused four-digit number.
2. Describe the forces and constraints before proposing a solution.
3. State the decision, its scope, exclusions, consequences, and alternatives.
4. Link relevant architecture, issues, contracts, or implementation evidence.
5. Open the ADR as **Proposed** unless it only records an already-established invariant.
6. Update this index when status changes.

ADR numbers and accepted text are historical records. Do not silently rewrite an accepted decision to mean something materially different. Create a new ADR, mark the old one **Superseded**, and link both records.

Do not create an ADR for every directory or routine implementation detail. Use ADRs for decisions that are expensive to reverse, cross capability boundaries, affect public contracts, change security posture, or constrain future design.
