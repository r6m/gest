package gest

import (
	"bytes"
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

type typedRouteController struct {
	calls int
}

type typedRouteRequest struct {
	ID       string `param:"id"`
	Page     int    `query:"page" default:"1"`
	Trace    string `header:"X-Trace-ID"`
	Name     string `json:"name"`
	Featured bool   `query:"featured" default:"false"`
}

type typedRouteResponse struct {
	ID       string `json:"id"`
	Page     int    `json:"page"`
	Trace    string `json:"trace"`
	Name     string `json:"name"`
	Featured bool   `json:"featured"`
}

func newTypedRouteController() *typedRouteController {
	return &typedRouteController{}
}

func (c *typedRouteController) Show(ctx *Context, req *typedRouteRequest) (*typedRouteResponse, error) {
	c.calls++
	return &typedRouteResponse{
		ID:       req.ID,
		Page:     req.Page,
		Trace:    req.Trace,
		Name:     req.Name,
		Featured: req.Featured,
	}, nil
}

func (c *typedRouteController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "TypedRouteController",
		Tag:      "typed",
		BasePath: "/typed",
		Routes: []RouteDefinition{
			{
				Name:     "Show",
				Method:   http.MethodPost,
				Path:     "/{id}",
				Handler:  Handle(c.Show, Status(http.StatusCreated)),
				Request:  (*typedRouteRequest)(nil),
				Response: (*typedRouteResponse)(nil),
				Statuses: []int{http.StatusCreated},
				Metadata: RouteMetadata{
					Summary:     "Show typed route",
					Description: "Returns a typed response from bound request data.",
				},
			},
		},
	}
}

type hiddenOpenAPIController struct{}

func newHiddenOpenAPIController() *hiddenOpenAPIController {
	return &hiddenOpenAPIController{}
}

func (c *hiddenOpenAPIController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "HiddenOpenAPIController",
		BasePath: "/hidden-controller",
		Hidden:   true,
		Routes: []RouteDefinition{
			{
				Name:   "Index",
				Method: http.MethodGet,
				Path:   "/",
				Handler: HandleContext(func(ctx *Context) error {
					return ctx.NoContent(http.StatusNoContent)
				}),
			},
		},
	}
}

type routeHiddenOpenAPIController struct{}

func newRouteHiddenOpenAPIController() *routeHiddenOpenAPIController {
	return &routeHiddenOpenAPIController{}
}

func (c *routeHiddenOpenAPIController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "RouteHiddenOpenAPIController",
		BasePath: "/route-hidden",
		Routes: []RouteDefinition{
			{
				Name:   "Visible",
				Method: http.MethodGet,
				Path:   "/visible",
				Handler: HandleContext(func(ctx *Context) error {
					return ctx.NoContent(http.StatusNoContent)
				}),
			},
			{
				Name:   "Hidden",
				Method: http.MethodGet,
				Path:   "/hidden",
				Handler: HandleContext(func(ctx *Context) error {
					return ctx.NoContent(http.StatusNoContent)
				}),
				Metadata: RouteMetadata{Hidden: true},
			},
		},
	}
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

func TestAppServesTypedDTORouteThroughChi(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}

	request := httptest.NewRequest(http.MethodPost, "/typed/user-1?page=3&featured=true", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("X-Trace-ID", "trace-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var body typedRouteResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body returned error: %v", err)
	}
	if body.ID != "user-1" || body.Page != 3 || body.Trace != "trace-1" || body.Name != "Ada" || !body.Featured {
		t.Fatalf("body = %#v, want bound typed DTO response", body)
	}
}

func TestAppTypedDTORouteBadInputReturnsStable400JSON(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}

	request := httptest.NewRequest(http.MethodPost, "/typed/user-1?page=many", strings.NewReader(`{"name":"Ada"}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var body struct {
		Error HTTPError `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body returned error: %v", err)
	}
	if body.Error.Kind != ErrorKindBadRequest {
		t.Fatalf("error kind = %q, want %q", body.Error.Kind, ErrorKindBadRequest)
	}
	if body.Error.Code != "BINDING_CONVERSION_FAILURE" {
		t.Fatalf("error code = %q, want BINDING_CONVERSION_FAILURE", body.Error.Code)
	}
	if body.Error.Field != "query.page" {
		t.Fatalf("error field = %q, want query.page", body.Error.Field)
	}
	if !strings.Contains(body.Error.Message, "many") {
		t.Fatalf("error message = %q, want safe received value", body.Error.Message)
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

func TestAppOpenAPIRoutesCapturesHandWrittenMetadata(t *testing.T) {
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

	routes := app.OpenAPIRoutes()
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	route := routes[0]
	if route.ControllerName != "AppController" {
		t.Fatalf("ControllerName = %q, want AppController", route.ControllerName)
	}
	if route.BasePath != "/api" {
		t.Fatalf("BasePath = %q, want /api", route.BasePath)
	}
	if route.RouteName != "Hello" {
		t.Fatalf("RouteName = %q, want Hello", route.RouteName)
	}
	if route.Method != http.MethodGet {
		t.Fatalf("Method = %q, want GET", route.Method)
	}
	if route.Path != "/api/hello" {
		t.Fatalf("Path = %q, want /api/hello", route.Path)
	}
}

func TestAppOpenAPIRoutesCapturesGeneratedStyleMetadata(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	routes := app.OpenAPIRoutes()
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	route := routes[0]
	if route.ControllerName != "TypedRouteController" {
		t.Fatalf("ControllerName = %q, want TypedRouteController", route.ControllerName)
	}
	if route.Tag != "typed" {
		t.Fatalf("Tag = %q, want typed", route.Tag)
	}
	if route.RouteName != "Show" {
		t.Fatalf("RouteName = %q, want Show", route.RouteName)
	}
	if route.Path != "/typed/{id}" {
		t.Fatalf("Path = %q, want /typed/{id}", route.Path)
	}
	if len(route.Statuses) != 1 || route.Statuses[0] != http.StatusCreated {
		t.Fatalf("Statuses = %#v, want [201]", route.Statuses)
	}
	if route.Summary != "Show typed route" {
		t.Fatalf("Summary = %q, want generated-style summary", route.Summary)
	}
	if route.Description != "Returns a typed response from bound request data." {
		t.Fatalf("Description = %q, want generated-style description", route.Description)
	}
	if route.Request != (*typedRouteRequest)(nil) {
		t.Fatalf("Request = %#v, want typed request metadata", route.Request)
	}
	if route.Response != (*typedRouteResponse)(nil) {
		t.Fatalf("Response = %#v, want typed response metadata", route.Response)
	}
}

func TestAppOpenAPIRoutesOrderIsDeterministic(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(
		NewModule(ModuleConfig{
			Name:      "FirstModule",
			Providers: Providers(Controller(newOrderedControllerA)),
		}),
		NewModule(ModuleConfig{
			Name:      "SecondModule",
			Providers: Providers(Controller(newOrderedControllerB)),
		}),
	)

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	routes := app.OpenAPIRoutes()
	got := make([]string, 0, len(routes))
	for _, route := range routes {
		got = append(got, route.ControllerName+"."+route.RouteName)
	}
	want := []string{"OrderedControllerA.First", "OrderedControllerA.Second", "OrderedControllerB.Third"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("route order = %#v, want %#v", got, want)
	}
}

func TestAppOpenAPIRoutesDoesNotExposeMutableInternals(t *testing.T) {
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	routes := app.OpenAPIRoutes()
	routes[0].ControllerName = "Changed"
	routes[0].Statuses[0] = http.StatusAccepted

	fresh := app.OpenAPIRoutes()
	if fresh[0].ControllerName != "TypedRouteController" {
		t.Fatalf("ControllerName mutated to %q", fresh[0].ControllerName)
	}
	if fresh[0].Statuses[0] != http.StatusCreated {
		t.Fatalf("Statuses mutated to %#v", fresh[0].Statuses)
	}
}

func TestAppOpenAPIServesDocumentAtConfiguredPath(t *testing.T) {
	app := New()
	app.OpenAPI("/docs/openapi.json", OpenAPITitle("Example API"), OpenAPIVersion("2.3.4"))
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	document := decodeOpenAPIDocument(t, recorder.Body.Bytes())
	if document["openapi"] != "3.0.3" {
		t.Fatalf("openapi = %q, want 3.0.3", document["openapi"])
	}
	info := documentObject(t, document, "info")
	if info["title"] != "Example API" {
		t.Fatalf("info.title = %q, want Example API", info["title"])
	}
	if info["version"] != "2.3.4" {
		t.Fatalf("info.version = %q, want 2.3.4", info["version"])
	}
}

func TestAppOpenAPIDocumentIncludesOperationsResponsesAndSchemas(t *testing.T) {
	app := New()
	app.OpenAPI("")
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	body := serveOpenAPI(t, app, "/openapi.json")
	document := decodeOpenAPIDocument(t, body)
	paths := documentObject(t, document, "paths")
	typedPath := objectFromAny(t, paths["/typed/{id}"])
	post := objectFromAny(t, typedPath["post"])

	if post["operationId"] != "TypedRouteController.Show" {
		t.Fatalf("operationId = %q, want TypedRouteController.Show", post["operationId"])
	}
	tags := arrayFromAny(t, post["tags"])
	if len(tags) != 1 || tags[0] != "typed" {
		t.Fatalf("tags = %#v, want [typed]", tags)
	}
	if post["summary"] != "Show typed route" {
		t.Fatalf("summary = %q, want route summary", post["summary"])
	}
	if post["description"] != "Returns a typed response from bound request data." {
		t.Fatalf("description = %q, want route description", post["description"])
	}

	responses := objectFromAny(t, post["responses"])
	created := objectFromAny(t, responses["201"])
	if created["description"] != "Created" {
		t.Fatalf("201 description = %q, want Created", created["description"])
	}
	content := objectFromAny(t, created["content"])
	jsonMedia := objectFromAny(t, content["application/json"])
	responseSchema := objectFromAny(t, jsonMedia["schema"])
	if responseSchema["$ref"] != "#/components/schemas/typedRouteResponse" {
		t.Fatalf("response schema = %#v, want typedRouteResponse ref", responseSchema)
	}

	components := documentObject(t, document, "components")
	schemas := objectFromAny(t, components["schemas"])
	if schemas["typedRouteRequest"] == nil {
		t.Fatal("missing typedRouteRequest component")
	}
	if schemas["typedRouteResponse"] == nil {
		t.Fatal("missing typedRouteResponse component")
	}
}

func TestAppOpenAPIDocumentIncludesRequestParametersAndBody(t *testing.T) {
	app := New()
	app.OpenAPI("")
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	document := decodeOpenAPIDocument(t, serveOpenAPI(t, app, "/openapi.json"))
	post := objectFromAny(t, objectFromAny(t, documentObject(t, document, "paths")["/typed/{id}"])["post"])
	parameters := arrayFromAny(t, post["parameters"])
	seen := map[string]string{}
	for _, parameter := range parameters {
		item := objectFromAny(t, parameter)
		name, ok := item["name"].(string)
		if !ok {
			t.Fatalf("parameter name = %#v, want string", item["name"])
		}
		in, ok := item["in"].(string)
		if !ok {
			t.Fatalf("parameter in = %#v, want string", item["in"])
		}
		seen[name] = in
	}
	wantParameters := map[string]string{
		"id":         "path",
		"page":       "query",
		"featured":   "query",
		"X-Trace-ID": "header",
	}
	for name, in := range wantParameters {
		if seen[name] != in {
			t.Fatalf("parameter %q in = %q, want %q; all parameters %#v", name, seen[name], in, parameters)
		}
	}

	requestBody := objectFromAny(t, post["requestBody"])
	content := objectFromAny(t, requestBody["content"])
	jsonMedia := objectFromAny(t, content["application/json"])
	bodySchema := objectFromAny(t, jsonMedia["schema"])
	properties := objectFromAny(t, bodySchema["properties"])
	if properties["name"] == nil {
		t.Fatalf("request body properties = %#v, want name", properties)
	}
	if properties["id"] != nil || properties["page"] != nil || properties["trace"] != nil {
		t.Fatalf("request body included non-json fields: %#v", properties)
	}
}

func TestAppOpenAPIDocumentExcludesHiddenControllerRoutes(t *testing.T) {
	app := New()
	app.OpenAPI("")
	app.Import(NewModule(ModuleConfig{
		Name:      "HiddenModule",
		Providers: Providers(Controller(newHiddenOpenAPIController)),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	document := decodeOpenAPIDocument(t, serveOpenAPI(t, app, "/openapi.json"))
	paths := documentObject(t, document, "paths")
	if paths["/hidden-controller/"] != nil {
		t.Fatalf("paths included hidden controller route: %#v", paths)
	}

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hidden-controller/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("hidden controller route status = %d, want 204", recorder.Code)
	}
}

func TestAppOpenAPIDocumentExcludesHiddenRouteOnly(t *testing.T) {
	app := New()
	app.OpenAPI("")
	app.Import(NewModule(ModuleConfig{
		Name:      "RouteHiddenModule",
		Providers: Providers(Controller(newRouteHiddenOpenAPIController)),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	document := decodeOpenAPIDocument(t, serveOpenAPI(t, app, "/openapi.json"))
	paths := documentObject(t, document, "paths")
	if paths["/route-hidden/hidden"] != nil {
		t.Fatalf("paths included hidden route: %#v", paths)
	}
	if paths["/route-hidden/visible"] == nil {
		t.Fatalf("paths missing visible route: %#v", paths)
	}
}

func TestAppOpenAPIOutputIsDeterministic(t *testing.T) {
	app := New()
	app.OpenAPI("")
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	first := serveOpenAPI(t, app, "/openapi.json")
	second := serveOpenAPI(t, app, "/openapi.json")
	if string(first) != string(second) {
		t.Fatalf("OpenAPI output is not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestAppWithoutOpenAPIBehavesUnchanged(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
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
	app := New(WithRouter(fake), WithBootLogs(true), WithBootLogWriter(discardWriter{}))
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

func TestAppBootLogsDisabledByDefault(t *testing.T) {
	var output bytes.Buffer
	app := New(WithRouter(newFakeRouter()), WithBootLogWriter(&output))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(newAppService),
			Controller(newAppController),
		),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	if output.String() != "" {
		t.Fatalf("boot logs = %q, want empty output", output.String())
	}
}

func TestAppBootLogsIncludeModulesProvidersControllersAndRoutes(t *testing.T) {
	var output bytes.Buffer
	app := New(WithRouter(newFakeRouter()), WithBootLogs(true), WithBootLogWriter(&output))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(newAppService),
			Controller(newAppController),
		),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	logs := output.String()
	assertContains(t, logs, "GEST starting application\n")
	assertContains(t, logs, "GEST module: App eager providers=0 controllers=0\n")
	assertContains(t, logs, "GEST module: AppModule eager providers=1 controllers=1\n")
	assertContains(t, logs, "GEST route: GET /api/hello -> AppController.Hello\n")
	assertContains(t, logs, "GEST boot duration: ")
}

func TestAppBootLogsIncludeOpenAPIWhenConfigured(t *testing.T) {
	var output bytes.Buffer
	app := New(WithRouter(newFakeRouter()), WithBootLogs(true), WithBootLogWriter(&output))
	app.OpenAPI("/docs/openapi.json")
	app.Import(NewModule(ModuleConfig{
		Name:      "TypedModule",
		Providers: Providers(Controller(newTypedRouteController)),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	assertContains(t, output.String(), "GEST OpenAPI route: GET /docs/openapi.json\n")
}

func TestAppBootLogsCustomWriterReceivesListenAddress(t *testing.T) {
	var output bytes.Buffer
	app := New(WithRouter(newFakeRouter()), WithBootLogs(true), WithLogger(&output))
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newDuplicateControllerA)),
	}))

	if err := app.Listen(":0"); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	assertContains(t, output.String(), "GEST listen address: :0\n")
}

func TestAppInstallsValidatorOnRegisteredRoutes(t *testing.T) {
	fake := newFakeRouter()
	validator := &recordingValidator{}
	app := New(WithRouter(fake), WithValidator(validator))
	app.Import(NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Controller(newDuplicateControllerA)),
	}))

	err := app.bootstrap()
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	if len(fake.routes) != 1 {
		t.Fatalf("registered routes = %d, want 1", len(fake.routes))
	}
	if fake.routes[0].Validator != validator {
		t.Fatal("route validator was not installed")
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

func TestAppRunsGuardsBeforeHandler(t *testing.T) {
	order := []string{}
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					order = append(order, "handler")
					return ctx.NoContent(http.StatusNoContent)
				},
				guards: []GuardFactory{staticGuardFactory(GuardFunc(func(ctx *Context) error {
					order = append(order, "guard")
					return nil
				}))},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertStringSlice(t, order, []string{"guard", "handler"})
}

func TestAppMiddlewareRunsForEveryRoute(t *testing.T) {
	calls := 0
	app := New()
	app.Use(MiddlewareFunc(func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			calls++
			return next(ctx)
		}
	}))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *multiRouteController {
			return &multiRouteController{}
		})),
	}))

	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/one", nil))
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/two", nil))

	if calls != 2 {
		t.Fatalf("middleware calls = %d, want 2", calls)
	}
}

func TestAppMiddlewareOrderAndGuardOrder(t *testing.T) {
	order := []string{}
	app := New()
	app.Use(recordMiddleware("app", &order))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					order = append(order, "handler")
					return ctx.NoContent(http.StatusNoContent)
				},
				middlewares: []MiddlewareFactory{staticMiddlewareFactory(recordMiddleware("route", &order))},
				guards:      []GuardFactory{staticGuardFactory(recordGuard("guard", &order))},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertStringSlice(t, order, []string{"app-before", "route-before", "guard", "handler", "route-after", "app-after"})
}

func TestMiddlewareCanObserveFinalResponseStatus(t *testing.T) {
	observed := -1
	app := New()
	app.Use(MiddlewareFunc(func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if before := ctx.ResponseStatus(); before != 0 {
				t.Fatalf("ResponseStatus before write = %d, want 0", before)
			}
			err := next(ctx)
			observed = ctx.ResponseStatus()
			return err
		}
	}))
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					return ctx.JSON(http.StatusCreated, map[string]string{"ok": "true"})
				},
			}
		})),
	}))

	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if observed != http.StatusCreated {
		t.Fatalf("observed status = %d, want %d", observed, http.StatusCreated)
	}
}

func TestResponseStatusTracksNoContentAndErrors(t *testing.T) {
	noContentRecorder := httptest.NewRecorder()
	noContent := NewContext(noContentRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if before := noContent.ResponseStatus(); before != 0 {
		t.Fatalf("ResponseStatus before write = %d, want 0", before)
	}
	if err := noContent.NoContent(http.StatusAccepted); err != nil {
		t.Fatalf("NoContent returned error: %v", err)
	}
	if got := noContent.ResponseStatus(); got != http.StatusAccepted {
		t.Fatalf("NoContent ResponseStatus = %d, want %d", got, http.StatusAccepted)
	}

	errorRecorder := httptest.NewRecorder()
	errorContext := NewContext(errorRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := WriteError(errorContext.RawResponse(), Forbidden("blocked")); err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}
	if got := errorContext.ResponseStatus(); got != http.StatusForbidden {
		t.Fatalf("error ResponseStatus = %d, want %d", got, http.StatusForbidden)
	}
}

func TestAppFailingGuardPreventsHandler(t *testing.T) {
	called := false
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					called = true
					return ctx.NoContent(http.StatusNoContent)
				},
				guards: []GuardFactory{staticGuardFactory(GuardFunc(func(ctx *Context) error {
					return Forbidden("blocked")
				}))},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("handler was called after failing guard")
	}
}

func TestAppDIResolvedMiddlewareReceivesDependencies(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *guardDependency {
				return &guardDependency{value: "from middleware"}
			}),
			Provide(func(dependency *guardDependency) *dependencyMiddleware {
				return &dependencyMiddleware{dependency: dependency}
			}),
			Controller(func() *guardController {
				return &guardController{
					handler: func(ctx *Context) error {
						value, _ := ctx.Get("dependency")
						return ctx.JSON(http.StatusOK, map[string]any{"dependency": value})
					},
					middlewares: []MiddlewareFactory{ResolveMiddleware[*dependencyMiddleware]()},
				}
			}),
		),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"dependency\":\"from middleware\"}\n" {
		t.Fatalf("body = %q, want dependency JSON", got)
	}
}

func TestAppRouteComponentImplementingNeitherMiddlewareNorGuardErrors(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *plainRouteComponent {
				return &plainRouteComponent{}
			}),
			Controller(func() *guardController {
				return &guardController{
					handler:    emptyHandler,
					components: []RouteComponentFactory{ResolveRouteComponent[*plainRouteComponent]()},
				}
			}),
		),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want invalid route component error")
	}
	if !strings.Contains(err.Error(), "ROUTE_COMPONENT_INVALID") {
		t.Fatalf("error = %q, want route component invalid code", err)
	}
	if !strings.Contains(err.Error(), "implements neither gest.Middleware nor gest.Guard") {
		t.Fatalf("error = %q, want clear interface classification message", err)
	}
}

func TestResolveGuardReceivesDependencies(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *guardDependency {
				return &guardDependency{value: "allowed"}
			}),
			Provide(func(dependency *guardDependency) *dependencyGuard {
				return &dependencyGuard{dependency: dependency}
			}),
			Controller(func() *guardController {
				return &guardController{
					handler: func(ctx *Context) error {
						value, _ := ctx.Get("dependency")
						return ctx.JSON(http.StatusOK, map[string]any{"dependency": value})
					},
					guards: []GuardFactory{ResolveGuard[*dependencyGuard]()},
				}
			}),
		),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"dependency\":\"allowed\"}\n" {
		t.Fatalf("body = %q, want dependency JSON", got)
	}
}

func TestAppRunsGuardsInDeclaredOrder(t *testing.T) {
	order := []string{}
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					order = append(order, "handler")
					return ctx.NoContent(http.StatusNoContent)
				},
				guards: []GuardFactory{
					staticGuardFactory(recordGuard("first", &order)),
					staticGuardFactory(recordGuard("second", &order)),
				},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertStringSlice(t, order, []string{"first", "second", "handler"})
}

func TestAppGuardCanSetContextValueForHandler(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					value, _ := ctx.Get("guard-value")
					return ctx.JSON(http.StatusOK, map[string]any{"value": value})
				},
				guards: []GuardFactory{staticGuardFactory(GuardFunc(func(ctx *Context) error {
					ctx.Set("guard-value", "from guard")
					return nil
				}))},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"value\":\"from guard\"}\n" {
		t.Fatalf("body = %q, want guard value JSON", got)
	}
}

func TestAppDIResolvedGuardReceivesDependencies(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *guardDependency {
				return &guardDependency{value: "allowed"}
			}),
			Controller(func() *guardController {
				return &guardController{
					handler: func(ctx *Context) error {
						value, _ := ctx.Get("dependency")
						return ctx.JSON(http.StatusOK, map[string]any{"dependency": value})
					},
					guards: []GuardFactory{dependencyGuardFactory},
				}
			}),
		),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"dependency\":\"allowed\"}\n" {
		t.Fatalf("body = %q, want dependency JSON", got)
	}
}

func TestAppMissingGuardDependencyReturnsStartupError(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: emptyHandler,
				guards:  []GuardFactory{dependencyGuardFactory},
			}
		})),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want guard dependency error")
	}
	if !strings.Contains(err.Error(), "ROUTE_GUARD_RESOLVE") {
		t.Fatalf("error = %q, want guard resolve code", err)
	}
	if !strings.Contains(err.Error(), "DI_MISSING_PROVIDER") {
		t.Fatalf("error = %q, want missing provider diagnostic", err)
	}
}

func TestAppNoGuardLeavesRouteBehaviorUnchanged(t *testing.T) {
	app := New()
	app.Import(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(Controller(func() *guardController {
			return &guardController{
				handler: func(ctx *Context) error {
					return ctx.JSON(http.StatusOK, map[string]string{"message": "ok"})
				},
			}
		})),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/guarded", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"message\":\"ok\"}\n" {
		t.Fatalf("body = %q, want unchanged route JSON", got)
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

type guardController struct {
	handler     HandlerFunc
	components  []RouteComponentFactory
	middlewares []MiddlewareFactory
	guards      []GuardFactory
}

func (c *guardController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name: "GuardController",
		Routes: []RouteDefinition{
			{
				Name:        "Guarded",
				Method:      http.MethodGet,
				Path:        "/guarded",
				Handler:     c.handler,
				Components:  c.components,
				Middlewares: c.middlewares,
				Guards:      c.guards,
			},
		},
	}
}

type multiRouteController struct{}

func (c *multiRouteController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name: "MultiRouteController",
		Routes: []RouteDefinition{
			{
				Name:    "One",
				Method:  http.MethodGet,
				Path:    "/one",
				Handler: emptyHandler,
			},
			{
				Name:    "Two",
				Method:  http.MethodGet,
				Path:    "/two",
				Handler: emptyHandler,
			},
		},
	}
}

type guardDependency struct {
	value string
}

type dependencyGuard struct {
	dependency *guardDependency
}

func (g *dependencyGuard) CanActivate(ctx *Context) error {
	ctx.Set("dependency", g.dependency.value)
	return nil
}

type dependencyMiddleware struct {
	dependency *guardDependency
}

func (m *dependencyMiddleware) Handle(next HandlerFunc) HandlerFunc {
	return func(ctx *Context) error {
		ctx.Set("dependency", m.dependency.value)
		return next(ctx)
	}
}

type plainRouteComponent struct{}

func dependencyGuardFactory(container Container) (Guard, error) {
	value, err := container.Resolve(TokenOf[*guardDependency]())
	if err != nil {
		return nil, err
	}
	dependency, ok := value.(*guardDependency)
	if !ok {
		return nil, errors.New("dependency guard received unexpected dependency type")
	}
	return &dependencyGuard{dependency: dependency}, nil
}

func staticGuardFactory(guard Guard) GuardFactory {
	return func(Container) (Guard, error) {
		return guard, nil
	}
}

func staticMiddlewareFactory(middleware Middleware) MiddlewareFactory {
	return func(Container) (Middleware, error) {
		return middleware, nil
	}
}

func recordGuard(value string, order *[]string) Guard {
	return GuardFunc(func(ctx *Context) error {
		*order = append(*order, value)
		return nil
	})
}

func recordMiddleware(value string, order *[]string) Middleware {
	return MiddlewareFunc(func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			*order = append(*order, value+"-before")
			err := next(ctx)
			*order = append(*order, value+"-after")
			return err
		}
	})
}

func assertStringSlice(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q; full slice %#v", i, got[i], want[i], got)
		}
	}
}

type orderedControllerA struct{}

func newOrderedControllerA() *orderedControllerA {
	return &orderedControllerA{}
}

func (c *orderedControllerA) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "OrderedControllerA",
		BasePath: "/ordered",
		Routes: []RouteDefinition{
			{
				Name:    "First",
				Method:  http.MethodGet,
				Path:    "/first",
				Handler: emptyHandler,
			},
			{
				Name:    "Second",
				Method:  http.MethodGet,
				Path:    "/second",
				Handler: emptyHandler,
			},
		},
	}
}

type orderedControllerB struct{}

func newOrderedControllerB() *orderedControllerB {
	return &orderedControllerB{}
}

func (c *orderedControllerB) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "OrderedControllerB",
		BasePath: "/ordered",
		Routes: []RouteDefinition{
			{
				Name:    "Third",
				Method:  http.MethodGet,
				Path:    "/third",
				Handler: emptyHandler,
			},
		},
	}
}

func emptyHandler(ctx *Context) error {
	return ctx.NoContent(http.StatusNoContent)
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output = %q, want substring %q", got, want)
	}
}

func serveOpenAPI(t *testing.T, app *App, path string) []byte {
	t.Helper()

	router, ok := app.router.(*defaultRouter)
	if !ok {
		t.Fatalf("router = %T, want *defaultRouter", app.router)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func decodeOpenAPIDocument(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("decode OpenAPI document: %v\n%s", err, body)
	}
	return document
}

func documentObject(t *testing.T, document map[string]any, key string) map[string]any {
	t.Helper()
	return objectFromAny(t, document[key])
}

func objectFromAny(t *testing.T, value any) map[string]any {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v (%T), want object", value, value)
	}
	return object
}

func arrayFromAny(t *testing.T, value any) []any {
	t.Helper()

	array, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v (%T), want array", value, value)
	}
	return array
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

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
