package gesttest_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/gesttest"
)

type userService interface {
	Find(id string) string
	Create(name string) string
}

type realUserService struct{}

func newUserService() userService {
	return realUserService{}
}

func (realUserService) Find(id string) string {
	return "real-" + id
}

func (realUserService) Create(name string) string {
	return "created-" + name
}

type fakeUserService struct{}

func (fakeUserService) Find(id string) string {
	return "fake-" + id
}

func (fakeUserService) Create(name string) string {
	return "fake-created-" + name
}

type userController struct {
	service userService
}

func newUserController(service userService) *userController {
	return &userController{service: service}
}

type userResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type createUserRequest struct {
	Name string `json:"name"`
}

func (c *userController) Show(ctx *gest.Context) error {
	id := ctx.Param("id")
	return ctx.JSON(http.StatusOK, userResponse{
		ID:   id,
		Name: c.service.Find(id),
	})
}

func (c *userController) Create(_ *gest.Context, request *createUserRequest) (*userResponse, error) {
	return &userResponse{
		ID:   "new",
		Name: c.service.Create(request.Name),
	}, nil
}

func (c *userController) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name:     "UserController",
		BasePath: "/users",
		Routes: []gest.RouteDefinition{
			{
				Name:    "Show",
				Method:  http.MethodGet,
				Path:    "/{id}",
				Handler: c.Show,
			},
			{
				Name:    "Create",
				Method:  http.MethodPost,
				Path:    "/",
				Handler: gest.JSON(c.Create, gest.Status(http.StatusCreated)),
			},
		},
	}
}

func userModule() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "UsersModule",
		Providers: gest.Providers(
			gest.Provide(newUserService),
			gest.Controller(newUserController),
		),
	})
}

func TestGETSuccessAndDecodeJSON(t *testing.T) {
	app := gesttest.New(t, userModule())

	var response userResponse
	app.GET("/users/123").
		ExpectStatus(http.StatusOK).
		DecodeJSON(&response)

	if response.ID != "123" || response.Name != "real-123" {
		t.Fatalf("response = %#v, want user payload", response)
	}
}

func TestPOSTJSONRequest(t *testing.T) {
	app := gesttest.New(t, userModule())

	var response userResponse
	app.POST("/users/").
		Header("X-Test", "yes").
		JSONBody(createUserRequest{Name: "Ada"}).
		ExpectStatus(http.StatusCreated).
		DecodeJSON(&response)

	if response.Name != "created-Ada" {
		t.Fatalf("response.Name = %q, want created-Ada", response.Name)
	}
}

func TestProviderOverrideReplacesService(t *testing.T) {
	app := gesttest.New(t, userModule(), gesttest.Override(newUserService, fakeUserService{}))

	var response userResponse
	app.GET("/users/123").
		ExpectStatus(http.StatusOK).
		DecodeJSON(&response)

	if response.Name != "fake-123" {
		t.Fatalf("response.Name = %q, want fake-123", response.Name)
	}
}

func TestRawResponseAccess(t *testing.T) {
	app := gesttest.New(t, userModule())

	response := app.GET("/users/123").ExpectStatus(http.StatusOK).RawResponse()
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", contentType)
	}
}

func TestStatusAssertionFailureIsHelpfulAndMarksHelper(t *testing.T) {
	tb := &recordingTB{}
	app := gesttest.New(tb, userModule())

	app.GET("/users/123").ExpectStatus(http.StatusCreated)

	if tb.helperCalls == 0 {
		t.Fatal("Helper was not called")
	}
	if !strings.Contains(tb.message, "HTTP status = 200, want 201") {
		t.Fatalf("failure message = %q, want status details", tb.message)
	}
	if !strings.Contains(tb.message, `"name":"real-123"`) {
		t.Fatalf("failure message = %q, want response body", tb.message)
	}
}

type recordingTB struct {
	helperCalls int
	message     string
}

func (t *recordingTB) Helper() {
	t.helperCalls++
}

func (t *recordingTB) Fatalf(format string, args ...any) {
	t.message = fmt.Sprintf(format, args...)
}
