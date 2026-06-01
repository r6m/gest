# Gest Implementation Tasks

This roadmap is the implementation source of truth. The framework should be built as a sequence of small, testable vertical slices rather than as the full design document at once.

Status legend:

- `Planned`: Not started.
- `In Progress`: Actively being implemented.
- `Blocked`: Needs a decision or prerequisite.
- `Done`: Implemented, tested, and documented enough for the next phase.

## CTO Decisions

- First-party runtime starts with Chi/net-http only.
- No Fiber adapter in the core roadmap.
- No built-in database module. Users bring their own database packages and expose them through normal Gest modules/providers.
- No Uber Fx in the user-facing API. A simple internal DI container comes first.
- Lazy modules, dev server, OpenAPI, WebSockets, queues, scheduler, tracing, metrics, and other ecosystem modules are deferred until the core framework is proven.
- Build every phase around a working vertical slice, tests, and clear errors.
- Every non-documentation implementation task must include tests and pass lint before it is marked `Done`.

## Non-Negotiable Engineering Rules

These rules apply to every task in this file.

1. Keep generated code boring.
   - Generated files must be readable, deterministic, gofmt-formatted Go.
   - Generated code must call normal public APIs.
   - Do not use hidden global registries, `init()` registration, runtime source scanning, or reflection-only route discovery.

2. Keep runtime separate from tooling.
   - Runtime packages must not import generator, AST parser, CLI, filesystem scanner, or config-loader packages.
   - Runtime accepts explicit modules, providers, controller definitions, and handlers.
   - Hand-written `GestController()` metadata must keep working.

3. Treat errors as API.
   - Add specific errors with actionable messages.
   - Include module, provider, route, file, and line context when available.
   - Do not use panics for expected user mistakes.

4. Keep DI conservative.
   - Implement singleton constructor injection first.
   - Do not implement request scope, transient scope, global service locator patterns, or runtime container lookups in user-facing examples.

5. Keep decorators small.
   - Only simple line-based comments are allowed.
   - Do not add object literal syntax, nested config, arbitrary expressions, or decorator import package scanning until explicitly planned.

6. Preserve escape hatches.
   - Expose raw net-http request/response access.
   - Support ordinary Go tests and ordinary Go builds.
   - Keep user-owned modules and hand-written generated metadata viable.

7. Guard scope aggressively.
   - Do not implement deferred features while working on earlier phases.
   - If a task requires a deferred feature, mark it blocked and explain the dependency instead of expanding scope.

8. Require tests.
   - Every implementation task must add or update tests unless it is documentation-only.
   - Runtime changes need unit tests and integration tests when behavior crosses package or HTTP boundaries.
   - Generator changes need fixture-based tests for generated output, stable output, and diagnostics.
   - CLI changes need command tests for exit codes, output, config defaults, and failure cases.
   - A task cannot be marked `Done` unless `go test ./...` passes or a concrete blocker is documented.

9. Require lint.
   - The root `.golangci.yml` is the lint contract.
   - A task cannot be marked `Done` unless `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.
   - Do not add blanket lint disables. Fix the code or document a narrow exception.

## Phase 0: Product Scope And Architecture Baseline

Goal: make the MVP contract explicit before implementation starts.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P0.1 | Done | Define MVP surface | Document the v0 promise: modules, providers, singleton DI, generated controller metadata, typed JSON handlers, Chi adapter, basic context, CLI generate/build. |
| P0.2 | Done | Define package layout | Choose package boundaries for `gest`, `router/chiadapter`, generator internals, and `cmd/gest`. Keep runtime independent from generator code. |
| P0.3 | Done | Define error model | Specify framework error types, HTTP status mapping, validation errors, DI errors, route conflicts, and generator diagnostics. |
| P0.4 | Done | Define test strategy | Establish unit tests for DI/generator/binding and integration tests for a tiny app served through Chi. |
| P0.5 | Done | Create example target app | Add a minimal example app that will become the acceptance fixture for all phases. |
| P0.6 | Done | Define generated-code contract | Document the exact shape and restrictions for `*_gest.gen.go`: no `init()`, no hidden registries, deterministic output, public runtime calls only. |
| P0.7 | Done | Define architecture dependency rules | Document package import rules that prevent runtime packages from depending on generator, CLI, config loading, or filesystem scanning. |
| P0.8 | Done | Define lint contract | Keep `.golangci.yml` at the repository root and document that `rtk proxy golangci-lint run ./...` is required for done work. |

Exit criteria:

- A short architecture note exists.
- The MVP surface is smaller than the full design doc.
- The example target app is defined, even if it does not compile yet.
- Generated-code and package-dependency rules are documented.
- Test and lint requirements are documented.

## Phase 1: Core Runtime

Goal: run an app with hand-written controller metadata before writing the generator.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P1.1 | Done | Implement module API | Add `Module`, `ModuleConfig`, `NewModule`, `Imports`, and `Providers`. Support `Name`, `Global`, `Imports`, `Providers`, and basic boot hooks only if needed. Defer `Lazy`. |
| P1.2 | Done | Implement provider API | Add `Provider`, `Provide`, `Controller`, `Value`, `Export`, `Name`, `As`, and `WithScope`. Implement only singleton scope first; reject or ignore unsupported scopes with clear errors. |
| P1.3 | Done | Implement token model | Add `Token`, `TokenOf[T]`, and named tokens for advanced cases. Keep normal APIs constructor-oriented. |
| P1.4 | Done | Implement DI container | Support constructor injection, singleton caching, value providers, imports/exports, missing dependency errors, and cycle detection. |
| P1.5 | Done | Implement app bootstrap | Add `App`, `New`, `Import`, route registration, provider initialization, and `Listen`. |
| P1.6 | Done | Implement controller definitions | Add `DescribedController`, `ControllerDefinition`, `RouteDefinition`, `RouteMetadata`, and route runtime config types. |
| P1.7 | Done | Implement Chi adapter | Add the first-party Chi/net-http adapter with groups, route handling, middleware registration, and server startup. |
| P1.8 | Done | Implement context | Add `Context` helpers for params, query, headers, bearer token, JSON, no-content, storage, native request/response escape hatches. |
| P1.9 | Done | Implement framework errors | Add `BadRequest`, `NotFound`, `Unauthorized`, `Forbidden`, `Internal`, and HTTP response mapping. |
| P1.10 | Done | Add runtime tests | Cover module graph, DI resolution, exports/imports, route registration, context helpers, and error responses. |

Exit criteria:

- A hand-written `GestController()` can serve JSON through Chi.
- `go test ./...` passes.
- Missing providers and duplicate routes produce useful errors.
- Runtime packages do not import generator, CLI, filesystem scanning, AST parser, or config-loader packages.
- No request scope, transient scope, lazy modules, OpenAPI, or dev server behavior exists in Phase 1.
- Runtime work includes tests for success and failure paths.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 2: Generator MVP

Goal: generate `GestController()` methods from simple comments.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P2.1 | Done | Add generator package scanner | Scan Go packages under the configured root and ignore generated files, vendor, `.git`, and `.gest`. |
| P2.2 | Done | Parse controller decorators | Support only `@Controller("path")` and optional `@Tag("name")` on types. |
| P2.3 | Done | Parse route decorators | Support `@Get`, `@Post`, `@Put`, `@Patch`, `@Delete`, `@Status`, `@Summary`, and `@Description`. |
| P2.4 | Done | Validate handler signatures | Accept only initial runtime-supported signatures. Produce file/line diagnostics for invalid signatures. |
| P2.5 | Done | Generate metadata files | Emit deterministic `*_gest.gen.go` files with `GestController()` methods and route definitions. |
| P2.6 | Done | Format generated code | Run gofmt on generated files and avoid noisy output when files are unchanged. |
| P2.7 | Done | Add generator tests | Use fixture packages to test parser behavior, generated output, invalid decorators, and invalid signatures. |

Deferred:

- `@Auth`, `@Roles`, `@Permissions`, `@Use`, `@Cache`, `@Throttle`, `@Stream`, `@WebSocket`, import alias resolution, processors, cron jobs.

Exit criteria:

- A controller with MVP decorators generates compilable metadata.
- Generator errors point to concrete files and lines.
- Generated files contain no `init()` functions and no hidden registration side effects.
- Running the generator twice without source changes produces identical output.
- Generator work includes fixture tests for successful generation and diagnostics.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 3: Typed JSON Handlers And Binding

Goal: make the preferred controller style work end to end.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P3.1 | Done | Implement `gest.JSON` | Support typed handlers returning `(*Res, error)` or `error`. Map nil responses to no-content. |
| P3.2 | Planned | Implement request binding | Bind `param`, `query`, `header`, and JSON body tags into request DTO structs. |
| P3.3 | Planned | Add default values | Support simple `default` tags for query/header fields where conversion is unambiguous. |
| P3.4 | Planned | Add validation hook | Integrate an optional validator behind `Context.Validate`; keep validation module optional. |
| P3.5 | Planned | Add type conversion | Convert strings into common scalar types and return useful binding errors. |
| P3.6 | Planned | Expand generator handler output | Generate `gest.JSON(c.Method, gest.Status(...))` wrappers for typed JSON routes. |
| P3.7 | Planned | Add binding tests | Cover params, query, headers, JSON body, defaults, conversion failures, and validation failures. |

Deferred:

- Multipart, file uploads, custom binders, streaming handlers, WebSocket handlers.

Exit criteria:

- The example app can implement a typed DTO route without hand-written binding.
- Bad input returns stable 400 responses with actionable messages.
- Binding does not require global state or a database/cache/auth module.
- Binding work includes tests for valid input, invalid input, and conversion failures.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 4: CLI MVP

Goal: provide the minimum CLI needed for generation and production builds.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P4.1 | Planned | Add `cmd/gest` | Create the CLI entrypoint and command structure. |
| P4.2 | Planned | Implement config loading | Load `gest.yaml` with defaults for entry, generate root, router adapter, and build output. |
| P4.3 | Planned | Implement `gest generate` | Wire the generator to the CLI, print concise colored output, and return non-zero on validation failure. |
| P4.4 | Planned | Implement `gest build` | Run generate, validate, optional tests, then `go build`. Keep the underlying Go command visible. |
| P4.5 | Planned | Implement basic generators | Add `gest g module`, `gest g controller`, and `gest g service`. Prefer AST edits for parent module updates. |
| P4.6 | Planned | Add CLI tests | Cover command parsing, config defaults, dry-run generation, and failure output. |

Deferred:

- `gest dev`, resource generator, guard/middleware/interceptor/pipe generators, Docker builds.

Exit criteria:

- A generated small app can run `gest generate` and `gest build`.
- CLI output is concise and diagnostics are copy-paste useful.
- `gest build` prints or clearly reports the underlying `go build` command it runs.
- CLI packages do not leak into runtime imports.
- CLI work includes command tests for success and failure cases.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 5: OpenAPI And Swagger

Goal: generate useful API metadata after handlers and DTOs are stable.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P5.1 | Planned | Add OpenAPI registry | Collect route metadata during app bootstrap or generation without coupling runtime to Swagger UI. |
| P5.2 | Planned | Generate DTO schemas | Infer request/response schemas from DTO structs, JSON tags, validation tags, and status metadata. |
| P5.3 | Planned | Add OpenAPI module | Provide `openapi.Module` or app-level `OpenAPI("/openapi.json")`. |
| P5.4 | Planned | Add Swagger module | Serve Swagger UI outside core runtime as an optional module. |
| P5.5 | Planned | Add OpenAPI validation tests | Verify stable output for the example app and common DTO shapes. |

Exit criteria:

- The example app exposes a valid OpenAPI document.
- Swagger UI is optional and does not expand the core runtime.
- OpenAPI work includes schema and route metadata tests.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 6: Developer Experience

Goal: improve daily workflow once generation and build behavior are reliable.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P6.1 | Planned | Implement `gest dev` | Watch files, debounce changes, regenerate, rebuild, and restart. Keep the previous process alive on build failure. |
| P6.2 | Planned | Add boot logs | Print modules, providers, routes, OpenAPI path, listen address, and boot timing. Support production JSON logs later. |
| P6.3 | Planned | Add lifecycle hooks | Implement init/bootstrap/shutdown interfaces after the eager module model is stable. |
| P6.4 | Planned | Add `gesttest` | Provide testing helpers around standard Go `testing`, HTTP requests, response assertions, and provider overrides. |
| P6.5 | Planned | Add docs examples | Convert the example app into user-facing guides for modules, controllers, DTOs, CLI, and testing. |

Exit criteria:

- A developer can iterate on the example app with `gest dev`.
- Build failures do not kill the last running app.
- Testing helpers reduce boilerplate without replacing Go testing.
- Developer-experience work includes tests for failure paths where practical.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 7: Optional Official Modules

Goal: add infrastructure modules only after the extension model is proven.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P7.1 | Planned | Config module | Load environment files and expose typed config helpers. |
| P7.2 | Planned | Logger module | Provide a simple structured logger integration and app boot log integration. |
| P7.3 | Planned | Validation module | Package validator integration as an optional module. |
| P7.4 | Planned | Health module | Add `/health`, `/health/live`, and `/health/ready`. |
| P7.5 | Planned | JWT/auth modules | Add optional JWT helpers and guard conventions after guard metadata exists. |
| P7.6 | Planned | Cache/throttle/events modules | Add only after module ergonomics and DI override patterns are proven. |

Explicitly out of scope:

- Built-in database module.
- Built-in ORM abstraction.
- First-party Fiber adapter.

Exit criteria:

- Each official module is optional.
- Users can replace official modules with their own modules without special cases.
- Each optional module has unit tests and at least one integration-style usage test when it exposes runtime behavior.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 8: Advanced Runtime

Goal: add advanced features only after real user feedback.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P8.1 | Planned | Lazy modules | Register routes at boot and initialize providers on first use. Requires strong concurrency and lifecycle tests. |
| P8.2 | Planned | Guards and route metadata | Add `@Auth`, `@Public`, `@Roles`, `@Permissions`, and DI-resolved guard factories. |
| P8.3 | Planned | Import alias resolution | Add explicit `@GestImport` first, then existing Go imports. Defer package scan aliases until there is clear demand. |
| P8.4 | Planned | Streaming | Add stream and SSE helpers while preserving raw `http.ResponseWriter` escape hatches. |
| P8.5 | Planned | WebSockets | Add WebSocket routes and socket abstractions as an optional module. |
| P8.6 | Planned | Queue and scheduler modules | Add job processors and cron/every decorators as optional ecosystem modules. |
| P8.7 | Planned | Metrics and tracing modules | Add observability modules after core middleware and context conventions are stable. |

Exit criteria:

- Advanced features do not make the simple JSON API path harder to understand.
- Every advanced feature has clear opt-in behavior and escape hatches.
- Advanced runtime work includes concurrency, lifecycle, or integration tests appropriate to the feature.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Agent Prompt Template

Use this format when assigning one task to an implementation agent:

```txt
Implement task <ID> from docs/TASKS.md.

Constraints:
- Follow AGENTS.md and .skills/RTK.md; prefix shell commands with rtk.
- Keep the change scoped to the task.
- Add or update tests appropriate to the task.
- Run `rtk go test ./...` and `rtk proxy golangci-lint run ./...`; document exact results.
- Do not implement deferred features.
- Do not introduce hidden registries, init-time route registration, runtime source scanning, or runtime imports of generator/CLI packages.
- Update docs/TASKS.md status only if the task is actually complete.

Acceptance:
- Explain changed files.
- Include verification commands and results.
- Mention any follow-up tasks or blockers.
```
