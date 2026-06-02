package gest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type jsonRequest struct {
	ID string `json:"id"`
}

type jsonResponse struct {
	Name string `json:"name"`
}

func TestJSONResponseErrorHandlerWritesDefaultStatusAndJSON(t *testing.T) {
	handler := JSON(func(ctx *Context, req *jsonRequest) (*jsonResponse, error) {
		if req == nil {
			t.Fatal("request is nil")
		}

		return &jsonResponse{Name: "Ada"}, nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body jsonResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Name != "Ada" {
		t.Fatalf("Name = %q, want %q", body.Name, "Ada")
	}
}

func TestJSONStatusOptionChangesSuccessStatus(t *testing.T) {
	handler := JSON(func(ctx *Context, req *jsonRequest) (*jsonResponse, error) {
		return &jsonResponse{Name: "Ada"}, nil
	}, Status(http.StatusCreated))

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestJSONContextResponseErrorHandlerWritesDefaultStatusAndJSON(t *testing.T) {
	handler := JSON(func(ctx *Context) (*jsonResponse, error) {
		return &jsonResponse{Name: "Ada"}, nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body jsonResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Name != "Ada" {
		t.Fatalf("Name = %q, want %q", body.Name, "Ada")
	}
}

func TestJSONContextResponseErrorHandlerUsesStatusOption(t *testing.T) {
	handler := JSON(func(ctx *Context) (*jsonResponse, error) {
		return &jsonResponse{Name: "Ada"}, nil
	}, Status(http.StatusCreated))

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestJSONNilResponseReturnsNoContent(t *testing.T) {
	handler := JSON(func(ctx *Context, req *jsonRequest) (*jsonResponse, error) {
		return nil, nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestJSONContextResponseErrorHandlerNilResponseReturnsNoContent(t *testing.T) {
	handler := JSON(func(ctx *Context) (*jsonResponse, error) {
		return nil, nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestJSONHandlerErrorMapsThroughHTTPErrorResponse(t *testing.T) {
	handler := JSON(func(ctx *Context, req *jsonRequest) (*jsonResponse, error) {
		return nil, NotFound("user missing")
	})

	recorder, request := newJSONTestContext()
	err := handler(NewContext(recorder, request))
	if err == nil {
		t.Fatal("handler returned nil error")
	}
	if writeErr := WriteError(recorder, err); writeErr != nil {
		t.Fatalf("WriteError returned error: %v", writeErr)
	}

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	var body struct {
		Error HTTPError `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Kind != ErrorKindNotFound {
		t.Fatalf("error kind = %q, want %q", body.Error.Kind, ErrorKindNotFound)
	}
}

func TestJSONContextResponseErrorHandlerErrorMapsThroughHTTPErrorResponse(t *testing.T) {
	handler := JSON(func(ctx *Context) (*jsonResponse, error) {
		return nil, NotFound("user missing")
	})

	recorder, request := newJSONTestContext()
	err := handler(NewContext(recorder, request))
	if err == nil {
		t.Fatal("handler returned nil error")
	}
	if writeErr := WriteError(recorder, err); writeErr != nil {
		t.Fatalf("WriteError returned error: %v", writeErr)
	}

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	var body struct {
		Error HTTPError `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Kind != ErrorKindNotFound {
		t.Fatalf("error kind = %q, want %q", body.Error.Kind, ErrorKindNotFound)
	}
}

func TestJSONRequestErrorHandlerReturnsNoContentOnNilError(t *testing.T) {
	handler := JSON(func(ctx *Context, req *jsonRequest) error {
		if req == nil {
			t.Fatal("request is nil")
		}

		return nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestJSONBindsRequestBeforeCallingHandler(t *testing.T) {
	type requestDTO struct {
		ID      string `param:"id"`
		Page    int    `query:"page"`
		Request string `header:"X-Request-ID"`
		Name    string `json:"name"`
	}

	handler := JSON(func(ctx *Context, req *requestDTO) (*jsonResponse, error) {
		return &jsonResponse{Name: req.ID + "|" + req.Name + "|page-" + strconv.Itoa(req.Page) + "|" + req.Request}, nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users/1?page=2", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("X-Request-ID", "req-1")
	context := NewContext(recorder, request)
	context.SetParam("id", "user-1")

	if err := handler(context); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var body jsonResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Name != "user-1|Ada|page-2|req-1" {
		t.Fatalf("Name = %q, want bound DTO values", body.Name)
	}
}

func TestJSONBindingErrorPreventsHandlerExecution(t *testing.T) {
	type requestDTO struct {
		Limit int `query:"limit"`
	}

	called := false
	handler := JSON(func(ctx *Context, req *requestDTO) (*jsonResponse, error) {
		called = true
		return &jsonResponse{Name: "unexpected"}, nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/users?limit=many", nil)
	err := handler(NewContext(recorder, request))
	if err == nil {
		t.Fatal("handler returned nil error")
	}
	if called {
		t.Fatal("handler was called after binding failure")
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
	if httpErr.Field != "query.limit" {
		t.Fatalf("Field = %q, want query.limit", httpErr.Field)
	}
}

func TestJSONValidationErrorPreventsHandlerExecution(t *testing.T) {
	type requestDTO struct {
		Name string `json:"name"`
	}

	called := false
	handler := JSON(func(ctx *Context, req *requestDTO) (*jsonResponse, error) {
		called = true
		return &jsonResponse{Name: "unexpected"}, nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada"}`))
	context := NewContext(recorder, request)
	validator := &recordingValidator{err: errors.New("name is invalid")}
	context.SetValidator(validator)

	err := handler(context)
	if err == nil {
		t.Fatal("handler returned nil error")
	}
	if called {
		t.Fatal("handler was called after validation failure")
	}
	if validator.calls != 1 {
		t.Fatalf("validator calls = %d, want %d", validator.calls, 1)
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Code != "BINDING_VALIDATION_FAILURE" {
		t.Fatalf("Code = %q, want BINDING_VALIDATION_FAILURE", httpErr.Code)
	}
}

func TestJSONValidationErrorMapsThroughHTTPErrorResponse(t *testing.T) {
	type requestDTO struct {
		Name string `json:"name"`
	}

	handler := JSON(func(ctx *Context, req *requestDTO) (*jsonResponse, error) {
		return &jsonResponse{Name: "unexpected"}, nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada"}`))
	context := NewContext(recorder, request)
	context.SetValidator(&recordingValidator{err: errors.New("name is invalid")})

	err := handler(context)
	if err == nil {
		t.Fatal("handler returned nil error")
	}
	if writeErr := WriteError(recorder, err); writeErr != nil {
		t.Fatalf("WriteError returned error: %v", writeErr)
	}

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var body struct {
		Error HTTPError `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Kind != ErrorKindBadRequest {
		t.Fatalf("error kind = %q, want %q", body.Error.Kind, ErrorKindBadRequest)
	}
	if body.Error.Code != "BINDING_VALIDATION_FAILURE" {
		t.Fatalf("error code = %q, want BINDING_VALIDATION_FAILURE", body.Error.Code)
	}
}

func TestJSONContextErrorHandlerReturnsNoContentOnNilError(t *testing.T) {
	handler := JSON(func(ctx *Context) error {
		return nil
	})

	recorder, request := newJSONTestContext()
	if err := handler(NewContext(recorder, request)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func newJSONTestContext() (*httptest.ResponseRecorder, *http.Request) {
	return httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/1", nil)
}
