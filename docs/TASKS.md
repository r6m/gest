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
- Module imports compose provider sets directly; there is no `gest.Export()`, `gest.Private()`, or module-private provider visibility model.
- Lazy modules, WebSockets, queues, scheduler, tracing, metrics, and other ecosystem modules are deferred until the core framework is proven.
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
| P1.1 | Done | Implement module API | Add `Module`, `ModuleConfig`, `NewModule`, `Imports`, and `Providers`. Support `Name`, `Imports`, `Providers`, and basic boot hooks only if needed. Defer `Lazy` and global semantics until their own alignment task. |
| P1.2 | Done | Implement provider API | Add `Provider`, `Provide`, `Controller`, `Value`, `Name`, `As`, and `WithScope`. Implement only singleton scope first; reject or ignore unsupported scopes with clear errors. |
| P1.3 | Done | Implement token model | Add `Token`, `TokenOf[T]`, and named tokens for advanced cases. Keep normal APIs constructor-oriented. |
| P1.4 | Done | Implement DI container | Support constructor injection, singleton caching, value providers, imported provider sets, missing dependency errors, and cycle detection. |
| P1.5 | Done | Implement app bootstrap | Add `App`, `New`, `Import`, route registration, provider initialization, and `Listen`. |
| P1.6 | Done | Implement controller definitions | Add `DescribedController`, `ControllerDefinition`, `RouteDefinition`, `RouteMetadata`, and route runtime config types. |
| P1.7 | Done | Implement Chi adapter | Add the first-party Chi/net-http adapter with groups, route handling, middleware registration, and server startup. |
| P1.8 | Done | Implement context | Add `Context` helpers for params, query, headers, bearer token, JSON, no-content, storage, native request/response escape hatches. |
| P1.9 | Done | Implement framework errors | Add `BadRequest`, `NotFound`, `Unauthorized`, `Forbidden`, `Internal`, and HTTP response mapping. |
| P1.10 | Done | Add runtime tests | Cover module graph, DI resolution, imported providers, route registration, context helpers, and error responses. |

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

- `@Cache`, `@Throttle`, import alias resolution beyond current rules, processors, cron jobs, and WebSocket gateways. Built-in `@Auth`, `@Roles`, and `@Permissions` are not planned; auth policy is user-owned. `@Stream` and `@WebSocket` are not MVP decorators.

Exit criteria:

- A controller with MVP decorators generates compilable metadata.
- Generator errors point to concrete files and lines.
- Generated files contain no `init()` functions and no hidden registration side effects.
- Running the generator twice without source changes produces identical output.
- Generator work includes fixture tests for successful generation and diagnostics.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 3: Typed Handlers And Binding

Goal: make the preferred controller style work end to end.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P3.1 | Done | Implement `gest.Handle` | Support typed handlers returning `(*Res, error)` or `error`, with or without request DTOs. Map nil responses to no-content. |
| P3.2 | Done | Implement request binding | Bind `param`, `query`, `header`, and JSON body tags into request DTO structs. |
| P3.3 | Done | Add default values | Support simple `default` tags for query/header fields where conversion is unambiguous. |
| P3.4 | Done | Add validation hook | Integrate an optional validator behind `Context.Validate`; keep validation module optional. |
| P3.5 | Done | Add type conversion | Convert strings into common scalar types and return useful binding errors. |
| P3.6 | Done | Expand generator handler output | Generate explicit `gest.Handle*` adapters for typed routes. |
| P3.7 | Done | Add binding tests | Cover params, query, headers, JSON body, defaults, conversion failures, and validation failures. |

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
| P4.1 | Done | Add `cmd/gest` | Create the CLI entrypoint and command structure. |
| P4.2 | Done | Implement config loading | Load `gest.yaml` with defaults for entry, generate root, router adapter, and build output. |
| P4.3 | Done | Implement `gest generate` | Wire the generator to the CLI, print concise colored output, and return non-zero on validation failure. |
| P4.4 | Done | Implement `gest build` | Run generate, validate, optional tests, then `go build`. Keep the underlying Go command visible. |
| P4.5 | Done | Implement basic generators | Add `gest g module`, `gest g controller`, and `gest g service`. Prefer AST edits for parent module updates. |
| P4.6 | Done | Add CLI tests | Cover command parsing, config defaults, dry-run generation, and failure output. |
| P4.7 | Done | Remove provider export API | Remove `gest.Export()`, `Provider.Exported`, exported-provider checks, unexported-provider errors, and generator/template usage. Imported modules should make all providers available. |
| P4.8 | Done | Add `gest new` | Generate a minimal buildable Gest web app with module, service/controller, DTOs, `gest.yaml`, generated metadata, tests, and build/generate workflow. |

Deferred:

- `gest dev`, resource generator, guard/middleware/interceptor/pipe generators, Docker builds.

Exit criteria:

- A generated small app can run `gest generate` and `gest build`.
- `gest new <name>` creates a minimal app that can run `gest generate`, `go test ./...`, and `gest build`.
- Generated apps and generators do not emit `gest.Export()`.
- CLI output is concise and diagnostics are copy-paste useful.
- `gest build` prints or clearly reports the underlying `go build` command it runs.
- CLI packages do not leak into runtime imports.
- CLI work includes command tests for success and failure cases.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 5: OpenAPI And Swagger

Goal: generate useful API metadata after handlers and DTOs are stable.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P5.1 | Done | Add OpenAPI registry | Collect route metadata during app bootstrap or generation without coupling runtime to Swagger UI. |
| P5.2 | Done | Generate DTO schemas | Infer request/response schemas from DTO structs, JSON tags, validation tags, and status metadata. |
| P5.3 | Done | Add OpenAPI module | Provide `openapi.Module` or app-level `OpenAPI("/openapi.json")`. |
| P5.4 | Done | Add Swagger module | Serve Swagger UI outside core runtime as an optional module. |
| P5.5 | Done | Add OpenAPI validation tests | Verify stable output for the example app and common DTO shapes. |
| P5.6 | Done | Add OpenAPI inclusion controls | Include all registered routes by default when `app.OpenAPI(...)` is enabled, infer request/response docs from typed handler metadata, and add `@Hide()` for route/controller exclusion. Do not add Swagger-specific response/body decorators. |

Exit criteria:

- The example app exposes a valid OpenAPI document.
- Typed request/response handler metadata is enough to document request bodies, parameters, and response schemas.
- Routes can be explicitly hidden from OpenAPI with `@Hide()`.
- Swagger UI is optional and does not expand the core runtime.
- OpenAPI work includes schema and route metadata tests.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 6: Developer Experience

Goal: improve daily workflow once generation and build behavior are reliable.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P6.1 | Done | Implement `gest dev` | Watch files, debounce changes, regenerate, rebuild, and restart. Keep the previous process alive on build failure. |
| P6.2 | Done | Add boot logs | Print modules, providers, routes, OpenAPI path, listen address, and boot timing. Support production JSON logs later. |
| P6.3 | Done | Add lifecycle hooks | Implement init/bootstrap/shutdown interfaces after the eager module model is stable. |
| P6.4 | Done | Add `gesttest` | Provide testing helpers around standard Go `testing`, HTTP requests, response assertions, and provider overrides. |
| P6.5 | Done | Add docs examples | Convert the example app into user-facing guides for modules, controllers, DTOs, CLI, and testing. |

Exit criteria:

- A developer can iterate on the example app with `gest dev`.
- Build failures do not kill the last running app.
- Testing helpers reduce boilerplate without replacing Go testing.
- Developer-experience work includes tests for failure paths where practical.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 7: Optional Official Modules

Goal: add optional official modules only after the extension model is proven.

Design rules:

- Official modules are conveniences, not framework assumptions.
- Core runtime must not import official module packages.
- Global modules are allowed only as explicit app-composition convenience. Use constructor injection; do not add a service locator.
- No built-in database module or ORM abstraction.
- No cache/throttle/events in this phase.
- No built-in auth, role, or permission module. Auth policy is user-owned.
- App-specific config should be represented by user-owned structs loaded and provided through DI.
- Validation module should provide a concrete validator, but app installation may stay explicit with `gest.WithValidator(...)` if automatic installation would require special runtime hooks.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P7.1 | Done | Config module | Add `modules/config` with `.env` loading, typed getters, and user-owned struct loading through DI. |
| P7.2 | Done | Logger module | Add `modules/logger` using `log/slog`; provide `*slog.Logger` through DI without coupling boot logs to the module. |
| P7.3 | Done | Validation module | Add `modules/validation` with a concrete `gest.Validator` implementation and explicit app installation if needed. |
| P7.4 | Done | Health module | Add `modules/health` with `/health`, `/health/live`, and `/health/ready` returning `{"status":"ok"}`. |
| P7.5 | Done | JWT module | Add `modules/jwt` for signing/verifying tokens with explicit `Secret` or `SecretFromEnv`; no database or user model assumptions. |
| P7.6 | Done | No built-in auth module decision | Document that auth, roles, and permissions are user-owned modules; Gest provides guard mechanics and JWT utility only. |
| P7.7 | Done | Optional modules checkpoint | Verify config/logger/validation/health/jwt are optional, core runtime imports none of them, and an example app can use them together. |
| P7.8 | Done | Add explicit global modules | Implement `ModuleConfig.Global` semantics so imported global module providers are available throughout the app graph. Cover config/logger use cases, duplicate provider conflicts, import-order determinism, nested imports, lifecycle order, and no service-locator API. |

Explicitly out of scope:

- Built-in database module.
- Built-in ORM abstraction.
- First-party Fiber adapter.
- Cache/throttle/events modules.
- Queue/scheduler modules.
- Hidden global module discovery or package scanning.
- Built-in auth, role, or permission modules.

Exit criteria:

- Each official module is optional.
- Config/logger can be installed once as explicit global modules and injected by constructors across feature modules.
- Users can replace official modules with their own modules without special cases.
- Each optional module has unit tests and at least one integration-style usage test when it exposes runtime behavior.
- An example or fixture app imports config/logger/validation/health/jwt together where practical.
- Core runtime imports no `modules/...` package.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 8: Advanced Runtime

Goal: add advanced features only after real user feedback.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P8.1 | Planned | Lazy modules | Register routes at boot and initialize providers on first use. Requires strong concurrency and lifecycle tests. |
| P8.2 | Done | App middleware and status tracking | Add `app.Use(...)`, middleware interfaces, `MiddlewareFunc`, route/controller middleware execution, and `Context.ResponseStatus()`. |
| P8.3 | Planned | Import alias resolution | Add explicit `@GestImport` first, then existing Go imports. Defer package scan aliases until there is clear demand. |
| P8.4 | Done | Unified `@Use(...)` decorator | Add `@Use(...)` for middleware and guards, resolving from existing Go imports; classify providers by interface and do not add built-in `@Auth`, `@Roles`, or `@Permissions`. |
| P8.5 | Planned | Typed handler performance checkpoint | Verify `gest.Handle(...)` and generated explicit adapters resolve signature shape once at route-definition time, with no per-request signature reflection. |
| P8.6 | Done | Smarter dev diagnostics | Add framework-aware hints to `gest generate` and `gest dev` for skipped routes, detached decorators, generated controllers not provided in a module, and likely missing module imports. Keep raw Go output visible. |
| P8.7 | Done | Route generation debug output | Add concise `gest generate --explain` or equivalent output that lists parsed controllers/routes and why route-like methods were rejected. |
| P8.8 | Planned | Streaming and SSE helpers | Add core HTTP `Context.Stream(...)` and `Context.SSE(...)` helpers while preserving raw `http.ResponseWriter` escape hatches. SSE uses normal `@Get` routes; do not add `@SSE` or `@Stream` decorators in the MVP. |
| P8.9 | Planned | WebSocket boundary checkpoint | Keep WebSocket out of core runtime. Document and test that core runtime imports no `modules/websocket`; defer gateway implementation to the WebSocket module phase. |
| P8.10 | Planned | Ecosystem module checkpoint | Verify advanced runtime boundaries before ecosystem modules: core runtime imports no optional modules, generator output stays explicit, and global module semantics are stable enough for events/cache. |
| P8.11 | Planned | Metrics and tracing modules | Add observability modules after core middleware and context conventions are stable. |

Exit criteria:

- Advanced features do not make the simple JSON API path harder to understand.
- Every advanced feature has clear opt-in behavior and escape hatches.
- Dev diagnostics explain common Gest mistakes without hiding compiler, test, or build output.
- Advanced runtime work includes concurrency, lifecycle, or integration tests appropriate to the feature.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 9: CLI Resource Generation

Goal: make generators useful for real module trees while keeping generated code minimal and testable.

Design rules:

- All `gest g ...` commands understand nested module paths such as `projects/members`.
- Generators update the nearest matching module using AST-guided edits where practical.
- Generated components include focused tests by default.
- Generated code must not assume database/auth/cache/queue infrastructure.
- Dry-run and force behavior must remain deterministic and scoped to generated files.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P9.1 | Done | Normalize nested generator paths | Centralize path parsing for `gest g module`, `g controller`, `g service`, and `g resource`. `projects/members` maps to `internal/projects/members`, type prefix `Members`, package `members`, and parent module `internal/projects/projects.module.go` when present. |
| P9.2 | Done | Update module generator for nesting | Make `gest g module projects/members` create `members.module.go` in the nested folder and import it into the nearest parent module. Cover fallback behavior and warnings when no parent exists. |
| P9.3 | Done | Generate tests for controller/service | Update `gest g controller` and `gest g service` to create minimal compiling tests by default, with `--no-test` to skip. Tests should use ordinary Go testing and `gesttest` when HTTP behavior is generated. |
| P9.4 | Done | Add `gest g resource` | Generate module, controller, service, DTO, generated metadata or decorators, and tests for a complete simple REST-ish resource. Keep the template infrastructure-free and no auth/database assumptions. |
| P9.5 | Planned | Add generator checkpoint tests | Add end-to-end CLI tests for nested module generation, resource generation, force/dry-run/no-update flags, generated tests, `gest generate`, `go test ./...`, and `gest build`. |

Exit criteria:

- `gest g resource projects/members` creates a compiling nested resource.
- Existing `gest g module/controller/service` behavior remains backward-compatible.
- Generated tests pass in the created app.
- Nested parent module updates are deterministic and import-correct.
- CLI work includes command tests for success, failure, dry-run, force, and no-update paths.
- `rtk proxy golangci-lint run ./...` passes or a concrete blocker is documented.

## Phase 10: Ecosystem Modules

Goal: add optional events, scheduler, cache, and queue modules without expanding core runtime or turning Gest into a platform.

Design rules:

- Keep modules under `modules/...` in the main Go module for now; do not create a separate `contrib` repo, `go.work`, or separate `go.mod` files until APIs stabilize.
- Put adapters inside each module directory, such as `modules/queue/adapters/memory` and `modules/cache/adapters/redis`.
- Core runtime must not import ecosystem modules.
- The generator may emit references to ecosystem module public APIs only when the related decorator is used.
- User structs are normal providers with constructor-injected services.
- Use non-generic user handler methods first: `Handle(ctx, event) error`, `Run(ctx) error`, and `Process(ctx, job) error`.
- Do not add `@Name`; primary decorators provide identity.
- Events and cache may support explicit global module mode. Queue may support it but should not require it. Scheduler should generally remain module-owned.
- Do not implement database, auth, mailer, distributed locking, Redis queue, or production queue semantics unless the task explicitly says so.

Recommended layout:

```txt
modules/
  events/
    adapters/
      memory/
  scheduler/
    adapters/
      memory/
  cache/
    adapters/
      memory/
      redis/
  queue/
    adapters/
      memory/
      redis/
```

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P10.1 | Done | Add events module | Add `modules/events` with an injectable `*events.Bus`, sync in-process emit/listen behavior, `@OnEvent("name")`, generated listener metadata, and `gest g listener <path>`. Support explicit global module mode. |
| P10.2 | Done | Add scheduler module | Add `modules/scheduler` with `@Cron("expr")` and optional `@Every("duration")`, generated task metadata, lifecycle start/stop behavior, and `gest g task <path>`. Start with `Run(ctx context.Context) error` only. |
| P10.3 | Done | Add cache module | Add `modules/cache` with a small cache service interface, memory adapter, optional global module mode, typed JSON helpers if useful, and no decorators initially. Defer Redis unless the memory contract is stable. |
| P10.4 | Done | Add queue module | Add `modules/queue` with in-memory dev/test adapter, `@Processor("queue.name")`, generated processor metadata, and `gest g processor <path>`. Start with `Process(ctx context.Context, job Payload) error`; defer `queue.Job[T]`, retries, backoff, dead-lettering, and Redis. |
| P10.5 | Done | Add ecosystem generator checkpoint | Verify `gest generate`, generated metadata, nested module updates, generated tests, lifecycle shutdown, and no core runtime imports for events/scheduler/cache/queue. |
| P10.6 | Planned | Add ecosystem examples | Add focused examples showing events, scheduler, cache, and queue usage without database/auth assumptions. Include tests and docs. |

Exit criteria:

- Each ecosystem module is optional and replaceable.
- Each module keeps its adapters under its own directory.
- Services, listeners, tasks, and processors use normal constructor injection.
- Generated metadata contains no `init()`, hidden registries, runtime source scanning, or package scanning.
- Events/cache global mode works only after explicit app imports.
- Scheduler shuts down cleanly through app lifecycle hooks.
- Queue MVP is useful for dev/test without pretending to be production durable infrastructure.
- CLI generators support nested module paths, dry-run, force, no-update, and generated tests where applicable.
- `rtk go test ./...` and `rtk proxy golangci-lint run ./...` pass or a concrete blocker is documented.

## Phase 11: WebSocket Module

Goal: add WebSocket support as an optional module without expanding core runtime or copying Socket.IO.

Design rules:

- WebSocket lives under `modules/websocket`.
- Core runtime must not import `modules/websocket`.
- Gateway structs are normal providers with constructor-injected services.
- Use `@Gateway("/path")` on gateway structs and `@Subscribe("event.name")` on gateway methods.
- User handlers should use plain payload shapes first: `Handle(ctx context.Context, client *websocket.Client, msg Message) error`.
- Generated gateway metadata must be deterministic and public-API based.
- Do not add rooms, namespaces, distributed pub/sub, built-in auth policy, or Socket.IO compatibility in the MVP.
- Middleware/guards should run before upgrade where practical.
- Keep internal events, queues, SSE, and WebSocket separate concepts.

| ID | Status | Task | Description |
| --- | --- | --- | --- |
| P11.1 | Planned | Add WebSocket module API | Add `modules/websocket` with module options, client type, connection lifecycle hooks, JSON message codec, and adapter boundary. Start with one net-http compatible adapter. |
| P11.2 | Planned | Add gateway metadata generator | Parse `@Gateway("path")` and `@Subscribe("event")`, validate gateway handler signatures, and emit deterministic `GestGateway()` metadata using public WebSocket module APIs. |
| P11.3 | Planned | Add gateway registration runtime | Have the WebSocket module resolve gateway providers through normal DI, register upgrade routes, dispatch JSON messages by event, and handle cancellation/close errors predictably. |
| P11.4 | Planned | Add `gest g gateway` | Generate a gateway provider with one example subscription, update the nearest module, generate tests by default, and support nested paths, dry-run, force, no-update, and no-test flags. |
| P11.5 | Planned | Add WebSocket checkpoint tests | Verify generated metadata, route upgrade behavior, message dispatch, middleware/guard-before-upgrade behavior where practical, shutdown cleanup, no core runtime imports, and no hidden registries. |

Exit criteria:

- A user can create a gateway with constructor-injected services.
- Generated gateway metadata compiles and is deterministic.
- WebSocket support is optional and imported only by applications that use it.
- Core runtime imports no `modules/websocket`.
- The MVP handles JSON event messages, connection close, and app shutdown cleanly.
- `rtk go test ./...` and `rtk proxy golangci-lint run ./...` pass or a concrete blocker is documented.

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
