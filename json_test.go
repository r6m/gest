package gest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
