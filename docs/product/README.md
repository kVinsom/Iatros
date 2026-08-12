# IATROS Product Specifications

Product specifications define user-visible behavior before implementation. They complement the [target architecture](../architecture/README.md) without turning planned behavior into an implementation claim.

## Canonical product contract

The [IATROS Product Contract](product-contract.md) defines the final product boundary, the Analyze → Plan → Generate → Validate → Deploy → Monitor → Fix lifecycle, and the Basic, Pro, and Enterprise plans. Basic is free, local, and AI-free; Pro adds cloud AI; Enterprise adds cloud or local AI plus closed plugins. It applies to every feature specification without claiming that target capabilities are already implemented.

## Active specifications

| ID | Specification | Product status | Implementation status | Approved |
| --- | --- | --- | --- | --- |
| [PS-0001](0001-local-repository-analysis.md) | Local repository analysis | Approved | In progress | 2026-08-01 |
| [PS-0002](0002-core-domain-contracts.md) | Stable core domain contracts | Approved | Implemented | 2026-08-03 |
| [PS-0003](0003-static-code-analysis-contract.md) | Static code analysis contract | Approved | In progress | 2026-08-10 |
| [PS-0004](0004-dependency-analysis-contract.md) | Dependency analysis contract | Approved | Not started | 2026-08-10 |
| [PS-0005](0005-devops-stack-analysis-contract.md) | DevOps stack analysis contract | Approved | In progress | 2026-08-12 |

## Status model

| Status | Meaning |
| --- | --- |
| **Draft** | The problem or contract is still being discussed. |
| **Approved** | The product scope and acceptance criteria are ready for implementation. |
| **Not started** | The product contract is approved, but implementation has not begun. |
| **In progress** | Implementation has started but does not yet satisfy all acceptance criteria. |
| **Implemented** | Source, tests, and documentation prove the specified behavior. |
| **Superseded** | A newer specification replaces this contract. |

Product status and implementation status are separate. Approval authorizes implementation; it does not imply that the feature exists.
