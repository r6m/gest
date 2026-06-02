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
	config.Module(config.Options{}),
	logger.Module(logger.Options{
		Format: "json",
	}),
	jwt.Module(jwt.Options{
		SecretFromEnv: "JWT_SECRET",
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
   - Generated code must call public runtime APIs such as `gest.HandleContext(...)`, `gest.HandleRequestResponse(...)`, `gest.Status(...)`, and controller methods directly.
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
			}),

			logger.Module(logger.Options{}),

			auth.Module(auth.Options{
				JWTSecretFromEnv: "JWT_SECRET",
				AccessTTL:          time.Hour,
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

				Handler: gest.HandleRequestResponse(
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

// @Use(auth.JWTGuard)
// @Throttle("login")
// @Cache("user:{id}", ttl="5m")

// @OnEvent("user.created")
// @Processor("email.welcome")
// @Cron("0 */5 * * * *")
// @Gateway("/ws/chat")
// @Subscribe("message.send")
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

func(ctx *gest.Context) (*Res, error)

func(ctx *gest.Context, req *Req) (*Res, error)

func(ctx *gest.Context, req *Req) error

func(w http.ResponseWriter, r *http.Request)
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

## Handler Adapter

Typed handler adaptation must be resolved when routes are defined, not by inspecting handler signatures on every request.

`gest.Handle(...)` adapts supported typed handlers into `HandlerFunc` once. It may use reflection or type switches during adapter construction, but the returned `HandlerFunc` must not repeat signature inspection per request.

For handlers with a request DTO, the adapter binds params, query values, headers, and JSON body fields into `*Req`, then validates it before calling the controller method.

For handlers that return `(*Res, error)`, a non-nil response is written as JSON with the configured success status. A nil response writes no content with the configured empty status.

Typed handlers return JSON responses by default when they return a non-nil response DTO. Generated controller metadata should choose the appropriate explicit adapter at generation time:

```go
Handler: gest.HandleRequestResponse(c.FindUser, gest.Status(200))
```

The generated file should never emit a generic runtime dispatcher that re-checks the controller method signature on every request.

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

SSE uses normal HTTP routes. Do not add `@SSE` or `@Stream` decorators in the MVP. The helper should set `text/event-stream`, flush after sends, respect request cancellation, and update response status tracking.

---

# 13. WebSockets

Gest should support WebSocket through an optional module, not core runtime.

```go
// @Gateway("/ws/chat")
type ChatGateway struct {
	service *ChatService
}

func NewChatGateway(service *ChatService) *ChatGateway {
	return &ChatGateway{service: service}
}

// @Subscribe("message.send")
func (g *ChatGateway) SendMessage(
	ctx context.Context,
	client *websocket.Client,
	msg SendMessage,
) error {
	return g.service.Send(ctx, client.ID(), msg)
}
```

DTO:

```go
type SendMessage struct {
	RoomID string `json:"roomId" validate:"required"`
	Text   string `json:"text" validate:"required"`
}
```

WebSocket module usage:

```go
app.Import(
	websocket.Module(websocket.Options{
		Path: "/ws",
	}),
)
```

Generated metadata should be explicit:

```go
func (g *ChatGateway) GestGateway() websocket.GatewayDefinition {
	return websocket.GatewayDefinition{
		Path: "/ws/chat",
		Subscriptions: []websocket.SubscriptionDefinition{
			{
				Event:   "message.send",
				Handler: websocket.Handle[SendMessage](g.SendMessage),
			},
		},
	}
}
```

WebSocket is separate from internal events, queues, and SSE. Do not build a Socket.IO clone in the MVP. Do not add rooms, namespaces, distributed pub/sub, or built-in auth policy yet. Use existing middleware/guards before upgrade where practical.

---

# 14. Guards and User-Owned Auth

Auth, roles, and permissions are application policy. Gest must not ship a built-in auth, role, or permission module.

Gest should provide mechanics that make user-owned auth modules pleasant:

- app-level middleware
- controller and route middleware
- DI-resolved guards
- route metadata through a single `@Use(...)` decorator
- context storage
- bearer-token helpers
- OpenAPI security metadata hooks later

Built-in auth policy shortcuts are not part of v0. User applications can define their own decorators or guard conventions later, but Gest should start with explicit `@Use(...)` references:

```go
// @Use(auth.JWTGuard)
// @Use(requestlog.Audit)
```

`@Use(...)` is intentionally broad. The referenced provider decides what it is by implementing a Gest interface.

## Middleware and guard interfaces

```go
type Middleware interface {
	Handle(next HandlerFunc) HandlerFunc
}

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

func (f MiddlewareFunc) Handle(next HandlerFunc) HandlerFunc {
	return f(next)
}

type Guard interface {
	CanActivate(ctx *Context) error
}
```

Function middleware remains supported through `MiddlewareFunc`, while dependency-injected middleware should usually be structs:

```go
type RequestLogger struct {
	log *slog.Logger
}

func NewRequestLogger(log *slog.Logger) *RequestLogger {
	return &RequestLogger{log: log}
}

func (m *RequestLogger) Handle(next gest.HandlerFunc) gest.HandlerFunc {
	return func(ctx *gest.Context) error {
		start := time.Now()
		err := next(ctx)

		req := ctx.RawRequest()
		m.log.Info("request",
			"method", req.Method,
			"path", req.URL.Path,
			"status", ctx.ResponseStatus(),
			"duration_ms", time.Since(start).Milliseconds(),
		)

		return err
	}
}
```

App-level middleware applies to every route:

```go
app.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
	return func(ctx *gest.Context) error {
		return next(ctx)
	}
}))
```

Request logging skip logic belongs in the middleware itself, not in framework-level route skip rules.

Generated route metadata should classify `@Use(...)` references by interface:

```go
Components: []gest.RouteComponentFactory{
	gest.ResolveRouteComponent[*requestlog.Audit](),
	gest.ResolveRouteComponent[*auth.JWTGuard](),
},
```

Execution order is strict:

```txt
app middleware
controller middleware
route middleware
guards
handler
```

Within middleware and guard categories, declaration order is preserved. If a user mixes middleware and guards in `@Use(...)`, category order wins.

User-owned auth module example:

```go
package auth

import (
	"time"

	"myapp/gest"
	"myapp/gest/modules/jwt"
)

type Options struct {
	JWTSecretFromEnv    string
	AccessTTL          time.Duration
}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "auth",

		Imports: gest.Imports(
			jwt.Module(jwt.Options{
				SecretFromEnv: options.JWTSecretFromEnv,
				AccessTTL:     options.AccessTTL,
			}),
		),

		Providers: gest.Providers(
			gest.Value(options),

			gest.Provide(NewAuthService),
			gest.Provide(NewJWTGuard),

			gest.Controller(NewAuthController),
		),
	})
}
```

This module lives in the user's application, such as `myapp/internal/auth`. It is not a Gest official module. It can define its own roles, permissions, users, repositories, policies, and token claims.

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
	SecretFromEnv: "JWT_SECRET",
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
- validation tags

OpenAPI is enabled at the app level:

```go
app.OpenAPI("/openapi.json", gest.OpenAPITitle("My API"), gest.OpenAPIVersion("1.0.0"))
```

When OpenAPI is enabled, registered routes are included by default. Request and response schemas should be inferred from typed handler metadata. Use `@Hide()` on a controller or route to exclude it. Do not require Swagger-specific body/response decorators for the MVP.

Swagger UI should live outside core as an optional module:

```go
app.Import(swagger.Module(swagger.Options{
	Path:        "/docs",
	OpenAPIPath: "/openapi.json",
}))
```

---

# 20. Optional Official Modules

Gest should provide official modules, but keep core lightweight.

Recommended modules:

```txt
config
logger
validation
swagger
jwt
health
testing
```

Official modules are optional conveniences. Core runtime must not import `gest/modules/...`, and users can replace official modules with their own modules.

Global modules are allowed only as explicit app composition. If the app imports a module marked global, that module's providers are available across the app graph through normal constructor injection. This is for config/logger ergonomics, not service-locator access, package scanning, or hidden route registration.

Phase 7 scope is limited to:

```txt
config
logger
validation
health
jwt
```

Auth, roles, and permissions are user-owned modules. Cache, throttle, events, queue, scheduler, metrics, tracing, mailer, files, and WebSocket modules are deferred.

## Config

```go
type AppConfig struct {
	Port      string `env:"PORT" default:"3000"`
	JWTSecret string `env:"JWT_SECRET" validate:"required"`
}

config.Module(config.Options{
	EnvFiles: []string{".env", ".env.local"},
	Load: []config.LoadTarget{
		config.Struct[AppConfig](),
	},
})
```

The config module should provide `*config.Service` and loaded user-owned structs such as `*AppConfig` through DI.

## Logger

```go
logger.Module(logger.Options{
	Level:  "info",
	Format: "json",
})
```

Logger should use Go `log/slog` and provide `*slog.Logger` through DI. Boot logs remain controlled by `gest.WithBootLogs(true)`.

## Validation

```go
validation.Module()
```

Validation stays behind the core `gest.Validator` interface. If automatic installation through normal module mechanics is not clean, install explicitly:

```go
app := gest.New(
	gest.WithValidator(validation.NewValidator()),
)
```

## JWT

```go
jwt.Module(jwt.Options{
	Secret:           "dev-secret",
	SecretFromEnv:    "JWT_SECRET",
	Issuer:           "my-api",
	AccessTTL:        time.Hour,
})
```

JWT must not assume a user database or user model.

## Deferred Ecosystem Modules

The following modules are intentionally deferred beyond Phase 7. They should use the same optional module model when implemented.

Keep these modules under `modules/...` for now. Do not split them into a separate `contrib` workspace or separate Go modules until the APIs are stable enough to justify independent releases.

Each module owns its adapters:

```txt
modules/events/adapters/memory
modules/scheduler/adapters/memory
modules/queue/adapters/memory
modules/queue/adapters/redis
modules/cache/adapters/memory
modules/cache/adapters/redis
```

Core runtime must not import these packages. Applications opt in by importing the module they use.

Recommended global behavior:

- `events`: may be global
- `cache`: may be global
- `queue`: may support global but should not require it
- `scheduler`: usually module-owned, not global by default

## Queue

BullMQ-like module for Go.

```go
queue.Module(queue.Options{
	Adapter: redisqueue.New(redisqueue.Options{
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

func (p *WelcomeEmailProcessor) Process(
	ctx context.Context,
	job WelcomeEmailJob,
) error {
	return p.mailer.SendWelcome(ctx, job.UserID)
}
```

Use non-generic payload handlers first. Add `queue.Job[T]` later only if users need job metadata such as ID, attempts, or headers in the handler.

## Scheduler

```go
// @Cron("0 */5 * * * *")
type CleanupTask struct {
	service *CleanupService
}

func NewCleanupTask(service *CleanupService) *CleanupTask {
	return &CleanupTask{service: service}
}

// @Cron("0 */5 * * * *")
func (t *CleanupTask) Run(ctx context.Context) error {
	return t.service.Cleanup(ctx)
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
type UserService struct {
	events *events.Bus
}

func NewUserService(events *events.Bus) *UserService {
	return &UserService{events: events}
}

func (s *UserService) Create(ctx context.Context) error {
	return s.events.Emit(ctx, "user.created", UserCreatedEvent{
		UserID: "user_123",
	})
}
```

Handler:

```go
// @OnEvent("user.created")
type SendWelcomeEmailListener struct {
	mailer *MailerService
}

func NewSendWelcomeEmailListener(mailer *MailerService) *SendWelcomeEmailListener {
	return &SendWelcomeEmailListener{mailer: mailer}
}

func (l *SendWelcomeEmailListener) Handle(ctx context.Context, event UserCreatedEvent) error {
	return l.mailer.SendWelcome(ctx, event.UserID)
}
```

## Cache

Cache should start as an injectable service, not a decorator system.

```go
cache.Module(cache.Options{
	Global: true,
	Store:  memorycache.New(memorycache.Options{}),
})
```

Usage:

```go
type UserService struct {
	cache *cache.Cache
}

func NewUserService(cache *cache.Cache) *UserService {
	return &UserService{cache: cache}
}

func (s *UserService) Find(ctx context.Context, id string) (*User, error) {
	var user User
	if ok, err := s.cache.GetJSON(ctx, "users:"+id, &user); err != nil || ok {
		return &user, err
	}
	user = User{ID: id}
	if err := s.cache.SetJSON(ctx, "users:"+id, user, time.Minute); err != nil {
		return nil, err
	}
	return &user, nil
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
gest g resource project/team
gest g listener users/send-welcome
gest g processor email/welcome
gest g task reports/sync
gest g gateway chat
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
```

Do not add `gest g cache` unless it generates a concrete cache service wrapper. Most cache usage should be ordinary service code with an injected cache provider.

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

For `gest g resource project/team`, the generated tree should include a module, controller, service, DTOs, generated metadata after `gest generate`, and tests. For `gest g controller` and `gest g service`, tests should be generated by default with a `--no-test` escape hatch.

All `gest g ...` commands must understand nesting. `gest g module projects/members` creates `internal/projects/members/members.module.go` and updates `internal/projects/projects.module.go` when that parent module exists. These updates are source edits that the developer can inspect.

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
6. run gofmt on generated files
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

- `gest.Handle(...)`
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
- health
- jwt

## Phase 8: Advanced Runtime

- lazy modules
- streaming response
- metrics
- tracing

## Phase 9: CLI Resource Generation

- nested module generators
- generated controller/service tests
- `gest g resource`

## Phase 10: Ecosystem Modules

- events
- scheduler
- cache
- queue

## Phase 11: WebSocket Module

- optional `modules/websocket`
- `@Gateway`
- `@Subscribe`
- `gest g gateway`

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
// @Use(auth.JWTGuard)
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
