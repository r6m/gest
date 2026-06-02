package generator

import (
	"reflect"
	"testing"
)

func TestExplainGenerationReportsParsedRoutesAndSkippedRouteLikeMethods(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import "github.com/r6m/gest"

// @Controller("/users")
type UsersController struct{}

func NewUsersController() *UsersController { return &UsersController{} }

// @Get("/")
func (c *UsersController) List(ctx *gest.Context) error { return nil }

func (c *UsersController) GetAll(ctx *gest.Context, req *GetAllRequest) error { return nil }

type GetAllRequest struct{}
`,
		"users/users.module.go": `package users

import "github.com/r6m/gest"

func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "UsersModule",
		Providers: gest.Providers(
			gest.Controller(NewUsersController),
		),
	})
}
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

	explanation, err := ExplainGeneration(packages, controllers)
	if err != nil {
		t.Fatalf("ExplainGeneration returned error: %v", err)
	}

	if len(explanation.Controllers) != 1 {
		t.Fatalf("controllers length = %d, want 1", len(explanation.Controllers))
	}
	gotRoutes := []string{}
	for _, route := range explanation.Controllers[0].Routes {
		gotRoutes = append(gotRoutes, route.Method+" "+route.Path+" -> "+route.HandlerName)
	}
	if !reflect.DeepEqual(gotRoutes, []string{"GET / -> List"}) {
		t.Fatalf("routes = %#v, want parsed List route", gotRoutes)
	}
	if len(explanation.Rejected) != 1 {
		t.Fatalf("rejected length = %d, want 1: %#v", len(explanation.Rejected), explanation.Rejected)
	}
	rejected := explanation.Rejected[0]
	if rejected.HandlerName != "GetAll" || rejected.TypeName != "UsersController" {
		t.Fatalf("rejected = %#v, want UsersController.GetAll", rejected)
	}
	if rejected.Reason != "method has a valid Gest handler signature but no route decorator" {
		t.Fatalf("reason = %q", rejected.Reason)
	}
}

func TestExplainGenerationReportsDetachedRouteAndMissingModuleProvider(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import "github.com/r6m/gest"

type UsersController struct{}

// @Get("/")
func (c *UsersController) GetAll(ctx *gest.Context, req *GetAllRequest) error { return nil }

type GetAllRequest struct{}
`,
	})
	packages := scanFixturePackages(t, root)
	controllers, diagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("controllers = %#v, want none", controllers)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != DiagnosticInvalidTarget {
		t.Fatalf("diagnostics = %#v, want invalid target", diagnostics)
	}

	explanation, err := ExplainGeneration(packages, controllers)
	if err != nil {
		t.Fatalf("ExplainGeneration returned error: %v", err)
	}

	if len(explanation.Rejected) != 1 {
		t.Fatalf("rejected length = %d, want 1: %#v", len(explanation.Rejected), explanation.Rejected)
	}
	if explanation.Rejected[0].Reason != "method has a route decorator but receiver type UsersController is not a parsed controller" {
		t.Fatalf("reason = %q", explanation.Rejected[0].Reason)
	}
	if !hasDiagnosticCode(explanation.Hints, DiagnosticDetachedRoute) {
		t.Fatalf("hints = %#v, want detached route hint", explanation.Hints)
	}
}

func hasDiagnosticCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
