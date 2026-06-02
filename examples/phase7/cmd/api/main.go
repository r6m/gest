package main

import (
	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/phase7/internal/app"
	"github.com/r6m/gest/modules/validation"
)

func main() {
	server := gest.New(
		gest.WithBootLogs(true),
		gest.WithValidator(validation.NewValidator()),
	)
	server.OpenAPI("")
	server.Import(app.Module())

	if err := server.Listen(":3000"); err != nil {
		panic(err)
	}
}
