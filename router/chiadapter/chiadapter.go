package chiadapter

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/r6m/gest"
)

// Adapter is the first-party Chi/net-http router adapter.
type Adapter struct {
	router chi.Router
}

// New creates a Chi adapter with a new router.
func New() *Adapter {
	return From(chi.NewRouter())
}

// From creates a Chi adapter from an existing router.
func From(router chi.Router) *Adapter {
	return &Adapter{router: router}
}

// Name returns the adapter name.
func (a *Adapter) Name() string {
	return "chi"
}

// Group registers routes under a prefix.
func (a *Adapter) Group(prefix string, fn func(group gest.RouterAdapter)) {
	a.router.Route(prefix, func(router chi.Router) {
		fn(From(router))
	})
}

// Handle registers a route handler.
func (a *Adapter) Handle(route gest.RouteRuntimeConfig) {
	a.router.MethodFunc(normalizeMethod(route.Method), route.Path, func(response http.ResponseWriter, request *http.Request) {
		context := gest.NewContext(response, request)
		for _, key := range chi.RouteContext(request.Context()).URLParams.Keys {
			context.SetParam(key, chi.URLParam(request, key))
		}

		if err := route.Handler(context); err != nil {
			_ = gest.WriteError(response, err)
		}
	})
}

// Use registers a Gest middleware.
func (a *Adapter) Use(middleware gest.Middleware) {
	a.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			handler := func(context *gest.Context) error {
				next.ServeHTTP(context.RawResponse(), context.RawRequest())
				return nil
			}
			context := gest.NewContext(response, request)
			for _, key := range chi.RouteContext(request.Context()).URLParams.Keys {
				context.SetParam(key, chi.URLParam(request, key))
			}
			if err := middleware(handler)(context); err != nil {
				_ = gest.WriteError(response, err)
			}
		})
	})
}

// Serve starts an HTTP server at addr.
func (a *Adapter) Serve(addr string) error {
	return http.ListenAndServe(addr, a.router)
}

// ServeHTTP exposes the underlying router for tests and advanced net/http usage.
func (a *Adapter) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	a.router.ServeHTTP(response, request)
}

func normalizeMethod(method string) string {
	return strings.ToUpper(method)
}
