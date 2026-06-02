package swagger_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/swagger"
)

func TestSwaggerModuleServesHTML(t *testing.T) {
	router := newTestRouter()
	app := gest.New(gest.WithRouter(router))
	app.OpenAPI("")
	app.Import(swagger.Module(swagger.Options{}))
	bootstrap(t, app)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("body does not look like Swagger UI HTML:\n%s", body)
	}
	if !strings.Contains(body, `url: "/openapi.json"`) {
		t.Fatalf("body does not contain default OpenAPI path:\n%s", body)
	}
}

func TestSwaggerModuleUsesConfiguredPaths(t *testing.T) {
	router := newTestRouter()
	app := gest.New(gest.WithRouter(router))
	app.OpenAPI("/api/openapi.json")
	app.Import(swagger.Module(swagger.Options{
		Path:        "/reference",
		OpenAPIPath: "/api/openapi.json",
	}))
	bootstrap(t, app)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reference", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `url: "/api/openapi.json"`) {
		t.Fatalf("body does not contain configured OpenAPI path:\n%s", body)
	}
}

func TestSwaggerModuleRedirectsTrailingSlash(t *testing.T) {
	router := newTestRouter()
	app := gest.New(gest.WithRouter(router))
	app.Import(swagger.Module(swagger.Options{Path: "/reference"}))
	bootstrap(t, app)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reference/", nil))

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if location := recorder.Header().Get("Location"); location != "/reference" {
		t.Fatalf("Location = %q, want /reference", location)
	}
}

func TestSwaggerModuleIsOptional(t *testing.T) {
	router := newTestRouter()
	app := gest.New(gest.WithRouter(router))
	app.OpenAPI("")
	bootstrap(t, app)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestCoreRuntimeDoesNotImportSwaggerModule(t *testing.T) {
	command := exec.Command("go", "list", "-deps", "github.com/r6m/gest")
	command.Dir = projectRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list runtime deps: %v", err)
	}

	for _, dep := range strings.Fields(string(output)) {
		if dep == "github.com/r6m/gest/modules/swagger" {
			t.Fatalf("core runtime imports Swagger module")
		}
	}
}

var errServeSkipped = errors.New("serve skipped")

type testRouter struct {
	routes map[string]gest.RouteRuntimeConfig
}

func newTestRouter() *testRouter {
	return &testRouter{routes: make(map[string]gest.RouteRuntimeConfig)}
}

func (r *testRouter) Name() string {
	return "test"
}

func (r *testRouter) Group(prefix string, fn func(group gest.RouterAdapter)) {
	group := newTestRouter()
	fn(group)
	for key, route := range group.routes {
		delete(group.routes, key)
		route.Path = prefix + route.Path
		r.routes[route.Method+" "+route.Path] = route
	}
}

func (r *testRouter) Handle(route gest.RouteRuntimeConfig) {
	r.routes[route.Method+" "+route.Path] = route
}

func (r *testRouter) Use(gest.Middleware) {}

func (r *testRouter) Serve(string) error {
	return errServeSkipped
}

func (r *testRouter) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	route, ok := r.routes[request.Method+" "+request.URL.Path]
	if !ok {
		http.NotFound(response, request)
		return
	}
	if err := route.Handler(gest.NewContext(response, request)); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func bootstrap(t *testing.T, app *gest.App) {
	t.Helper()

	if err := app.Listen(":0"); !errors.Is(err, errServeSkipped) {
		t.Fatalf("Listen returned %v, want serve skipped", err)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
