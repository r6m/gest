package app

import (
	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/hello/internal/users"
	"github.com/r6m/gest/modules/swagger"
)

// Module is the root application module for the hello example.
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			users.Module(),
			swagger.Module(swagger.Options{
				Path:        "/docs",
				OpenAPIPath: "/openapi.json",
			}),
		),
	})
}
