package gest

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Context wraps net/http request and response objects without hiding them.
type Context struct {
	response  *statusResponseWriter
	request   *http.Request
	params    map[string]string
	values    map[string]any
	validator Validator
	native    any
	engine    string
}

// NewContext creates a net/http-backed context.
func NewContext(response http.ResponseWriter, request *http.Request) *Context {
	trackedResponse, ok := response.(*statusResponseWriter)
	if !ok {
		trackedResponse = &statusResponseWriter{ResponseWriter: response}
	}
	return &Context{
		response:  trackedResponse,
		request:   request,
		params:    make(map[string]string),
		values:    make(map[string]any),
		validator: noopValidator{},
		native:    request.Context(),
		engine:    "net/http",
	}
}

// Param returns a path parameter by name.
func (c *Context) Param(name string) string {
	return c.params[name]
}

// SetParam stores a path parameter for router adapters.
func (c *Context) SetParam(name string, value string) {
	c.params[name] = value
}

// Query returns the first query value by name.
func (c *Context) Query(name string) string {
	return c.request.URL.Query().Get(name)
}

// Header returns the request header value by name.
func (c *Context) Header(name string) string {
	return c.request.Header.Get(name)
}

// BearerToken returns the bearer token from Authorization, if present.
func (c *Context) BearerToken() string {
	value := strings.TrimSpace(c.Header("Authorization"))
	if value == "" {
		return ""
	}

	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}

	return strings.TrimSpace(token)
}

// JSON writes a JSON response.
func (c *Context) JSON(status int, value any) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(status)
	return json.NewEncoder(c.response).Encode(value)
}

// NoContent writes a response status with no body.
func (c *Context) NoContent(status int) error {
	c.response.WriteHeader(status)
	return nil
}

// ResponseStatus returns the response status written so far, or 0 before a status is written.
func (c *Context) ResponseStatus() int {
	return c.response.status
}

// Validate validates a bound request DTO using the configured validator.
func (c *Context) Validate(v any) error {
	if c.validator == nil {
		return nil
	}
	if err := c.validator.Validate(v); err != nil {
		return bindingError(
			"BINDING_VALIDATION_FAILURE",
			"request validation failed: "+err.Error(),
			"",
			"",
		)
	}

	return nil
}

// SetValidator configures request validation for this context.
func (c *Context) SetValidator(validator Validator) {
	if validator == nil {
		c.validator = noopValidator{}
		return
	}
	c.validator = validator
}

// Set stores a request-local value.
func (c *Context) Set(key string, value any) {
	c.values[key] = value
}

// Get returns a request-local value.
func (c *Context) Get(key string) (any, bool) {
	value, ok := c.values[key]
	return value, ok
}

// Native returns the underlying native router context.
func (c *Context) Native() any {
	return c.native
}

// Engine returns the backing router engine name.
func (c *Context) Engine() string {
	return c.engine
}

// RawResponse returns the underlying net/http response writer.
func (c *Context) RawResponse() http.ResponseWriter {
	return c.response
}

// RawRequest returns the underlying net/http request.
func (c *Context) RawRequest() *http.Request {
	return c.request
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
