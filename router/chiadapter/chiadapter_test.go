package chiadapter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r6m/gest"
)

func TestAdapterRegistersGETRouteAndServesResponse(t *testing.T) {
	adapter := New()
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/hello",
		Handler: func(ctx *gest.Context) error {
			return ctx.JSON(http.StatusOK, map[string]string{"message": "hello"})
		},
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hello", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"message\":\"hello\"}\n" {
		t.Fatalf("body = %q, want hello JSON", got)
	}
}

func TestAdapterRegistersGroupedRoute(t *testing.T) {
	adapter := New()
	adapter.Group("/api", func(group gest.RouterAdapter) {
		group.Handle(gest.RouteRuntimeConfig{
			Method: http.MethodGet,
			Path:   "/health",
			Handler: func(ctx *gest.Context) error {
				return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
			},
		})
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q, want grouped JSON", got)
	}
}

func TestAdapterPathParamsReachContext(t *testing.T) {
	adapter := New()
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/users/{id}",
		Handler: func(ctx *gest.Context) error {
			return ctx.JSON(http.StatusOK, map[string]string{"id": ctx.Param("id")})
		},
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/123", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"id\":\"123\"}\n" {
		t.Fatalf("body = %q, want id JSON", got)
	}
}

func TestAdapterQueryAndHeaderHelpersWork(t *testing.T) {
	adapter := New()
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/reports",
		Handler: func(ctx *gest.Context) error {
			return ctx.JSON(http.StatusOK, map[string]string{
				"limit":   ctx.Query("limit"),
				"request": ctx.Header("X-Request-ID"),
			})
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/reports?limit=25", nil)
	request.Header.Set("X-Request-ID", "req-1")

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "{\"limit\":\"25\",\"request\":\"req-1\"}\n" {
		t.Fatalf("body = %q, want query/header JSON", got)
	}
}

func TestAdapterMiddlewareOrderWorks(t *testing.T) {
	adapter := New()
	order := make([]string, 0, 3)
	adapter.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
		return func(ctx *gest.Context) error {
			order = append(order, "first-before")
			err := next(ctx)
			order = append(order, "first-after")
			return err
		}
	}))
	adapter.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
		return func(ctx *gest.Context) error {
			order = append(order, "second-before")
			err := next(ctx)
			order = append(order, "second-after")
			return err
		}
	}))
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/ordered",
		Handler: func(ctx *gest.Context) error {
			order = append(order, "handler")
			return ctx.NoContent(http.StatusNoContent)
		},
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ordered", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	want := []string{"first-before", "second-before", "handler", "second-after", "first-after"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d: %#v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q; full order %#v", i, order[i], want[i], order)
		}
	}
}

func TestAdapterServesSSEThroughNormalGetRoute(t *testing.T) {
	adapter := New()
	observedStatus := -1
	adapter.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
		return func(ctx *gest.Context) error {
			err := next(ctx)
			observedStatus = ctx.ResponseStatus()
			return err
		}
	}))
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/events",
		Handler: func(ctx *gest.Context) error {
			return ctx.SSE(func(events *gest.SSE) error {
				return events.Send("ready", map[string]string{"id": "evt-1"})
			})
		},
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/events", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if observedStatus != http.StatusOK {
		t.Fatalf("observed status = %d, want %d", observedStatus, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if got, want := recorder.Body.String(), "event: ready\ndata: {\"id\":\"evt-1\"}\n\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if !recorder.Flushed {
		t.Fatal("recorder.Flushed = false, want true")
	}
}

func TestAdapterPropagatesSSEHandlerErrorToErrorWriter(t *testing.T) {
	adapter := New()
	adapter.Handle(gest.RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   "/events",
		Handler: func(ctx *gest.Context) error {
			return ctx.SSE(func(events *gest.SSE) error {
				return errors.New("source failed")
			})
		},
	})

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/events", nil))
	response := recorder.Result()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d after SSE status was written", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := recorder.Body.String(); got != "{\"error\":{\"kind\":\"Internal\",\"code\":\"INTERNAL\",\"message\":\"Internal Server Error\"}}\n" {
		t.Fatalf("body = %q, want propagated framework error body", got)
	}
}

func TestAdapterName(t *testing.T) {
	if got := New().Name(); got != "chi" {
		t.Fatalf("Name() = %q, want %q", got, "chi")
	}
}
