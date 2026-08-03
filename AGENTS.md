# IATROS Repository Instructions

These instructions apply to the entire repository.

## Language

- Keep source code, comments, documentation, commit suggestions, diagnostics, and user-visible output in English.

## Readability and naming

- Avoid variable shadowing.
- Reduce nesting with early returns and keep the main successful path flush left.
- Choose names that explain purpose. Do not use context-free names such as `data`, `item`, `obj`, or `value`.
- Do not create catch-all packages named `utils`, `common`, `helpers`, or `shared`; place behavior with the capability that owns it.
- Use short names only in small scopes, and avoid abbreviations unless they are widely understood.
- Name booleans as assertions, such as `isValid`, `hasAccess`, or `canRetry`.
- Use one term for one concept throughout the project, and do not reuse the same term for unrelated concepts.
- Replace magic numbers and unclear string literals with meaningfully named constants.
- Remove dead, unreachable, and commented-out code; Git preserves history.
- Keep related types and functions close together.
- Do not mix different levels of abstraction in one code fragment.
- Avoid overly long lines and expressions. Extract a named condition or function when it makes complex logic easier to understand.
- Avoid double negatives.
- Prefer simple, explicit code over clever code.
- Do not trade readability for an optimization unless measurement demonstrates a relevant need.

## Go import policy

- Do not rename imports by default.
- Add an import alias only when the declared package name differs from the final import-path element, when two imports have a direct name collision, or when the unaliased name would be genuinely ambiguous.
- When resolving a collision, prefer renaming the most local or project-specific package and choose a concise semantic alias.
- Do not add comments to routine imports.
- Separate standard-library, first-party, and third-party imports with blank lines.
- Do not use dot imports.
- Use blank imports only when registration through side effects is required, and explain that requirement in the import comment.
- Avoid names that create ambiguity between packages and local variables.
- Generated Go files are exempt; change their generator or template instead of editing generated imports manually.

## Go comment policy

- Add a package comment once per package.
- Document exported declarations with complete English sentences that begin with the declaration name when practical.
- Use implementation comments to explain intent, constraints, safety properties, or a non-obvious choice rather than restating the code.
- Do not comment obvious assignments, calls, control flow, or routine imports.
- Do not add positional section comments such as `// Helpers`.
- Keep comments accurate when behavior changes and remove comments that no longer add information.

## Functions, methods, and interfaces

- Keep each function focused on one logical operation and one level of abstraction.
- Extract a function when a coherent part has a useful name, but do not fragment code into tiny functions that make the flow harder to follow.
- Keep argument lists small. Group genuinely related arguments in a purpose-specific struct.
- Avoid boolean flag arguments; prefer separate operations or an explicit option type when the behaviors are meaningfully different.
- Make side effects visible in the function contract. Do not mutate input values unless ownership and mutation are clear to the caller.
- Do not use global state as a hidden argument.
- Separate commands that change state from queries that return data when practical.
- Accept `io.Reader` or `io.Writer` when an operation needs a stream rather than a specific file, HTTP response, or storage implementation.
- Accept interfaces and return concrete types when doing so keeps the API useful and clear.
- Define interfaces near the consuming code, keep them small, and create them only for an existing boundary or test seam. Do not create one interface per struct or speculative interfaces for future use.
- Use functional options only when configuration complexity and API evolution justify them.
- Validate untrusted external data at the system boundary; do not repeat identical validation at every layer.
- Do not expose mutable internal collections unless shared ownership is an intentional part of the contract.
- Avoid named result parameters when they make a function harder to read, and avoid naked returns in long functions.
- Use pointer or value receivers consistently for a type unless a specific method requires a documented exception.

## Architecture and quality

- Organize packages around business or product capabilities. Prefer specific capability names over generic layer-only names such as `service` or `repository`.
- Keep transport, persistence, and business rules separate. Do not pass `http.Request`, `sql.Rows`, ORM models, or third-party SDK models into business logic.
- Convert external representations into internal models at system boundaries. Do not force HTTP, database, and domain concerns into one universal model.
- Domain and use-case packages must not import HTTP frameworks, database drivers, CLI frameworks, or provider-specific integrations.
- Direct dependencies toward stable business logic and isolate third-party APIs behind adapters.
- Inject dependencies through constructors. Create concrete implementations in the composition root; do not use a service locator and minimize global singletons.
- Keep `cmd/*` entry points thin. Leave configuration, dependency assembly, lifecycle management, and startup in `main`; keep provider-neutral behavior outside it.
- Use `internal` for non-public implementation packages and export declarations only when another package genuinely needs them.
- Do not introduce cyclic package dependencies or a separate package for one trivial type.
- Keep architecture proportional to current complexity. Start simple, split only for concrete reasons, and do not add multiple layers or a microservice merely to separate code.
- Keep configuration at a high system level and postpone framework or storage choices until requirements make them necessary.
- Enforce architectural boundaries with package structure and the compiler where possible.
- Document important architectural decisions and the reasons behind them.
- Design core capabilities for both small repositories and very large company-scale repositories; do not bake one repository size into domain contracts.
- Express file, byte, entry, depth, time, and memory budgets as validated injectable limits with conservative defaults and larger selectable profiles.
- Keep parser and processing backends replaceable behind consumer-owned interfaces. Standard-library implementations may be the default, but maintained third-party or specialized streaming implementations are allowed when requirements, benchmarks, file sizes, or format fidelity justify them.
- Require every third-party backend to preserve the same normalized contracts, safety limits, cancellation, deterministic behavior, diagnostics, and cross-platform expectations as the default implementation.
- Preserve user changes that are unrelated to the active task.
- Format changed Go files with `gofmt`.
- Run `go test ./...`, `go vet ./...`, and `go build ./...` after changing Go code.
- Update documentation when implemented behavior, architecture, or contributor requirements change.

## Errors and resource ownership

- Do not ignore returned errors. When an error is intentionally discarded, make that choice explicit and explain a non-obvious reason.
- Add useful operation context when returning an error and wrap underlying errors with `%w` when callers may need the chain.
- Use `errors.Is` to match error values and `errors.As` to match error types. Do not compare error text or make it part of a business contract.
- Handle an error at one appropriate level. Do not log the same error at every layer; log it at a system boundary that has enough context.
- Use `panic` only for programmer errors or conditions that make startup impossible, never for routine business errors.
- Do not return a typed `nil` inside an interface.
- Check errors from deferred operations when those errors can affect correctness.
- Close every acquired `io.Closer` owned by the current code path, and make ownership explicit when it is transferred.
- Check `rows.Err()` after iterating over SQL rows.
- Ensure every transaction reaches `Commit` or `Rollback` on every path.
- Configure timeouts for HTTP clients and servers.
- Give outbound and potentially unbounded operations a `context.Context` and a bounded lifetime.

## Go data and language rules

- Avoid excessive use of `any`; use generics only for a concrete, repeated need.
- Use type embedding deliberately and review the promoted API it creates.
- Do not copy values containing `sync.Mutex`, `sync.WaitGroup`, or other non-copyable synchronization state.
- Remember that a `range` element may be a copy. Mutate slice elements by index when the stored value must change.
- Do not rely on map iteration order, and do not read and write a regular map concurrently without synchronization.
- Distinguish a `nil` slice from an empty slice only when an API or encoding contract requires it. Use `len(slice) == 0` for an emptiness check and avoid inventing different business meanings without a requirement.
- Remember that slices can share a backing array. Copy a slice when independent ownership is required, and use `append` carefully when aliases may exist.
- Preallocate capacity only when the size is reliably known or measurement shows a benefit.
- Remember that string indexes are byte offsets. Decode runes when logic operates on Unicode code points rather than bytes.
- Use `strings.Builder` for repeated string construction when it avoids unnecessary allocations, and avoid needless conversions between `string` and `[]byte`.
- Do not place `defer` directly inside a long-running loop when resources need to be released per iteration; move that iteration into a function or close the resource explicitly.
- Minimize `init` usage. Do not perform complex logic or I/O in `init`.

## Concurrency

- Add concurrency only for a concrete requirement, and do not assume it improves performance without benchmarks.
- Define how every goroutine terminates; do not start uncontrolled or immortal goroutines.
- Propagate cancellation with `context.Context`. Pass it as the first parameter, do not pass `nil`, do not store it in a struct without a specific lifecycle reason, and call every cancel function returned by `context.WithCancel`, `context.WithTimeout`, or `context.WithDeadline`.
- Use channels for coordination or ownership transfer and mutexes for protecting shared state. Do not choose channels merely because the implementation is in Go.
- Derive channel buffer sizes from the protocol or measurement; do not guess them. Use an unbuffered channel when a rendezvous is required.
- Do not rely on `select` choosing cases in a particular order.
- Call `sync.WaitGroup.Add` before starting the corresponding goroutine.
- Use `errgroup` when a group of goroutines needs coordinated cancellation and error propagation.
- Protect shared slices and maps from concurrent access.
- Do not use `time.Sleep` as a synchronization mechanism.
- Run the race detector for concurrency-sensitive changes and regularly in CI.

## Testing, verification, and performance

- Test observable behavior rather than internal implementation details.
- Keep business-logic unit tests independent from HTTP, real databases, and the network.
- Separate unit and integration tests clearly.
- Use table-driven tests for related scenarios and name cases after the expected behavior.
- Cover success, error, empty, boundary, cancellation, and resource-cleanup paths as applicable.
- Do not use arbitrary delays in tests. Inject a controllable clock when time affects behavior.
- Use `t.Cleanup` for test-owned resources and `httptest` for HTTP clients and handlers.
- Use test shuffling to detect order dependencies, and never require tests to run in a particular order.
- Avoid excessive mocking; difficult setup can indicate an architectural boundary problem.
- Treat coverage as a way to find important untested behavior, not as proof of quality or a goal of 100 percent by itself.
- Run formatters, `go vet`, configured linters, tests, and builds in CI.
- Measure performance before optimizing, and verify with benchmarks that an optimization improves the relevant result.
