package main

import (
	"log"

	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/hello/internal/app"
)

func main() {
	server := gest.New(gest.WithBootLogs(true))
	server.OpenAPI("/openapi.json", gest.OpenAPITitle("Hello API"), gest.OpenAPIVersion("1.0.0"))
	server.Import(app.Module())

	if err := server.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
