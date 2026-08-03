<p align="center">
  <img src="assets/iatros_logo.png" alt="IATROS DevOps logo" width="500">
</p>

<h1 align="center">IATROS DevOps</h1>

<p align="center">
  Provider-neutral DevOps automation with a free local foundation and optional AI assistance.
</p>

<p align="center">
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-Apache%202.0-D22128.svg" alt="Apache License 2.0">
  </a>
</p>

> [!IMPORTANT]
> **Project status:** Early implementation. The MVP uses the free local Basic plan without AI; its current approved user-facing slice is read-only repository analysis. The repository contains a runnable Cobra-based CLI contract stub plus stable private core contracts for project topology, configuration, schema compatibility, and lifecycle results. All broader analysis and deterministic architecture-generation capabilities remain planned until source and tests prove them.

## Overview

IATROS DevOps is intended to help users build, deploy, operate, troubleshoot, and improve software through deterministic workflows with optional AI assistance. The long-term goal is to connect repository context and operational intent with tools that can analyze projects, prepare infrastructure and delivery artifacts, validate proposed changes, and assist with day-to-day operations.

The proposed architecture separates provider-neutral domain logic from vendor-specific integrations. Stable contracts are intended to connect the core, public interfaces, plugins, and optional extensions without coupling the entire system to a particular cloud, source-control platform, observability backend, or AI provider.

## Planned capabilities

- **Project understanding:** discover repositories, model topology, and assess production readiness.
- **Deterministic architecture generation:** use validated templates, rules, catalogs, and open plugins to prepare Docker and supported DevOps architecture locally in Basic.
- **Optional AI assistance:** add cloud AI in Pro or cloud/local AI in Enterprise for planning, generation, risk analysis, diagnosis, and remediation.
- **Delivery and infrastructure generation:** prepare CI/CD pipelines, infrastructure, orchestration, observability, security, container configurations, scripts, and operational documentation.
- **Validation and diagnostics:** check proposed artifacts and provide an IATROS Doctor workflow for actionable findings.
- **Deployment operations:** assist with promotion, rollback, GitOps workflows, and drift detection.
- **Operational insight:** bring together observability, reliability, security, and cost-awareness workflows.
- **Extensible integrations:** create open plugins in every plan and closed private plugins in Enterprise through stable contracts.

These items describe the intended product direction. Only the initial CLI contract stub is currently user-facing; the [stable core domain contracts](docs/product/0002-core-domain-contracts.md) are implemented internally for later workflows.

The [IATROS Product Contract](docs/product/product-contract.md) defines the final product boundary, the Analyze → Plan → Generate → Validate → Deploy → Monitor → Fix lifecycle, and the Basic, Pro, and Enterprise plans. Basic is free, local, and AI-free; Pro adds cloud AI; Enterprise adds cloud or local AI plus closed plugins. It separates long-term commitments from currently released behavior.

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
| [`extensions/`](extensions/) | Reserved Pro and Enterprise product overlays; customer-specific closed plugin source may be distributed separately. |
| [`internal/`](internal/) | Private, provider-neutral application and domain logic. |
| [`plugins/`](plugins/) | Planned open first-party adapters grouped by capability. |
| [`sdk/`](sdk/) | Intended public Go client, plugin and extension contracts, and a plugin-testing toolkit. |
| [`test/`](test/) | Cross-component integration and end-to-end test suites. |

The intended dependency rules are:

1. Maintain one provider-neutral core that remains fully usable through Basic without Pro or Enterprise capabilities; do not duplicate core packages by subscription.
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
| Stable core domain contracts | Private project and workflow schema `1.0` implemented and tested |
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
