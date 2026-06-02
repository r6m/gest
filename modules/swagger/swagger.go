package swagger

import (
	"html/template"
	"net/http"

	"github.com/r6m/gest"
)

const (
	defaultPath        = "/docs"
	defaultOpenAPIPath = "/openapi.json"
)

// Options configures the optional Swagger UI module.
type Options struct {
	Path        string
	OpenAPIPath string
}

// Module returns a Gest module that serves Swagger UI HTML.
func Module(options Options) gest.Module {
	config := newConfig(options)
	return gest.NewModule(gest.ModuleConfig{
		Name: "SwaggerModule",
		Providers: gest.Providers(
			gest.Value(config),
			gest.Controller(newController),
		),
	})
}

type config struct {
	Path        string
	OpenAPIPath string
}

func newConfig(options Options) config {
	config := config{
		Path:        options.Path,
		OpenAPIPath: options.OpenAPIPath,
	}
	if config.Path == "" {
		config.Path = defaultPath
	}
	if config.OpenAPIPath == "" {
		config.OpenAPIPath = defaultOpenAPIPath
	}
	return config
}

type controller struct {
	config config
}

func newController(config config) *controller {
	return &controller{config: config}
}

func (c *controller) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name:     "SwaggerController",
		BasePath: c.config.Path,
		Tag:      "swagger",
		Routes: []gest.RouteDefinition{
			{
				Name:    "Index",
				Method:  http.MethodGet,
				Path:    ".",
				Handler: c.index,
				Metadata: gest.RouteMetadata{
					Summary:     "Swagger UI",
					Description: "Serves Swagger UI for the configured OpenAPI document.",
				},
			},
			{
				Name:    "RedirectTrailingSlash",
				Method:  http.MethodGet,
				Path:    "/",
				Handler: c.redirectTrailingSlash,
			},
		},
	}
}

func (c *controller) index(ctx *gest.Context) error {
	ctx.RawResponse().Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx.RawResponse().WriteHeader(http.StatusOK)
	return swaggerTemplate.Execute(ctx.RawResponse(), c.config)
}

func (c *controller) redirectTrailingSlash(ctx *gest.Context) error {
	http.Redirect(ctx.RawResponse(), ctx.RawRequest(), c.config.Path, http.StatusMovedPermanently)
	return nil
}

var swaggerTemplate = template.Must(template.New("swagger").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: {{ .OpenAPIPath }},
      dom_id: '#swagger-ui'
    });
  </script>
</body>
</html>
`))
