package gest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindRequestBindsPathParam(t *testing.T) {
	type requestDTO struct {
		ID string `param:"id"`
	}

	context := newBindingContext(http.MethodGet, "/users/123", "")
	context.SetParam("id", "123")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.ID != "123" {
		t.Fatalf("ID = %q, want %q", dto.ID, "123")
	}
}

func TestBindRequestBindsQueryParam(t *testing.T) {
	type requestDTO struct {
		Page int `query:"page"`
	}

	context := newBindingContext(http.MethodGet, "/users?page=2", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Page != 2 {
		t.Fatalf("Page = %d, want %d", dto.Page, 2)
	}
}

func TestBindRequestBindsHeader(t *testing.T) {
	type requestDTO struct {
		Token string `header:"Authorization"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")
	context.RawRequest().Header.Set("Authorization", "Bearer token-1")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Token != "Bearer token-1" {
		t.Fatalf("Token = %q, want %q", dto.Token, "Bearer token-1")
	}
}

func TestBindRequestBindsJSONBody(t *testing.T) {
	type requestDTO struct {
		Name   string  `json:"name"`
		Active bool    `json:"active"`
		Score  float64 `json:"score"`
	}

	context := newBindingContext(http.MethodPost, "/users", `{"name":"Ada","active":true,"score":9.5}`)

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Name != "Ada" || !dto.Active || dto.Score != 9.5 {
		t.Fatalf("dto = %#v, want bound JSON fields", dto)
	}
}

func TestBindRequestBindsMixedSourcesWithExplicitOverrides(t *testing.T) {
	type requestDTO struct {
		ID      string `json:"id" param:"id"`
		Limit   *uint  `json:"limit" query:"limit"`
		Request string `header:"X-Request-ID"`
		Name    string `json:"name"`
	}

	context := newBindingContext(http.MethodPatch, "/users/route-id?limit=25", `{"id":"body-id","limit":10,"name":"Ada"}`)
	context.SetParam("id", "route-id")
	context.RawRequest().Header.Set("X-Request-ID", "req-1")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.ID != "route-id" {
		t.Fatalf("ID = %q, want param override", dto.ID)
	}
	if dto.Limit == nil || *dto.Limit != 25 {
		t.Fatalf("Limit = %#v, want query override 25", dto.Limit)
	}
	if dto.Request != "req-1" {
		t.Fatalf("Request = %q, want header value", dto.Request)
	}
	if dto.Name != "Ada" {
		t.Fatalf("Name = %q, want JSON value", dto.Name)
	}
}

func TestBindRequestConversionFailureIncludesRequestFacingField(t *testing.T) {
	type requestDTO struct {
		Limit int `query:"limit"`
	}

	context := newBindingContext(http.MethodGet, "/users?limit=many", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	if err == nil {
		t.Fatal("BindRequest returned nil error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Code != "BINDING_CONVERSION_FAILURE" {
		t.Fatalf("Code = %q, want BINDING_CONVERSION_FAILURE", httpErr.Code)
	}
	if httpErr.Field != "query.limit" {
		t.Fatalf("Field = %q, want query.limit", httpErr.Field)
	}
}

func TestBindRequestMalformedJSONReturnsBadRequestStyleError(t *testing.T) {
	type requestDTO struct {
		Name string `json:"name"`
	}

	context := newBindingContext(http.MethodPost, "/users", `{"name":`)

	var dto requestDTO
	err := context.BindRequest(&dto)
	if err == nil {
		t.Fatal("BindRequest returned nil error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Kind != ErrorKindBadRequest {
		t.Fatalf("Kind = %q, want %q", httpErr.Kind, ErrorKindBadRequest)
	}
	if httpErr.Code != "BINDING_MALFORMED_JSON" {
		t.Fatalf("Code = %q, want BINDING_MALFORMED_JSON", httpErr.Code)
	}
	if httpErr.Field != "body" {
		t.Fatalf("Field = %q, want body", httpErr.Field)
	}
}

func TestBindRequestEmptyBodyWorksForNonBodyDTO(t *testing.T) {
	type requestDTO struct {
		Limit int `query:"limit"`
	}

	context := newBindingContext(http.MethodGet, "/users?limit=3", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}
	if dto.Limit != 3 {
		t.Fatalf("Limit = %d, want %d", dto.Limit, 3)
	}
}

func TestBindRequestMissingPathParamReturnsStructuredError(t *testing.T) {
	type requestDTO struct {
		ID string `param:"id"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	if err == nil {
		t.Fatal("BindRequest returned nil error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Code != "BINDING_MISSING_REQUIRED_PARAM" {
		t.Fatalf("Code = %q, want BINDING_MISSING_REQUIRED_PARAM", httpErr.Code)
	}
	if httpErr.Field != "param.id" {
		t.Fatalf("Field = %q, want param.id", httpErr.Field)
	}
}

func TestBindRequestUnsupportedTargetReturnsUsefulError(t *testing.T) {
	context := newBindingContext(http.MethodGet, "/users", "")

	err := context.BindRequest(requestDTOValue{})
	if err == nil {
		t.Fatal("BindRequest returned nil error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Kind != ErrorKindBadRequest {
		t.Fatalf("Kind = %q, want %q", httpErr.Kind, ErrorKindBadRequest)
	}
	if !strings.Contains(httpErr.Message, "pointer to a struct") {
		t.Fatalf("Message = %q, want pointer to a struct context", httpErr.Message)
	}
}

type requestDTOValue struct{}

func newBindingContext(method string, target string, body string) *Context {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	return NewContext(httptest.NewRecorder(), httptest.NewRequest(method, target, reader))
}
