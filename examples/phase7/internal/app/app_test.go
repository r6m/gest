package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/phase7/internal/app"
	"github.com/r6m/gest/modules/validation"
)

func TestPhase7FixtureUsesOptionalModulesTogether(t *testing.T) {
	server := newServer()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/sessions/", strings.NewReader(`{"subject":"user-1"}`))
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var body struct {
		Service string `json:"service"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal body returned error: %v", err)
	}
	if body.Service != "phase7" {
		t.Fatalf("service = %q, want phase7", body.Service)
	}
	if body.Token == "" {
		t.Fatal("token is empty")
	}
}

func TestPhase7FixtureInstallsValidationExplicitly(t *testing.T) {
	server := newServer()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/sessions/", strings.NewReader(`{}`))
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "BINDING_VALIDATION_FAILURE") {
		t.Fatalf("body = %s, want validation failure code", recorder.Body.String())
	}
}

func TestPhase7FixtureIncludesHealthRoutes(t *testing.T) {
	server := newServer()

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if strings.TrimSpace(recorder.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("body = %q, want health JSON", recorder.Body.String())
	}
}

func TestPhase7FixtureOpenAPIIncludesRoutes(t *testing.T) {
	server := newServer()

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"/sessions/"`) || !strings.Contains(body, `"/health/ready"`) {
		t.Fatalf("OpenAPI body missing fixture routes: %s", body)
	}
}

func newServer() *gest.App {
	server := gest.New(gest.WithValidator(validation.NewValidator()))
	server.OpenAPI("")
	server.Import(app.Module())
	return server
}
