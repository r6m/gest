package generator

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseControllersValidControllerWithTag(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
// @Tag("Users")
type UserController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}

	got := controllerSummaries(root, controllers)
	want := []controllerSummary{
		{
			TypeName: "UserController",
			BasePath: "/users",
			Tag:      "Users",
			File:     "users/controller.go",
			Line:     3,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("controllers = %#v, want %#v", got, want)
	}
}

func TestParseControllersValidControllerWithoutTag(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"reports/controller.go": `package reports

// @Controller("/reports")
type ReportController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(controllers))
	}
	if controllers[0].TypeName != "ReportController" {
		t.Fatalf("TypeName = %q, want ReportController", controllers[0].TypeName)
	}
	if controllers[0].BasePath != "/reports" {
		t.Fatalf("BasePath = %q, want /reports", controllers[0].BasePath)
	}
	if controllers[0].Tag != "" {
		t.Fatalf("Tag = %q, want empty", controllers[0].Tag)
	}
}

func TestParseControllersHideDecorator(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
// @Hide()
type UserController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(controllers))
	}
	if !controllers[0].Hidden {
		t.Fatal("Hidden = false, want true")
	}
}

func TestParseControllersInvalidControllerPathSyntax(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller({path: "/users"})
type UserController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("controllers = %#v, want none", controllers)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidDecoratorSyntax, "users/controller.go", 3, "Controller")
}

func TestParseControllersControllerOnInvalidTarget(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
func NewUserController() {}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("controllers = %#v, want none", controllers)
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticInvalidTarget, "users/controller.go", 3, "NewUserController")
}

func TestParseControllersUnknownDecoratorDiagnostic(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @Controller("/users")
// @Cache("users")
type UserController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	controllers, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(controllers))
	}
	assertDiagnostic(t, root, diagnostics, DiagnosticUnknownDecorator, "users/controller.go", 4, "Cache")
}

func TestParseControllersDiagnosticOrderingIsDeterministic(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"z/controller.go": `package z

// @Unknown
type ZController struct{}
`,
		"a/controller.go": `package a

// @Controller(123)
type AController struct{}
`,
	})
	packages := scanFixturePackages(t, root)

	firstControllers, firstDiagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("first ParseControllers returned error: %v", err)
	}
	secondControllers, secondDiagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("second ParseControllers returned error: %v", err)
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
	first := diagnosticSummary(root, firstDiagnostics[0])
	second := diagnosticSummary(root, firstDiagnostics[1])
	if first.File != "a/controller.go" || second.File != "z/controller.go" {
		t.Fatalf("diagnostic order = %#v, %#v; want a before z", first, second)
	}
}

func TestParseControllersUnknownDecoratorRuleOnlyDeclarationComments(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

// @UnknownFileComment

// @Controller("/users")
type UserController struct{}

func run() {
	// @UnknownInsideFunction
}
`,
	})
	packages := scanFixturePackages(t, root)

	_, diagnostics, err := ParseControllers(packages)
	if err != nil {
		t.Fatalf("ParseControllers returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none for non-declaration comments", diagnostics)
	}
}

type controllerSummary struct {
	TypeName string
	BasePath string
	Tag      string
	File     string
	Line     int
}

type diagnosticSummaryValue struct {
	Code   string
	File   string
	Line   int
	Target string
}

func controllerSummaries(root string, controllers []Controller) []controllerSummary {
	summaries := make([]controllerSummary, 0, len(controllers))
	for _, controller := range controllers {
		relative, err := filepath.Rel(root, controller.File)
		if err != nil {
			relative = controller.File
		}
		summaries = append(summaries, controllerSummary{
			TypeName: controller.TypeName,
			BasePath: controller.BasePath,
			Tag:      controller.Tag,
			File:     filepath.ToSlash(relative),
			Line:     controller.Line,
		})
	}
	return summaries
}

func diagnosticSummary(root string, diagnostic Diagnostic) diagnosticSummaryValue {
	relative, err := filepath.Rel(root, diagnostic.File)
	if err != nil {
		relative = diagnostic.File
	}
	return diagnosticSummaryValue{
		Code:   diagnostic.Code,
		File:   filepath.ToSlash(relative),
		Line:   diagnostic.Line,
		Target: diagnostic.Target,
	}
}

func assertDiagnostic(t *testing.T, root string, diagnostics []Diagnostic, code string, file string, line int, target string) {
	t.Helper()

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics length = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnosticSummary(root, diagnostics[0])
	want := diagnosticSummaryValue{
		Code:   code,
		File:   file,
		Line:   line,
		Target: target,
	}
	if got != want {
		t.Fatalf("diagnostic = %#v, want %#v", got, want)
	}
	if diagnostics[0].Severity != SeverityError {
		t.Fatalf("Severity = %q, want %q", diagnostics[0].Severity, SeverityError)
	}
	if diagnostics[0].Message == "" {
		t.Fatal("Message is empty, want actionable diagnostic message")
	}
	if diagnostics[0].Hint == "" {
		t.Fatal("Hint is empty, want actionable diagnostic hint")
	}
}

func scanFixturePackages(t *testing.T, root string) []Package {
	t.Helper()

	packages, err := ScanPackages(root, ScanOptions{IncludeTestdata: true})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	return packages
}
