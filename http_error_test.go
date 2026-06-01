package gest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFrameworkErrorsMapToExpectedStatusJSON(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		kind    ErrorKind
		code    string
		message string
	}{
		{
			name:    "bad request",
			err:     BadRequest("bad input"),
			status:  http.StatusBadRequest,
			kind:    ErrorKindBadRequest,
			code:    "BAD_REQUEST",
			message: "bad input",
		},
		{
			name:    "unauthorized",
			err:     Unauthorized("missing token"),
			status:  http.StatusUnauthorized,
			kind:    ErrorKindUnauthorized,
			code:    "UNAUTHORIZED",
			message: "missing token",
		},
		{
			name:    "forbidden",
			err:     Forbidden("denied"),
			status:  http.StatusForbidden,
			kind:    ErrorKindForbidden,
			code:    "FORBIDDEN",
			message: "denied",
		},
		{
			name:    "not found",
			err:     NotFound("missing"),
			status:  http.StatusNotFound,
			kind:    ErrorKindNotFound,
			code:    "NOT_FOUND",
			message: "missing",
		},
		{
			name:    "internal",
			err:     Internal("failed"),
			status:  http.StatusInternalServerError,
			kind:    ErrorKindInternal,
			code:    "INTERNAL",
			message: "failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			err := WriteError(recorder, test.err)
			if err != nil {
				t.Fatalf("WriteError returned error: %v", err)
			}

			response := recorder.Result()
			if response.StatusCode != test.status {
				t.Fatalf("StatusCode = %d, want %d", response.StatusCode, test.status)
			}
			if got := response.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", got, "application/json")
			}

			var body struct {
				Error HTTPError `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("Decode body returned error: %v", err)
			}
			if body.Error.Kind != test.kind {
				t.Fatalf("Kind = %q, want %q", body.Error.Kind, test.kind)
			}
			if body.Error.Code != test.code {
				t.Fatalf("Code = %q, want %q", body.Error.Code, test.code)
			}
			if body.Error.Message != test.message {
				t.Fatalf("Message = %q, want %q", body.Error.Message, test.message)
			}
		})
	}
}

func TestHTTPStatusMapsUnknownErrorsToInternal(t *testing.T) {
	status := HTTPStatus(errors.New("boom"))

	if status != http.StatusInternalServerError {
		t.Fatalf("HTTPStatus = %d, want %d", status, http.StatusInternalServerError)
	}
}
