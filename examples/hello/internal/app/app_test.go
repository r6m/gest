package app_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/hello/internal/app"
	"github.com/r6m/gest/gesttest"
)

func TestHelloExampleFindsUser(t *testing.T) {
	server := gesttest.New(t, app.Module())

	var response struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Details string `json:"details"`
	}
	server.GET("/users/123?expand=true").
		ExpectStatus(http.StatusOK).
		DecodeJSON(&response)

	if response.ID != "123" || response.Name != "Ada Lovelace" || response.Details == "" {
		t.Fatalf("response = %#v, want expanded user", response)
	}
}

func TestHelloExampleCreatesUser(t *testing.T) {
	server := gesttest.New(t, app.Module())

	var response struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	server.POST("/users/").
		JSONBody(map[string]string{
			"name":  "Grace Hopper",
			"email": "grace@example.test",
		}).
		ExpectStatus(http.StatusCreated).
		DecodeJSON(&response)

	if response.ID != "new" || response.Name != "Grace Hopper" {
		t.Fatalf("response = %#v, want created user", response)
	}
}

func TestHelloExampleServesOpenAPIAndSwagger(t *testing.T) {
	server := gest.New()
	server.OpenAPI("/openapi.json", gest.OpenAPITitle("Hello API"), gest.OpenAPIVersion("1.0.0"))
	server.Import(app.Module())

	openAPI := httptest.NewRecorder()
	server.ServeHTTP(openAPI, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if openAPI.Code != http.StatusOK {
		t.Fatalf("OpenAPI status = %d, want 200; body %s", openAPI.Code, openAPI.Body.String())
	}

	docs := httptest.NewRecorder()
	server.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if docs.Code != http.StatusOK {
		t.Fatalf("Swagger status = %d, want 200; body %s", docs.Code, docs.Body.String())
	}
}
