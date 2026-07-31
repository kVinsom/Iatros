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
> **Project status:** Pre-implementation / architecture scaffold. This repository currently defines the project vision, identity, and intended system boundaries. It does not yet contain a runnable CLI, services, SDK, plugins, tests, or an installation procedure. Every capability described below is planned, not released.

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

These items describe the intended product direction. They are not available in the current repository state.

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
| [`docs/`](docs/) | Architecture, security, testing, decision records, and future product documentation. |
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
| Target architecture, security, and testing documentation | Present |
| Application source and Go module | Not yet added |
| Runnable CLI, services, and applications | Not yet available |
| API, SDK, plugins, and extensions | Directory placeholders only |
| Automated tests and CI workflows | Not yet added |
| Published releases | Not yet available |

## Getting started

You can clone the scaffold to inspect the project direction and repository boundaries:

```bash
git clone https://github.com/kVinsom/Iatros.git
cd Iatros
```

There is currently no software to build, install, or run. Prerequisites and verified build, test, and execution commands will be documented here when the first implementation is introduced.

## Contributing

Contributions, design feedback, and focused proposals are welcome. Because the project is still defining its foundations, please [open an issue](https://github.com/kVinsom/Iatros/issues) before starting a substantial implementation so the scope and architectural impact can be discussed first.

When proposing a change:

- clearly distinguish implemented behavior from future plans;
- keep changes focused and explain important trade-offs;
- preserve the dependency boundaries described above;
- keep vendor-specific behavior in the appropriate plugin area;
- add tests and documentation together with behavior once implementation begins;
- never commit credentials, local environment files, or generated build artifacts.

A dedicated contribution guide and development workflow will be added when the project has runnable code and established tooling.

## Author

IATROS DevOps was created by Nikolas (Mykola) Kachmaryk. See [About the Author](ABOUT_THE_AUTHOR.md) for background and contact information.

## License

Licensed under the [Apache License 2.0](LICENSE).
