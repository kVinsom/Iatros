# IATROS Repository Instructions

These instructions apply to the entire repository.

## Language

- Keep source code, comments, documentation, commit suggestions, diagnostics, and user-visible output in English.

## Go import policy

- Do not rename imports by default.
- Add an import alias only when the declared package name differs from the final import-path element, when two imports have a direct name collision, or when the unaliased name would be genuinely ambiguous.
- When resolving a collision, prefer renaming the most local or project-specific package and choose a concise semantic alias.
- Do not add comments to routine imports.
- Separate standard-library, first-party, and third-party imports with blank lines.
- Do not use dot imports.
- Use blank imports only when registration through side effects is required, and explain that requirement in the import comment.
- Generated Go files are exempt; change their generator or template instead of editing generated imports manually.

## Go comment policy

- Add a package comment once per package.
- Document exported declarations with complete English sentences that begin with the declaration name when practical.
- Use implementation comments to explain intent, constraints, safety properties, or a non-obvious choice rather than restating the code.
- Do not comment obvious assignments, calls, control flow, or routine imports.
- Keep comments accurate when behavior changes and remove comments that no longer add information.

## Architecture and quality

- Keep `cmd/*` entry points thin and keep provider-neutral behavior independent from CLI and integration frameworks.
- Design core capabilities for both small repositories and very large company-scale repositories; do not bake one repository size into domain contracts.
- Express file, byte, entry, depth, time, and memory budgets as validated injectable limits with conservative defaults and larger selectable profiles.
- Keep parser and processing backends replaceable behind consumer-owned interfaces. Standard-library implementations may be the default, but maintained third-party or specialized streaming implementations are allowed when requirements, benchmarks, file sizes, or format fidelity justify them.
- Require every third-party backend to preserve the same normalized contracts, safety limits, cancellation, deterministic behavior, diagnostics, and cross-platform expectations as the default implementation.
- Preserve user changes that are unrelated to the active task.
- Format changed Go files with `gofmt`.
- Run `go test ./...`, `go vet ./...`, and `go build ./...` after changing Go code.
- Update documentation when implemented behavior, architecture, or contributor requirements change.
