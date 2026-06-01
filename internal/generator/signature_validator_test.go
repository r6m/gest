package generator

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateHandlerSignaturesAcceptedContextError(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) List(ctx *gest.Context) error {
	return nil
}
`)

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	route := controllers[0].Routes[0]
	if route.RequestType != "" {
		t.Fatalf("RequestType = %q, want empty", route.RequestType)
	}
	if route.ResponseType != "" {
		t.Fatalf("ResponseType = %q, want empty", route.ResponseType)
	}
}

func TestValidateHandlerSignaturesAcceptedRequestResponseError(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) Find(ctx *gest.Context, req *FindUserRequest) (*FindUserResponse, error) {
	return nil, nil
}
`)

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	route := controllers[0].Routes[0]
	if route.RequestType != "FindUserRequest" {
		t.Fatalf("RequestType = %q, want FindUserRequest", route.RequestType)
	}
	if route.ResponseType != "FindUserResponse" {
		t.Fatalf("ResponseType = %q, want FindUserResponse", route.ResponseType)
	}
}

func TestValidateHandlerSignaturesAcceptedRequestError(t *testing.T) {
	root := signatureFixture(t, `// @Post("/")
func (c *UserController) Create(ctx *gest.Context, req *CreateUserRequest) error {
	return nil
}
`)

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	route := controllers[0].Routes[0]
	if route.RequestType != "CreateUserRequest" {
		t.Fatalf("RequestType = %q, want CreateUserRequest", route.RequestType)
	}
	if route.ResponseType != "" {
		t.Fatalf("ResponseType = %q, want empty", route.ResponseType)
	}
}

func TestValidateHandlerSignaturesMissingContext(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) List() error {
	return nil
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "List")
}

func TestValidateHandlerSignaturesWrongContextType(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) List(ctx *Context) error {
	return nil
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "List")
}

func TestValidateHandlerSignaturesNonPointerRequest(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) Find(ctx *gest.Context, req FindUserRequest) error {
	return nil
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "Find")
}

func TestValidateHandlerSignaturesUnsupportedExtraParameter(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) Find(ctx *gest.Context, req *FindUserRequest, extra string) error {
	return nil
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "Find")
}

func TestValidateHandlerSignaturesUnsupportedReturnCount(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) Find(ctx *gest.Context, req *FindUserRequest) {
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "Find")
}

func TestValidateHandlerSignaturesUnsupportedResponseShape(t *testing.T) {
	root := signatureFixture(t, `// @Get("/")
func (c *UserController) Find(ctx *gest.Context, req *FindUserRequest) (FindUserResponse, error) {
	return FindUserResponse{}, nil
}
`)

	assertInvalidSignature(t, root, "users/controller.go", 13, "Find")
}

func TestValidateHandlerSignaturesDiagnosticOrderingIsDeterministic(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"z/controller.go": signatureSource(`// @Get("/")
func (c *UserController) Zed() error {
	return nil
}
`),
		"a/controller.go": signatureSource(`// @Get("/")
func (c *UserController) Alpha() error {
	return nil
}
`),
	})
	packages := scanFixturePackages(t, root)

	firstControllers, firstDiagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("first ParseControllerRoutes returned error: %v", err)
	}
	secondControllers, secondDiagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("second ParseControllerRoutes returned error: %v", err)
	}
	if !reflect.DeepEqual(firstControllers, secondControllers) {
		t.Fatalf("controllers changed between runs: first %#v second %#v", firstControllers, secondControllers)
	}
	if !reflect.DeepEqual(firstDiagnostics, secondDiagnostics) {
		t.Fatalf("diagnostics changed between runs: first %#v second %#v", firstDiagnostics, secondDiagnostics)
	}
	if len(firstDiagnostics) != 2 {
		t.Fatalf("diagnostics length = %d, want 2", len(firstDiagnostics))
	}
	if diagnosticSummary(root, firstDiagnostics[0]).File != "a/controller.go" {
		t.Fatalf("first diagnostic = %#v, want a/controller.go", diagnosticSummary(root, firstDiagnostics[0]))
	}
}

func assertInvalidSignature(t *testing.T, root string, file string, line int, target string) {
	t.Helper()

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(controllers))
	}
	if len(controllers[0].Routes) != 0 {
		t.Fatalf("routes = %#v, want none for invalid handler signature", controllers[0].Routes)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidHandlerSignature, file, line, target)
	if !contains(diagnostics[0].Message, "invalid handler signature") {
		t.Fatalf("Message = %q, want received signature", diagnostics[0].Message)
	}
	if !contains(diagnostics[0].Hint, "func(ctx *gest.Context) error") {
		t.Fatalf("Hint = %q, want accepted signatures", diagnostics[0].Hint)
	}
}

func signatureFixture(t *testing.T, methodSource string) string {
	t.Helper()

	return newFixture(t, map[string]string{
		"go.mod":              "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": signatureSource(methodSource),
	})
}

func signatureSource(methodSource string) string {
	return `package users

import "github.com/r6m/gest"

// @Controller("/users")
type UserController struct{}

type FindUserRequest struct{}
type FindUserResponse struct{}
type CreateUserRequest struct{}

` + methodSource
}

func contains(value string, pattern string) bool {
	return strings.Contains(value, pattern)
}
