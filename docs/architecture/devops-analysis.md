# DevOps Stack Analysis Architecture

> **Status: normalized model and resource profiles implemented; content parsers, correlation, and CLI integration are pending.**

See also:

- [PS-0005: DevOps Stack Analysis Contract](../product/0005-devops-stack-analysis-contract.md)
- [Technology detection](detection.md)
- [Repository discovery](repository-discovery.md)
- [Repository topology](topology.md)
- [Scaling profiles](scaling-profiles.md)
- [Security architecture](security.md)

## 1. Capability boundary

`internal/devopsanalysis` owns provider-neutral facts parsed from repository-owned DevOps configuration. It
does not own filesystem traversal, ignore handling, project boundaries, package manifests, source-code facts,
CLI rendering, runtime provider inventory, policy decisions, generation, or deployment.

Filename marker detection remains in `internal/detection`. It can identify candidate technologies but cannot
claim configuration semantics. Future DevOps parsers consume the already bounded discovery inventory and a
confined source, then emit only the normalized model.

```text
bounded discovery + boundaries + filename candidates
                         |
                         v
              confined parser registry
                         |
                         v
             devopsanalysis.Model 1.0
                         |
                         v
         correlation + future CLI report
```

## 2. Model design

The model uses separate fact collections for container builds, Compose services, Kubernetes resources, Helm
charts, Terraform blocks, pipelines and jobs, GitOps resources, observability resources, and security
controls. This keeps format-specific semantics explicit and prevents a universal metadata map from becoming
an unstable core contract.

All facts reference a known tool in the expected category. Evidence is an ordered repository-relative file
and an optional complete source span. Facts distinguish direct observation from deterministic inference.
Partial results require a warning or error diagnostic that explains what could not be represented.

The model intentionally excludes:

- raw configuration, scripts, commands, expressions, and parser syntax trees;
- resolved environment, build-argument, variable, output, credential, and secret values;
- absolute paths, authenticated URLs, provider responses, and runtime status;
- arbitrary extension maps that bypass validation.

## 3. Parser boundary

The parser registry will be consumer-owned and created when the first Docker vertical slice is implemented.
Each parser will receive `context.Context`, one bounded document stream, document metadata, and validated
limits. It will not receive an unrestricted host path, network client, command runner, or global registry.

Parsers must consume or close owned streams according to the analyzer contract, periodically honor
cancellation, reject unsupported ambiguity, and translate external syntax into concrete normalized facts.
External YAML, HCL, Dockerfile, or CI parser types cannot cross the package boundary.

## 4. Correlation boundary

Correlation will occur after individual documents validate. It may connect only positive local evidence such
as a Compose build context to a Dockerfile, a Kubernetes image to a container build, a Helm or GitOps local
path to a known chart, or an explicit pipeline dependency to a known job.

Remote or dynamic references remain sanitized identities and cannot trigger I/O. Ambiguous matches stay
ambiguous. Repository topology supplies project ownership; DevOps analysis must not invent a second project
boundary model.

## 5. Resource behavior

`devopsanalysis.Limits` is part of the unified analysis profile. It bounds input files and bytes, parser
nesting, every retained fact collection, jobs, total job dependencies per pipeline, nested values, evidence,
text, diagnostics, and duration. Pipeline dependency graphs must reference known jobs and remain acyclic;
validation uses bounded iterative traversal rather than recursion.
DevOps candidate files cannot exceed the discovery inventory.

Large capacities are ceilings. Implementations must allocate from observed content, stream or process one
file at a time where practical, release parser structures after normalization, and stop with explicit partial
diagnostics before exceeding a budget. Concurrency requires a measured benefit and a defined cancellation and
memory budget.

## 6. Security properties

The capability is read-only and offline. It never runs templating, builds, plans, pipelines, scanners, or
repository scripts. Content is untrusted and cannot become authority. Sensitive values are omitted at the
parser boundary, and source identities are sanitized before entering the model.

The existence of configuration proves only repository intent. Claims about deployed resources, effective
policy, monitoring health, vulnerabilities, drift, or compliance require separate provider-aware workflows
with explicit authorization.

## 7. Implemented and deferred work

Implemented now:

- schema `1.0` concrete facts for every requested DevOps category;
- deterministic detached normalization;
- structural, reference, path, evidence, privacy-shape, and diagnostic validation;
- validated `small`, `monorepo`, and Enterprise per-worker limits;
- negative, boundary, normalization, JSON, and profile tests.

Deferred in order:

1. confined document source and parser registry;
2. Dockerfile and Compose parsers;
3. Kubernetes, Kustomize, and Helm parsers;
4. Terraform and OpenTofu parsers;
5. CI/CD and GitOps parsers;
6. observability and security parsers;
7. deterministic correlation and a separate CLI report.
