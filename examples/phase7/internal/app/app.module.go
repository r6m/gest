package app

import (
	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/phase7/internal/session"
	"github.com/r6m/gest/modules/health"
)

// Module is the root application module for the Phase 7 optional modules fixture.
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			health.Module(health.Options{}),
			session.Module(),
		),
	})
}
