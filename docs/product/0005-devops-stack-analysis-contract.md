# PS-0005: DevOps Stack Analysis Contract

- **Product status:** Approved
- **Implementation status:** In progress
- **Date:** 2026-08-12
- **Plan:** Basic foundation shared by every plan
- **Scope:** Local, read-only, deterministic analysis of repository-owned DevOps configuration
- **Model schema version:** `1.0`

Related documents:

- [IATROS Product Contract](product-contract.md)
- [PS-0001: Local Repository Analysis](0001-local-repository-analysis.md)
- [PS-0003: Static Code Analysis Contract](0003-static-code-analysis-contract.md)
- [DevOps stack analysis architecture](../architecture/devops-analysis.md)
- [Scaling profiles](../architecture/scaling-profiles.md)
- [Security architecture](../architecture/security.md)

## 1. Purpose

IATROS must explain the DevOps stack declared by a repository: how applications are built into containers,
composed locally, deployed to Kubernetes, packaged with Helm, provisioned through Terraform-compatible
configuration, delivered by CI/CD or GitOps, observed, and protected. The result must remain useful for a
small repository and a company monorepository without executing repository code or contacting a provider.

The first implementation stage establishes the normalized result and resource contracts. It does not yet
claim that content parsers or a CLI command are implemented. Filename-only technology detection remains a
separate, weaker signal and must not be presented as proof of configuration semantics.

## 2. Meaning of understanding

The capability uses four explicit evidence levels:

1. **Detected:** an exact allowlisted filename indicates that a tool may be present.
2. **Parsed:** a supported parser read a concrete declaration from bounded local content.
3. **Correlated:** multiple local declarations refer to the same image, path, job, chart, workload, or
   environment through deterministic rules.
4. **Unknown or partial:** dynamic expressions, unsupported syntax, failed reads, ambiguity, or limits prevent
   a stronger conclusion.

IATROS must not describe a resource as deployed, active, secure, monitored, or policy-enforced solely because
configuration exists. Runtime and provider state require a separate explicitly authorized observation.

## 3. Normalized facts

`internal/devopsanalysis.Model` schema `1.0` owns concrete collections for:

- evidenced DevOps tools and categories;
- container build targets, base-image references, argument names, and exposed ports;
- Compose services, local build contexts, image references, profiles, dependencies, environment-variable
  names, and ports;
- Kubernetes resources, identities, image references, and ports;
- Helm charts and local chart dependency identities;
- Terraform or OpenTofu provider, module, resource, data, variable, and output blocks;
- CI/CD pipelines, triggers, jobs, job dependencies, and named environments;
- GitOps reconciliation resources and repository-relative source paths;
- observability resources and their metrics, logs, traces, profiles, or alert signals;
- security and secret-management controls with `unknown`, `advisory`, or positively evidenced `blocking`
  enforcement;
- bounded diagnostics and explicit partial outcomes.

Every semantic fact references a known tool in the matching category and carries ordered local evidence. The
model does not use a universal property bag, retain parser trees, or expose provider SDK objects.

## 4. Initial format coverage

The implementation will grow in independent vertical slices:

| Area | Initial local formats and tools | Required first semantics |
| --- | --- | --- |
| Containers | Dockerfile/Containerfile, Docker Compose | build stages, base images, argument names, services, local contexts, images, dependencies, ports |
| Orchestration | Kubernetes YAML/JSON, Kustomize | object identity, workload images, service/container ports, local references |
| Packaging | Helm `Chart.yaml`, values, templates | chart identity, dependencies, values references, rendered semantics only when a safe renderer exists |
| Infrastructure as code | Terraform and OpenTofu HCL/JSON | required providers, modules, resources, data, variables, outputs, static references |
| CI/CD | GitHub Actions, GitLab CI, Jenkins, Azure Pipelines, CircleCI, Buildkite | workflows, triggers, jobs, dependencies, named environments, configured security gates |
| GitOps | Argo CD and Flux | reconciliation identity, local source path, destination namespace, managed object references |
| Observability | Prometheus, Grafana, OpenTelemetry Collector, Loki, Tempo, Jaeger | telemetry roles, signals, local rules and dashboards, non-secret target identities |
| Security | CodeQL, Dependabot, Renovate, Trivy, Snyk, Semgrep, Gitleaks, OPA/Conftest, SOPS | configured control, scope, trigger, and evidenced enforcement without secret values |

Support is claimed per parser and semantic field, not per filename catalog. Unsupported providers may be
added through a parser backend only when they preserve this contract.

## 5. Privacy and safety

DevOps analysis is local and read-only. It must:

- read only files admitted by bounded repository discovery and ignore nested-repository boundaries;
- reject links, irregular files, unsafe paths, unstable file identities, oversized content, and excessive
  parser nesting;
- never execute Docker builds, Compose, Helm templates, Terraform, CI steps, scripts, plugins, or project code;
- perform no network, registry, cluster, cloud, CI, GitOps, monitoring, or security-provider access;
- retain variable and build-argument names but never their resolved or default secret values;
- omit scripts and commands from the normalized model;
- sanitize image, module, provider, and remote-source identities by removing credentials and sensitive query
  material before retention;
- preserve only normalized repository-relative evidence paths and bounded English diagnostics.

Repository content is untrusted data and cannot grant capabilities, change policy, or authorize execution.

## 6. Determinism and ambiguity

Collections and nested values are detached, sorted, and deduplicated. The same normalized repository content,
parser versions, and profile must produce the same model on Windows, macOS, and Linux. Output contains no
timestamps, host paths, map-order dependence, environment values, or provider state.

Static expressions may be correlated only through format-specific deterministic rules. Templating,
conditionals, matrices, interpolation, aliases, custom resources, includes, and generated configuration that
cannot be resolved safely produce unknown facts or a partial diagnostic. An analyzer must not guess a
deployment relationship or enforcement state.

## 7. Resource profiles

The unified `small`, `monorepo`, and Enterprise per-worker profiles bound:

- candidate files, bytes per file, total bytes, parser nesting, and total duration;
- tools and every concrete fact collection;
- jobs and total job dependencies per pipeline, nested values per fact, evidence, diagnostics, and retained text.

Configured maxima are budgets, not preallocation targets or fixed product ceilings. Parsers should process
one bounded document at a time, check cancellation during potentially long work, and release syntax trees
after normalized facts are emitted.

## 8. Parser and extension contract

Parsers will be selected by exact allowlisted paths or content signatures. The consuming analyzer owns the
small parser interface and supplies a confined reader, validated limits, and cancellation. A parser receives
no filesystem root, network client, process runner, credentials, or provider connection.

Standard-library implementations are preferred when they preserve fidelity. Maintained YAML, HCL, Dockerfile,
or other specialized parsers are permitted after license, maintenance, vulnerability, performance, and
format-fidelity review. Every backend must pass shared malformed-input, cancellation, limit, determinism,
redaction, and cross-platform contract tests.

## 9. Delivery sequence

1. normalized model, validation, normalization, and limits;
2. confined source and replaceable parser registry;
3. Dockerfile and Compose analysis;
4. Kubernetes and Kustomize analysis;
5. Helm analysis without unsafe template execution;
6. Terraform and OpenTofu analysis;
7. CI/CD pipeline analysis;
8. Argo CD and Flux analysis;
9. observability and security configuration analysis;
10. deterministic cross-file correlation and a separate versioned CLI report.

Each slice expands support without weakening safety or changing the meaning of existing normalized facts.

## 10. Contract-stage acceptance criteria

This foundation is complete when:

- every requested DevOps area has a concrete normalized fact type;
- validation rejects unknown tools, category mismatches, unsafe paths, malformed evidence, cyclic or invalid job graphs,
  inconsistent diagnostics, and non-canonical ordering;
- normalization detaches, orders, and deduplicates every nested collection;
- JSON round-trip and negative contract tests pass;
- all three scaling profiles include valid monotonic DevOps-analysis budgets;
- documentation distinguishes filename detection, parsed configuration, correlation, and runtime state;
- documentation does not claim that deferred parsers or CLI integration exist.

## 11. Feature acceptance criteria

The full feature is implemented only when every format claimed in the support matrix has a bounded parser,
fixtures for valid and malformed content, cancellation and limit tests, redaction tests, deterministic output,
cross-file correlation tests, and semantically equivalent validated text and JSON reports. Runtime inventory,
drift, vulnerability intelligence, and deployment validation remain separate later capabilities.
