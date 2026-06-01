package gest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type appService struct {
	message string
}

type appController struct {
	service *appService
}

func newAppService() *appService {
	return &appService{message: "hello"}
}

func newAppController(service *appService) *appController {
	return &appController{service: service}
}

func (c *appController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "AppController",
		BasePath: "/api",
		Routes: []RouteDefinition{
			{
				Name:   "Hello",
				Method: http.MethodGet,
				Path:   "/hello",
				Handler: func(ctx *Context) error {
					return ctx.JSON(http.StatusOK, map[string]string{"message": c.service.message})
				},
			},
		},
	}
}

type missingMetadataController struct{}

func newMissingMetadataController() *missingMetadataController {
	return &missingMetadataController{}
}

type missingDependencyController struct {
	service *appService
}

func newMissingDependencyController(service *appService) *missingDependencyController {
	return &missingDependencyController{service: service}
}

func (c *missingDependencyController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name: "MissingDependencyController",
		Routes: []RouteDefinition{
			{
				Method: http.MethodGet,
				Path:   "/missing",
				Handler: func(ctx *Context) error {
					return ctx.NoContent(http.StatusNoContent)
				},
			},
		},
	}
}

type duplicateControllerA struct{}

func newDuplicateControllerA() *duplicateControllerA {
	return &duplicateControllerA{}
}

func (c *duplicateControllerA) GestController() ControllerDefinition {
	return duplicateControllerDefinition("A")
}

type duplicateControllerB struct{}

func newDuplicateControllerB() *duplicateControllerB {
	return &duplicateControllerB{}
}

func (c *duplicateControllerB) GestController() ControllerDefinition {
	return duplicateControllerDefinition("B")
}

func TestAppServesRouteFromHandWrittenMetadata(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(newAppService),
			Controller(newAppController),
		),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/hello", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"message\":\"hello\"}\n" {
		t.Fatalf("body = %q, want hello JSON", got)
	}
}

func TestAppDefaultRouterWorks(t *testing.T) {
	app := New()

	if app.router.Name() != "chi" {
		t.Fatalf("default router name = %q, want chi", app.router.Name())
	}
}

func TestAppConstructorInjectionWorksForControllerDependency(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(newAppService),
			Controller(newAppController),
		),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	fake, ok := app.router.(*fakeRouter)
	if !ok {
		t.Fatalf("router = %T, want *fakeRouter", app.router)
	}
	if len(fake.routes) != 1 {
		t.Fatalf("registered routes = %d, want 1", len(fake.routes))
	}
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/api/hello", nil))
	if err := fake.routes[0].Handler(context); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if got := recorder.Body.String(); got != "{\"message\":\"hello\"}\n" {
		t.Fatalf("body = %q, want injected service response", got)
	}
}

func TestAppDuplicateRouteReturnsUsefulError(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Controller(newDuplicateControllerA),
			Controller(newDuplicateControllerB),
		),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want duplicate route error")
	}
	var appErr *appError
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *appError", err)
	}
	if appErr.Code != "ROUTE_DUPLICATE" {
		t.Fatalf("Code = %q, want ROUTE_DUPLICATE", appErr.Code)
	}
	if !strings.Contains(err.Error(), "GET /duplicate") {
		t.Fatalf("error = %q, want duplicate route path", err.Error())
	}
}

func TestAppControllerMissingMetadataReturnsUsefulError(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newMissingMetadataController)),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want metadata error")
	}
	var appErr *appError
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *appError", err)
	}
	if appErr.Code != "ROUTE_MISSING_CONTROLLER_METADATA" {
		t.Fatalf("Code = %q, want ROUTE_MISSING_CONTROLLER_METADATA", appErr.Code)
	}
	if !strings.Contains(err.Error(), "GestController") {
		t.Fatalf("error = %q, want GestController hint", err.Error())
	}
}

func TestAppMissingProviderReturnsUsefulError(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newMissingDependencyController)),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want missing provider error")
	}
	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_MISSING_PROVIDER" {
		t.Fatalf("Code = %q, want DI_MISSING_PROVIDER", diErr.Code)
	}
	if !strings.Contains(err.Error(), TokenOf[*appService]().String()) {
		t.Fatalf("error = %q, want missing token", err.Error())
	}
}

func TestAppCustomAdapterCanBeInjected(t *testing.T) {
	fake := newFakeRouter()
	app := New(WithRouter(fake), WithBootLogs(true))
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newDuplicateControllerA)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	if app.router != fake {
		t.Fatal("app did not keep injected router")
	}
	if !app.bootLogs {
		t.Fatal("bootLogs = false, want true")
	}
	if len(fake.routes) != 1 {
		t.Fatalf("registered routes = %d, want 1", len(fake.routes))
	}
}

func TestAppMapsHandlerFrameworkErrors(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newErrorController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var body struct {
		Error HTTPError `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body returned error: %v", err)
	}
	if body.Error.Kind != ErrorKindNotFound {
		t.Fatalf("error kind = %q, want %q", body.Error.Kind, ErrorKindNotFound)
	}
	if body.Error.Message != "missing resource" {
		t.Fatalf("error message = %q, want missing resource", body.Error.Message)
	}
}

func duplicateControllerDefinition(name string) ControllerDefinition {
	return ControllerDefinition{
		Name: name,
		Routes: []RouteDefinition{
			{
				Method: http.MethodGet,
				Path:   "/duplicate",
				Handler: func(ctx *Context) error {
					return ctx.NoContent(http.StatusNoContent)
				},
			},
		},
	}
}

type errorController struct{}

func newErrorController() *errorController {
	return &errorController{}
}

func (c *errorController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name: "ErrorController",
		Routes: []RouteDefinition{
			{
				Method: http.MethodGet,
				Path:   "/missing",
				Handler: func(ctx *Context) error {
					return NotFound("missing resource")
				},
			},
		},
	}
}

type fakeRouter struct {
	routes []RouteRuntimeConfig
}

func newFakeRouter() *fakeRouter {
	return &fakeRouter{}
}

func (r *fakeRouter) Name() string {
	return "fake"
}

func (r *fakeRouter) Group(prefix string, fn func(group RouterAdapter)) {
	group := newFakeRouter()
	fn(group)
	for _, route := range group.routes {
		route.Path = joinRoutePath(prefix, route.Path)
		r.routes = append(r.routes, route)
	}
}

func (r *fakeRouter) Handle(route RouteRuntimeConfig) {
	r.routes = append(r.routes, route)
}

func (r *fakeRouter) Use(Middleware) {}

func (r *fakeRouter) Serve(string) error {
	return nil
}
