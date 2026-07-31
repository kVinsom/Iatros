# IATROS Architecture Decision Records

Architecture decision records (ADRs) capture consequential choices, their context, and their trade-offs. The [target architecture](../README.md) describes the complete system view; ADRs explain why durable choices were made.

## Index

| ID | Decision | Status | Date | Supersedes |
| --- | --- | --- | --- | --- |
| [ADR-0001](0001-capability-boundaries-and-dependency-direction.md) | Capability boundaries and dependency direction | Accepted | 2026-07-31 | — |
| [ADR-0002](0002-single-community-core-with-optional-overlays.md) | Single Community core with optional overlays | Accepted | 2026-07-31 | — |
| [ADR-0003](0003-start-with-one-go-module.md) | Start with one Go module | Proposed | 2026-07-31 | — |
| [ADR-0004](0004-control-state-changing-operations.md) | Control state-changing operations | Proposed | 2026-07-31 | — |

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
