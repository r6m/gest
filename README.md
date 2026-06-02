# Gest

Gest is a Go web framework for building modular HTTP services with explicit dependency injection, generated controller metadata, typed request binding, and OpenAPI support.

It is inspired by NestJS, but it stays Go-native: no runtime source scanning, no hidden global route registry, and no decorator magic during application boot. You write normal Go modules, providers, controllers, and DTOs; Gest handles the repetitive routing and binding glue.

## Why Gest

- Modular application structure with constructor-based singleton injection.
- Controller comments such as `// @Get("/{id}")` are compiled into boring `*_gest.gen.go` metadata.
- Typed handlers support JSON body, path params, query values, headers, validation, and status metadata.
- OpenAPI JSON and optional Swagger UI are first-class.
- Router behavior starts with Chi/net-http and keeps raw HTTP escape hatches available.
- `gesttest` gives fast in-memory HTTP tests without opening ports.

## Install

```bash
go install github.com/r6m/gest/cmd/gest@latest
```

Or use the CLI from a checkout:

```bash
go run ./cmd/gest --help
```

## Create An App

```bash
gest new my-api --module example.test/my-api
cd my-api
gest generate
gest build
```

The starter app exposes `GET /hello`.

## What It Looks Like

Define a feature module:

```go
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "UsersModule",
		Providers: gest.Providers(
			gest.Provide(NewUserService),
			gest.Controller(NewUserController),
		),
	})
}
```

Write a controller as normal Go:

```go
// @Controller("/users")
// @Tag("Users")
type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

// @Get("/{id}")
// @Status(200)
func (c *UserController) FindUser(
	ctx *gest.Context,
	req *FindUserRequest,
) (*UserResponse, error) {
	return c.service.FindUser(req)
}
```

Bind request data into DTOs:

```go
type FindUserRequest struct {
	ID     string `param:"id" validate:"required"`
	Expand bool   `query:"expand" default:"false"`
}
```

Bootstrap the app:

```go
server := gest.New(gest.WithBootLogs(true))
server.OpenAPI("/openapi.json", gest.OpenAPITitle("Users API"))
server.Import(app.Module())

if err := server.Listen(":3000"); err != nil {
	log.Fatal(err)
}
```

## CLI

```bash
gest new my-api --module example.test/my-api
gest generate
gest build
```

`gest generate` parses controller comments and writes deterministic Go metadata. `gest build` runs generation, optional tests, and the underlying Go build.

## Built-In Modules

Gest ships optional modules under `github.com/r6m/gest/modules/...`. They are normal Gest modules, so you import only the pieces your app needs.

Define app-owned config as a normal Go type:

```go
package config

import (
	"time"
)

type AppConfig struct {
	Port      int           `env:"PORT" default:"3000"`
	JWTSecret string        `env:"JWT_SECRET" validate:"required"`
	TokenTTL  time.Duration `env:"TOKEN_TTL" default:"1h"`
}
```

Load and provide it through the built-in config module:

```go
package app

import (
	"log/slog"

	"github.com/r6m/gest"
	gestconfig "github.com/r6m/gest/modules/config"
	"github.com/r6m/gest/modules/health"
	"github.com/r6m/gest/modules/jwt"
	"github.com/r6m/gest/modules/logger"
	"github.com/r6m/gest/modules/swagger"

	"example.test/my-api/internal/config"
)

func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			gestconfig.Module(gestconfig.Options{
				EnvFiles: []string{".env"},
				Load:     []gestconfig.LoadTarget{gestconfig.Struct[config.AppConfig]()},
			}),
			logger.Module(logger.Options{
				Level:  "info",
				Format: "json",
			}),
			jwt.Module(jwt.Options{
				SecretFromEnv: "JWT_SECRET",
				Issuer:        "my-api",
			}),
			health.Module(health.Options{
				Path: "/health",
			}),
			swagger.Module(swagger.Options{
				Path:        "/docs",
				OpenAPIPath: "/openapi.json",
			}),
		),
		Providers: gest.Providers(
			gest.Provide(NewUserService),
			gest.Controller(NewUserController),
		),
	})
}

type UserService struct {
	config *config.AppConfig
	logger *slog.Logger
	tokens *jwt.Service
}

func NewUserService(config *config.AppConfig, logger *slog.Logger, tokens *jwt.Service) *UserService {
	return &UserService{config: config, logger: logger, tokens: tokens}
}
```

Use validation at app startup when you want `validate` tags on bound DTOs to run automatically:

```go
server := gest.New(
	gest.WithValidator(validation.NewValidator()),
)
server.OpenAPI("/openapi.json", gest.OpenAPITitle("Users API"))
server.Import(app.Module())
```

Available modules:

- `modules/config`: load `.env` files and env vars into `config.Service` or app-owned structs.
- `modules/logger`: provide a configured `*slog.Logger`.
- `modules/validation`: provide `gest.Validator`, or use `validation.NewValidator()` with `gest.WithValidator(...)`.
- `modules/health`: expose `/health`, `/health/live`, and `/health/ready`.
- `modules/jwt`: sign and verify HS256 access tokens.
- `modules/swagger`: serve Swagger UI for your OpenAPI route.

## Testing

```go
func TestFindUser(t *testing.T) {
	server := gesttest.New(t, app.Module())

	var response UserResponse
	server.GET("/users/123").
		ExpectStatus(http.StatusOK).
		DecodeJSON(&response)
}
```

## Documentation

- [Quickstart](docs/QUICKSTART.md)
- [Architecture Contract](docs/ARCHITECTURE.md)
- [Design Document](docs/README.md)
- [Contributing](docs/CONTRIBUTING.md)

## Status

Gest is early-stage. The core runtime, generator, CLI workflow, official utility modules, OpenAPI support, and `gesttest` are being built against the checked-in examples.
