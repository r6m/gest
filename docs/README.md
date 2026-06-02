# Gest Framework Design Document

For the current compiling example and CLI workflow, start with [Quickstart](QUICKSTART.md) and `examples/hello`.

## Purpose

Gest is a Go web framework inspired by NestJS, designed to make modular backend development simple, explicit, and productive.

The goal is not to copy NestJS directly. Go does not have real decorators, runtime metadata, or TypeScript-style reflection. Gest should provide a NestJS-like developer experience using Go-native patterns:

- explicit modules
- constructor-based dependency injection
- generated controller metadata
- typed handlers with automatic DTO binding
- router adapters
- optional infrastructure modules
- CLI generators
- boot logs
- dev/build tooling

Gest should push repetitive plumbing into the framework and generated code while keeping user-owned application code clean, editable, and idiomatic.

---

## Core Philosophy

Gest should feel like this:

```go
app.Import(
	config.Module(config.Options{Global: true}),
	auth.Module(auth.Options{
		JWTSecretFromConfig: "JWT_SECRET",
		Global:              true,
	}),
	reports.Module(reports.Options{}),
)
```

A feature module should feel like this:

```go
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "reports",

		Imports: gest.Imports(
			repositories.Module(repositories.Options{}),
		),

		Providers: gest.Providers(
			gest.Provide(NewReportService),
			gest.Provide(NewPDFRenderer),
			gest.Controller(NewReportController),
		),
	})
}
```

A controller should feel like this:

```go
// @Controller("/reports")
// @Tag("Reports")
type ReportController struct {
	service *ReportService
}

func NewReportController(service *ReportService) *ReportController {
	return &ReportController{service: service}
}

// @Get("/:id")
// @Status(200)
// @Status(404)
func (c *ReportController) FindReport(
	ctx *gest.Context,
	req *FindReportRequest,
) (*FindReportResponse, error) {
	return c.service.FindReport(ctx, req)
}
```

The user writes intent. Gest generates routing metadata, handles binding, validation, OpenAPI metadata, boot logs, and wiring.

---

## Engineering Rules

These rules are strict. Any implementation that violates them is out of scope unless this document is explicitly changed first.

1. Generated code must be boring.
   - Generated `*_gest.gen.go` files must be readable, deterministic, gofmt-formatted Go.
   - Generated code must call public runtime APIs such as `gest.JSON(...)`, `gest.Status(...)`, and controller methods directly.
   - Generated code must not rely on hidden global registries, `init()` side effects, runtime source scanning, or reflection-only route discovery.

2. Runtime and generator must stay separate.
   - The runtime package must not import generator, parser, AST, filesystem, CLI, or config-loader packages.
   - The runtime consumes explicit `Module`, `Provider`, and `ControllerDefinition` values.
   - Hand-written `GestController()` metadata must remain a supported path.

3. Errors are part of the product.
   - Missing providers, provider cycles, duplicate routes, invalid handler signatures, invalid decorators, ambiguous imports, binding failures, and validation failures must produce actionable errors.
   - Errors should include module/provider/route names and file/line details when available.
   - Do not replace specific failures with generic panics.

4. Dependency injection must stay conservative.
   - V0 supports singleton constructor injection only.
   - Request scope and transient providers are deferred until singleton semantics, lifecycle, and tests are stable.
   - User application code should receive dependencies through constructors, not by pulling from a global container.

5. Decorators must stay tiny.
   - Decorators are simple, line-based comments.
   - Supported syntax should look like `// @Get("/:id")`, not object literals or embedded expressions.
   - Comments must not become a second programming language.

6. Users own infrastructure choices.
   - Gest must not ship a built-in database module or ORM abstraction.
   - Database, cache, queue, mailer, and similar integrations must work as normal user-owned modules/providers.
   - Official modules are optional conveniences, not framework assumptions.

7. Escape hatches are required.
   - `gest.Context` must expose raw `http.ResponseWriter` and `*http.Request` for net-http adapters.
   - Normal Go tests, normal Go builds, hand-written metadata, and user-owned modules must remain viable.
   - `gest build` orchestrates Go tooling; it does not replace the Go compiler.

8. Prove the framework with a contract app.
   - A small example app must serve as the acceptance fixture for runtime, generator, binding, CLI, and docs.
   - Each phase must move that app closer to the target developer experience.

9. Tests are mandatory.
   - Every implementation task must add or update tests unless the task is documentation-only.
   - Runtime behavior needs unit tests and, where useful, integration tests through Chi/net-http.
   - Generator behavior needs fixture-based tests for valid output and failure diagnostics.
   - CLI behavior needs command tests that verify exit codes, output, and config defaults.
   - A task is not done until `go test ./...` passes, or the blocker is documented explicitly.

10. Linting is mandatory.
   - The repository must keep a `.golangci.yml` at the root.
   - Implementations must pass `golangci-lint run ./...` before being marked done, unless the blocker is documented explicitly.
   - Generated code must be gofmt/gofumpt-compatible and must not require blanket lint disables.

11. Do not build workflow tools before the core is stable.
   - `gest dev` is deferred until `gest generate` and `gest build` are reliable.
   - File watching and process restart behavior must not hide generator or runtime defects.

---

## Non-Goals

Gest should not:

- copy NestJS exactly
- require runtime scanning of source files
- parse comments during application boot
- force a single router forever, while the first-party runtime starts with Chi/net-http only
- force one ORM/database style
- ship a built-in database module; application teams should bring their own database modules
- expose Uber Fx directly as the user-facing API
- hide raw Go escape hatches
- require users to manually register routes in every controller
- require users to write generated route factory names inside module files

---

# 1. Project Structure

Recommended generated project:

```txt
my-api/
  cmd/
    api/
      main.go

  internal/
    app/
      app.module.go

    auth/
      auth.module.go
      auth.controller.go
      auth.service.go
      jwt.guard.go
      roles.guard.go
      auth.dto.go

    reports/
      reports.module.go
      report.controller.go
      report.service.go
      report.repository.go
      report.dto.go
      reports_gest.gen.go

  gest.yaml
  go.mod
```

---

# 2. Application Bootstrap

## `cmd/api/main.go`

```go
package main

import (
	"log"

	"myapp/gest"
	"myapp/internal/app"
)

func main() {
	server := gest.New(
		gest.WithBootLogs(true),
	)

	server.Import(
		app.Module(),
	)

	if err := server.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
```

---

# 3. Root App Module

## `internal/app/app.module.go`

```go
package app

import (
	"time"

	"myapp/gest"
	"myapp/gest/modules/config"
	"myapp/gest/modules/logger"
	"myapp/internal/auth"
	"myapp/internal/reports"
)

func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "app",

		Imports: gest.Imports(
			config.Module(config.Options{
				EnvFiles: []string{".env"},
				Global:   true,
			}),

			logger.Module(logger.Options{
				Global: true,
			}),

			auth.Module(auth.Options{
				JWTSecretFromConfig: "JWT_SECRET",
				AccessTTL:          time.Hour,
				Global:             true,
			}),

			reports.Module(reports.Options{}),
		),
	})
}
```

---

# 4. Module Design

Gest should use explicit `Module()` functions. Do not expose `GestModule()` as the normal user-facing API.

Each package/module owns a file like:

```go
package reports

import (
	"myapp/gest"
	"myapp/internal/repositories"
)

type Options struct {
	Lazy bool
}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "reports",
		Lazy: options.Lazy,

		Imports: gest.Imports(
			repositories.Module(repositories.Options{}),
		),

		Providers: gest.Providers(
			gest.Provide(NewReportService),
			gest.Provide(NewPDFRenderer),
			gest.Controller(NewReportController),
		),
	})
}
```

## Module API

```go
package gest

type Module interface {
	Definition() ModuleConfig
}

type ModuleConfig struct {
	Name   string
	Lazy   bool
	Global bool

	Imports   []Module
	Providers []Provider

	OnBoot []BootHook
}

func NewModule(config ModuleConfig) Module {
	return module{config: config}
}

func Imports(modules ...Module) []Module {
	return modules
}

func Providers(providers ...Provider) []Provider {
	return providers
}
```

---

# 5. Providers

Providers should be simple and pleasant.

Preferred:

```go
gest.Provide(NewAuthService)
```

When a module imports another module, providers from the imported module are available to the importing module. There is no Nest-style `exports` list, no `gest.Export()`, and no module-private provider concept in v0.

Tokens can exist internally and for advanced usage, but the common API should use provider options.

## Provider API

```go
package gest

type ProviderKind string

const (
	ProviderKindService    ProviderKind = "service"
	ProviderKindController ProviderKind = "controller"
	ProviderKindValue      ProviderKind = "value"
)

type Scope string

const (
	Singleton Scope = "singleton"

	// Deferred until after v0. Implementations must reject these scopes
	// with clear errors until request/transient lifecycles are designed.
	Transient Scope = "transient"
	Request   Scope = "request"
)

type Provider struct {
	Kind ProviderKind

	Constructor any
	Value       any

	Scope    Scope

	Name    string
	Aliases []Token
}

type ProviderOption func(*Provider)

func Provide(constructor any, options ...ProviderOption) Provider {
	p := Provider{
		Kind:        ProviderKindService,
		Constructor: constructor,
		Scope:       Singleton,
	}

	for _, option := range options {
		option(&p)
	}

	return p
}

func Controller(constructor any, options ...ProviderOption) Provider {
	p := Provide(constructor, options...)
	p.Kind = ProviderKindController
	return p
}

func Value(value any, options ...ProviderOption) Provider {
	p := Provider{
		Kind:  ProviderKindValue,
		Value: value,
		Scope: Singleton,
	}

	for _, option := range options {
		option(&p)
	}

	return p
}

func Name(name string) ProviderOption {
	return func(p *Provider) {
		p.Name = name
	}
}

func As[T any]() ProviderOption {
	return func(p *Provider) {
		p.Aliases = append(p.Aliases, TokenOf[T]())
	}
}

func WithScope(scope Scope) ProviderOption {
	return func(p *Provider) {
		p.Scope = scope
	}
}
```

---

# 6. Controllers

Controllers are providers with generated route metadata.

User-owned controller:

```go
package reports

import "myapp/gest"

// @Controller("/reports")
// @Tag("Reports")
type ReportController struct {
	service *ReportService
}

func NewReportController(service *ReportService) *ReportController {
	return &ReportController{service: service}
}

// @Get("/:id")
// @Status(200)
// @Status(404)
func (c *ReportController) FindReport(
	ctx *gest.Context,
	req *FindReportRequest,
) (*FindReportResponse, error) {
	return c.service.FindReport(ctx, req)
}
```

Generated file:

```go
// Code generated by gest. DO NOT EDIT.

package reports

import "myapp/gest"

func (c *ReportController) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name:     "ReportController",
		BasePath: "/reports",
		Tag:      "Reports",

		Routes: []gest.RouteDefinition{
			{
				Name:   "FindReport",
				Method: "GET",
				Path:   "/:id",

				Handler: gest.JSON(
					c.FindReport,
					gest.Status(200),
				),

				Request:  (*FindReportRequest)(nil),
				Response: (*FindReportResponse)(nil),

				Statuses: []int{200, 404},
			},
		},
	}
}
```

Framework-side interface:

```go
package gest

type DescribedController interface {
	GestController() ControllerDefinition
}

type ControllerDefinition struct {
	Name     string
	BasePath string
	Tag      string
	Routes   []RouteDefinition
}

type RouteDefinition struct {
	Name        string
	Method      string
	Path        string
	Handler     HandlerFunc
	Request     any
	Response    any
	Statuses    []int
	Metadata    RouteMetadata
	Guards      []GuardFactory
	Middlewares []Middleware
}
```

Module files should only use:

```go
gest.Controller(NewReportController)
```

They should not reference generated route functions.

---

# 7. Decorators

Gest decorators are Go comments parsed at generation time.

## Supported decorators

```go
// @Controller("/users")
// @Tag("Users")
// @Service
// @Repository

// @Get("/")
// @Post("/")
// @Put("/:id")
// @Patch("/:id")
// @Delete("/:id")

// @Status(200)
// @Status(404)
// @Summary("Find user")
// @Description("Returns a user by ID")

// @Body(CreateUserRequest)
// @Query(ListUsersQuery)
// @Param("id", "string", "User ID")

// @Auth
// @Public
// @Roles("admin", "owner")
// @Permissions("project:read")
// @Use(auth.JWTGuard)
// @Throttle("login")
// @Cache("user:{id}", ttl="5m")

// @WebSocket("/rooms/:id")
// @Stream("text/event-stream")
```

Keep decorator syntax line-based and simple. Avoid complex object syntax inside comments.

Good:

```go
// @Controller("/users")
// @Tag("Users")
```

Avoid:

```go
// @Controller({
//   path: "/users",
//   version: "1"
// })
```

---

# 8. Decorator Import Resolution

Users should usually be able to write:

```go
// @Use(auth.JWTGuard)
```

without manually declaring:

```go
// @GestImport(auth "myapp/internal/auth")
```

`@GestImport` should exist only as a manual override.

## Resolution order

When Gest sees `auth.JWTGuard`, it should resolve `auth` in this order:

1. explicit `@GestImport`
2. existing Go imports in the file
3. `gest.yaml` aliases
4. project package scan using `go list ./...`
5. built-in Gest namespaces
6. error if ambiguous or unresolved

Example manual override:

```go
// @GestImport(auth "myapp/internal/security/auth")
```

Example `gest.yaml` aliases:

```yaml
decorators:
  imports:
    auth: myapp/internal/auth
    policy: myapp/internal/policy
```

If ambiguous:

```txt
ERROR ambiguous decorator package alias "auth"

found:
- myapp/internal/auth
- myapp/pkg/auth

fix with:

// @GestImport(auth "myapp/internal/auth")
```

---

# 9. Typed Handlers and DTO Binding

Typed handlers are a core feature.

Supported signatures:

```go
func(ctx *gest.Context) error

func(ctx *gest.Context, req *Req) (*Res, error)

func(ctx *gest.Context, req *Req) error

func(w http.ResponseWriter, r *http.Request)

func(ctx *gest.Context, socket *gest.Socket, req *Req) error
```

Preferred style:

```go
func (c *UserController) FindUser(
	ctx *gest.Context,
	req *FindUserRequest,
) (*FindUserResponse, error) {
	user, err := c.service.FindByID(req.ID)
	if err != nil {
		return nil, gest.NotFound("user not found")
	}

	return &FindUserResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}
```

DTO:

```go
type FindUserRequest struct {
	ID     string `param:"id" validate:"required"`
	Expand bool   `query:"expand"`
}

type FindUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
```

## Handler wrapper

```go
package gest

type HandlerFunc func(ctx *Context) error

type TypedHandlerFunc[Req any, Res any] func(
	ctx *Context,
	req *Req,
) (*Res, error)

func JSON[Req any, Res any](
	handler TypedHandlerFunc[Req, Res],
	options ...HandlerOption,
) HandlerFunc {
	cfg := newHandlerConfig(options...)

	return func(ctx *Context) error {
		var req Req

		if err := ctx.BindRequest(&req); err != nil {
			return BadRequest(err.Error())
		}

		if err := ctx.Validate(&req); err != nil {
			return BadRequest(err.Error())
		}

		res, err := handler(ctx, &req)
		if err != nil {
			return err
		}

		if res == nil {
			return ctx.NoContent(cfg.emptyStatus)
		}

		return ctx.JSON(cfg.successStatus, res)
	}
}
```

## Binding rules

DTO fields may use:

```go
type Request struct {
	ID        string `param:"id"`
	Page      int    `query:"page" default:"1"`
	Token     string `header:"Authorization"`
	Name      string `json:"name"`
	Avatar    *gest.File `file:"avatar"`
}
```

Gest should bind:

1. JSON body
2. path params
3. query params
4. headers
5. form/multipart fields
6. files

Explicit tags override ambiguity.

---

# 10. Context

Gest context should wrap the underlying router context but keep escape hatches.

```go
package gest

type Context struct {
	// internal fields
}

func (c *Context) Param(name string) string
func (c *Context) Query(name string) string
func (c *Context) Header(name string) string
func (c *Context) BearerToken() string

func (c *Context) Bind(v any) error
func (c *Context) BindRequest(v any) error
func (c *Context) Validate(v any) error

func (c *Context) JSON(status int, value any) error
func (c *Context) NoContent(status int) error

func (c *Context) Set(key string, value any)
func (c *Context) Get(key string) (any, bool)

func (c *Context) Native() any
func (c *Context) Engine() string
```

For `net/http` adapters:

```go
func (c *Context) RawResponse() http.ResponseWriter
func (c *Context) RawRequest() *http.Request
```


---

# 11. Router Adapters

Gest should support router adapters.

Default should be Chi or a standard `net/http` adapter.

Supported:

- Chi
- custom user routers

## Router adapter interface

```go
package gest

type RouterAdapter interface {
	Name() string

	Group(prefix string, fn func(group RouterAdapter))
	Handle(route RouteRuntimeConfig)
	Use(middleware Middleware)

	Serve(addr string) error
}
```

App configuration:

```go
app := gest.New(
	gest.WithRouter(chiadapter.New()),
)
```

Existing router override:

```go
r := chi.NewRouter()

app := gest.New(
	gest.WithRouter(chiadapter.From(r)),
)
```

# 12. Streaming

Gest should provide streaming helpers, but must not hide raw `http.ResponseWriter`.

Recommended API:

```go
func (c *Context) Stream(
	status int,
	contentType string,
	fn func(stream *Stream) error,
) error
```

Example:

```go
func (c *ReportController) Export(
	ctx *gest.Context,
	req *ExportReportRequest,
) error {
	return ctx.Stream(200, "text/csv", func(stream *gest.Stream) error {
		if err := stream.WriteString("id,name\n"); err != nil {
			return err
		}

		for _, row := range c.service.Rows(req.ReportID) {
			if err := stream.WriteString(row.ID + "," + row.Name + "\n"); err != nil {
				return err
			}

			if err := stream.Flush(); err != nil {
				return err
			}
		}

		return nil
	})
}
```

SSE helper:

```go
func (c *Context) SSE(fn func(events *SSE) error) error
```

Example:

```go
// @Get("/events")
// @Stream("text/event-stream")
func (c *UserController) Events(
	ctx *gest.Context,
	req *UserEventsRequest,
) error {
	return ctx.SSE(func(events *gest.SSE) error {
		for event := range c.service.Events(req.UserID) {
			if err := events.Send("user.updated", event); err != nil {
				return err
			}
		}

		return nil
	})
}
```

---

# 13. WebSockets

Gest should support WebSocket routes through decorators and typed handlers.

```go
// @Controller("/chat")
type ChatController struct {
	hub *ChatHub
}

func NewChatController(hub *ChatHub) *ChatController {
	return &ChatController{hub: hub}
}

// @WebSocket("/rooms/:id")
// @Auth
func (c *ChatController) JoinRoom(
	ctx *gest.Context,
	socket *gest.Socket,
	req *JoinRoomRequest,
) error {
	for {
		var msg ChatMessage

		if err := socket.ReadJSON(&msg); err != nil {
			return err
		}

		c.hub.Broadcast(req.RoomID, msg)
	}
}
```

DTO:

```go
type JoinRoomRequest struct {
	RoomID string `param:"id" validate:"required"`
}
```

WebSocket backend should be swappable:

```go
app.Import(
	websocket.Module(websocket.Options{
		Engine: websocket.Gorilla(),
	}),
)
```

or:

```go
websocket.Coder()
```

---

# 14. Auth, Guards, and Roles

Built-in shortcuts:

```go
// @Auth
// @Public
// @Roles("admin")
// @Permissions("project:read")
```

Custom guards:

```go
// @Use(auth.JWTGuard)
```

Generated route metadata should include:

```go
Metadata: gest.RouteMetadata{
	Auth:        true,
	Roles:       []string{"admin"},
	Permissions: []string{"project:read"},
}
```

Route guards should be resolved from DI:

```go
Guards: []gest.GuardFactory{
	gest.ResolveGuard[*auth.JWTGuard](),
	gest.ResolveGuard[*auth.RolesGuard](),
}
```

Auth module example:

```go
package auth

import (
	"time"

	"myapp/gest"
	"myapp/gest/modules/jwt"
)

type Options struct {
	JWTSecretFromConfig string
	AccessTTL          time.Duration
	Global             bool
}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name:   "auth",
		Global: options.Global,

		Imports: gest.Imports(
			jwt.Module(jwt.Options{
				SecretFromConfig: options.JWTSecretFromConfig,
				AccessTTL:        options.AccessTTL,
			}),
		),

		Providers: gest.Providers(
			gest.Value(options),

			gest.Provide(NewAuthService),
			gest.Provide(NewJWTGuard),
			gest.Provide(NewRolesGuard),

			gest.Controller(NewAuthController),
		),
	})
}
```

Guard example:

```go
type JWTGuard struct {
	jwt  *jwt.Service
	auth *AuthService
}

func NewJWTGuard(jwt *jwt.Service, auth *AuthService) *JWTGuard {
	return &JWTGuard{jwt: jwt, auth: auth}
}

func (g *JWTGuard) CanActivate(ctx *gest.Context) error {
	token := ctx.BearerToken()
	if token == "" {
		return gest.Unauthorized("missing bearer token")
	}

	claims, err := g.jwt.Verify(token)
	if err != nil {
		return gest.Unauthorized("invalid token")
	}

	user, err := g.auth.FindUserByID(claims.Subject)
	if err != nil {
		return gest.Unauthorized("user not found")
	}

	ctx.Set("user", user)

	return nil
}
```

---

# 15. Dynamic Modules

Gest dynamic modules should use Go functions with `Options` structs.

Example:

```go
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "jwt",

		Providers: gest.Providers(
			gest.Value(options),
			gest.Provide(NewService),
			gest.Provide(NewGuard),
		),
	})
}
```

Usage:

```go
jwt.Module(jwt.Options{
	SecretFromConfig: "JWT_SECRET",
	Issuer:           "my-api",
	AccessTTL:        time.Hour,
})
```

Prefer:

```go
auth.Module(...)
reports.Module(...)
```

over Nest-style:

```go
ForRoot(...)
ForFeature(...)
```

Go-style names are cleaner.

---

# 16. Lazy Modules

Gest lazy modules should mean:

```txt
registered at boot
routes known at boot
providers initialized on first use
```

Not runtime loading of Go source code.

Example:

```go
reports.Module(reports.Options{
	Lazy: true,
})
```

Behavior:

```txt
App boots.
Routes are registered.
Report providers are not constructed.
First request to /reports initializes the module.
OnModuleInit runs for initialized providers.
Subsequent requests reuse cached providers.
```

Lazy container sketch:

```go
type LazyContainer struct {
	parent Container
	module ModuleConfig

	once  sync.Once
	err   error
	child Container
}

func (c *LazyContainer) Resolve(token Token) (any, error) {
	c.once.Do(func() {
		c.child, c.err = buildModuleContainer(c.parent, c.module)
	})

	if c.err != nil {
		return nil, c.err
	}

	return c.child.Resolve(token)
}
```

---

# 17. Lifecycle Events

Gest should support lifecycle interfaces.

```go
type OnModuleInit interface {
	OnModuleInit(ctx context.Context) error
}

type OnApplicationBootstrap interface {
	OnApplicationBootstrap(ctx context.Context) error
}

type OnModuleDestroy interface {
	OnModuleDestroy(ctx context.Context) error
}

type BeforeApplicationShutdown interface {
	BeforeApplicationShutdown(ctx context.Context) error
}

type OnApplicationShutdown interface {
	OnApplicationShutdown(ctx context.Context) error
}
```

## Lifecycle order

Boot:

```txt
1. Load root modules
2. Build module graph
3. Detect cycles
4. Resolve imported provider sets
5. Register eager providers
6. Initialize eager modules
7. Call OnModuleInit
8. Register controllers/routes
9. Register OpenAPI metadata
10. Call OnApplicationBootstrap
11. Start server
```

Shutdown:

```txt
1. Call BeforeApplicationShutdown
2. Stop server
3. Call OnModuleDestroy
4. Call OnApplicationShutdown
```

Lazy module lifecycle:

```txt
OnModuleInit runs when lazy module is first initialized.
OnModuleDestroy runs during app shutdown only if the module was initialized.
```

---

# 18. Dependency Injection

Gest should expose its own DI API.

It may use Uber Fx internally later, but users should not need to import Fx.

Container interface:

```go
type Container interface {
	Resolve(token Token) (any, error)
	MustResolve(token Token) any
	Invoke(constructor any) (any, error)
}
```

Token API can exist for advanced use:

```go
type Token struct {
	Type reflect.Type
	Name string
}

func TokenOf[T any]() Token
func Named(name string) Token
```

But normal users should use:

```go
gest.Provide(NewUserService)
gest.Provide(NewRedisCache, gest.Name("cache.redis"))
gest.Provide(NewJWTGuard, gest.As[gest.Guard]())
```

---

# 19. OpenAPI and Swagger

Gest should generate OpenAPI from:

- controller decorators
- route decorators
- typed handler signatures
- request DTOs
- response DTOs
- status decorators
- auth/role metadata
- validation tags

App usage:

```go
app.Import(
	openapi.Module(openapi.Options{
		Title:   "My API",
		Version: "1.0.0",
	}),
	swagger.Module(swagger.Options{
		Path: "/docs",
	}),
)
```

or app-level convenience:

```go
app.OpenAPI("/openapi.json")
app.Swagger("/docs")
```

Swagger UI should live outside core.

Recommended packages:

```txt
gest/modules/openapi
gest/modules/swagger
```

---

# 20. Optional Official Modules

Gest should provide official modules, but keep core lightweight.

Recommended modules:

```txt
config
logger
validation
openapi
swagger
jwt
auth
redis
cache
queue
scheduler
health
metrics
tracing
throttle
mailer
websocket
events
files
testing
```

## Config

```go
config.Module(config.Options{
	EnvFiles: []string{".env", ".env.local"},
	Global:   true,
})
```

## Logger

```go
logger.Module(logger.Options{
	Level:  "info",
	Format: "json",
	Global: true,
})
```

## Validation

```go
validation.Module()
```

## JWT

```go
jwt.Module(jwt.Options{
	SecretFromConfig: "JWT_SECRET",
	Issuer:           "my-api",
	AccessTTL:        time.Hour,
})
```

## Queue

BullMQ-like module for Go.

```go
queue.Module(queue.Options{
	Driver: queueasynq.New(queueasynq.Options{
		RedisURLFromConfig: "REDIS_URL",
	}),
})
```

Job processor:

```go
// @Processor("email.welcome")
type WelcomeEmailProcessor struct {
	mailer *MailerService
}

func NewWelcomeEmailProcessor(mailer *MailerService) *WelcomeEmailProcessor {
	return &WelcomeEmailProcessor{mailer: mailer}
}

// @Process
func (p *WelcomeEmailProcessor) Handle(
	ctx context.Context,
	job *WelcomeEmailJob,
) error {
	return p.mailer.SendWelcome(job.UserID)
}
```

## Scheduler

```go
// @Cron("0 */5 * * * *")
func (j *CleanupJob) Run(ctx context.Context) error {
	return j.service.Cleanup()
}
```

Also support:

```go
// @Every("10m")
```

## Health

```go
health.Module(health.Options{
	Path: "/health",
})
```

Routes:

```txt
GET /health
GET /health/live
GET /health/ready
```

## Metrics

```go
metrics.Module(metrics.Options{
	Path: "/metrics",
})
```

## Tracing

```go
tracing.Module(tracing.Options{
	ServiceName: "my-api",
	Exporter:    "otlp",
})
```

## Throttler

```go
throttle.Module(throttle.Options{
	Default: throttle.Rule{
		Limit:  100,
		Window: time.Minute,
	},
})
```

Decorator:

```go
// @Throttle("login")
```

## Events

```go
events.Emit(ctx, "user.created", UserCreatedEvent{
	UserID: user.ID,
})
```

Handler:

```go
// @On("user.created")
func (h *UserCreatedHandler) Handle(
	ctx context.Context,
	event UserCreatedEvent,
) error {
	return nil
}
```

---

# 21. CLI

Gest should provide:

```bash
gest new my-api
gest new my-api --module example.test/my-api
gest g module project/team
gest g controller project/team
gest g service project/team
gest g repo project/team
gest g dto project/team create-team
gest g guard auth/jwt
gest g middleware logger
gest g interceptor cache
gest g pipe validation
gest g resource project/team
gest g websocket chat
gest g job emails/send-welcome
gest g test project/team
gest generate
gest dev
gest build
```

`gest new` creates a minimal buildable Chi/net-http app with an app module, a hello module, generated controller metadata, `gest.yaml`, and a `/hello` route returning `{"message":"hello"}`. It does not add database, auth, cache, queue, config, or logger modules by default.

Aliases:

```bash
gest g mo project/team
gest g co project/team
gest g s project/team
gest g r project/team
gest g gu auth/jwt
gest g ws chat
```

---

# 22. CLI Parent Module Updates

When running:

```bash
gest g module project/team
```

Gest should create the module and update its parent module automatically.

Created:

```txt
internal/project/team/
  team.module.go
  team.controller.go
  team.service.go
  team.dto.go
  team_test.go
```

Parent update target:

```txt
internal/project/project.module.go
```

Before:

```go
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "project",

		Providers: gest.Providers(
			gest.Provide(NewProjectService),
			gest.Controller(NewProjectController),
		),
	})
}
```

After:

```go
package project

import (
	"myapp/gest"
	"myapp/internal/project/team"
)

func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "project",

		Imports: gest.Imports(
			team.Module(team.Options{}),
		),

		Providers: gest.Providers(
			gest.Provide(NewProjectService),
			gest.Controller(NewProjectController),
		),
	})
}
```

If no parent module exists, update the app module.

If no suitable module is found:

```txt
WARN parent module not found
HINT add team.Module(team.Options{}) manually
```

Do not fail the entire generation.

---

# 23. Rails-Style Colored CLI Output

Example:

```txt
CREATE  internal/project/team/team.module.go
CREATE  internal/project/team/team.controller.go
CREATE  internal/project/team/team.service.go
CREATE  internal/project/team/team.dto.go
CREATE  internal/project/team/team_test.go

UPDATE  internal/project/project.module.go
  ADD   import "myapp/internal/project/team"
  ADD   team.Module(team.Options{}) to Imports

RUN     gofmt internal/project/team internal/project
RUN     gest generate internal/project/team

DONE    generated module project/team
```

Color conventions:

```txt
CREATE = green
UPDATE = yellow
SKIP   = blue
ERROR  = red
WARN   = magenta/yellow
RUN    = cyan
DONE   = green/bold
```

Flags:

```bash
gest g module project/team --dry-run
gest g module project/team --no-update-parent
gest g module project/team --force
```

---

# 24. `gest generate`

`gest generate` should:

```txt
1. scan packages
2. parse decorators
3. resolve decorator imports
4. validate controller signatures
5. emit *_gest.gen.go files
6. generate OpenAPI metadata if enabled
7. run gofmt on generated files
```

Generated files should use names like:

```txt
reports_gest.gen.go
users_gest.gen.go
```

Generated files should be ignored by `gest dev` watcher to avoid reload loops.

---

# 25. `gest dev`

`gest dev` should support auto reload.

Command:

```bash
gest dev
```

Behavior:

```txt
1. load gest.yaml
2. run gest generate
3. optionally run go test ./...
4. build temporary binary
5. start app
6. watch files
7. on change:
   - debounce
   - regenerate
   - rebuild
   - gracefully restart
```

Important behavior:

```txt
Build the new binary first.
Only stop the old process if the new build succeeds.
Keep the previous process alive on build failure.
```

This is crucial for a pleasant dev loop.

## Example output

```txt
GEST  dev mode

GEN   scanning decorators
GEN   generated internal/reports/reports_gest.gen.go
GEN   generated internal/users/users_gest.gen.go

BUILD compiling ./cmd/api
READY built .gest/tmp/api-dev

BOOT  starting application
APP   GEST starting application
APP   BOOT module config      eager providers=1
APP   BOOT module auth        eager providers=4 controllers=1
APP   BOOT module reports     lazy  providers=3 controllers=1
APP   ROUTE GET /reports/:id  ReportController.FindReport
APP   READY listening :3000
APP   READY boot time 52ms

WATCH watching .go, .env, .yaml, .toml
```

On change:

```txt
CHANGE internal/reports/report.controller.go

GEN    regenerating affected packages
GEN    updated internal/reports/reports_gest.gen.go
BUILD  compiling ./cmd/api
STOP   stopping application
BOOT   restarting application
READY  listening :3000
```

On failure:

```txt
CHANGE internal/reports/report.controller.go

GEN    regenerating affected packages
ERROR  decorator parse failed

internal/reports/report.controller.go:18:1
  unknown decorator @Gets
  did you mean @Get?

WAIT   keeping previous process alive
```

## `gest dev` flags

```bash
gest dev
gest dev --entry ./cmd/api
gest dev --port 3000
gest dev --watch internal,cmd,config
gest dev --ignore "**/*_gest.gen.go,tmp,vendor,node_modules"
gest dev --test
gest dev --no-test
gest dev --generate
gest dev --no-generate
gest dev --openapi
gest dev --race
gest dev --tags dev
gest dev --env .env.local
```

Recommended defaults:

```txt
entry: ./cmd/api
watch: cmd, internal, pkg, config, .env, gest.yaml
ignore: vendor, .git, tmp, .gest, *_gest.gen.go
generate: true
test: false
openapi: true
debounce: 250ms
keep_previous_on_failure: true
```

---

# 26. `gest build`

`gest build` should be a production build orchestrator.

Command:

```bash
gest build
```

Behavior:

```txt
1. load gest.yaml
2. run gest generate
3. validate module graph
4. validate decorators
5. validate routes
6. validate provider graph
7. generate OpenAPI
8. optionally run tests
9. call go build
10. write binary
```

Example output:

```txt
GEST  building my-api

GEN   scanning decorators
GEN   generated 8 controller descriptors

CHECK module graph
CHECK routes
CHECK providers
CHECK openapi

TEST  go test ./...
PASS  tests passed

BUILD go build -trimpath -o bin/my-api ./cmd/api
DONE  binary written bin/my-api
DONE  build time 3.2s
```

Flags:

```bash
gest build
gest build --entry ./cmd/api
gest build --out bin/api
gest build --os linux --arch amd64
gest build --race
gest build --tags prod
gest build --ldflags "-s -w"
gest build --no-test
gest build --no-generate
gest build --docker
```

Cross-compile:

```bash
gest build --os linux --arch amd64 --out bin/api-linux-amd64
```

Maps to:

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o bin/api-linux-amd64 ./cmd/api
```

Gest should orchestrate Go builds, not replace the Go compiler.

---

# 27. `gest.yaml`

Recommended config:

```yaml
project:
  name: my-api

entry: ./cmd/api

router:
  adapter: chi

decorators:
  imports:
    auth: myapp/internal/auth
    policy: myapp/internal/policy

generate:
  root: internal
  openapi: true

dev:
  watch:
    - cmd
    - internal
    - pkg
    - config
    - .env
    - gest.yaml

  ignore:
    - vendor
    - .git
    - tmp
    - .gest
    - "**/*_gest.gen.go"

  generate: true
  openapi: true
  test: false
  race: false
  debounce: 250ms
  keep_previous_on_failure: true

build:
  output: bin/my-api
  entry: ./cmd/api
  generate: true
  openapi: true
  test: true
  trimpath: true
```

---

# 28. Boot Logs

Gest should provide useful boot logs.

Example:

```txt
GEST  starting application

BOOT  module          config      eager   providers=1
BOOT  module          logger      eager   providers=1
BOOT  module          auth        eager   providers=5 controllers=1
BOOT  module          reports     lazy    providers=3 controllers=1

ROUTE GET             /auth/login
ROUTE GET             /auth/me
ROUTE GET             /reports/:id
ROUTE POST            /reports

OPENAPI              /openapi.json
SWAGGER              /docs

READY listening      :3000
READY boot time      48ms
```

Lazy module logs:

```txt
LAZY  initializing    reports
INIT  provider        ReportService
INIT  provider        PDFRenderer
INIT  controller      ReportController
READY module          reports initialized in 17ms
```

Shutdown logs:

```txt
STOP  shutting down
STOP  module          reports
STOP  provider        PDFRenderer
STOP  provider        ReportService
DONE  shutdown        22ms
```

Config:

```go
app := gest.New(
	gest.WithBootLogs(true),
	gest.WithLogLevel(gest.InfoLevel),
)
```

Production should support JSON logs.

---

# 29. Test Support

Gest should provide a `gesttest` package that wraps Go's standard testing style.

Example:

```go
func TestFindUser(t *testing.T) {
	app := gesttest.New(t,
		users.Module(users.Options{}),
	)

	var res users.FindUserResponse

	app.GET("/users/123").
		ExpectStatus(200).
		DecodeJSON(&res)

	if res.ID != "123" {
		t.Fatalf("expected user ID 123, got %s", res.ID)
	}
}
```

Provider override:

```go
func TestFindUser_NotFound(t *testing.T) {
	fakeService := &FakeUserService{}

	app := gesttest.New(t,
		users.Module(users.Options{}),
		gesttest.Override(NewUserService, fakeService),
	)

	app.GET("/users/missing").
		ExpectStatus(404)
}
```

Do not replace Go's `testing` package. Provide helpers.

---

# 30. Build-Time Validation

`gest build` should catch:

```txt
unknown decorators
invalid handler signatures
duplicate routes
missing generated controller metadata
provider cycles
missing providers
ambiguous decorator imports
invalid module imports
lazy module route conflicts
OpenAPI schema generation failures
```

Example error:

```txt
ERROR provider dependency not found

module: reports
provider: NewReportService
missing: *reports.ReportRepository

hint:
  add gest.Provide(NewReportRepository)
  or import the user-owned module that provides *reports.ReportRepository
```

Good errors are a product feature.

---

# 31. Implementation Milestones

## Phase 1: Core Runtime

- `gest.App`
- `gest.Module`
- `gest.ModuleConfig`
- `gest.Provider`
- simple DI container
- constructor injection
- controller provider support
- Chi adapter
- basic context
- JSON responses
- error helpers

## Phase 2: Generator

- decorator parser
- controller parser
- route parser
- generated `GestController()` methods
- decorator import resolution
- gofmt
- validation of handler signatures

## Phase 3: Typed Handlers

- `gest.JSON(...)`
- DTO binding
- path/query/header/body tags
- validation integration
- error mapping

## Phase 4: CLI

- `gest new`
- `gest generate`
- `gest g module`
- `gest g controller`
- `gest g service`
- parent module AST updates
- colored output

## Phase 5: Dev and Build

- `gest dev`
- file watching
- auto reload
- keep previous process on build failure
- `gest build`
- build-time validation

## Phase 6: OpenAPI and Swagger

- OpenAPI metadata registry
- schema generation from DTOs
- Swagger UI module
- auth metadata in OpenAPI

## Phase 7: Optional Official Modules

- config
- logger
- validation
- jwt
- auth
- cache
- health

## Phase 8: Advanced Runtime

- lazy modules
- lifecycle events
- websocket support
- streaming response
- queue module
- scheduler
- metrics
- tracing

---

# 32. Final Target Developer Experience

Create project:

```bash
gest new my-api
cd my-api
```

Generate module:

```bash
gest g module project/team
```

Run dev server:

```bash
gest dev
```

Write controller:

```go
// @Controller("/projects/:projectId/teams")
// @Auth
// @Roles("admin")
type TeamController struct {
	service *TeamService
}

func NewTeamController(service *TeamService) *TeamController {
	return &TeamController{service: service}
}

// @Get("/:teamId")
// @Status(200)
// @Status(404)
func (c *TeamController) FindTeam(
	ctx *gest.Context,
	req *FindTeamRequest,
) (*FindTeamResponse, error) {
	return c.service.FindTeam(ctx, req)
}
```

Write module:

```go
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "project.team",

		Imports: gest.Imports(
			repositories.Module(repositories.Options{}),
		),

		Providers: gest.Providers(
			gest.Provide(NewTeamService),
			gest.Controller(NewTeamController),
		),
	})
}
```

Build:

```bash
gest build --out bin/my-api
```

That is Gest:

```txt
explicit modules
decorator comments
generated controller metadata
typed DTO handlers
router adapters
optional infrastructure modules
CLI scaffolding
auto reload dev server
production build orchestration
boot logs
Go escape hatches
```
