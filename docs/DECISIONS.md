# Gest Decisions

This file records settled technical decisions. Keep entries short and concrete.

## ADR-0001: Chi/net-http Is The First-Party Router

Status: Accepted

Gest starts with Chi/net-http as the only first-party router adapter.

Rationale:

- `net/http` is the standard Go baseline.
- Chi is small and idiomatic.
- Starting with one adapter keeps context, middleware, routing, and tests coherent.

Consequences:

- No first-party Fiber adapter is planned.
- `gest.Context` must expose net-http escape hatches.
- Adapter abstractions should not pretend all engines have identical capabilities.

## ADR-0002: No Built-In Database Module

Status: Accepted

Gest will not ship a built-in database module or ORM abstraction.

Rationale:

- Go teams use different database approaches: `database/sql`, pgx, sqlc, ent, bun, gorm, and others.
- A generic framework database module would either be too thin to matter or too opinionated.
- Users should bring their own database modules and expose repositories/services through normal providers.

Consequences:

- Official examples must not rely on `database.Module(...)`.
- Provider errors should suggest adding a provider or importing a user-owned module.
- Future docs may show database integration patterns, not a blessed ORM.

## ADR-0003: Generated Code Must Be Boring

Status: Accepted

Generated `*_gest.gen.go` files must be readable, deterministic Go that calls public runtime APIs.

Rationale:

- Framework users need to debug generated output.
- Deterministic output makes tests, reviews, and CI stable.
- Public API calls keep runtime behavior explicit.

Consequences:

- No generated `init()` route registration.
- No hidden global route registry.
- No runtime source scanning.
- Generator tests must include golden output and stable repeated output.

## ADR-0004: Runtime And Generator Are Separate

Status: Accepted

The runtime must not import generator, CLI, AST parser, filesystem scanner, or config-loader packages.

Rationale:

- Runtime should work with hand-written metadata.
- Tests stay simpler when booting an app does not require source files.
- Applications should not pay for generator dependencies at runtime.

Consequences:

- Runtime consumes explicit module/provider/controller definitions.
- CLI and generator can orchestrate files and code generation.
- Package dependency rules must be checked during implementation reviews.

## ADR-0005: Singleton DI First

Status: Accepted

V0 supports singleton constructor injection only.

Rationale:

- Singleton DI is enough to prove module imports, provider-set composition, providers, and controllers.
- Request/transient scopes require more lifecycle, concurrency, and context design.
- A conservative container reduces early framework magic.

Consequences:

- `Request` and `Transient` scopes are deferred.
- If scope options exist in the API sketch, implementations must reject unsupported scopes clearly.
- User examples should use constructors, not container lookups.

## ADR-0008: Module Imports Expose Provider Sets

Status: Accepted

Gest does not use Nest-style provider exports.

If module A imports module B, module A can inject providers from module B. There is no `gest.Export()`, no `gest.Private()`, and no module-private provider visibility model in v0.

Rationale:

- Go already has package visibility.
- Requiring `Export()` duplicates Go's exported identifier model and adds ceremony.
- Imported modules should compose provider sets directly.
- The model maps more naturally to possible future Fx-style composition.

Consequences:

- Remove `gest.Export()` from public examples, generators, official modules, and runtime API.
- Remove `Provider.Exported`.
- Remove unexported-provider errors and tests.
- Missing-provider hints should say to add a provider or import a module that provides it.
- CLI generators must not emit `gest.Export()`.

## ADR-0009: Official Modules Are Optional And Explicit

Status: Accepted

Phase 7 official modules are `config`, `logger`, `validation`, `health`, and `jwt`. Gest does not ship `modules/auth`.

Rationale:

- These modules prove the extension model without taking ownership of application infrastructure.
- Config, logging, validation, health, and JWT are common enough to provide as conveniences.
- Auth, roles, and permissions are application policy and should be user-owned modules.

Consequences:

- Core runtime must not import `modules/...`.
- No global module behavior in v0.
- No built-in database or ORM module.
- Cache, throttle, events, queues, scheduler, metrics, tracing, and mailer remain deferred.
- Official modules must use normal module/provider APIs and be replaceable by user modules.
- Gest may provide guard mechanics and JWT utility, but not an auth platform.

## ADR-0010: Typed App Config Uses User-Owned Structs

Status: Accepted

The config module can load user-owned structs and provide them through DI.

Example:

```go
type AppConfig struct {
	Port      string `env:"PORT" default:"3000"`
	JWTSecret string `env:"JWT_SECRET" validate:"required"`
}
```

Rationale:

- App config is application-owned, not framework-owned.
- Injecting `*AppConfig` into services is Go-native and testable.
- This avoids global config lookups and stringly typed service code.

Consequences:

- `modules/config` provides `*config.Service`.
- `modules/config` may also provide loaded user structs such as `*AppConfig`.
- Config module is separate from CLI `gest.yaml` config loading.

## ADR-0011: Auth Is User-Owned

Status: Accepted

Gest does not provide built-in auth, role, or permission modules.

Rationale:

- User identity, roles, permissions, tenants, organizations, sessions, OAuth/OIDC, repositories, and policy checks are application-specific.
- A built-in auth module would push Gest toward a full platform.
- The framework should provide Nest-ish structure and guard mechanics, not security policy.

Consequences:

- No `modules/auth`.
- No built-in role or permission module.
- User apps may create `internal/auth` modules using normal Gest providers.
- `modules/jwt` remains an optional low-level utility because it does not require a user model.
- Guard decorators should prioritize `@Use(...)`; `@Auth`, `@Roles`, and `@Permissions` are not built-in framework policy.

## ADR-0012: Typed Handler Wrappers Are Precomputed

Status: Accepted

Typed handler shape must be resolved when route metadata is generated or when `gest.Handle(...)` is called, not on every request.

Rationale:

- Per-request signature reflection is avoidable overhead.
- Generated metadata already knows handler shape.
- The runtime should execute a concrete `HandlerFunc` path for each route.

Consequences:

- `gest.Handle(...)` may inspect handler shape once while constructing a handler adapter.
- The returned `HandlerFunc` must not repeat signature inspection per request.
- Generated code should emit the correct wrapper call directly.
- Future specialized helpers are allowed if they improve clarity or performance without complicating user code.

## ADR-0013: Unified Use Decorator For Middleware And Guards

Status: Accepted

Gest uses one `@Use(...)` decorator for middleware and guards. The referenced provider is classified by the interface it implements.

Rationale:

- `@UseMiddleware` is too narrow and verbose.
- A single decorator keeps the Nest-ish feel without copying Nest's separate auth/role/policy decorators.
- Middleware and guards are route components; users should not need separate decorator families for the MVP.

Consequences:

- App-level middleware uses `app.Use(...)`.
- Dependency-injected middleware implements `Middleware`.
- Function middleware uses `MiddlewareFunc`.
- Guards implement `Guard`.
- Generated metadata classifies `@Use(...)` references into middleware or guard factories.
- Execution order is app middleware, controller middleware, route middleware, guards, handler.
- Auth, roles, and permissions remain user-owned policy.

## ADR-0006: Tests And Lint Are Required

Status: Accepted

Every non-documentation implementation task must include tests and pass lint before being marked done.

Required commands:

```bash
rtk go test ./...
rtk proxy golangci-lint run ./...
```

Rationale:

- Framework regressions are expensive for users.
- Generator behavior needs deterministic tests from the beginning.
- Lint keeps public API code and generated code disciplined.

Consequences:

- The root `.golangci.yml` is part of the project contract.
- Documentation-only tasks may skip tests and lint only when no Go packages exist or the blocker is documented.
- Agents must report exact verification results.

## ADR-0007: Workflow Tools Come After Core Reliability

Status: Accepted

`gest dev` is deferred until `gest generate` and `gest build` are reliable.

Rationale:

- File watching and process restarts are a separate product surface.
- A dev server can hide generator/runtime defects if added too early.
- Keeping the previous process alive on build failure requires careful process handling.

Consequences:

- Early CLI work focuses on `gest generate` and `gest build`.
- `gest dev` remains in the developer-experience phase.
- Build and generator diagnostics must be good before watch mode exists.
