# Hello Contract App

This example is the acceptance target for Gest. It starts as a design fixture and should become a compiling example as implementation phases land.

Do not add non-compiling Go files here. Until the runtime exists, keep target code as snippets in this README or in fixture files that tests explicitly control.

## Final Shape

The finished example should prove:

- app bootstrap
- root module import
- feature module registration
- singleton constructor injection
- controller provider registration
- generated `GestController()` metadata
- typed JSON handler
- DTO binding from path/query/body
- Chi/net-http route serving
- `go test ./...`
- `golangci-lint run ./...`

## Target Layout

```txt
examples/hello/
  cmd/api/main.go
  internal/app/app.module.go
  internal/users/users.module.go
  internal/users/users.controller.go
  internal/users/users.service.go
  internal/users/users.dto.go
  internal/users/users_gest.gen.go
  gest.yaml
```

## Target Bootstrap

```go
package main

import (
	"log"

	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/hello/internal/app"
)

func main() {
	server := gest.New(gest.WithBootLogs(true))
	server.Import(app.Module())

	if err := server.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
```

## Target Controller

```go
// @Controller("/users")
// @Tag("Users")
type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

// @Get("/:id")
// @Status(200)
// @Status(404)
func (c *UserController) FindUser(
	ctx *gest.Context,
	req *FindUserRequest,
) (*FindUserResponse, error) {
	return c.service.FindUser(ctx, req)
}
```

## Target DTOs

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

## Acceptance Behavior

When the relevant phases are complete:

```bash
rtk go test ./...
rtk proxy golangci-lint run ./...
rtk go run ./examples/hello/cmd/api
```

Then:

```bash
curl http://localhost:3000/users/123
```

should return a JSON user response through generated controller metadata and the Chi/net-http adapter.
