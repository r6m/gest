package gest

import (
	"net/http"
	"strings"
	"testing"
)

type exampleUserController struct{}

type exampleCreateUserRequest struct {
	AccountID string `param:"accountId" validate:"required"`
	TraceID   string `header:"X-Trace-ID"`
	Page      int    `query:"page,omitempty"`
	Name      string `json:"name" validate:"required"`
	Email     string `json:"email,omitempty"`
	Ignored   string `json:"-"`
}

type exampleUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

func newExampleUserController() *exampleUserController {
	return &exampleUserController{}
}

func (c *exampleUserController) Create(ctx *Context, req *exampleCreateUserRequest) (*exampleUserResponse, error) {
	return &exampleUserResponse{
		ID:    req.AccountID,
		Name:  req.Name,
		Email: req.Email,
	}, nil
}

func (c *exampleUserController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "ExampleUserController",
		Tag:      "users",
		BasePath: "/accounts/{accountId}/users",
		Routes: []RouteDefinition{
			{
				Name:     "Create",
				Method:   http.MethodPost,
				Path:     "",
				Handler:  Handle(c.Create, Status(http.StatusCreated)),
				Request:  (*exampleCreateUserRequest)(nil),
				Response: (*exampleUserResponse)(nil),
				Statuses: []int{http.StatusCreated},
				Metadata: RouteMetadata{
					Summary:     "Create user",
					Description: "Creates a user in an account.",
				},
			},
		},
	}
}

func TestPhase5ExampleAppOpenAPIGolden(t *testing.T) {
	app := New()
	app.OpenAPI("", OpenAPITitle("Example Users API"), OpenAPIVersion("1.2.3"))
	app.Import(NewModule(ModuleConfig{
		Name:      "ExampleModule",
		Providers: Providers(Controller(newExampleUserController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	got := strings.TrimSpace(string(serveOpenAPI(t, app, "/openapi.json")))
	want := strings.TrimSpace(`{"openapi":"3.0.3","info":{"title":"Example Users API","version":"1.2.3"},"paths":{"/accounts/{accountId}/users/":{"post":{"operationId":"ExampleUserController.Create","tags":["users"],"summary":"Create user","description":"Creates a user in an account.","parameters":[{"name":"accountId","in":"path","required":true,"schema":{"type":"string"}},{"name":"X-Trace-ID","in":"header","schema":{"type":"string"}},{"name":"page","in":"query","schema":{"type":"integer"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"email":{"type":"string"},"name":{"type":"string"}},"required":["name"]}}}},"responses":{"201":{"description":"Created","content":{"application/json":{"schema":{"$ref":"#/components/schemas/exampleUserResponse","nullable":true}}}}}}}},"components":{"schemas":{"exampleCreateUserRequest":{"type":"object","properties":{"AccountID":{"type":"string"},"Page":{"type":"integer"},"TraceID":{"type":"string"},"email":{"type":"string"},"name":{"type":"string"}},"required":["AccountID","TraceID","Page","name"]},"exampleUserResponse":{"type":"object","properties":{"email":{"type":"string"},"id":{"type":"string"},"name":{"type":"string"}},"required":["id","name"]}}}}`)
	if got != want {
		t.Fatalf("OpenAPI golden mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}
