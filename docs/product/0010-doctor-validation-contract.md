# PS-0010: Doctor and Artifact Validation Contract

- **Status:** Approved
- **Implementation status:** Core engine and built-in local rules implemented; workflow and CLI integration pending
- **Approved:** 2026-08-14

See also:

- [Artifact validation architecture](../architecture/artifact-validation.md)
- [Doctor architecture](../architecture/doctor.md)
- [Unified findings contract](0009-unified-findings-contract.md)
- [DevOps stack analysis contract](0005-devops-stack-analysis-contract.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Product rule

IATROS validates existing and generated DevOps artifacts through one provider-neutral engine before a later workflow
may use them. IATROS Doctor audits normalized repository and system evidence across production, reliability,
performance, security, deployment, and cost categories. Neither capability executes an artifact, contacts a cloud,
deploys a change, or grants authorization.

The first implementation is local, read-only, deterministic, and available to Basic. It provides the internal
contracts and built-in checks; no user-facing Doctor or validation CLI command is claimed yet.

## 2. Artifact validation

Every artifact declares:

- stable identity and safe repository-relative target path;
- `existing` or `generated` origin;
- DevOps kind and independent serialization format;
- producer identity and version; and
- a content source opened only for the duration of validation.

The engine reads each source once under per-artifact and total byte limits, computes a SHA-256 content identity,
closes the owned reader, and never retains raw content in its report. The same engine validates both origins. The
built-in implementation supports:

- UTF-8, empty-content, and NUL-byte safety checks;
- JSON syntax, single-root, nesting, and node limits;
- XML syntax, root, nesting, token, directive, and processing-instruction policy;
- YAML syntax, nesting, node, alias, and duplicate-key checks;
- native HCL/Terraform syntax;
- conservative Dockerfile instruction, continuation, and `FROM` checks;
- Compose `services` structure; and
- Kubernetes `apiVersion`, `kind`, `metadata.name`, and `List.items` structure.

JSON, XML, and Dockerfile checks use the Go standard library or bounded local logic. YAML uses the official YAML
Organization stable v3 parser. HCL uses the official HashiCorp HCL v2 parser. These dependencies parse in memory;
they do not invoke external tools or provider APIs.

Specialized schema, policy, provider, server-side dry-run, and external-tool validators may be registered later
behind the same consumer-owned interface. They must preserve cancellation, limits, deterministic diagnostics,
safe messages, and normalized contracts.

## 3. Validation results

The schema `1.0` report returns:

- overall `passed`, `passed_with_warnings`, `failed`, or `partial` status;
- one result for every requested artifact;
- artifact digest, bytes read, outcome, and validator identities; and
- normalized diagnostics with code, level, safe message, artifact identity, validator provenance, and optional source
  location.

`partial` means validation could not finish, such as unavailable input, resource exhaustion, cancellation, or a
validator operational failure. It is not a successful result. Syntax or structure errors produce `failed` and an
`invalid` artifact outcome. Raw source and validator errors are not copied into user-visible diagnostics.

## 4. Doctor audit

Doctor consumes normalized static code analysis, DevOps analysis, system maps, artifact-validation reports, and
existing unified repository findings. Nil input is unavailable evidence, not an empty successful analysis. Partial
input suppresses absence-based rules that could otherwise create false findings.

Every report records coverage for:

- production;
- reliability;
- performance;
- security;
- deployment; and
- cost.

Coverage is `evaluated`, `partial`, or `not_evaluated` and counts evaluated and skipped registered rules. `evaluated`
means every registered rule for that category ran; it does not claim that the current rule catalog represents every
possible organization policy or live-system check.

The initial built-in catalog checks production environment and service ownership, observability and alerting,
container image pinning, blocking security controls, sensitive default configuration, delivery pipelines, GitOps
for Kubernetes, invalid artifacts, infrastructure environment scope, and infrastructure-as-code evidence.

Doctor returns unified findings with severity, confidence, evidence, provenance, risk, recommendations, and
controlled exclusions. Its report is `ready`, `attention_required`, `not_ready`, or `partial`. Missing or incomplete
category evidence always makes the report partial; it never produces false readiness.

## 5. Safety and scale

Both capabilities use validated `small`, `monorepo`, and `enterprise` per-worker profiles. Budgets cover artifacts,
bytes, parsers, validators, rules, facts, findings, diagnostics, text, and execution time. Validators and Doctor
rules receive their applicable retention budgets; exceeding one produces an explicit partial result instead of
silent truncation. Larger profiles increase limits without changing semantics or authority.

Existing local artifact reads use a confined directory handle, reject links, verify stable regular-file identity,
and enforce containment below one configured root. Effectful workflows still require the additional execution and
authorization controls defined by the security architecture.

## 6. Acceptance criteria

- Existing and generated artifacts use the same validation path and normalized result model.
- Raw artifact content is absent from reports.
- Every owned reader is closed and close failures are explicit.
- JSON, XML, YAML, HCL, Dockerfile, Compose, and Kubernetes baseline checks are bounded and tested.
- Custom validators and Doctor rules have stable identities and reject duplicate registration.
- Missing and partial Doctor evidence cannot produce a ready result or unsupported absence finding.
- All six Doctor categories have explicit coverage.
- Doctor findings use the unified model and controlled exclusions.
- Input collections remain caller-owned and output ordering is deterministic.
- Cancellation, malformed inputs, resource limits, and safe error messages are tested.
- The unified scaling profile can retain the configured normalized inputs for both capabilities.
