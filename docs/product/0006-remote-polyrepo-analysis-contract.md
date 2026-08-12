# PS-0006: Remote and Polyrepository Analysis Contract

- **Product status:** Approved
- **Implementation status:** In progress
- **Date:** 2026-08-12
- **Plan:** Optional source-control capability shared by every plan
- **Scope:** Read-only GitHub, GitLab, Bitbucket, and multi-repository analysis
- **Model schema version:** `1.0`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [Unified system map contract](0005-system-map-contract.md)
- [Remote analysis architecture](../architecture/remote-analysis.md)
- [Security architecture](../architecture/security.md)
- [Scaling profiles](../architecture/scaling-profiles.md)

## 1. Purpose

Many software systems span application, library, infrastructure, deployment, and policy repositories hosted
on different source-control platforms. IATROS needs a deterministic description of that composition before a
provider adapter can fetch content and project the results into the unified system map.

The implemented stage defines provider, repository, immutable revision, relationship, evidence, diagnostic,
normalization, validation, and resource contracts for GitHub, GitLab, and Bitbucket provider families.
Provider authentication, API clients, content acquisition, analysis orchestration, and CLI commands remain
later stages. The default Basic CLI therefore remains local-only.

## 2. Provider and installation scope

The model supports:

- GitHub.com and GitHub Enterprise installations;
- GitLab.com and self-managed GitLab installations;
- Bitbucket Cloud and Bitbucket Data Center installations;
- one system whose repositories span multiple providers or hosts.

Provider-specific APIs and SDKs belong in source-control plugins. The provider-neutral core stores only the
normalized provider family and host. It does not import provider SDK models or assume a hosted public domain.

An installed open source-control plugin may be available to any plan when the user explicitly requests remote
analysis and grants the required access. Subscription entitlement does not grant repository authority. Closed
company adapters remain an Enterprise plugin capability.

## 3. Polyrepository composition

One result represents one explicitly selected system. Every repository has a stable system-local identifier,
provider family, normalized host, lowercase namespace, lowercase repository name, selected Git reference,
exact immutable commit, visibility, fork state, archive state, and evidence.

Provider, host, namespace, and repository name form one canonical physical location. The same location cannot
appear under multiple system-local IDs.

Repository relationships are directed and evidence-backed. Examples include `depends_on`, `deploys`,
`contains_submodule`, `mirrors`, and `forked_from`; the contract does not infer them merely because repositories
share an organization, topic, owner, or similar name.

System membership must come from an explicit request, an approved system catalog, or a declaration in an
already admitted repository. Organization-wide enumeration is discovery input, not automatic consent to
analyze every returned repository.

## 4. Reproducibility and evidence

Every repository is pinned to a lowercase full Git object identifier: 40 hexadecimal characters for SHA-1 or
64 for SHA-256 repositories. A branch, tag, default branch, or direct commit selector is preserved separately.
Analysis must read content from the recorded commit, not from a branch that may move between provider calls.

Repository identity requires sanitized provider metadata evidence. System membership and relationships may
also use safe repository-file evidence. Every evidence record identifies its repository, preventing an
ambiguous `catalog.yaml` path in a multi-repository result.

## 5. Credentials and trust

Tokens, passwords, cookies, authorization headers, signed URLs, SSH private keys, provider client objects, raw
API payloads, and credential-bearing clone URLs are outside the model. Credential resolution belongs to the
adapter boundary and must return only normalized metadata. Hosts are lowercase DNS names or explicit IP
addresses with an optional canonical port. Namespaces and names contain safe path segments, not URLs.

Provider metadata references use one shared security validator. It rejects URL, user-information, query,
fragment, parameter, authorization-header, bearer, and basic credential forms before data enters either the
remote model or unified system map.

Remote metadata and repository content are untrusted input. A provider response cannot add repositories beyond
the admitted scope, expand permissions, enable writes, execute repository code, or weaken file, byte, request,
time, and cancellation limits. Redirects and alternate hosts require separate destination validation.

## 6. Resource and failure behavior

The `small`, `monorepo`, and `enterprise` profiles bound:

- repositories and repository relationships;
- provider pages, page size, requests, and request concurrency;
- bytes per provider response and total response bytes;
- evidence, diagnostics, retained text, and total analysis duration.

Pagination and rate-limit handling must be bounded. Reaching a limit, losing access to one admitted repository,
or receiving unsupported metadata produces an explicit partial result with a diagnostic. Authentication or
authorization failure must not be reported as an empty successful system. Cancellation stops outstanding work
and prevents new provider requests.

## 7. Acceptance criteria for this contract stage

This stage is complete when:

- GitHub, GitLab, and Bitbucket have explicit provider identities without provider code in the core;
- one result can contain repositories and relationships across all three provider families;
- every repository is pinned to an immutable revision and retains its selected reference;
- validation rejects unsafe hosts, names, references, revisions, evidence, and unknown relationships;
- normalization is deterministic, detached, ordered, and deduplicated;
- request, response-byte, entity, evidence, diagnostic, text, and timeout budgets are validated;
- the system-map profile can retain every permitted remote repository, relationship, evidence record,
  diagnostic, and text field;
- JSON, provider, self-hosted, safety, partial-result, and limit tests pass.

## 8. Deferred implementation stages

Later stages will define the public plugin capability, implement separately tested GitHub, GitLab, and
Bitbucket adapters, add explicit authentication and host-admission policies, acquire immutable repository
content, correlate submodules and catalog declarations, translate analyzed repositories into the system map,
and expose remote workflows through an approved product surface.
