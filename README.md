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
> **Project status:** Early implementation. The repository contains a runnable Cobra-based CLI contract stub for local repository analysis. The command validates a local directory and returns an explicit `not_implemented` report; it does not scan repository content yet. All broader capabilities remain planned, not released.

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

These items describe the intended product direction. Only the initial CLI contract stub is currently available.

The first approved implementation slice is [local repository analysis](docs/product/0001-local-repository-analysis.md): a local-only, read-only `iatros analyze` command with deterministic text and JSON contracts.

## Architecture and repository layout

The scaffold is organized around explicit product and dependency boundaries.

The complete architecture package is available in [`docs/`](docs/README.md), including the [target architecture](docs/architecture/README.md), [security architecture](docs/architecture/security.md), [testing strategy](docs/architecture/testing.md), and [architecture decision records](docs/architecture/decisions/README.md).

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
| First product specification | Approved; contract stub implemented, analysis in progress |
| Target architecture, security, and testing documentation | Present |
| Application source and Go module | Root module and initial CLI source present |
| Runnable CLI, services, and applications | CLI contract stub available; other runtimes not implemented |
| API, SDK, plugins, and extensions | Directory placeholders only |
| Automated tests and CI workflows | Unit tests present; CI workflows not yet added |
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

Build and run the placeholder on Linux or macOS:

```bash
go build -o ./bin/iatros ./cmd/iatros
./bin/iatros analyze --format json .
```

On Windows PowerShell:

```powershell
go build -o .\bin\iatros.exe .\cmd\iatros
.\bin\iatros.exe analyze --format json .
```

Until internal discovery and marker detection are connected to the CLI analyzer, `analyze` returns the documented JSON or text placeholder and exits with code `4`. The runnable command validates the selected directory but does not invoke those internal capabilities, read repository contents, access the network, or modify files.

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
