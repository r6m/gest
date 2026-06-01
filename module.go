package gest

// Module is an explicit runtime module definition.
type Module interface {
	Definition() ModuleConfig
}

// ModuleConfig describes a module and its immediate dependencies.
type ModuleConfig struct {
	Name      string
	Global    bool
	Imports   []Module
	Providers []Provider
}

type module struct {
	config ModuleConfig
}

// NewModule creates a module from config.
func NewModule(config ModuleConfig) Module {
	return module{
		config: ModuleConfig{
			Name:      config.Name,
			Global:    config.Global,
			Imports:   Imports(config.Imports...),
			Providers: Providers(config.Providers...),
		},
	}
}

// Definition returns a copy of the module definition.
func (m module) Definition() ModuleConfig {
	return ModuleConfig{
		Name:      m.config.Name,
		Global:    m.config.Global,
		Imports:   Imports(m.config.Imports...),
		Providers: Providers(m.config.Providers...),
	}
}

// Imports returns modules in the order provided.
func Imports(modules ...Module) []Module {
	return append([]Module(nil), modules...)
}

// Providers returns providers in the order provided.
func Providers(providers ...Provider) []Provider {
	return append([]Provider(nil), providers...)
}
