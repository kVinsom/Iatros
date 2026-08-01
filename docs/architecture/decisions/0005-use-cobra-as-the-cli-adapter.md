# ADR-0005: Use Cobra as the CLI adapter

- **Status:** Accepted
- **Date:** 2026-08-01
- **Supersedes:** None
- **Superseded by:** None

## Context

IATROS starts with a local command-line surface and is expected to gain additional commands, nested workflows, contextual help, version reporting, and shell completion. The CLI framework must support that growth without becoming a dependency of provider-neutral analysis and domain packages.

The project also needs deterministic output, explicit exit-code ownership, injected input and output streams for tests, context propagation, and no hidden process termination below the composition root.

## Decision

Use [Cobra](https://github.com/spf13/cobra) as the command parsing and help framework for the IATROS CLI.

- Pin Cobra through the root Go module rather than installing or depending on the `cobra-cli` generator.
- Confine Cobra types to the CLI adapter under `internal/cli` and the `cmd/iatros` composition path.
- Keep analysis models and behavior independent from Cobra.
- Construct a fresh command tree with explicit dependencies and I/O streams; do not use package-level command state or registration through `init()`.
- Propagate `context.Context` into command handlers and application services.
- Keep report formatting and IATROS exit-code mapping under project control instead of relying on framework defaults.
- Do not add Viper until IATROS has an approved configuration contract that requires layered files, environment variables, or other configuration sources.
- If Viper is added later, isolate it behind a configuration adapter so domain packages do not depend on it.

Cobra uses the Apache License 2.0, which is compatible with the IATROS project license. Release packaging must preserve any notices required by Cobra and its transitive dependencies.

## Consequences

### Positive

- Mature command, flag, help, suggestion, and shell-completion behavior.
- A familiar CLI experience for DevOps users.
- A command tree that can grow without custom parsing infrastructure.
- Testable commands through injected dependencies and streams.
- Framework replacement remains bounded to the CLI adapter.

### Trade-offs

- Cobra and its transitive packages become production dependencies.
- Exact help and parsing behavior can change when Cobra is upgraded, so upgrades require contract tests.
- Maintaining isolation requires discipline because Cobra makes global command patterns easy to create.

## Alternatives considered

### Kong

Kong provides a concise declarative model and dependency binding. It was not selected because Cobra has a broader DevOps ecosystem and more familiar command-tree conventions for the anticipated IATROS surface.

### urfave/cli

urfave/cli is mature and commercially usable, but it did not provide a decisive advantage over Cobra for the planned command hierarchy.

### Go standard library only

The standard `flag` package would minimize dependencies but would require IATROS to build and maintain nested commands, help, suggestions, and completion behavior.

## Implementation validation

- `cmd/iatros` remains a thin composition root.
- Only the CLI adapter imports Cobra.
- Core report types and the local analysis stub compile without Cobra imports.
- Tests cover help, version, parsing, output formats, target validation, determinism, and exit-code mapping.
- `go build ./...`, `go vet ./...`, and `go test ./...` pass from the root module.

## References

- [PS-0001: Local repository analysis](../../product/0001-local-repository-analysis.md)
- [ADR-0001: Capability boundaries and dependency direction](0001-capability-boundaries-and-dependency-direction.md)
- [ADR-0003: Start with one Go module](0003-start-with-one-go-module.md)
- [Cobra repository](https://github.com/spf13/cobra)
