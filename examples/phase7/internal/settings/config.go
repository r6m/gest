package settings

import "time"

// AppConfig is user-owned runtime app config loaded by modules/config.
type AppConfig struct {
	ServiceName string        `env:"SERVICE_NAME" default:"phase7"`
	JWTIssuer   string        `env:"JWT_ISSUER" default:"phase7-api"`
	AccessTTL   time.Duration `env:"ACCESS_TTL" default:"1h"`
}
