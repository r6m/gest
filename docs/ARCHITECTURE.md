# Gest Architecture Contract

This document defines package boundaries and implementation constraints for Gest. It is stricter than the design document. If an implementation conflicts with this file, the implementation is wrong unless this file is changed first.

## Package Layers

Gest is split into layers. Dependencies may only point downward.

```txt
cmd/gest
  -> internal/cli
  -> internal/generator
  -> gest runtime packages

internal/cli
  -> internal/generator
  -> internal/config
  -> gest runtime packages

internal/generator
  -> Go parser/AST packages
  -> filesystem scanning
  -> optional references to public runtime package names for emitted code

gest runtime packages
  -> standard library
  -> router adapter packages
  -> no generator, CLI, filesystem scanner, or AST parser imports

router adapter packages
  -> gest runtime packages
  -> concrete router dependencies such as Chi

examples
  -> public Gest APIs only, as real users would
```

## Runtime Boundary

The runtime consumes explicit values:

- `Module`
- `ModuleConfig`
- `Provider`
- `ControllerDefinition`
- `RouteDefinition`
- `HandlerFunc`

The runtime must not:

- scan source files
- parse comments
- inspect package directories
- import generator packages
- import CLI packages
- depend on `gest.yaml`
- rely on `init()` route registration
- rely on hidden global route registries

Hand-written `GestController()` metadata must remain valid forever. The generator is a convenience for producing that metadata, not a required runtime subsystem.

## Generator Boundary

The generator reads user source files and emits deterministic Go files.

Generated files must:

- use package-local controller methods directly
- call public runtime APIs
- be gofmt/gofumpt-compatible
- be deterministic across repeated runs with unchanged input
- contain no `init()` functions
- contain no hidden registration side effects
- be readable enough to debug by hand

Generated files may include explicit metadata such as:

```go
func (c *UserController) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name:     "UserController",
		BasePath: "/users",
		Routes: []gest.RouteDefinition{
			{
				Name:    "FindUser",
				Method:  "GET",
				Path:    "/:id",
				Handler: gest.JSON(c.FindUser, gest.Status(200)),
			},
		},
	}
}
```

Generated files must not include:

```go
func init() {
	gest.RegisterController(...)
}
```

## CLI Boundary

The CLI orchestrates tools. It does not own runtime semantics.

`gest generate` may:

- load `gest.yaml`
- scan packages
- parse decorators
- validate handlers
- write generated files
- run gofmt/gofumpt-compatible formatting

`gest build` may:

- run `gest generate`
- validate generated metadata
- optionally run tests
- call `go build`
- print the underlying Go command

`gest build` must not replace the Go compiler or hide Go build failures behind generic framework errors.

## Router Boundary

The first-party adapter is Chi/net-http.

Gest must expose net-http escape hatches through `gest.Context`:

- `RawResponse() http.ResponseWriter`
- `RawRequest() *http.Request`
- `Native() any`
- `Engine() string`

No first-party Fiber adapter is planned. If another adapter is introduced later, it must preserve the simple JSON API path and document non-portable behavior explicitly.

## Dependency Injection Boundary

V0 supports singleton constructor injection.

The DI container must support:

- constructor providers
- value providers
- controller providers
- singleton caching
- module imports as provider-set composition
- missing provider errors
- provider cycle errors

The DI container must not introduce:

- request scope
- transient scope
- provider export/private visibility controls
- global service locator patterns
- required container lookups in normal user code

Module imports are the only module-level provider boundary. If module A imports module B, providers from module B are available to module A's providers and controllers. There is no `gest.Export()`, no `gest.Private()`, and no Nest-style provider export list in the v0 model.

Go package visibility remains the privacy mechanism. If another package cannot name a provider type or constructor, it cannot directly request that dependency in normal Go code.

`Resolve` may exist for internals, testing, and advanced escape hatches. It must not be the primary user-facing dependency pattern.

## Official Module Boundary

Official modules live under `modules/...` and are optional conveniences. Core runtime packages must not import official modules.

Phase 7 official module scope is:

- `modules/config`
- `modules/logger`
- `modules/validation`
- `modules/health`
- `modules/jwt`

Gest does not provide `modules/auth`; auth, roles, and permissions are user-owned modules.

Official modules must:

- use normal `gest.Module` and provider APIs
- be replaceable by user-owned modules
- avoid database and ORM assumptions
- avoid global-module behavior
- avoid hidden app-wide side effects

Official modules must not:

- require users to import all official modules
- add special runtime cases for their package names
- rely on `gest.Export()` or provider privacy concepts
- make core runtime import `modules/...`

Config is a runtime module, not the CLI `gest.yaml` loader. App-specific config should be represented by user-owned structs that are loaded by `modules/config` and provided through DI.

Logger should use Go `log/slog`. Boot logs remain controlled by `gest.WithBootLogs(...)` and do not require `modules/logger`.

Validation should keep core validation behind the `gest.Validator` interface. If automatic installation through module imports would require special runtime hooks, use explicit `gest.WithValidator(validation.NewValidator())`.

Health should expose simple dependency-free health routes by default.

JWT must not assume a user database, repository, ORM, or user model.

Gest must not ship built-in auth, role, or permission modules. Auth policy is user-owned application code. Gest may provide guard mechanics, route metadata, bearer-token helpers, and optional JWT utilities, but it must not own user identity, roles, permissions, repositories, or policy semantics.

Typed handler adaptation must happen at route-definition time. Generated metadata and public wrapper helpers such as `gest.JSON(...)` may inspect the handler shape while creating a `HandlerFunc`, but the resulting handler must not perform signature reflection on every request.

## Error Contract

Expected user mistakes must return structured, actionable errors. Do not panic for normal configuration, provider, route, binding, or decorator failures.

Errors should include available context:

- module name
- provider constructor or token
- controller name
- route method and path
- file and line for generator errors
- suggested fix when obvious

## Testing Boundary

Runtime behavior needs package-level unit tests and integration tests through Chi/net-http when behavior crosses HTTP boundaries.

Generator behavior needs fixture tests:

- valid controllers
- invalid decorators
- invalid signatures
- golden generated output
- stable repeated output

CLI behavior needs command tests:

- success exit code
- failure exit code
- concise output
- config defaults
- visible underlying Go commands

## Lint Boundary

The root `.golangci.yml` is mandatory. Code must pass:

```bash
rtk go test ./...
rtk proxy golangci-lint run ./...
```

Generated code must not require broad lint suppressions. If a narrow suppression is unavoidable, document why in the generated-code contract and test it.
