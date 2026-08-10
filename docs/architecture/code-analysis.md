# Static Code Analysis Architecture

> **Status: contract and resource profiles implemented; analyzers and CLI integration are pending.**

See also:

- [PS-0003: Static Code Analysis Contract](../product/0003-static-code-analysis-contract.md)
- [Scaling profiles](scaling-profiles.md)
- [Technology detection](detection.md)
- [Manifest analysis](manifest-analysis.md)
- [Repository topology](topology.md)
- [Security architecture](security.md)

## 1. Capability boundary

`internal/codeanalysis` owns normalized, provider-neutral facts produced from static source and local
configuration analysis. It does not own filesystem discovery, project-boundary detection, manifest parsing,
topology association, CLI rendering, public API messages, or provider integrations.

Filename marker detection remains in `internal/detection`. Package declarations remain in
`internal/manifest`. Repository topology remains in `internal/topology`. Future analyzers may consume those
validated facts, but they must translate language-specific syntax into the code-analysis model rather than
exposing parser nodes outside their adapter.

```text
bounded discovery + boundaries + manifests
                    |
                    v
       language/configuration analyzers
                    |
                    v
       internal/codeanalysis.Model 1.0
                    |
                    v
      future aggregation and CLI report
```

## 2. Contract shape

The model uses concrete collections for services, frameworks, ports, API endpoints, environment variables,
and database/cache/message-broker dependencies. All facts reference a known service and include ordered
evidence. This prevents a universal property bag from erasing meaning or allowing incompatible analyzer
outputs.

Every service entrypoint is a normalized repository file at or below that service's root. A service cannot
claim an entrypoint from a sibling or parent project, which keeps nested-project analysis confined to its
declared boundary.

Evidence records its representation (`source`, `import`, `manifest`, `configuration`, or `infrastructure`),
a repository-relative path, and an optional complete source span. Certainty is either `observed` or
`inferred`. Unsupported or dynamic behavior becomes a bounded diagnostic and, when information was omitted,
an explicit partial result.

## 3. Analyzer boundary

No speculative analyzer interface is introduced at the contract-only stage. The first Go vertical slice
will establish the consuming orchestration boundary and the smallest interface required by a real parser and
its tests. Later language analyzers must implement that consumer-owned interface and return concrete
code-analysis facts, not parser-specific objects.

Analyzer implementations must:

- accept `context.Context` first and terminate on cancellation;
- receive validated limits and confined source access through explicit dependencies;
- avoid retaining whole syntax trees after a file result is normalized;
- avoid concurrency until measurements demonstrate a benefit and termination is defined;
- report dynamic values without guessing;
- return no raw environment values, connection strings, or credentials;
- remain deterministic across Windows, macOS, and Linux for the same normalized repository content.

## 4. Resource model

One `codeanalysis.Limits` value bounds input I/O, syntax complexity, output cardinality, evidence, text, and
duration. It is part of the unified `analysis.ScalingProfile`, so selecting `small`, `monorepo`, or
`enterprise` configures discovery and deep analysis together. Validation rejects a code-analysis file budget
larger than the discovery inventory.

Large configured capacities are ceilings, not preallocation instructions. Implementations should process one
file at a time where possible, retain normalized facts only, periodically check cancellation inside bounded
loops, and stop with explicit diagnostics before exceeding a budget.

## 5. Parser strategy

Go analysis should begin with the standard `go/parser`, `go/ast`, and `go/token` packages. Other languages
may require maintained third-party parsers because regex-only source analysis cannot reliably handle nested
syntax, comments, strings, aliases, or generated constructs. Dependency choice belongs to each language
adapter and requires license, maintenance, vulnerability, fidelity, performance, and cross-platform review.

Every implementation, including a specialized streaming or incremental backend, must pass shared contract
tests and preserve normalized evidence, certainty, partial outcomes, privacy, and resource limits.

## 6. Planned vertical order

1. confined bounded source reader;
2. Go services, frameworks, ports, endpoints, environment variables, and resource dependencies;
3. JavaScript and TypeScript;
4. Python;
5. Java, Kotlin, and .NET;
6. PHP, Ruby, and Rust;
7. Docker Compose, Kubernetes, and infrastructure-as-code correlation;
8. aggregation into a versioned CLI report.

Each stage expands conformance fixtures without weakening the central model or safety boundary.
