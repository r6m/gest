package gest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContextQueryAndHeaderHelpers(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/reports?limit=10", nil)
	request.Header.Set("X-Request-ID", "req-1")
	context := NewContext(httptest.NewRecorder(), request)

	if got := context.Query("limit"); got != "10" {
		t.Fatalf("Query(limit) = %q, want %q", got, "10")
	}
	if got := context.Header("X-Request-ID"); got != "req-1" {
		t.Fatalf("Header(X-Request-ID) = %q, want %q", got, "req-1")
	}
}

func TestContextParamHelper(t *testing.T) {
	context := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/reports/123", nil))
	context.SetParam("id", "123")

	if got := context.Param("id"); got != "123" {
		t.Fatalf("Param(id) = %q, want %q", got, "123")
	}
}

func TestContextBearerTokenParsesValidAuthorizationHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token-123")
	context := NewContext(httptest.NewRecorder(), request)

	if got := context.BearerToken(); got != "token-123" {
		t.Fatalf("BearerToken() = %q, want %q", got, "token-123")
	}
}

func TestContextBearerTokenReturnsEmptyForMissingOrNonBearerAuth(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
	}{
		{name: "missing"},
		{name: "basic", authorization: "Basic abc"},
		{name: "malformed", authorization: "Bearer"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			context := NewContext(httptest.NewRecorder(), request)

			if got := context.BearerToken(); got != "" {
				t.Fatalf("BearerToken() = %q, want empty", got)
			}
		})
	}
}

func TestContextJSONWritesStatusContentTypeAndBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	err := context.JSON(http.StatusCreated, map[string]string{"id": "123"})
	if err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}

	response := recorder.Result()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body returned error: %v", err)
	}
	if body["id"] != "123" {
		t.Fatalf("body[id] = %q, want %q", body["id"], "123")
	}
}

func TestContextNoContentWritesStatusWithNoBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	err := context.NoContent(http.StatusNoContent)
	if err != nil {
		t.Fatalf("NoContent returned error: %v", err)
	}

	response := recorder.Result()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("Body length = %d, want 0", recorder.Body.Len())
	}
}

func TestContextSetGetStoresRequestLocalValues(t *testing.T) {
	context := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	context.Set("userID", "u-1")
	value, ok := context.Get("userID")
	if !ok {
		t.Fatal("Get(userID) ok = false, want true")
	}
	if value != "u-1" {
		t.Fatalf("Get(userID) = %#v, want %q", value, "u-1")
	}
	_, ok = context.Get("missing")
	if ok {
		t.Fatal("Get(missing) ok = true, want false")
	}
}

func TestContextEscapeHatchesReturnNetHTTPObjects(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	context := NewContext(recorder, request)

	if context.RawRequest() != request {
		t.Fatal("RawRequest() did not return original request")
	}
	if context.RawResponse() != recorder {
		t.Fatal("RawResponse() did not return original response writer")
	}
	if context.Native() != request.Context() {
		t.Fatal("Native() did not return request context")
	}
	if context.Engine() != "net/http" {
		t.Fatalf("Engine() = %q, want %q", context.Engine(), "net/http")
	}
}
