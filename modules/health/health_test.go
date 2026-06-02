package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/health"
)

func TestHealthReturnsOKJSON(t *testing.T) {
	app := newApp(health.Options{})

	assertHealthResponse(t, app, "/health")
}

func TestHealthLiveReturnsOKJSON(t *testing.T) {
	app := newApp(health.Options{})

	assertHealthResponse(t, app, "/health/live")
}

func TestHealthReadyReturnsOKJSON(t *testing.T) {
	app := newApp(health.Options{})

	assertHealthResponse(t, app, "/health/ready")
}

func TestHealthCustomPathWorks(t *testing.T) {
	app := newApp(health.Options{Path: "/status"})

	assertHealthResponse(t, app, "/status")
	assertHealthResponse(t, app, "/status/live")
	assertHealthResponse(t, app, "/status/ready")
}

func TestHealthModuleIsOptional(t *testing.T) {
	app := gest.New()

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestOpenAPIIncludesHealthRoutes(t *testing.T) {
	app := newApp(health.Options{})
	app.OpenAPI("")

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var document map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatalf("Unmarshal OpenAPI returned error: %v", err)
	}
	paths := objectFromAny(t, document["paths"])
	for _, path := range []string{"/health", "/health/live", "/health/ready"} {
		item := objectFromAny(t, paths[path])
		get := objectFromAny(t, item["get"])
		if get["operationId"] == "" {
			t.Fatalf("operationId for %s is empty", path)
		}
		tags := arrayFromAny(t, get["tags"])
		if len(tags) != 1 || tags[0] != "health" {
			t.Fatalf("tags for %s = %#v, want [health]", path, tags)
		}
	}
}

func TestCoreRuntimeDoesNotImportHealthModule(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	matches, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	for _, file := range matches {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s returned error: %v", file, err)
		}
		if strings.Contains(string(content), "github.com/r6m/gest/modules/health") {
			t.Fatalf("core runtime file %s imports modules/health", file)
		}
	}
}

func newApp(options health.Options) *gest.App {
	app := gest.New()
	app.Import(health.Module(options))
	return app
}

func assertHealthResponse(t *testing.T, app *gest.App, path string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal body returned error: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status body = %q, want ok", body.Status)
	}
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
