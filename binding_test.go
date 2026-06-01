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

func TestBindRequestConvertsEveryScalarType(t *testing.T) {
	type requestDTO struct {
		String  string  `query:"string"`
		Bool    bool    `query:"bool"`
		Int     int     `query:"int"`
		Int8    int8    `query:"int8"`
		Int16   int16   `query:"int16"`
		Int32   int32   `query:"int32"`
		Int64   int64   `query:"int64"`
		Uint    uint    `query:"uint"`
		Uint8   uint8   `query:"uint8"`
		Uint16  uint16  `query:"uint16"`
		Uint32  uint32  `query:"uint32"`
		Uint64  uint64  `query:"uint64"`
		Float32 float32 `query:"float32"`
		Float64 float64 `query:"float64"`
	}

	context := newBindingContext(http.MethodGet, "/users?string=Ada&bool=true&int=-1&int8=-8&int16=-16&int32=-32&int64=-64&uint=1&uint8=8&uint16=16&uint32=32&uint64=64&float32=3.5&float64=7.25", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.String != "Ada" || !dto.Bool || dto.Int != -1 || dto.Int8 != -8 || dto.Int16 != -16 || dto.Int32 != -32 || dto.Int64 != -64 {
		t.Fatalf("signed/string/bool values = %#v, want converted values", dto)
	}
	if dto.Uint != 1 || dto.Uint8 != 8 || dto.Uint16 != 16 || dto.Uint32 != 32 || dto.Uint64 != 64 {
		t.Fatalf("unsigned values = %#v, want converted values", dto)
	}
	if dto.Float32 != 3.5 || dto.Float64 != 7.25 {
		t.Fatalf("float values = %#v, want converted values", dto)
	}
}

func TestBindRequestScalarOverflowErrors(t *testing.T) {
	type requestDTO struct {
		Small int8 `query:"small"`
	}

	context := newBindingContext(http.MethodGet, "/users?small=128", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	_ = assertBindingConversionError(t, err, "query.small", "128")
}

func TestBindRequestInvalidBoolErrors(t *testing.T) {
	type requestDTO struct {
		Active bool `query:"active"`
	}

	context := newBindingContext(http.MethodGet, "/users?active=yespe", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	_ = assertBindingConversionError(t, err, "query.active", "yespe")
}

func TestBindRequestInvalidIntErrors(t *testing.T) {
	type requestDTO struct {
		Limit int `query:"limit"`
	}

	context := newBindingContext(http.MethodGet, "/users?limit=ten", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	_ = assertBindingConversionError(t, err, "query.limit", "ten")
}

func TestBindRequestInvalidFloatErrors(t *testing.T) {
	type requestDTO struct {
		Score float64 `query:"score"`
	}

	context := newBindingContext(http.MethodGet, "/users?score=high", "")

	var dto requestDTO
	err := context.BindRequest(&dto)
	_ = assertBindingConversionError(t, err, "query.score", "high")
}

func TestBindRequestPointerScalarProvidedValueAllocatesPointer(t *testing.T) {
	type requestDTO struct {
		Limit *int `query:"limit"`
	}

	context := newBindingContext(http.MethodGet, "/users?limit=10", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}
	if dto.Limit == nil {
		t.Fatal("Limit = nil, want allocated pointer")
	}
	if *dto.Limit != 10 {
		t.Fatalf("Limit = %d, want %d", *dto.Limit, 10)
	}
}

func TestBindRequestUnsupportedScalarTargetsReturnExplicitErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     any
		wantField  string
		wantValue  string
		wantPhrase string
	}{
		{
			name: "struct",
			target: &struct {
				Filter struct{} `query:"filter"`
			}{},
			wantField:  "query.filter",
			wantValue:  "value",
			wantPhrase: "unsupported struct type",
		},
		{
			name: "map",
			target: &struct {
				Filter map[string]string `query:"filter"`
			}{},
			wantField:  "query.filter",
			wantValue:  "value",
			wantPhrase: "unsupported map type",
		},
		{
			name: "slice",
			target: &struct {
				IDs []int `query:"ids"`
			}{},
			wantField:  "query.ids",
			wantValue:  "1",
			wantPhrase: "unsupported slice type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context := newBindingContext(http.MethodGet, "/users?filter=value&ids=1", "")

			err := context.BindRequest(test.target)
			httpErr := assertBindingConversionError(t, err, test.wantField, test.wantValue)
			if !strings.Contains(httpErr.Message, test.wantPhrase) {
				t.Fatalf("Message = %q, want %q", httpErr.Message, test.wantPhrase)
			}
		})
	}
}

func TestBindRequestAppliesQueryDefaultInt(t *testing.T) {
	type requestDTO struct {
		Page int `query:"page" default:"1"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Page != 1 {
		t.Fatalf("Page = %d, want %d", dto.Page, 1)
	}
}

func TestBindRequestAppliesQueryDefaultBool(t *testing.T) {
	type requestDTO struct {
		Active bool `query:"active" default:"true"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if !dto.Active {
		t.Fatal("Active = false, want true")
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

func TestBindRequestAppliesHeaderDefaultString(t *testing.T) {
	type requestDTO struct {
		Locale string `header:"Accept-Language" default:"en-US"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Locale != "en-US" {
		t.Fatalf("Locale = %q, want %q", dto.Locale, "en-US")
	}
}

func TestBindRequestProvidedValueOverridesDefault(t *testing.T) {
	type requestDTO struct {
		Page   int    `query:"page" default:"1"`
		Locale string `header:"Accept-Language" default:"en-US"`
	}

	context := newBindingContext(http.MethodGet, "/users?page=3", "")
	context.RawRequest().Header.Set("Accept-Language", "fa-IR")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Page != 3 {
		t.Fatalf("Page = %d, want %d", dto.Page, 3)
	}
	if dto.Locale != "fa-IR" {
		t.Fatalf("Locale = %q, want %q", dto.Locale, "fa-IR")
	}
}

func TestBindRequestMalformedProvidedValueStillErrorsWithDefault(t *testing.T) {
	type requestDTO struct {
		Page int `query:"page" default:"1"`
	}

	context := newBindingContext(http.MethodGet, "/users?page=many", "")

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
	if httpErr.Field != "query.page" {
		t.Fatalf("Field = %q, want query.page", httpErr.Field)
	}
}

func TestBindRequestMalformedDefaultValueReturnsFieldError(t *testing.T) {
	type requestDTO struct {
		Page int `query:"page" default:"many"`
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
	if httpErr.Code != "BINDING_CONVERSION_FAILURE" {
		t.Fatalf("Code = %q, want BINDING_CONVERSION_FAILURE", httpErr.Code)
	}
	if httpErr.Field != "query.page" {
		t.Fatalf("Field = %q, want query.page", httpErr.Field)
	}
}

func TestBindRequestPointerScalarDefaultAllocatesPointer(t *testing.T) {
	type requestDTO struct {
		Limit *uint `query:"limit" default:"25"`
	}

	context := newBindingContext(http.MethodGet, "/users", "")

	var dto requestDTO
	if err := context.BindRequest(&dto); err != nil {
		t.Fatalf("BindRequest returned error: %v", err)
	}

	if dto.Limit == nil {
		t.Fatal("Limit = nil, want allocated pointer")
	}
	if *dto.Limit != 25 {
		t.Fatalf("Limit = %d, want %d", *dto.Limit, 25)
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

func assertBindingConversionError(t *testing.T, err error, field string, value string) *HTTPError {
	t.Helper()

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
	if httpErr.Code != "BINDING_CONVERSION_FAILURE" {
		t.Fatalf("Code = %q, want BINDING_CONVERSION_FAILURE", httpErr.Code)
	}
	if httpErr.Field != field {
		t.Fatalf("Field = %q, want %q", httpErr.Field, field)
	}
	if !strings.Contains(httpErr.Message, value) {
		t.Fatalf("Message = %q, want safe received value %q", httpErr.Message, value)
	}

	return httpErr
}

func newBindingContext(method string, target string, body string) *Context {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	return NewContext(httptest.NewRecorder(), httptest.NewRequest(method, target, reader))
}
