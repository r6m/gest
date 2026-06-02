package health

import (
	"net/http"

	"github.com/r6m/gest"
)

const defaultPath = "/health"

// Options configures the optional health module.
type Options struct {
	Path string
}

// Module returns a Gest module that serves simple health endpoints.
func Module(options Options) gest.Module {
	config := newConfig(options)
	return gest.NewModule(gest.ModuleConfig{
		Name: "HealthModule",
		Providers: gest.Providers(
			gest.Value(config),
			gest.Controller(newController),
		),
	})
}

type config struct {
	Path string
}

func newConfig(options Options) config {
	if options.Path == "" {
		return config{Path: defaultPath}
	}
	return config{Path: options.Path}
}

type controller struct {
	config config
}

type response struct {
	Status string `json:"status"`
}

func newController(config config) *controller {
	return &controller{config: config}
}

func (c *controller) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name:     "HealthController",
		BasePath: c.config.Path,
		Tag:      "health",
		Routes: []gest.RouteDefinition{
			{
				Name:     "Check",
				Method:   http.MethodGet,
				Path:     ".",
				Handler:  c.ok,
				Response: (*response)(nil),
				Metadata: gest.RouteMetadata{
					Summary:     "Health check",
					Description: "Returns the application health status.",
				},
			},
			{
				Name:     "Live",
				Method:   http.MethodGet,
				Path:     "/live",
				Handler:  c.ok,
				Response: (*response)(nil),
				Metadata: gest.RouteMetadata{
					Summary:     "Liveness check",
					Description: "Returns the application liveness status.",
				},
			},
			{
				Name:     "Ready",
				Method:   http.MethodGet,
				Path:     "/ready",
				Handler:  c.ok,
				Response: (*response)(nil),
				Metadata: gest.RouteMetadata{
					Summary:     "Readiness check",
					Description: "Returns the application readiness status.",
				},
			},
		},
	}
}

func (c *controller) ok(ctx *gest.Context) error {
	return ctx.JSON(http.StatusOK, response{Status: "ok"})
}
