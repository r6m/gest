package gest

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorKind identifies a framework HTTP error category.
type ErrorKind string

const (
	ErrorKindBadRequest   ErrorKind = "BadRequest"
	ErrorKindUnauthorized ErrorKind = "Unauthorized"
	ErrorKindForbidden    ErrorKind = "Forbidden"
	ErrorKindNotFound     ErrorKind = "NotFound"
	ErrorKindInternal     ErrorKind = "Internal"
)

// HTTPError is a structured framework HTTP error.
type HTTPError struct {
	Kind    ErrorKind `json:"kind"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Hint    string    `json:"hint,omitempty"`
}

func (e *HTTPError) Error() string {
	return e.Code + ": " + e.Message
}

// BadRequest creates a 400 framework error.
func BadRequest(message string) error {
	return httpError(ErrorKindBadRequest, "BAD_REQUEST", message)
}

// Unauthorized creates a 401 framework error.
func Unauthorized(message string) error {
	return httpError(ErrorKindUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden creates a 403 framework error.
func Forbidden(message string) error {
	return httpError(ErrorKindForbidden, "FORBIDDEN", message)
}

// NotFound creates a 404 framework error.
func NotFound(message string) error {
	return httpError(ErrorKindNotFound, "NOT_FOUND", message)
}

// Internal creates a 500 framework error.
func Internal(message string) error {
	return httpError(ErrorKindInternal, "INTERNAL", message)
}

// HTTPStatus maps framework errors to HTTP status codes.
func HTTPStatus(err error) int {
	if frameworkError, ok := errors.AsType[*HTTPError](err); ok {
		switch frameworkError.Kind {
		case ErrorKindBadRequest:
			return http.StatusBadRequest
		case ErrorKindUnauthorized:
			return http.StatusUnauthorized
		case ErrorKindForbidden:
			return http.StatusForbidden
		case ErrorKindNotFound:
			return http.StatusNotFound
		case ErrorKindInternal:
			return http.StatusInternalServerError
		}
	}

	return http.StatusInternalServerError
}

// WriteError writes the default deterministic JSON error response.
func WriteError(response http.ResponseWriter, err error) error {
	frameworkError := toHTTPError(err)
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(HTTPStatus(frameworkError))
	return json.NewEncoder(response).Encode(struct {
		Error *HTTPError `json:"error"`
	}{
		Error: frameworkError,
	})
}

func httpError(kind ErrorKind, code string, message string) error {
	return &HTTPError{
		Kind:    kind,
		Code:    code,
		Message: message,
	}
}

func toHTTPError(err error) *HTTPError {
	if frameworkError, ok := errors.AsType[*HTTPError](err); ok {
		return frameworkError
	}

	return &HTTPError{
		Kind:    ErrorKindInternal,
		Code:    "INTERNAL",
		Message: http.StatusText(http.StatusInternalServerError),
	}
}
