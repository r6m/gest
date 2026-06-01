package generator

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseControllerRoutesValidRouteForEachHTTPMethod(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
type UserController struct{}

// @Get("/")
func (c *UserController) List() {}

// @Post("/")
func (c *UserController) Create() {}

// @Put("/:id")
func (c *UserController) Replace() {}

// @Patch("/:id")
func (c *UserController) Update() {}

// @Delete("/:id")
func (c *UserController) Delete() {}
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
func (c *UserController) Find() {}
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

func TestParseControllerRoutesMissingPathArgument(t *testing.T) {
	root := routeErrorFixture(t, `// @Get
func (c *UserController) Find() {}
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 6, "Get")
}

func TestParseControllerRoutesNonStringPath(t *testing.T) {
	root := routeErrorFixture(t, `// @Get(123)
func (c *UserController) Find() {}
`)

	_, diagnostics, err := ParseControllerRoutes(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 6, "Get")
}

func TestParseControllerRoutesPathMustStartWithSlash(t *testing.T) {
	root := routeErrorFixture(t, `// @Get("users")
func (c *UserController) Find() {}
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
func (c *UserController) Find() {}
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
func (c *UserController) Find() {}
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
func (c *UserController) Find() {}
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

func TestParseControllerRoutesDeterministicOrdering(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"z/controller.go": `package z

// @Controller("/z")
type ZController struct{}

// @Get("/z")
// @Auth
func (c *ZController) Zed() {}
`,
		"a/controller.go": `package a

// @Controller("/a")
type AController struct{}

// @Get("/a")
// @Auth
func (c *AController) Alpha() {}
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
