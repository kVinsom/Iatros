# Contributing to IATROS

Thank you for helping improve IATROS. The project is still establishing its implementation baseline, so focused changes with explicit behavior and verification are preferred.

## Before starting

1. Read the [target architecture](docs/architecture/README.md) and relevant [architecture decisions](docs/architecture/decisions/README.md).
2. Open an issue before beginning a substantial change that affects product scope, public contracts, security boundaries, or long-term architecture.
3. Keep all repository content and user-visible output in English.
4. Never commit credentials, local environment files, generated binaries, coverage output, or machine-specific state.

## Development baseline

Use Go 1.25.5 or newer, then verify the repository from its root:

```bash
go test ./...
go vet ./...
go build ./...
```

Format every changed Go file with `gofmt` before submitting it.

## Go imports and comments

IATROS follows the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [Go Doc Comments](https://go.dev/doc/comment) conventions.

Import rules:

- use the package's declared name without an alias by default;
- add an alias only when the package name differs from its path, two imports collide, or the default name is genuinely ambiguous;
- resolve collisions with a concise semantic alias, preferring to rename the most local or project-specific import;
- separate standard-library, first-party, and third-party imports with blank lines;
- do not comment routine imports or use dot imports;
- use a blank import only for a required side effect and explain that requirement;
- change generators or templates instead of manually editing generated Go files.

Comment rules:

- introduce each package with one package comment;
- document exported declarations with complete English sentences;
- explain intent, constraints, safety properties, and non-obvious decisions;
- do not restate obvious code, assignments, calls, or control flow;
- update or remove comments when the behavior they describe changes.

## Architecture expectations

- Keep composition roots under `cmd/*` thin.
- Keep provider-neutral domain and application behavior out of Cobra commands and provider plugins.
- Keep Cobra confined to the CLI adapter.
- Add Viper only after an approved configuration contract exists, and isolate it behind a configuration adapter.
- Keep resource limits injectable and validated so the same core can support conservative local defaults and larger company-scale profiles.
- Apply limits with deterministic selection, check cancellation between stages and periodically inside large bounded loops, and avoid allocating directly from an untrusted configurable maximum.
- Place parser or processing implementations behind consumer-owned interfaces when file size, throughput, or format requirements may require a specialized or third-party backend.
- Clone mutable slices and maps received across replaceable boundaries before sorting, normalizing, redacting, or truncating them unless the contract explicitly transfers ownership.
- Add a third-party backend only with evidence for the requirement, license and maintenance review, relevant benchmarks, and conformance tests against the provider-neutral contract.
- Extract shared helpers when behavior and invariants are genuinely identical; keep coincidentally similar domain rules in their owning packages.
- Record consequential or difficult-to-reverse choices in an architecture decision record.

## Tests and documentation

- Add or update tests at the lowest useful layer for every behavior change.
- Cover relevant failure, cancellation, privacy, and deterministic-output cases.
- Keep the README and product specifications aligned with behavior that is actually runnable.
- Verify local Markdown links when documentation changes.

## Pull request scope

Keep pull requests focused. Describe the problem, the chosen approach, important trade-offs, verification performed, and any intentionally deferred work.
