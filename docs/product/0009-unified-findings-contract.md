# PS-0009: Unified Findings Contract

- **Status:** Approved
- **Implementation status:** Core model, exclusions, readiness integration, analysis report, and CLI rendering implemented
- **Approved:** 2026-08-14

See also:

- [Unified findings architecture](../architecture/findings.md)
- [Local repository analysis](0001-local-repository-analysis.md)
- [Security architecture](../architecture/security.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Product rule

Every IATROS problem reported as a finding uses one provider-neutral model. Security, quality, dependency,
delivery, infrastructure, operational, cost, and readiness capabilities may define their own rules, but they must
not create incompatible finding shapes.

Diagnostics remain distinct: a diagnostic explains analyzer execution, incomplete input, or omitted work. A
finding describes a problem supported by trustworthy evidence. A partial analyzer must not turn absence caused by
omitted data into a finding.

## 2. Required finding dimensions

Every finding contains:

- a stable finding ID and stable dotted rule ID;
- a title and description;
- `severity`;
- `confidence`;
- one or more affected subjects;
- one or more structured evidence records;
- producer and rule `provenance`;
- an assessed `risk` with level, likelihood, and explanation;
- a `recommendation` with a summary and at least one concrete action; and
- a disposition that is either `active` or `excluded`.

The severity taxonomy is `informational`, `low`, `medium`, `high`, and `critical`. Confidence is `low`, `medium`,
`high`, or `confirmed`. Risk level is `low`, `medium`, `high`, or `critical`; likelihood is `unlikely`, `possible`,
`likely`, or `almost_certain`. Severity, confidence, risk, and likelihood are separate assessments and cannot be
substituted for one another.

## 3. Evidence and provenance

Evidence kinds are repository file, analyzer observation, runtime observation, and external-provider observation.
Repository evidence uses only a repository identity, safe relative path, and optional complete source range.
Runtime and external evidence use a sanitized reference. Evidence descriptions cannot contain control characters.

Provenance records the producer identity, producer version, rule version, and normalized source class. A finding
cannot be valid without provenance, even when the conclusion is obvious or confirmed.

## 4. Controlled exclusions

An exclusion never deletes or hides a finding from the normalized model. It changes the finding disposition to
`excluded` and attaches the applied decision so reports and later policy stages can still audit the problem.

Every exclusion has:

- a stable ID;
- exactly one target: an exact finding ID or a rule ID;
- a human-readable reason;
- requester and approver identities;
- UTC creation and expiration times; and
- an optional scope over repository, environment, subject, and repository path prefix.

A rule-level exclusion must have a non-empty scope. Global permanent rule suppression is invalid. Every exclusion
expires; renewal creates another explicit decision. Exact-finding exclusions outrank rule exclusions, and otherwise
the most specific matching scope wins. Equal-specificity ties resolve by stable exclusion ID. Expired, future, or
scope-mismatched records remain auditable but are not applied.

The core permits the requester and approver to be the same identity so local Basic use remains possible. An
Enterprise policy may require separation of duties without changing the normalized finding contract.

## 5. Determinism, safety, and scale

Findings are ordered by severity and finding ID. Subjects, evidence, recommendation actions, and exclusions are
sorted and deduplicated without mutating caller-owned data. The model has explicit `small`, `monorepo`, and
`enterprise` budgets for findings, subjects, evidence, actions, exclusions, text, and evaluation time.

Exclusion application accepts the evaluation time explicitly; it does not read a hidden process clock. This makes
expiration behavior reproducible in tests, reports, and durable workflows. Untrusted strings, unsafe repository
paths, incomplete source positions, unbounded exclusions, and pre-excluded input are rejected at the boundary.

## 6. Current integration

The readiness evaluator now emits the unified finding type directly. The local analysis report advanced to schema
`1.0` because structured evidence and the required assessment fields replace the earlier compact finding shape.
Text and JSON render the same information. Repository-topology remains an independent schema `0.3` report and does
not currently emit findings.

Future analyzers must adopt this model when they begin emitting problems. Compatibility adapters may translate
external scanner results, but raw vendor finding types must not enter business logic or public reports.

## 7. Acceptance criteria

- A finding missing any required dimension is rejected.
- Structured evidence and source positions are validated and normalized.
- Exclusions are explicit, scoped, time-bounded, attributable, retained, and deterministic.
- Applying an exclusion never removes the finding.
- Inactive and mismatched exclusions do not affect disposition.
- Input collections remain caller-owned.
- Cancellation and configured limits are enforced.
- JSON round trips preserve a valid model.
- Readiness, analysis reports, and CLI output use the same finding representation.
