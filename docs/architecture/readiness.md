# Repository Readiness Architecture

> [!IMPORTANT]
> **Status: implemented and integrated into local CLI analysis.** The evaluator applies five conservative absence-based rules to a complete bounded snapshot. It performs no filesystem access, content parsing, command execution, or network access.

See also:

- [IATROS target architecture](README.md)
- [Technology detection architecture](detection.md)
- [Security architecture](security.md)
- [Testing strategy](testing.md)
- [PS-0001: Local Repository Analysis](../product/0001-local-repository-analysis.md)
- [Unified findings architecture](findings.md)

## 1. Boundary

Repository readiness is a provider-neutral capability under `internal/readiness`. It owns its snapshot, technology vocabulary, and rule behavior. Findings use the provider-neutral `internal/finding` contract, which remains independent from CLI and report serialization.

```text
bounded inventory + detected technologies
                    |
                    v
       internal/readiness.Evaluator
                    |
                    v
          normalized unified findings
                    |
                    v
      internal analysis report and CLI
```

The analysis orchestrator translates discovery and detection results into a readiness `Snapshot`. Readiness returns the unified finding type directly, so the versioned analysis report does not perform a lossy finding translation. The dependency remains acyclic because both readiness and analysis consume `internal/finding`, which imports neither package.

## 2. Evaluation behavior

The evaluator:

- honors cancellation before and during indexing;
- evaluates only complete snapshots;
- validates all consumed paths and technology identifiers;
- indexes directories, files, and technologies once before applying rules;
- does not mutate or reorder caller-owned collections;
- deduplicates and sorts technology IDs used as evidence;
- emits every required finding dimension, including confidence, provenance, risk, and recommendation;
- returns findings ordered by severity, rule ID, and finding ID;
- returns non-nil empty collections when no finding is applicable.

When discovery reports `Partial`, every current rule is suppressed. The absence of a marker is not trustworthy when part of the repository may be unreadable or omitted by a safety limit. The analysis orchestrator maps each retained discovery issue to a warning diagnostic in the partial report.

## 3. Implemented rules

| Code | Severity | Condition |
| --- | --- | --- |
| `repository.readme.missing` | `medium` | No recognized README exists at the repository root. |
| `repository.license.missing` | `informational` | No recognized license file exists at the repository root. |
| `repository.gitignore.missing` | `informational` | No exact `.gitignore` exists at the repository root. |
| `quality.tests.not_detected` | `medium` | A language or runtime is detected, but no supported test marker exists anywhere in the snapshot. |
| `delivery.ci.not_detected` | `medium` | At least one technology is detected, but no technology in the `ci_cd` category is present. |

A complete empty repository receives only the three repository-foundation findings. Infrastructure-only repositories are eligible for the CI/CD finding but not the code-test finding.

## 4. Recognized foundations

README matching is case-insensitive and accepts root `README`, `README.adoc`, `README.markdown`, `README.md`, `README.rst`, and `README.txt` names.

License matching is case-insensitive and accepts root `COPYING`, `LICENCE`, `LICENSE`, and `UNLICENSE` names, common text or markup extensions, and variants such as `LICENSE-APACHE`.

The `.gitignore` marker is case-sensitive and root-only because that is the filename Git interprets. Nested foundation files do not satisfy root repository rules.

## 5. Supported test evidence

Test evidence is repository-wide and includes:

- exact test directories such as `test`, `tests`, `spec`, `specs`, and `__tests__`;
- language conventions such as `*_test.go`, `test_*.py`, `*.test.ts`, `*_spec.rb`, `*Test.java`, `*Tests.cs`, and `*_SUITE.erl`;
- recognized configuration files for pytest, tox, Jest, Vitest, Playwright, Karma, PHPUnit, and RSpec.

Generic names and unsupported file extensions are not treated as tests. Marker presence proves only that test-related repository evidence exists; it does not prove that tests pass, cover the detected code, or run in CI.

Test files or directories below the shared excluded-directory policy do not satisfy readiness. Downloaded `node_modules`, virtual environments, Terraform state, and other generated tool data cannot be used as evidence that the selected repository maintains its own tests.

## 6. Deferred rules

Missing lock files, dependency-manager conflicts, per-project test coverage, and per-project CI coverage remain deferred until the internal boundary model is enriched and integrated with readiness. Multiple managers or lock files can be valid in a monorepository, and a repository-wide absence rule must not misrepresent that structure.

Content-aware readiness rules remain deferred until bounded allowlisted parsing exists. Future rules must preserve the same evidence, cancellation, partial-snapshot, and deterministic-ordering policies.

## 7. Findings and exclusions

Every readiness finding has high confidence because absence rules run only on a complete validated snapshot. Evidence
is a structured analyzer observation, provenance identifies the readiness producer and rule version, and risk and
recommendation remain rule-specific rather than inferred by the CLI.

The evaluator emits active findings and does not read exclusion policy. A higher use-case boundary may apply
validated controlled exclusions through `internal/finding.ApplyExclusions`. Exclusion keeps the finding in the
result and cannot turn a partial snapshot into trustworthy absence evidence.
