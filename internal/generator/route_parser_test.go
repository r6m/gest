package generator

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseControllerRoutesValidRouteForEachHTTPMethod(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
type UserController struct{}

// @Get("/")
func (c *UserController) List(ctx *gest.Context) error { return nil }

// @Post("/")
func (c *UserController) Create(ctx *gest.Context) error { return nil }

// @Put("/:id")
func (c *UserController) Replace(ctx *gest.Context) error { return nil }

// @Patch("/:id")
func (c *UserController) Update(ctx *gest.Context) error { return nil }

// @Delete("/:id")
func (c *UserController) Delete(ctx *gest.Context) error { return nil }
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(controllers))
	}

	got := routeSummaries(root, controllers[0].Routes)
	want := []routeSummary{
		{HandlerName: "List", Method: "GET", Path: "/", File: "users/controller.go", Line: 6},
		{HandlerName: "Create", Method: "POST", Path: "/", File: "users/controller.go", Line: 9},
		{HandlerName: "Replace", Method: "PUT", Path: "/:id", File: "users/controller.go", Line: 12},
		{HandlerName: "Update", Method: "PATCH", Path: "/:id", File: "users/controller.go", Line: 15},
		{HandlerName: "Delete", Method: "DELETE", Path: "/:id", File: "users/controller.go", Line: 18},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %#v, want %#v", got, want)
	}
}

func TestParseControllerRoutesMetadata(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
type UserController struct{}

// @Get("/:id")
// @Status(200)
// @Status(404)
// @Summary("Find user")
// @Description("Returns a user by ID")
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	route := controllers[0].Routes[0]
	if !reflect.DeepEqual(route.Statuses, []int{200, 404}) {
		t.Fatalf("Statuses = %#v, want [200 404]", route.Statuses)
	}
	if route.Summary != "Find user" {
		t.Fatalf("Summary = %q, want Find user", route.Summary)
	}
	if route.Description != "Returns a user by ID" {
		t.Fatalf("Description = %q, want Returns a user by ID", route.Description)
	}
}

func TestParseControllerRoutesHideDecorator(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
type UserController struct{}

// @Get("/:id")
// @Hide()
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	route := controllers[0].Routes[0]
	if !route.Hidden {
		t.Fatal("Hidden = false, want true")
	}
}

func TestParseControllerRoutesRouteLevelUseGuard(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import (
	"github.com/r6m/gest"
	"example.test/app/auth"
)

// @Controller("/users")
type UserController struct{}

// @Get("/")
// @Use(auth.JWTGuard)
func (c *UserController) List(ctx *gest.Context) error { return nil }
`,
	})

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	guards := controllers[0].Routes[0].Guards
	if len(guards) != 1 {
		t.Fatalf("guards length = %d, want 1", len(guards))
	}
	if guards[0].Alias != "auth" || guards[0].Symbol != "JWTGuard" || guards[0].ImportPath != "example.test/app/auth" {
		t.Fatalf("guard = %#v, want auth.JWTGuard", guards[0])
	}
}

func TestParseControllerRoutesControllerLevelUseGuard(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import (
	"github.com/r6m/gest"
	"example.test/app/auth"
)

// @Controller("/users")
// @Use(auth.JWTGuard)
type UserController struct{}

// @Get("/")
func (c *UserController) List(ctx *gest.Context) error { return nil }
`,
	})

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	guards := controllers[0].Guards
	if len(guards) != 1 {
		t.Fatalf("controller guards length = %d, want 1", len(guards))
	}
	if guards[0].Alias != "auth" || guards[0].Symbol != "JWTGuard" {
		t.Fatalf("guard = %#v, want auth.JWTGuard", guards[0])
	}
}

func TestParseControllerRoutesUseGuardResolvesExistingImportAlias(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import (
	"github.com/r6m/gest"
	security "example.test/app/auth"
)

// @Controller("/users")
type UserController struct{}

// @Get("/")
// @Use(security.JWTGuard)
func (c *UserController) List(ctx *gest.Context) error { return nil }
`,
	})

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	guard := controllers[0].Routes[0].Guards[0]
	if guard.Alias != "security" || guard.ImportPath != "example.test/app/auth" {
		t.Fatalf("guard = %#v, want aliased auth import", guard)
	}
}

func TestParseControllerRoutesUseGuardUnresolvedAliasDiagnostic(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("/")
// @Use(auth.JWTGuard)
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 7, "Use")
	if !strings.Contains(diagnostics[0].Hint, "import the middleware or guard package") {
		t.Fatalf("hint = %q, want import guidance", diagnostics[0].Hint)
	}
}

func TestParseControllerRoutesMissingPathArgument(t *testing.T) {
	root := routeErrorFixture(t, `// @Get
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 6, "Get")
}

func TestParseControllerRoutesNonStringPath(t *testing.T) {
	root := routeErrorFixture(t, `// @Get(123)
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 6, "Get")
}

func TestParseControllerRoutesPathMustStartWithSlash(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("users")
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 6, "Get")
}

func TestParseControllerRoutesNonIntegerStatus(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("/")
// @Status("ok")
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 7, "Status")
}

func TestParseControllerRoutesDuplicateHTTPMethodDecorators(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("/")
// @Post("/")
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 7, "Post")
}

func TestParseControllerRoutesRouteDecoratorOutsideControllerMethod(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Get("/")
func NotControllerMethod() {}
`,
	})

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("controllers = %#v, want none", controllers)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidTarget, "users/controller.go", 3, "NotControllerMethod")
}

func TestParseControllerRoutesDeferredDecoratorsReturnDiagnostics(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("/")
// @Auth
// @Roles("admin")
func (c *UserController) Find(ctx *gest.Context) error { return nil }
`)

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("controllers = %#v, want parsed route despite deferred diagnostics", controllers)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics length = %d, want 2: %#v", len(diagnostics), diagnostics)
	}
	first := diagnosticSummary(root, diagnostics[0])
	second := diagnosticSummary(root, diagnostics[1])
	if first.Target != "Auth" || second.Target != "Roles" {
		t.Fatalf("diagnostics = %#v, %#v; want Auth then Roles", first, second)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != DiagnosticUnknownDecorator {
			t.Fatalf("Code = %q, want %q", diagnostic.Code, DiagnosticUnknownDecorator)
		}
		if diagnostic.Hint == "" {
			t.Fatal("Hint is empty, want clear deferred decorator hint")
		}
	}
}

func TestParseControllerRoutesWebSocketDecoratorExplainsGatewayDeferral(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("/")
// @WebSocket("/ws")
func (c *UserController) Events(ctx *gest.Context) error { return nil }
`)

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("controllers = %#v, want parsed HTTP route despite deferred WebSocket diagnostic", controllers)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics length = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	diagnostic := diagnosticSummary(root, diagnostics[0])
	if diagnostic.Code != DiagnosticUnknownDecorator {
		t.Fatalf("Code = %q, want %q", diagnostic.Code, DiagnosticUnknownDecorator)
	}
	if diagnostic.Target != "WebSocket" {
		t.Fatalf("Target = %q, want WebSocket", diagnostic.Target)
	}
	if !strings.Contains(diagnostics[0].Message, "not a core HTTP route decorator") {
		t.Fatalf("Message = %q, want core HTTP route guidance", diagnostics[0].Message)
	}
	if !strings.Contains(diagnostics[0].Hint, "@Gateway") || !strings.Contains(diagnostics[0].Hint, "@Subscribe") {
		t.Fatalf("Hint = %q, want @Gateway/@Subscribe guidance", diagnostics[0].Hint)
	}
}

func TestParseControllerRoutesDetachedWebSocketDecoratorExplainsGatewayDeferral(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @WebSocket("/ws")
func Events(ctx *gest.Context) error { return nil }
`,
	})

	controllers, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("controllers = %#v, want none", controllers)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics length = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	diagnostic := diagnosticSummary(root, diagnostics[0])
	if diagnostic.Code != DiagnosticInvalidTarget {
		t.Fatalf("Code = %q, want %q", diagnostic.Code, DiagnosticInvalidTarget)
	}
	if diagnostic.Target != "WebSocket" {
		t.Fatalf("Target = %q, want WebSocket", diagnostic.Target)
	}
	if !strings.Contains(diagnostics[0].Message, "not a core HTTP route decorator") {
		t.Fatalf("Message = %q, want core HTTP route guidance", diagnostics[0].Message)
	}
	if !strings.Contains(diagnostics[0].Hint, "@Gateway") || !strings.Contains(diagnostics[0].Hint, "@Subscribe") {
		t.Fatalf("Hint = %q, want @Gateway/@Subscribe guidance", diagnostics[0].Hint)
	}
}

func TestParseControllerRoutesDeterministicOrdering(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"z/controller.go": `package z

// @Controller("/z")
type ZController struct{}

// @Get("/z")
// @Auth
func (c *ZController) Zed(ctx *gest.Context) error { return nil }
`,
		"a/controller.go": `package a

// @Controller("/a")
type AController struct{}

// @Get("/a")
// @Auth
func (c *AController) Alpha(ctx *gest.Context) error { return nil }
`,
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
	if len(firstControllers) != 2 {
		t.Fatalf("controllers length = %d, want 2", len(firstControllers))
	}
	if firstControllers[0].TypeName != "AController" || firstControllers[1].TypeName != "ZController" {
		t.Fatalf("controller order = %s, %s; want AController, ZController", firstControllers[0].TypeName, firstControllers[1].TypeName)
	}
	if len(firstDiagnostics) != 2 {
		t.Fatalf("diagnostics length = %d, want 2", len(firstDiagnostics))
	}
	if diagnosticSummary(root, firstDiagnostics[0]).File != "a/controller.go" {
		t.Fatalf("first diagnostic = %#v, want a/controller.go", diagnosticSummary(root, firstDiagnostics[0]))
	}
}

type routeSummary struct {
	HandlerName string
	Method      string
	Path        string
	File        string
	Line        int
}

func routeSummaries(root string, routes []Route) []routeSummary {
	summaries := make([]routeSummary, 0, len(routes))
	for _, route := range routes {
		relative, err := filepath.Rel(root, route.File)
		if err != nil {
			relative = route.File
		}
		summaries = append(summaries, routeSummary{
			HandlerName: route.HandlerName,
			Method:      route.Method,
			Path:        route.Path,
			File:        filepath.ToSlash(relative),
			Line:        route.Line,
		})
	}
	return summaries
}

func routeErrorFixture(t *testing.T, methodSource string) string {
	t.Helper()

	return newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
type UserController struct{}

` + methodSource,
	})
}
