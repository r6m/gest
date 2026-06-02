# Quickstart

This guide follows the compiling app in `examples/hello`.

## Create An App

```go
server := gest.New(gest.WithBootLogs(true))
server.OpenAPI("/openapi.json", gest.OpenAPITitle("Hello API"), gest.OpenAPIVersion("1.0.0"))
server.Import(app.Module())

if err := server.Listen(":3000"); err != nil {
	log.Fatal(err)
}
```

`OpenAPI` is an app-level runtime route. Swagger UI stays optional through `modules/swagger`.

## Define Modules

```go
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			users.Module(),
			swagger.Module(swagger.Options{
				Path:        "/docs",
				OpenAPIPath: "/openapi.json",
			}),
		),
	})
}
```

Feature modules expose providers and controllers explicitly:

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

## Add A Service

Services are ordinary Go types constructed through provider functions.

```go
type UserService struct {
	users map[string]UserResponse
}

func NewUserService() *UserService {
	return &UserService{users: map[string]UserResponse{}}
}
```

Gest uses singleton constructor injection. Do not use hidden registries or global container lookups.

## Add DTOs

```go
type FindUserRequest struct {
	ID     string `param:"id" validate:"required"`
	Expand bool   `query:"expand" default:"false"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
```

Supported binding sources include path params, query values, headers, and JSON body fields.

## Add A Controller

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
// @Status(404)
func (c *UserController) FindUser(ctx *gest.Context, request *FindUserRequest) (*UserResponse, error) {
	return c.service.FindUser(request)
}

// @Post("/")
// @Status(201)
func (c *UserController) CreateUser(ctx *gest.Context, request *CreateUserRequest) (*UserResponse, error) {
	return c.service.CreateUser(request), nil
}
```

Route metadata is generated into `*_gest.gen.go`. Generated files are normal Go and call public runtime APIs such as `gest.JSON`.

## Generate And Build

Example `gest.yaml`:

```yaml
project:
  name: hello
entry: ./cmd/api
generate:
  root: .
  openapi: true
build:
  output: bin/hello
  test: true
```

Run from `examples/hello`:

```bash
go run ../../cmd/gest generate
go run ../../cmd/gest build
```

Run from the repository root:

```bash
rtk go run ./cmd/gest generate --root examples/hello
rtk go build ./examples/hello/cmd/api
```

## OpenAPI And Swagger

The hello example registers:

- `/openapi.json` through `server.OpenAPI(...)`
- `/docs` through `swagger.Module(...)`

Swagger is optional and lives outside the core runtime.

## Test With Gesttest

```go
func TestHelloExampleFindsUser(t *testing.T) {
	server := gesttest.New(t, app.Module())

	var response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	server.GET("/users/123").
		ExpectStatus(http.StatusOK).
		DecodeJSON(&response)
}
```

Provider overrides are explicit:

```go
server := gesttest.New(t, app.Module(), gesttest.Override(NewUserService, fakeService))
```

`gesttest` wraps normal `testing.T` and `httptest`; it does not replace Go tests.
