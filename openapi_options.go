package gest

const (
	defaultOpenAPIPath    = "/openapi.json"
	defaultOpenAPITitle   = "Gest API"
	defaultOpenAPIVersion = "0.1.0"
)

type openAPIConfig struct {
	Path    string
	Title   string
	Version string
}

// OpenAPIOption configures the generated OpenAPI document.
type OpenAPIOption func(*openAPIConfig)

// OpenAPITitle sets info.title in the generated OpenAPI document.
func OpenAPITitle(title string) OpenAPIOption {
	return func(config *openAPIConfig) {
		if title != "" {
			config.Title = title
		}
	}
}

// OpenAPIVersion sets info.version in the generated OpenAPI document.
func OpenAPIVersion(version string) OpenAPIOption {
	return func(config *openAPIConfig) {
		if version != "" {
			config.Version = version
		}
	}
}

func newOpenAPIConfig(routePath string, options ...OpenAPIOption) openAPIConfig {
	config := openAPIConfig{
		Path:    routePath,
		Title:   defaultOpenAPITitle,
		Version: defaultOpenAPIVersion,
	}
	if config.Path == "" {
		config.Path = defaultOpenAPIPath
	}
	for _, option := range options {
		option(&config)
	}
	return config
}
