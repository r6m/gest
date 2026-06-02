// Package gesttest provides small helpers for testing Gest apps with standard Go tests.
package gesttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"

	"github.com/r6m/gest"
)

// TB is the subset of testing.TB used by gesttest helpers.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// App is an in-memory test wrapper around a real Gest app.
type App struct {
	t   TB
	app *gest.App
}

// Option configures a test app.
type Option func(*config)

type config struct {
	overrides []override
}

type override struct {
	constructor any
	value       any
}

// New builds an in-memory Gest app for tests.
//
// Items may be gest.Module values and gesttest.Option values:
//
//	app := gesttest.New(t, users.Module(users.Options{}), gesttest.Override(NewUserService, fakeService))
func New(t TB, items ...any) *App {
	t.Helper()

	config := config{}
	modules := make([]gest.Module, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case gest.Module:
			modules = append(modules, value)
		case Option:
			value(&config)
		default:
			t.Fatalf("gesttest.New item %T is not a gest.Module or gesttest.Option", item)
		}
	}

	modules = applyOverrides(t, modules, config.overrides)
	app := gest.New()
	app.Import(modules...)

	return &App{t: t, app: app}
}

// Override replaces a provider declared with constructor by an explicit value provider.
func Override(constructor any, value any) Option {
	return func(config *config) {
		config.overrides = append(config.overrides, override{
			constructor: constructor,
			value:       value,
		})
	}
}

// GestApp returns the underlying app for escape hatches.
func (a *App) GestApp() *gest.App {
	a.t.Helper()
	return a.app
}

// GET issues an in-memory GET request.
func (a *App) GET(path string) *Request {
	a.t.Helper()
	return a.Request(http.MethodGet, path)
}

// POST issues an in-memory POST request.
func (a *App) POST(path string) *Request {
	a.t.Helper()
	return a.Request(http.MethodPost, path)
}

// Request starts an in-memory HTTP request.
func (a *App) Request(method string, path string) *Request {
	a.t.Helper()
	return &Request{
		t:      a.t,
		app:    a.app,
		method: method,
		path:   path,
		header: make(http.Header),
	}
}

// Request configures an in-memory HTTP request.
type Request struct {
	t      TB
	app    *gest.App
	method string
	path   string
	header http.Header
	body   io.Reader
}

// Header sets a request header.
func (r *Request) Header(key string, value string) *Request {
	r.t.Helper()
	r.header.Set(key, value)
	return r
}

// JSONBody encodes value as a JSON request body and sets Content-Type.
func (r *Request) JSONBody(value any) *Request {
	r.t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		r.t.Fatalf("encode JSON request body: %v", err)
	}
	r.body = bytes.NewReader(body)
	r.header.Set("Content-Type", "application/json")
	return r
}

// Send executes the request and returns the response assertion helper.
func (r *Request) Send() *Response {
	r.t.Helper()
	request := httptest.NewRequest(r.method, r.path, r.body)
	request.Header = r.header.Clone()
	recorder := httptest.NewRecorder()
	r.app.ServeHTTP(recorder, request)
	return &Response{t: r.t, recorder: recorder}
}

// ExpectStatus executes the request and asserts its HTTP status.
func (r *Request) ExpectStatus(status int) *Response {
	r.t.Helper()
	return r.Send().ExpectStatus(status)
}

// Response wraps an httptest response.
type Response struct {
	t        TB
	recorder *httptest.ResponseRecorder
}

// ExpectStatus asserts the response status code.
func (r *Response) ExpectStatus(status int) *Response {
	r.t.Helper()
	if r.recorder.Code != status {
		r.t.Fatalf("HTTP status = %d, want %d; body: %s", r.recorder.Code, status, r.recorder.Body.String())
	}
	return r
}

// DecodeJSON decodes the response body into target.
func (r *Response) DecodeJSON(target any) *Response {
	r.t.Helper()
	if err := json.NewDecoder(r.recorder.Body).Decode(target); err != nil {
		r.t.Fatalf("decode JSON response: %v; body: %s", err, r.recorder.Body.String())
	}
	return r
}

// RawResponse exposes the underlying net/http response.
func (r *Response) RawResponse() *http.Response {
	r.t.Helper()
	return r.recorder.Result()
}

func applyOverrides(t TB, modules []gest.Module, overrides []override) []gest.Module {
	t.Helper()
	if len(overrides) == 0 {
		return modules
	}

	applied := make([]bool, len(overrides))
	replaced := make([]gest.Module, 0, len(modules))
	for _, module := range modules {
		replaced = append(replaced, overrideModule(t, module, overrides, applied))
	}
	for index, ok := range applied {
		if !ok {
			t.Fatalf("gesttest.Override target %s did not match any provider constructor", describeConstructor(overrides[index].constructor))
		}
	}
	return replaced
}

func overrideModule(t TB, module gest.Module, overrides []override, applied []bool) gest.Module {
	t.Helper()
	definition := module.Definition()
	imports := make([]gest.Module, 0, len(definition.Imports))
	for _, imported := range definition.Imports {
		imports = append(imports, overrideModule(t, imported, overrides, applied))
	}
	providers := make([]gest.Provider, 0, len(definition.Providers))
	for _, provider := range definition.Providers {
		providers = append(providers, overrideProvider(t, provider, overrides, applied))
	}
	return gest.NewModule(gest.ModuleConfig{
		Name:      definition.Name,
		Global:    definition.Global,
		Imports:   imports,
		Providers: providers,
	})
}

func overrideProvider(t TB, provider gest.Provider, overrides []override, applied []bool) gest.Provider {
	t.Helper()
	for index, override := range overrides {
		if sameFunction(provider.Constructor, override.constructor) {
			applied[index] = true
			return replacementProvider(t, provider, override)
		}
	}
	return provider
}

func replacementProvider(t TB, provider gest.Provider, override override) gest.Provider {
	t.Helper()
	constructorType := reflect.TypeOf(provider.Constructor)
	if constructorType == nil || constructorType.Kind() != reflect.Func || constructorType.NumOut() == 0 {
		t.Fatalf("gesttest.Override target %s is not a valid provider constructor", describeConstructor(override.constructor))
	}

	resultType := constructorType.Out(0)
	value := reflect.ValueOf(override.value)
	if !value.IsValid() || !value.Type().AssignableTo(resultType) {
		t.Fatalf("gesttest.Override value %T is not assignable to provider result %s", override.value, resultType)
	}

	replacementType := reflect.FuncOf(nil, []reflect.Type{resultType}, false)
	replacementConstructor := reflect.MakeFunc(replacementType, func([]reflect.Value) []reflect.Value {
		return []reflect.Value{value}
	}).Interface()

	return gest.Provider{
		Kind:        provider.Kind,
		Constructor: replacementConstructor,
		Exported:    provider.Exported,
		Scope:       provider.Scope,
		Name:        provider.Name,
		Aliases:     append([]gest.Token(nil), provider.Aliases...),
	}
}

func sameFunction(left any, right any) bool {
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if !leftValue.IsValid() || !rightValue.IsValid() {
		return false
	}
	if leftValue.Kind() != reflect.Func || rightValue.Kind() != reflect.Func {
		return false
	}
	return leftValue.Pointer() == rightValue.Pointer()
}

func describeConstructor(constructor any) string {
	value := reflect.ValueOf(constructor)
	if !value.IsValid() || value.Kind() != reflect.Func {
		return fmt.Sprintf("%T", constructor)
	}
	return value.Type().String()
}
