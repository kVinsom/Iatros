<p align="center">
  <img src="assets/iatros_logo.png" alt="IATROS DevOps logo" width="500">
</p>

<h1 align="center">IATROS DevOps</h1>

<p align="center">
  An early-stage open-source project exploring AI-assisted automation for modern DevOps workflows.
</p>

<p align="center">
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-Apache%202.0-D22128.svg" alt="Apache License 2.0">
  </a>
</p>

> [!IMPORTANT]
> **Project status:** Early implementation. The repository contains two runnable local, read-only workflows. `iatros analyze` performs bounded metadata discovery, technology detection, and five conservative readiness checks. `iatros topology` safely parses allowlisted manifests and reports projects, components, workspaces, and direct dependency relationships. Both commands return deterministic text or JSON. Broader AI, remote-provider, generation, and operational capabilities remain planned, not released.

## Overview

IATROS DevOps is intended to help teams build, deploy, operate, troubleshoot, and improve software through natural-language DevOps workflows. The long-term goal is to connect repository context and operational intent with tools that can analyze projects, prepare infrastructure and delivery artifacts, validate proposed changes, and assist with day-to-day operations.

The proposed architecture separates provider-neutral domain logic from vendor-specific integrations. Stable contracts are intended to connect the core, public interfaces, plugins, and optional extensions without coupling the entire system to a particular cloud, source-control platform, observability backend, or AI provider.

## Planned capabilities

- **Project understanding:** discover repositories, model topology, and assess production readiness.
- **AI-assisted planning:** use project context to support implementation planning, risk analysis, and root-cause analysis.
- **Delivery and infrastructure generation:** prepare CI/CD pipelines, infrastructure and container configurations, scripts, and operational documentation.
- **Validation and diagnostics:** check proposed artifacts and provide an IATROS Doctor workflow for actionable findings.
- **Deployment operations:** assist with promotion, rollback, GitOps workflows, and drift detection.
- **Operational insight:** bring together observability, reliability, security, and cost-awareness workflows.
- **Extensible integrations:** connect external tools and providers through stable plugin and extension contracts.

These items describe the intended product direction. Only the local repository analysis and topology slice described below is currently runnable.

The first approved implementation slice is [local repository analysis](docs/product/0001-local-repository-analysis.md): local-only, read-only `iatros analyze` and `iatros topology` commands with deterministic text and JSON contracts.

## Architecture and repository layout

The scaffold is organized around explicit product and dependency boundaries.

The complete architecture package is available in [`docs/`](docs/README.md), including the [target architecture](docs/architecture/README.md), [project and workspace boundary model](docs/architecture/project-model.md), [security architecture](docs/architecture/security.md), [testing strategy](docs/architecture/testing.md), and [architecture decision records](docs/architecture/decisions/README.md).

| Path | Intended responsibility |
| --- | --- |
| [`api/`](api/) | Public wire contracts; API implementations are intended to live elsewhere. |
| [`apps/`](apps/) | Planned dashboard, GitHub, GitLab, and IDE-facing application surfaces. |
| [`assets/`](assets/) | Project branding and other static repository assets. |
| [`cmd/`](cmd/) | Thin entry points for the agent, control plane, IATROS CLI, Model Context Protocol (MCP) server, and worker. |
| [`deploy/`](deploy/) | Deployment resources for IATROS itself. |
| [`docs/`](docs/) | Product specifications, architecture, security, testing, and decision records. |
| [`examples/`](examples/) | Planned CLI, SDK, and plugin usage examples. |
| [`extensions/`](extensions/) | Reserved Commercial and Enterprise overlays; currently placeholders only. |
| [`internal/`](internal/) | Private, provider-neutral application and domain logic. |
| [`plugins/`](plugins/) | Planned first-party adapters grouped by capability. |
| [`sdk/`](sdk/) | Intended public Go client, plugin and extension contracts, and a plugin-testing toolkit. |
| [`test/`](test/) | Cross-component integration and end-to-end test suites. |

The intended dependency rules are:

1. Maintain a single Community core that remains usable without optional Commercial or Enterprise extensions; do not duplicate core packages across editions.
2. Keep business logic out of `cmd`; executable entry points should only compose applications.
3. Keep `internal` independent from concrete plugins and optional extensions.
4. Plugins should depend on stable contracts in `sdk/plugin` and, where necessary, `api` or `sdk/client`; optional extensions should implement `sdk/extension` or `sdk/plugin`.
5. Keep public `api` and `sdk` packages independent from private `internal` packages.
6. Place provider-specific integrations in `plugins` rather than in the core.

## Current repository state

| Area | Current state |
| --- | --- |
| Directory scaffold | Present |
| Project identity, logo, and license | Present |
| First product specification | Approved; initial local analysis implemented |
| Target architecture, security, and testing documentation | Present |
| Application source and Go module | Root module and initial CLI source present |
| Runnable CLI, services, and applications | Local analysis and repository-topology CLI available; other runtimes not implemented |
| Internal project model | Strong filename-based project and workspace boundaries implemented and exposed through topology reports |
| Internal manifest model | Seven bounded manifest formats, normalized direct declarations, two resource profiles, and replaceable parser backends implemented; conservative profile exposed through topology reports |
| Internal topology model | Project/component association, nested and overlapping workspaces, safe member resolution, local dependency edges, and public CLI report mapping implemented |
| API, SDK, plugins, and extensions | Directory placeholders only |
| Automated tests and CI workflows | Unit and local integration tests present; CI workflows not yet added |
| Published releases | Not yet available |

## Getting started

The current development baseline requires Go 1.25.5 or newer.

```bash
git clone https://github.com/kVinsom/Iatros.git
cd Iatros
go test ./...
go vet ./...
go build ./...
```

Inspect the command surface and version directly from source:

```bash
go run ./cmd/iatros help
go run ./cmd/iatros version
```

Build and run the analyzer on Linux or macOS:

```bash
go build -o ./bin/iatros ./cmd/iatros
./bin/iatros analyze --format json .
```

On Windows PowerShell:

```powershell
go build -o .\bin\iatros.exe .\cmd\iatros
.\bin\iatros.exe analyze --format json .
```

`path` defaults to the current directory and must identify an existing local directory. Text is the default format:

```bash
./bin/iatros analyze /path/to/repository
./bin/iatros analyze --format text /path/to/repository
./bin/iatros analyze --format json /path/to/repository
./bin/iatros topology /path/to/repository
./bin/iatros topology --format text /path/to/repository
./bin/iatros topology --format json /path/to/repository
```

Both commands return exit code `0` for complete and explicitly partial reports. Readiness findings do not fail `analyze`. Invalid usage returns `2`, an invalid or inaccessible target returns `3`, and an operational or internal failure returns `1`. Exit code `4` remains reserved for an `analyze` implementation that explicitly reports `not_implemented`; the default CLI does not use that placeholder.

## Local analysis behavior

The first runnable workflow is deliberately narrow and deterministic:

```text
local directory
      |
      v
bounded metadata discovery
      |
      v
filename-based technology detection
      |
      v
conservative readiness evaluation
      |
      v
versioned report -> text or JSON
```

The analyzer:

- walks one local root using directory entries and file metadata only;
- retains at most 2,000 files and 500 directories, traverses at most 20 levels, accepts at most 2,500 entries per directory, retains at most 50 discovery issues, and uses a five-second discovery deadline;
- skips VCS metadata plus common generated dependency and tool-state directories (`.gradle`, `.pnpm`, `.terraform`, `.venv`, `.yarn`, `node_modules`, and `venv`), symbolic links, junction-like irregular entries, and other irregular files;
- returns sorted slash-separated paths relative to the selected root;
- matches 124 filename-marker rules across 15 categories, including 18 common backend languages, three runtimes, 23 dependency managers, build systems, containers, orchestration, infrastructure as code, configuration management, CI/CD, GitOps, observability, networking, security, secrets, and cloud tooling;
- retains at most 20 evidence paths for each detected technology and reports when additional evidence was truncated;
- checks for a root README, root license, root `.gitignore`, supported test markers for detected code ecosystems, and supported CI/CD configuration;
- suppresses all absence-based readiness findings when discovery is partial, because skipped paths make absence untrustworthy;
- maps access failures and reached limits to structured report diagnostics instead of silently presenting an incomplete scan as complete.

The detector reports direct filename evidence only. A marker does not prove that a technology is installed, correctly configured, secure, used in production, or applicable to every project in a monorepository. See the complete [technology catalog](docs/architecture/detection.md) and [readiness rules](docs/architecture/readiness.md).

The [project and workspace boundary model](docs/architecture/project-model.md) recognizes nested code, infrastructure, mixed-project, and workspace roots from the same bounded inventory. The [manifest analysis stage](docs/architecture/manifest-analysis.md) safely parses `go.mod`, `go.work`, `package.json`, `pyproject.toml`, `Cargo.toml`, `composer.json`, and `pom.xml` into normalized direct declarations. Explicit Go, Node.js, and Cargo workspace declarations remain distinguishable from an absent declaration even when they contain no members. The [topology stage](docs/architecture/topology.md) associates both models, resolves repository-confined workspace members, and identifies direct local dependency edges. `iatros topology` exposes those normalized facts through its separate `repository_topology` schema `0.1`; it does not change the established `iatros analyze` schema.

IATROS uses the same provider-neutral core for small projects and large company monorepositories. Manifest limits are injectable rather than fixed product ceilings, with validated conservative and large-repository profiles. Parser backends are replaceable behind central path, byte, cancellation, normalization, redaction, determinism, and diagnostic contracts, so a measured large-file workload can adopt a specialized streaming or third-party implementation without changing domain semantics.

### Report contract

Text and JSON are two renderings of the same report. The current JSON schema is `0.1`; its top-level fields are always present:

```json
{
  "schema_version": "0.1",
  "status": "completed",
  "target": {
    "kind": "local_directory",
    "path": "."
  },
  "summary": {
    "directories_scanned": 3,
    "files_scanned": 7,
    "ecosystems_detected": 3,
    "findings_total": 0
  },
  "ecosystems": [
    {
      "id": "github-actions",
      "category": "ci_cd",
      "evidence": [
        ".github/workflows/ci.yml"
      ],
      "evidence_truncated": false
    },
    {
      "id": "go",
      "category": "language",
      "evidence": [
        "go.mod",
        "main.go",
        "main_test.go"
      ],
      "evidence_truncated": false
    },
    {
      "id": "go-modules",
      "category": "dependency_manager",
      "evidence": [
        "go.mod"
      ],
      "evidence_truncated": false
    }
  ],
  "findings": [],
  "diagnostics": []
}
```

`status` is `completed` when the bounded scan finishes, `partial` when a safe limit or recoverable access problem omits data, and `failed` when no trustworthy result can be produced. `not_implemented` remains part of the stable contract for future analyzer implementations that are unavailable by design. Results contain no timestamp, use deterministic ordering, and expose the target only as `.`.

### Topology report contract

`iatros topology` uses a separate schema `0.1` with `report_type: repository_topology`. Its always-present top-level fields are `schema_version`, `report_type`, `status`, `target`, `summary`, `projects`, `workspaces`, `dependencies`, and `diagnostics`. Nested collections are arrays even when empty. The summary counts included projects, workspaces, unique manifest components, and direct dependencies by `internal`, `unresolved`, or `ambiguous` resolution.

The report includes normalized component identity and constraints, an explicit `workspace_declared` flag for each component, workspace containment and declarations, and direct dependency edges. An explicitly empty workspace therefore remains visible without inventing a member. The report does not include raw manifest content, transitive dependency resolution, installed-package state, or proof that a build succeeds. Text and JSON describe the same model. Both omit timestamps and absolute paths and use deterministic, root-relative ordering.

See the complete [repository topology contract](docs/architecture/topology.md#10-public-cli-contract) for field meanings, resolution states, limits, and partial-result behavior.

### Current safety boundary

`iatros analyze` remains metadata-only and does not open regular files. `iatros topology` may open only exact registered manifest filenames through a confined root and independent byte limits. Neither command executes repository code or detected tools, installs dependencies, contacts providers, performs DNS or network requests, or modifies the target. Both reject unsafe targets. Root and nested `.gitignore` patterns are not interpreted yet; implementing only part of Git's matching rules could hide relevant evidence.

User-selectable or Enterprise-calibrated profiles, full `.gitignore` semantics, remote repositories, AI analysis, vulnerability scanning, artifact generation, and state-changing operations are deferred. The approved behavior and remaining decisions are documented in [PS-0001](docs/product/0001-local-repository-analysis.md).

## Contributing

Contributions, design feedback, and focused proposals are welcome. Read the [contribution guide](CONTRIBUTING.md) before making changes. Because the project is still defining its foundations, please [open an issue](https://github.com/kVinsom/Iatros/issues) before starting a substantial implementation so the scope and architectural impact can be discussed first.

### Go code style

Go code follows the standard [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [Go Doc Comments](https://go.dev/doc/comment) conventions. Imports use their declared package names by default; aliases are reserved for real name mismatches or collisions, and routine imports do not receive comments. Package and exported API comments document contracts, while implementation comments explain intent, constraints, or non-obvious safety decisions. The complete repository rules are in the [contribution guide](CONTRIBUTING.md).

When proposing a change:

- clearly distinguish implemented behavior from future plans;
- keep changes focused and explain important trade-offs;
- preserve the dependency boundaries described above;
- keep vendor-specific behavior in the appropriate plugin area;
- add tests and documentation together with behavior once implementation begins;
- never commit credentials, local environment files, or generated build artifacts.

## Author

IATROS DevOps was created by Nikolas (Mykola) Kachmaryk. See [About the Author](ABOUT_THE_AUTHOR.md) for background and contact information.

## License

Licensed under the [Apache License 2.0](LICENSE).
