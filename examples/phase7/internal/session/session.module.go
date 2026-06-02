package session

import (
	"io"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/examples/phase7/internal/settings"
	"github.com/r6m/gest/modules/config"
	"github.com/r6m/gest/modules/jwt"
	"github.com/r6m/gest/modules/logger"
)

func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "SessionModule",
		Imports: gest.Imports(
			config.Module(config.Options{
				EnvFiles: []string{".env", ".env.local"},
				Load: []config.LoadTarget{
					config.Struct[settings.AppConfig](),
				},
			}),
			logger.Module(logger.Options{
				Level:  "info",
				Format: "text",
				Writer: io.Discard,
			}),
			jwt.Module(jwt.Options{
				Secret:    "phase7-dev-secret",
				Issuer:    "phase7-api",
				AccessTTL: time.Hour,
			}),
		),
		Providers: gest.Providers(
			gest.Provide(NewService),
			gest.Controller(NewController),
		),
	})
}
